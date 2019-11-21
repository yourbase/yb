package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/mutagen-io/mutagen/cmd"
	"github.com/mutagen-io/mutagen/cmd/mutagen/daemon"
	"github.com/mutagen-io/mutagen/pkg/filesystem/behavior"

	"github.com/mutagen-io/mutagen/pkg/filesystem"
	"github.com/mutagen-io/mutagen/pkg/grpcutil"
	"github.com/mutagen-io/mutagen/pkg/prompt"
	"github.com/mutagen-io/mutagen/pkg/selection"
	synchronizationsvc "github.com/mutagen-io/mutagen/pkg/service/synchronization"
	"github.com/mutagen-io/mutagen/pkg/synchronization"
	"github.com/mutagen-io/mutagen/pkg/synchronization/core"
	"github.com/mutagen-io/mutagen/pkg/url"
)

func FlushSync(containerName string) error {
	return nil
}

func StartSyncService(localDir string, containerName string, containerWorkDir string) error {
	alpha, err := url.Parse(localDir, url.Kind_Synchronization, true)
	if err != nil {
		return errors.Wrap(err, "unable to parse alpha URL")
	}

	containerUrl := fmt.Sprintf("docker://%s%s", containerName, containerWorkDir)
	beta, err := url.Parse(containerUrl, url.Kind_Synchronization, false)
	if err != nil {
		return errors.Wrap(err, "unable to parse beta URL")
	}

	// Validate the name.
	syncName := fmt.Sprintf("ybsync%s", containerName)
	syncName = string(syncName[0:32])
	if err := selection.EnsureNameValid(syncName); err != nil {
		return errors.Wrap(err, fmt.Sprintf("invalid session name: %s", syncName))
	}

	configuration := &synchronization.Configuration{}
	var stageModeAlpha, stageModeBeta synchronization.StageMode
	var defaultFileModeAlpha, defaultFileModeBeta filesystem.Mode
	var defaultDirectoryModeAlpha, defaultDirectoryModeBeta filesystem.Mode
	var probeModeAlpha, probeModeBeta behavior.ProbeMode
	var scanModeAlpha, scanModeBeta synchronization.ScanMode
	var watchModeAlpha, watchModeBeta synchronization.WatchMode

	watchPollingIntervalAlpha := uint32(30)
	watchPollingIntervalBeta := uint32(30)
	defaultOwnerAlpha := ""
	defaultGroupAlpha := ""
	defaultOwnerBeta := ""
	defaultGroupBeta := ""

	ignoreVCSMode := core.IgnoreVCSMode_IgnoreVCSModeIgnore
	configuration = synchronization.MergeConfigurations(configuration, &synchronization.Configuration{
		/*
			SynchronizationMode:    synchronizationMode,
			MaximumEntryCount:      createConfiguration.maximumEntryCount,
			MaximumStagingFileSize: maximumStagingFileSize,
			ProbeMode:              probeMode,
			ScanMode:               scanMode,
			StageMode:              stageMode,
			SymlinkMode:            symbolicLinkMode,
			WatchMode:              watchMode,
			WatchPollingInterval:   createConfiguration.watchPollingInterval,
			Ignores:                createConfiguration.ignores,
		*/
		IgnoreVCSMode: ignoreVCSMode,
		/*
			DefaultFileMode:        uint32(defaultFileMode),
			DefaultDirectoryMode:   uint32(defaultDirectoryMode),
			DefaultOwner:           createConfiguration.defaultOwner,
			DefaultGroup:           createConfiguration.defaultGroup,u
		*/
	})
	specification := &synchronizationsvc.CreationSpecification{
		Alpha:         alpha,
		Beta:          beta,
		Configuration: configuration,
		ConfigurationAlpha: &synchronization.Configuration{
			ProbeMode:            probeModeAlpha,
			ScanMode:             scanModeAlpha,
			StageMode:            stageModeAlpha,
			WatchMode:            watchModeAlpha,
			WatchPollingInterval: watchPollingIntervalAlpha,
			DefaultFileMode:      uint32(defaultFileModeAlpha),
			DefaultDirectoryMode: uint32(defaultDirectoryModeAlpha),
			DefaultOwner:         defaultOwnerAlpha,
			DefaultGroup:         defaultGroupAlpha,
		},
		ConfigurationBeta: &synchronization.Configuration{
			ProbeMode:            probeModeBeta,
			ScanMode:             scanModeBeta,
			StageMode:            stageModeBeta,
			WatchMode:            watchModeBeta,
			WatchPollingInterval: watchPollingIntervalBeta,
			DefaultFileMode:      uint32(defaultFileModeBeta),
			DefaultDirectoryMode: uint32(defaultDirectoryModeBeta),
			DefaultOwner:         defaultOwnerBeta,
			DefaultGroup:         defaultGroupBeta,
		},
		Name:   syncName,
		Paused: false,
	}

	// Connect to the daemon and defer closure of the connection.
	daemonConnection, err := daemon.CreateClientConnection(true, true)
	if err != nil {
		return errors.Wrap(err, "unable to connect to daemon")
	}
	defer daemonConnection.Close()

	// Create a synchronization service client.
	service := synchronizationsvc.NewSynchronizationClient(daemonConnection)

	// Perform creation.
	err = CreateSyncWithSpecification(service, specification)
	if err != nil {
		return fmt.Errorf("Couldn't create sync spec: %v", err)
	}
	selection := &selection.Selection{
		All:            false,
		Specifications: []string{syncName},
	}
	if err := selection.EnsureValid(); err != nil {
		return errors.Wrap(err, "invalid session selection specification")
	}

	ready := false
	for !ready {
		fmt.Printf("Looking for sync entry...\n")
		// Invoke list.
		request := &synchronizationsvc.ListRequest{
			Selection: selection,
		}
		response, err := service.List(context.Background(), request)
		if err != nil {
			return errors.Wrap(grpcutil.PeelAwayRPCErrorLayer(err), "list failed")
		} else if err = response.EnsureValid(); err != nil {
			return errors.Wrap(err, "invalid list response received")
		}

		// Handle output based on whether or not any sessions were returned.
		if len(response.SessionStates) > 0 {
			for _, state := range response.SessionStates {
				status := state.Status
				if status != synchronization.Status_Watching {
					fmt.Printf("Waiting for sync to be ready...\n")
				} else {
					ready = true
				}
			}
		} else {
			fmt.Printf("Couldn't find session, will try again\n")
		}
		time.Sleep(5 * time.Second)
	}

	return nil
}

func CreateSyncWithSpecification(
	service synchronizationsvc.SynchronizationClient,
	specification *synchronizationsvc.CreationSpecification,
) error {
	// Invoke the session create method. The stream will close when the
	// associated context is cancelled.
	createContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream, err := service.Create(createContext)
	if err != nil {
		return errors.Wrap(grpcutil.PeelAwayRPCErrorLayer(err), "unable to invoke create")
	}

	// Send the initial request.
	request := &synchronizationsvc.CreateRequest{Specification: specification}
	if err := stream.Send(request); err != nil {
		return errors.Wrap(grpcutil.PeelAwayRPCErrorLayer(err), "unable to send create request")
	}

	// Create a status line printer and defer a break.
	statusLinePrinter := &cmd.StatusLinePrinter{}
	defer statusLinePrinter.BreakIfNonEmpty()

	// Receive and process responses until we're done.
	for {
		if response, err := stream.Recv(); err != nil {
			return errors.Wrap(grpcutil.PeelAwayRPCErrorLayer(err), "create failed")
		} else if err = response.EnsureValid(); err != nil {
			return errors.Wrap(err, "invalid create response received")
		} else if response.Session != "" {
			statusLinePrinter.Print(fmt.Sprintf("Created session %s", response.Session))
			return nil
		} else if response.Message != "" {
			statusLinePrinter.Print(response.Message)
			if err := stream.Send(&synchronizationsvc.CreateRequest{}); err != nil {
				return errors.Wrap(grpcutil.PeelAwayRPCErrorLayer(err), "unable to send message response")
			}
		} else if response.Prompt != "" {
			statusLinePrinter.BreakIfNonEmpty()
			if response, err := prompt.PromptCommandLine(response.Prompt); err != nil {
				return errors.Wrap(err, "unable to perform prompting")
			} else if err = stream.Send(&synchronizationsvc.CreateRequest{Response: response}); err != nil {
				return errors.Wrap(grpcutil.PeelAwayRPCErrorLayer(err), "unable to send prompt response")
			}
		}
	}
}
