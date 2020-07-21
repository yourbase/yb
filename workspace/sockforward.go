package workspace

import (
	"context"
	"io"

	"github.com/yourbase/yb/plumbing/log"
	"github.com/yourbase/yb/runtime"
)

// injectSockForward maps a running SSH agent into the container
func (bt *BuildTarget) injectSockForward(ctx context.Context, builder runtime.Target, output io.Writer, agentPath, baseDir string) error {
	hostAddr, err := runtime.ForwardUnixSocketToTcp(agentPath)
	if err != nil {
		log.Warnf("Could not forward SSH agent: %v", err)
	} else {
		log.Infof("Forwarding SSH agent via %s", hostAddr)
	}

	// Checks if sockforward is already running inside the container
	err = builder.Run(ctx, runtime.Process{Command: "pgrep -x sockforward"})
	if err == nil {
		if rerr := builder.Run(ctx, runtime.Process{Command: "pkill -x sockforward"}); rerr != nil {
			return rerr
		}
	}

	var (
		localSockForward = builder.ToolsDir(ctx) + "/sockforward"
		sshAgentSck      = baseDir + "/ssh_agent.sock"
	)

	builder.SetEnv("SSH_AUTH_SOCK", sshAgentSck)

	// start gouroutine
	startMe := func() {
		log.Infof("Running SSH agent socket forwarder...")
		forwardCmd := localSockForward + " " + sshAgentSck + " " + hostAddr
		if err := builder.Run(ctx, runtime.Process{Output: output, Command: forwardCmd}); err != nil {
			log.Warnf("Starting ssh_agent: %v", err)
		}
	}

	// Checks if we already injected it
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
	err = builder.Run(ctx, runtime.Process{Output: output, Command: "mv " + forwardPath + " " + localSockForward})
	if err != nil {
		return err
	}
	go startMe()
	return nil
}
