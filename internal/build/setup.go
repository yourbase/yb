// Copyright 2020 YourBase Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strings"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/yourbase/commons/xcontext"
	"github.com/yourbase/narwhal"
	"github.com/yourbase/yb"
	"github.com/yourbase/yb/internal/biome"
	"github.com/yourbase/yb/internal/buildpack"
	"github.com/yourbase/yb/internal/plumbing"
	"github.com/yourbase/yb/internal/ybtrace"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/label"
	"zombiezen.com/go/log"
)

// PhaseDeps defines the dependencies for a build target phase.
//
// TODO(ch2285): This should be moved out of this package and into a separate
// loader package.
type PhaseDeps struct {
	TargetName string
	Buildpacks []yb.BuildpackSpec
	Resources  map[string]*narwhal.ContainerDefinition

	// EnvironmentTemplate is the set of environment variables that should be set
	// in the biome. These variables may include substitutions for container IP
	// addresses in the form `{{ .Containers.IP "mycontainer" }}`.
	EnvironmentTemplate map[string]string
}

// Setup arranges for the phase's dependencies to be available, returning a new
// biome that has the dependencies configured. It is the caller's responsibility
// to call Close on the returned biome.
func Setup(ctx context.Context, sys Sys, phase *PhaseDeps) (_ biome.BiomeCloser, err error) {
	ctx, span := ybtrace.Start(ctx, "Setup "+phase.TargetName, trace.WithAttributes(
		label.String("target", phase.TargetName),
	))
	defer func() {
		if err != nil {
			span.SetStatus(codes.Unknown, err.Error())
		}
		span.End()
	}()

	// Randomize pack setup order to surface unexpected data dependencies.
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	packs := append([]yb.BuildpackSpec(nil), phase.Buildpacks...)
	if len(packs) > 0 {
		for i := range packs[:len(packs)-1] {
			j := i + rng.Intn(len(phase.Buildpacks)-i)
			packs[i], packs[j] = packs[j], packs[i]
		}
	}

	// Install all buildpacks.
	newEnv := biome.Environment{
		Vars: make(map[string]string),
	}
	for _, pack := range packs {
		packEnv, err := buildpack.Install(ctx, sys, pack)
		if err != nil {
			return nil, fmt.Errorf("setup %s: %w", phase.TargetName, err)
		}
		newEnv = newEnv.Merge(packEnv)
	}

	expContainers, closeFunc, err := startContainers(ctx, sys, phase.Resources)
	if err != nil {
		return nil, fmt.Errorf("setup %s: %w", phase.TargetName, err)
	}
	defer func() {
		if err != nil && closeFunc != nil {
			closeFunc()
		}
	}()
	exp := configExpansion{
		Containers: expContainers,
	}
	for k, t := range phase.EnvironmentTemplate {
		v, err := exp.expand(t)
		if err != nil {
			return nil, fmt.Errorf("setup %s: expand %s: %w", phase.TargetName, k, err)
		}
		newEnv.Vars[k] = v
	}
	return biome.WithClose(
		biome.EnvBiome{
			Biome: biome.NopCloser(sys.Biome),
			Env:   newEnv,
		},
		closeFunc,
	), nil
}

type container struct {
	client       *docker.Client
	resourceName string
	id           string
	ip           net.IP
}

// startContainers starts containers for the given set of container definitions,
// returning a map of container IP addresses and function to stop the containers.
func startContainers(ctx context.Context, sys Sys, defs map[string]*narwhal.ContainerDefinition) (_ containersExpansion, closeFunc func() error, err error) {
	exp := containersExpansion{
		ips: make(map[string]string),
	}
	containers := make(map[string]*container)
	for name := range defs {
		ip := os.Getenv("YB_CONTAINER_" + strings.ToUpper(name) + "_IP")
		if ip != "" {
			log.Infof(ctx, "Using %s address from environment: %s", name, ip)
			exp.ips[name] = ip
		} else {
			containers[name] = nil
		}
	}
	if len(containers) == 0 {
		return containersExpansion{}, func() error { return nil }, nil
	}
	if sys.DockerClient == nil {
		names := make([]string, 0, len(containers))
		for name := range containers {
			names = append(names, name)
		}
		if len(names) == 1 {
			return containersExpansion{}, nil, fmt.Errorf("start containers: docker disabled but no address found for %s", names[0])
		}
		return containersExpansion{}, nil, fmt.Errorf("start containers: docker disabled but no addresses found for %s",
			strings.Join(names, ", "))
	}
	// Ping first to ensure that Docker is available before attempting anything.
	if err := sys.DockerClient.PingWithContext(ctx); err != nil {
		return containersExpansion{}, nil, fmt.Errorf("%w\nTry installing Docker Desktop: https://hub.docker.com/search/?type=edition&offering=community", err)
	}

	// This variable has a different name than "closeFunc" to avoid getting
	// clobbered by returns, so it can be called in a defer on error.
	origCloseFunc := func() error {
		// This function can be called in an entirely different context,
		// so use Background.
		ctx := context.Background()
		for _, c := range containers {
			if c != nil {
				c.remove(ctx)
			}
		}
		return nil
	}
	defer func() {
		if err != nil {
			origCloseFunc()
		}
	}()

	for name := range containers {
		c, err := startContainer(ctx, sys, name, defs[name])
		if err != nil {
			return containersExpansion{}, nil, err
		}
		log.Infof(ctx, "%s address is %v", name, c.ip)
		containers[name] = c
		exp.ips[name] = c.ip.String()
	}
	return exp, origCloseFunc, nil
}

// startContainer starts a single container with the given definition.
func startContainer(ctx context.Context, sys Sys, resourceName string, cd *narwhal.ContainerDefinition) (_ *container, err error) {
	log.Infof(ctx, "Starting container %s...", resourceName)
	id, err := narwhal.CreateContainer(ctx, sys.DockerClient, sys.Stderr, cd)
	if err != nil {
		return nil, fmt.Errorf("start resource %s: %w", resourceName, err)
	}
	c := &container{
		client:       sys.DockerClient,
		resourceName: resourceName,
		id:           id,
	}
	defer func() {
		if err != nil {
			c.remove(xcontext.IgnoreDeadline(ctx))
		}
	}()
	err = sys.DockerClient.ConnectNetwork(sys.DockerNetworkID, docker.NetworkConnectionOptions{
		Context:   ctx,
		Container: id,
		EndpointConfig: &docker.EndpointConfig{
			NetworkID: sys.DockerNetworkID,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("start resource %s: connect %s to %s: %w", resourceName, id, sys.DockerNetworkID, err)
	}
	if err := narwhal.StartContainer(ctx, sys.DockerClient, id); err != nil {
		return nil, fmt.Errorf("start resource %s: %w", resourceName, err)
	}
	c.ip, err = narwhal.IPv4Address(ctx, sys.DockerClient, id)
	if err != nil {
		return nil, fmt.Errorf("start resource %s: %w", resourceName, err)
	}

	if cd.PortWaitCheck.Port != 0 {
		// Wait for port to be reachable.
		addr := &net.TCPAddr{
			IP:   c.ip,
			Port: cd.PortWaitCheck.Port,
		}
		if cd.PortWaitCheck.LocalPortMap != 0 {
			addr = &net.TCPAddr{
				IP:   net.IPv4(127, 0, 0, 1),
				Port: cd.PortWaitCheck.LocalPortMap,
			}
		}

		timeout := time.Duration(cd.PortWaitCheck.Timeout) * time.Second
		log.Infof(ctx, "Waiting up to %v for %s to be ready... ", timeout, resourceName)
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		if err := waitForTCPPort(ctx, addr.String()); err != nil {
			return nil, fmt.Errorf("start resource %s: %w", resourceName, err)
		}
	}

	return c, nil
}

func (c *container) remove(ctx context.Context) {
	err := c.client.RemoveContainer(docker.RemoveContainerOptions{
		Context: ctx,
		ID:      c.id,
		Force:   true,
	})
	if err != nil {
		// These errors aren't actionable, so log them instead of returning them.
		log.Warnf(ctx, "Leaked %s container %s: %v", c.resourceName, c.id, err)
	}
}

func waitForTCPPort(ctx context.Context, addr string) error {
	dialer := new(net.Dialer)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		c, err := dialer.DialContext(ctx, "tcp", addr)
		if err == nil {
			c.Close()
			return nil
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return fmt.Errorf("wait for %q: %w", addr, err)
		}
	}
}

// configExpansion expands templated substitutions in parts of the
// configuration file. The object itself is passed into text/template,
// so its public fields are public API surface.
type configExpansion struct {
	// Containers holds the set of resources for the target.
	// The field name is public API surface and must not change.
	Containers containersExpansion
}

type containersExpansion struct {
	ips map[string]string
}

func (exp configExpansion) expand(value string) (string, error) {
	return plumbing.TemplateToString(value, exp)
}

// IP returns the IP address of a particular container.
// The signature of this method is public API surface and must not change.
func (exp containersExpansion) IP(label string) (string, error) {
	ip := exp.ips[label]
	if ip == "" {
		return "", fmt.Errorf("find IP for %s: unknown container", label)
	}
	return ip, nil
}
