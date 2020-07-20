package workspace

import (
	"context"
	"io"

	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/runtime"
)

// injectSockForward maps a running SSH agent into the container
func (bt *BuildTarget) injectSockForward(ctx context.Context, builder runtime.Target, output io.Writer, agentPath, baseDir string) error {
	// Checks if sockforward is already running inside the container
	err := builder.Run(ctx, runtime.Process{Command: "pgrep -x sockforward"})
	if err == nil {
		return nil
	}

	var localSockForward = builder.ToolsDir(ctx) + "/sockforward"

	hostAddr, err := runtime.ForwardUnixSocketToTcp(agentPath)
	if err != nil {
		log.Warnf("Could not forward SSH agent: %v", err)
	} else {
		log.Infof("Forwarding SSH agent via %s", hostAddr)
	}

	builder.SetEnv("SSH_AUTH_SOCK", "/ssh_agent")

	// start gouroutine
	startMe := func() {
		log.Infof("Running SSH agent socket forwarder...")
		forwardCmd := localSockForward + " " + baseDir + "/ssh_agent " + hostAddr
		if err := builder.Run(ctx, runtime.Process{Output: output, Command: forwardCmd}); err != nil {
			log.Warnf("Starting ssh_agent: %v", err)
		}
	}

	// Not running, checks if we already injected it
	err = builder.Run(ctx, runtime.Process{Output: output, Command: "stat " + localSockForward})
	if err == nil {
		// No need to download it again
		go startMe()
		return nil
	}

	const sockForwardURL = "https://yourbase-artifacts.s3-us-west-2.amazonaws.com/sockforward"
	forwardPath, err := builder.DownloadFile(ctx, sockForwardURL)
	if err != nil {
		return err
	}
	err = builder.Run(ctx, runtime.Process{Output: output, Command: "chmod a+x " + forwardPath})
	if err != nil {
		return err
	}
	err = builder.Run(ctx, runtime.Process{Output: output, Command: "cp " + forwardPath + " " + localSockForward})
	if err != nil {
		return err
	}
	go startMe()
	return nil
}
