// CUE Schema for YourBase build configuration.
// See https://cuelang.org/docs/tutorials/tour/intro/validation/ for how to
// validate YAML.

Config :: {
	dependencies: Dependencies
	build_targets: [...BuildTarget]
	ci: CiConfig
}

Dependencies :: {
	build: [...string]
	runtime: [...string]
}

BuildTarget :: {
	name: string
	commands: [...string]
	// TODO: "osx" is the one documented, but "darwin" appears in practice.
	tags?: close({os: "linux" | "osx" | "darwin"})
	environment?: [... =~"^[^=]+="]
	root?: string
	build_after?: [...string]
	// TODO
	container?: {}
	// TODO undocumented
	host_only: bool | *false
}

CiConfig :: {
	builds: [...{
		name:         string
		build_target: string
		when?:        string
	}]
}

Config
