package runtime

import (
	"testing"

	"github.com/yourbase/narwhal"
)

func TestCanRunProcess(t *testing.T) {
	containers := []narwhal.ContainerDefinition{
		narwhal.ContainerDefinition{
			Image: "redis:latest",
			Label: "redis",
		},
	}

	opts := ContainerRuntimeOpts{
		Identifier:           "test-runtime",
		ContainerDefinitions: containers,
	}

	runtime, err := NewContainerRuntime(opts)

	defer runtime.Shutdown()
	if err != nil {
		t.Fatalf("Couldn't start container runtime: %v", err)
	}

	p := Process{Command: "ls /", Interactive: false, Directory: "/"}

	if err := runtime.Run(p, "redis"); err != nil {
		t.Fatalf("Error running process: %v", err)
	}
}
