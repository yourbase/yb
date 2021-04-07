# Package Configuration Reference

The .yourbase.yml file is broken into four primary sections:

-  Dependencies
-  Build Targets
-  Runtime Target
-  CI Instructions

## Dependencies

The `dependencies` section describes what build packs need to be loaded for
the build.

Example:

```yaml
dependencies:
  build:
    - python:3.6.3
  runtime:
    - heroku:latest
```

-  `build`: Dependencies used by all `build_targets` (see below).
-  `runtime`: Dependencies used by all `exec` environments (see below).

## Build Targets

The `build_targets` section is a list of build targets, each with a unique name
a sequence of commands.

Example:

```yaml
build_targets:
  - name: build_and_deploy
    commands:
      - rake test
      - rake package
      - rake deploy
```

### Attributes

-  `name`: The name of the build target. This is referenced on the
   command-line `yb build targetname`, in CI configuration sections or as things
   to build first.

-  `commands`: The commands to be executed; these can be anything you would run
   on the command-line. **Note** YourBase does not expand environment variables
   using the normal `${VALUE}` syntax, though there are ways to do this.
   We do not run `sh` or `bash` as this is not portable; if you need to do that,
   create a shell script and run it as a command explicitly.

-  `tags`: Tags are used to select which build server can handle each build.

   -  `os`: Specifies which operating system this build target is applicable to.
      Current valid options: `linux` (default) and `osx`.

      ```yaml
      - name: android
        tags:
          os: linux
        commands:
          - apt-get update
          - apt-get install -y software-properties-common
          - add-apt-repository -y ppa:ubuntu-toolchain-r/test
          - apt-get update
          - apt-get install -y lib32stdc++6
          - sh build.sh
      - name: ios
        tags:
          os: darwin
        commands:
            - brew update
            - brew install --HEAD usbmuxd
            - brew link usbmuxd
            - brew install --HEAD libimobiledevice
            - brew install ideviceinstaller
            - brew install ios-deploy
            - sh build.sh
      ```

-  `environment`: A list of `KEY=VALUE` items that are used as environment
   variables.

   ```yaml
   build_targets:
     - name: default
       environment:
         - ENVIRONMENT=development
         - DATABASE_URL=db://localhost:1234/foo
   ```

-  `root`: The name of a path where it should run commands from, relative to the
   root of the project.

-  `build_after`: A list of build targets whose commands will be executed before
   this target.

   ```yaml
   build_targets:
     - name: test
       commands:
         - python tests.py
     - name: release
       build_after:
         - test
       commands:
         - python kraken.py
   ```

-  `container`: If this build is to be built in a container, this directive
   allows you to describe the container. All the commands in the `commands`
   directive will be executed inside a container using the provided
   configuration.

   ```yaml
   build_targets:
     - name: container_build
       container:
         image: golang:1.12
         mounts:
            - data:/data
         ports:
            - 123:456
         environment:
            - FOO=bar
            - PASSWORD=sekret
         workdir:
            - /source
       commands:
         - go get
         - go build
         - go test ./...
   ```

## CI Build Directives

The CI section allows you to define what to build and when by using a
combination of build targets and conditions. In order for the CI system to
properly build your project, you must define the `dependencies`, `build_targets`
and `ci` sections.

Simple example:

```yaml
dependencies:
  build:
    - python:3.6.3

build_targets:
  - name: default
    commands:
      - python test.py

ci:
  builds:
    - name: all_commits
      build_target: default
```

Each list item in builds has the following attributes:

-  `name`: The name of the CI build itself (arbitrary string)
-  `build_target`: The name of the build target to run. Must match one of the
   names in the `build_targets` section.
-  `when`: (Optional) CI build conditions for this target. You can use the
   following variables and combine them using simple boolean logic using
   [PyPred syntax][]:
   -  `branch`: The name of the branch being built (e.g. `main`).
   -  `action`: Either `commit` or `pull_request`
   -  `tagged`: Boolean value, either `true` or `false`

Example of CI builds with conditions:

```yaml
dependencies:
  build:
    - python:3.6.3

build_targets:
  - name: integration_tests
    commands:
      - python integration_test.py

  - name: default
    commands:
      - python test.py

  - name: release
    commands:
      - python release.py

ci:
  builds:
    - name: main_builds
      build_target: integration_tests
      when: branch is 'main'

    - name: pr_builds
      build_target: default
      when: action is 'pull_request'

    - name: tags
      build_target: release
      when: tagged
```

[pypred syntax]: https://github.com/armon/pypred
