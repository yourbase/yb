module github.com/yourbase/yb

go 1.15

require (
	gg-scm.io/pkg/git v0.5.0
	github.com/blang/semver v3.5.1+incompatible
	github.com/containerd/containerd v1.4.0 // indirect
	github.com/containerd/continuity v0.0.0-20190827140505-75bee3e2ccb6 // indirect
	// Docker is pinned to a weird specific commit (https://github.com/moby/moby/commit/8312004f41e9500824fa16ae991eeee0083f4771)
	// to avoid use of github.com/docker/docker/pkg/term, which depends on
	// unsupported Darwin syscalls in golang.org/x/sys.
	github.com/docker/docker v17.12.0-ce-rc1.0.20200421142927-8312004f41e9+incompatible // indirect
	github.com/dsnet/compress v0.0.1 // indirect
	github.com/frankban/quicktest v1.5.0 // indirect
	github.com/fsouza/go-dockerclient v1.6.0
	github.com/gobwas/httphead v0.0.0-20180130184737-2c6c146eadee // indirect
	github.com/gobwas/pool v0.2.1 // indirect
	github.com/gobwas/ws v1.0.3
	github.com/golang/snappy v0.0.1 // indirect
	github.com/google/go-cmp v0.5.1
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/gopherjs/gopherjs v0.0.0-20190915194858-d3ddacdb130f // indirect
	github.com/johnewart/archiver v3.1.4+incompatible
	github.com/johnewart/subcommands v0.0.0-20181012225330-46f0354f6315
	github.com/matishsiao/goInfo v0.0.0-20200404012835-b5f882ee2288
	github.com/moby/sys/mount v0.1.1 // indirect
	github.com/moby/term v0.0.0-20200915141129-7f0af18e79f2 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/nwaples/rardecode v1.1.0 // indirect
	github.com/pierrec/lz4 v2.5.2+incompatible // indirect
	github.com/sergi/go-diff v1.1.0 // indirect
	github.com/smartystreets/assertions v1.0.1 // indirect
	github.com/smartystreets/goconvey v0.0.0-20190731233626-505e41936337 // indirect
	github.com/ulikunitz/xz v0.5.8
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
	github.com/yourbase/commons v0.6.0
	github.com/yourbase/narwhal v0.6.1
	go.opentelemetry.io/otel v0.11.0
	go.opentelemetry.io/otel/sdk v0.11.0
	go4.org v0.0.0-20200411211856-f5505b9728dd
	golang.org/x/crypto v0.0.0-20200820211705-5c72a883971a // indirect
	golang.org/x/mod v0.3.0
	golang.org/x/net v0.0.0-20200822124328-c89045814202 // indirect
	google.golang.org/genproto v0.0.0-20200831141814-d751682dd103 // indirect
	google.golang.org/grpc v1.31.1 // indirect
	google.golang.org/protobuf v1.25.0 // indirect
	gopkg.in/ini.v1 v1.60.2
	gopkg.in/src-d/go-git.v4 v4.13.1
	gopkg.in/yaml.v2 v2.3.0
	zombiezen.com/go/log v1.0.2
)
