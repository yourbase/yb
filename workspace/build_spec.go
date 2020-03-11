package workspace

import (
	"fmt"
	"github.com/yourbase/yb/plumbing/log"
	"io/ioutil"

	hcl2 "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/yourbase/narwhal"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

type BuildSpec struct {
	Services []ServiceBlock `hcl:"service,block"`
	RawBlock hcl2.Body
}

type Port struct {
	Name string `hcl:"name,label"`
	Type string `hcl:"type,attr"`
	Port string `hcl:"port,attr"`
}

type Dependency struct {
	Type string `hcl:"type,label"`
	Name string `hcl:"name,label"`
}

type ServiceBlock struct {
	Name       string      `hcl:"name,label"`
	DependsOn  *[]string   `hcl:"depends_on,attr"`
	Variables  []Variable  `hcl:"variable,block"`
	Ports      []Port      `hcl:"port,block"`
	Buildpacks []Buildpack `hcl:"buildpack,block"`
	Containers []Container `hcl:"container,block"`
	Setup      SetupBlock  `hcl:"setup,block"`
	Build      BuildBlock  `hcl:"build,block"`
	Test       TestBlock   `hcl:"test,block"`
	Run        RunBlock    `hcl:"run,block"`
}

type Buildpack struct {
	Name    string `hcl:"name,label"`
	Version string `hcl:"version,optional"`
}

type Variable struct {
	Key   string `hcl:"key,label"`
	Value string `hcl:"value"`
}

func (v *Variable) String() string {
	return fmt.Sprintf("%s=%s", v.Key, v.Value)
}

type Environment struct {
	Variables []Variable `hcl:"variable,block"`
}

func (e *Environment) Exports() []string {
	result := make([]string, 0)
	for _, v := range e.Variables {
		result = append(result, v.String())
	}
	return result
}

type Container struct {
	Name        string      `hcl:"name,label"`
	Image       string      `hcl:"image"`
	Mounts      []string    `hcl:"mounts"`
	Environment Environment `hcl:"environment,block"`
}

func (c *Container) EnvironmentVariables() []string {
	return c.Environment.Exports()
}

type ArtifactBlock struct {
}

type TestBlock struct {
	Commands []string `hcl:"commands,optional"`
	Command  string   `hcl:"command,optional"`
}

type BuildBlock struct {
	Commands []string  `hcl:"commands,optional"`
	Command  string    `hcl:"command,optional"`
	Config   hcl2.Body `hcl:",remain"`
}

func (b BuildBlock) CommandList() []string {
	if b.Command != "" {
		return []string{b.Command}
	}

	return b.Commands
}

type mRunBlock struct {
	RawBlock hcl2.Body `hcl:",remain"`
}

type RunBlock struct {
	Commands    []string    `hcl:"commands,optional"`
	Command     string      `hcl:"command,optional"`
	Environment Environment `hcl:"environment,block"`
}

func (b RunBlock) CommandList() []string {
	if b.Command != "" {
		return []string{b.Command}
	}

	return b.Commands
}

type SetupBlock struct {
	Commands []string `hcl:"commands,optional"`
	Command  string   `hcl:"command,optional"`
}

func (b *ServiceBlock) BuildPacks() []string {
	result := make([]string, 0)

	for _, b := range b.Buildpacks {
		spec := fmt.Sprintf("%s:%s", b.Name, b.Version)
		result = append(result, spec)
	}

	return result
}

func (b *ServiceBlock) ContainerMap() map[string]narwhal.ContainerDefinition {
	result := make(map[string]narwhal.ContainerDefinition, 0)
	for _, c := range b.Containers {
		cd := narwhal.ContainerDefinition{
			Label:       b.Name,
			Image:       c.Image,
			Mounts:      c.Mounts,
			Environment: c.EnvironmentVariables(),
		}
		result[c.Name] = cd
	}
	return result
}

type Context struct {
	Message string
}

func (b *BuildSpec) VariableContext() hcl2.EvalContext {

	services := make(map[string]cty.Value)
	for _, s := range b.Services {
		svcmap := make(map[string]cty.Value)

		vmap := make(map[string]cty.Value)
		for _, v := range s.Variables {
			vmap[v.Key] = cty.StringVal(v.Value)
		}
		if len(vmap) > 0 {
			svcmap["vars"] = cty.MapVal(vmap)
		}

		pmap := make(map[string]cty.Value)
		for _, p := range s.Ports {
			pmap[p.Name] = cty.StringVal(p.Port)
		}
		if len(pmap) > 0 {
			svcmap["ports"] = cty.MapVal(pmap)
		}

		cmap := make(map[string]cty.Value)
		for _, c := range s.Containers {
			eMap := make(map[string]cty.Value)
			for _, v := range c.Environment.Variables {
				eMap[v.Key] = cty.StringVal(v.Value)
			}
			ipVal := fmt.Sprintf("{{ .Containers.IP \"%s\" }}",c.Name)
			cmap[c.Name] = cty.ObjectVal(map[string]cty.Value{
				"environment": cty.MapVal(eMap),
				"ip":          cty.StringVal(ipVal),
			})
		}

		if len(cmap) > 0 {
			svcmap["containers"] = cty.MapVal(cmap)
		}

		ipVal := fmt.Sprintf("{{ .Services.IP \"%s\" }}", s.Name)
		svcmap["ip"] = cty.StringVal(ipVal)
		if len(svcmap) > 0 {
			services[s.Name] = cty.ObjectVal(svcmap)
		}
	}


	ctx := hcl2.EvalContext{
		Variables: map[string]cty.Value{
			"services": cty.ObjectVal(services),
		},
		Functions: map[string]function.Function{},
	}


	return ctx
}

func Evaluate() error {
	return nil
}

func LoadBuildSpec(filename string) (BuildSpec, error) {
	var spec BuildSpec
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return spec, err
	}

	parser := hclparse.NewParser()
	f, _ := parser.ParseHCL(bytes, filename)

	// first pass
	gohcl.DecodeBody(f.Body, nil, &spec)
	spec.RawBlock = f.Body

	ctx := spec.VariableContext()
	diag := gohcl.DecodeBody(f.Body, &ctx, &spec)
	if diag.HasErrors() {
		return spec, fmt.Errorf("Unable to decode run block: %s", diag.Error())
	}

	log.Debugf("Parsed spec: %#v", spec)

	if err != nil {
		return spec, err
	}

	return spec, nil
}

func (b *BuildSpec) Targets() []ServiceBlock {
	return b.Services
}

func (b *BuildSpec) GenerateManifest(target string) (BuildManifest, error) {
	for _, s := range b.Services {
		if s.Name == target {
			return BuildManifest{
				Dependencies: DependencySet{
					Build: s.BuildPacks(),
				},
				BuildTargets: []BuildTarget{
					BuildTarget{
						Name:     "default",
						Commands: s.Build.CommandList(),
					},
				},
				Exec: ExecPhase{
					Dependencies: ExecDependencies{
						Containers: s.ContainerMap(),
					},
					Environment: map[string][]string {
						"default": s.Run.Environment.Exports(),
					},
					Commands: s.Run.CommandList(),
				},
			}, nil

		}
	}

	return BuildManifest{}, ErrNoManifestFile

}

func (b *BuildSpec) Service(name string) *ServiceBlock {
	for _, s := range b.Services {
		if s.Name == name {
			return &s
		}
	}

	return nil
}
