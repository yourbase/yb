package sync

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"github.com/mutagen-io/mutagen/cmd"
	"github.com/mutagen-io/mutagen/cmd/mutagen/daemon"
	"github.com/mutagen-io/mutagen/pkg/filesystem"
	"github.com/mutagen-io/mutagen/pkg/forwarding"
	"github.com/mutagen-io/mutagen/pkg/grpcutil"
	"github.com/mutagen-io/mutagen/pkg/prompt"
	"github.com/mutagen-io/mutagen/pkg/selection"
	forwardingsvc "github.com/mutagen-io/mutagen/pkg/service/forwarding"
	"github.com/mutagen-io/mutagen/pkg/url"
)

// creation using the provided session specification. Unlike other orchestration
// methods, it requires provision of a client to avoid creating one for each
// request.
func CreateForwardWithSpecification(
	service forwardingsvc.ForwardingClient,
	specification *forwardingsvc.CreationSpecification,
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
	request := &forwardingsvc.CreateRequest{Specification: specification}
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
			if err := stream.Send(&forwardingsvc.CreateRequest{}); err != nil {
				return errors.Wrap(grpcutil.PeelAwayRPCErrorLayer(err), "unable to send message response")
			}
		} else if response.Prompt != "" {
			statusLinePrinter.BreakIfNonEmpty()
			if response, err := prompt.PromptCommandLine(response.Prompt); err != nil {
				return errors.Wrap(err, "unable to perform prompting")
			} else if err = stream.Send(&forwardingsvc.CreateRequest{Response: response}); err != nil {
				return errors.Wrap(grpcutil.PeelAwayRPCErrorLayer(err), "unable to send prompt response")
			}
		}
	}
}

func StartForwardService(localPort int, containerName string, containerPort int) error {
	sourceUrl := fmt.Sprintf("tcp:localhost:%d", localPort)
	destUrl := fmt.Sprintf("docker://%s:tcp:localhost:%d", containerName, containerPort)
	forwardName := fmt.Sprintf("fwd%d%s%d", localPort, containerName, containerPort)

	source, err := url.Parse(sourceUrl, url.Kind_Forwarding, true)
	if err != nil {
		return errors.Wrap(err, "unable to parse source URL")
	}
	destination, err := url.Parse(destUrl, url.Kind_Forwarding, false)
	if err != nil {
		return errors.Wrap(err, "unable to parse destination URL")
	}

	// Validate the name.
	if err := selection.EnsureNameValid(forwardName); err != nil {
		return errors.Wrap(err, "invalid session name")
	}

	configuration := &forwarding.Configuration{}
	var socketOverwriteModeSource, socketOverwriteModeDestination forwarding.SocketOverwriteMode
	var socketPermissionModeSource, socketPermissionModeDestination filesystem.Mode
	var socketOwnerSource, socketGroupSource string
	var socketOwnerDest, socketGroupDest string

	// Create the creation specification.
	specification := &forwardingsvc.CreationSpecification{
		Source:        source,
		Destination:   destination,
		Configuration: configuration,
		ConfigurationSource: &forwarding.Configuration{
			SocketOverwriteMode:  socketOverwriteModeSource,
			SocketOwner:          socketOwnerSource,
			SocketGroup:          socketGroupSource,
			SocketPermissionMode: uint32(socketPermissionModeSource),
		},
		ConfigurationDestination: &forwarding.Configuration{
			SocketOverwriteMode:  socketOverwriteModeDestination,
			SocketOwner:          socketOwnerDest,
			SocketGroup:          socketGroupDest,
			SocketPermissionMode: uint32(socketPermissionModeDestination),
		},
		Name:   forwardName,
		Paused: false,
	}

	// Connect to the daemon and defer closure of the connection.
	daemonConnection, err := daemon.CreateClientConnection(true, true)
	if err != nil {
		return errors.Wrap(err, "unable to connect to daemon")
	}
	defer daemonConnection.Close()

	// Create a forwarding service client.
	service := forwardingsvc.NewForwardingClient(daemonConnection)

	// Perform creation.
	return CreateForwardWithSpecification(service, specification)
}
