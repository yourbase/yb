---
parent: CI and CLI Documentation
nav_order: 4
---

# Package Configuration Reference

The .yourbase.yml file has four top-level sections:

- `build_targets`
- `dependencies`
- `exec`
- `ci`

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

- `name`: The name of the build target. This is referenced on the
  command-line `yb build targetname`, in CI configuration sections or as things
  to build first.

- `commands`: The commands to be executed; these can be anything you would run
  on the command-line. **Note** YourBase does not expand environment variables
  using the normal `${VALUE}` syntax, though there are ways to do this.
  We do not run `sh` or `bash` as this is not portable; if you need to do that,
  create a shell script and run it as a command explicitly.

- `dependencies`: See the [Dependencies section](#dependencies).

- `environment`: A map of environment variables.

  ```yaml
  build_targets:
    - name: default
      environment:
        ENVIRONMENT: development
        DATABASE_URL: db://localhost:1234/foo
  ```

  For backwards compatibility, environment variables may also be written as a
  list of `KEY=VALUE` pairs.

  ```yaml
  build_targets:
    - name: default
      environment:
        - ENVIRONMENT=development
        - DATABASE_URL=db://localhost:1234/foo
  ```

- `root`: The name of a path where it should run commands from, relative to the
  root of the project.

- `build_after`: A list of build targets whose commands will be executed before
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

- `container`: If present, all the commands in the `commands` directive will
  be executed inside a Docker container. See the [Containers section](#containers)
  for details on the attributes.

  ```yaml
  build_targets:
    - name: container_build
      container:
        image: golang:1.12
      commands:
        - go get
        - go build
        - go test ./...
  ```

  Using a container is the preferred way of installing software that has complex
  dependencies. For example, if you want to use an `apt-get` installable package
  in your build:

  ```yaml
  build_targets:
    - name: container_build
      container:
        image: yourbase/yb_ubuntu:18.04
      environment:
        DEBIAN_FRONTEND: noninteractive
      commands:
        - apt-get update
        - apt-get install -y --no-install-recommends cowsay
        - cowsay moo
  ```

- `tags`: Tags are used to select which build server can handle each build in CI.

  - `os`: Specifies which operating system this build target is applicable to.
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

## Dependencies

`dependencies` blocks describe what [build packs](buildpacks.md) should be
loaded.

At the top-level, the dependencies are added to all build targets:

```yaml
dependencies:
  build:
    - python:3.6.3
build_targets:
  # All build targets will have Python 3.6.3 available.
  - name: default
    commands:
      - python --version

  - name: foo
    commands:
      - pip install -r requirements.txt
```

But each target can also add its own set of dependencies or override versions:

```yaml
dependencies:
  build:
    - python:3.6.3
build_targets:
  # This target will have Python 3.6.3 and Go 1.16.4 available.
  - name: default
    commands:
      - python --version
      - go version
    dependencies:
      - go:1.16.4

  # This target will have Python 3.9.2 available.
  - name: foo
    commands:
      - pip install -r requirements.txt
    dependencies:
      - python:3.9.2
```

The top-level `dependencies` section also accepts a `runtime` section to have
build packs available for the [Executable Target](#executable-target).

```yaml
dependencies:
  runtime:
    - python:3.9.2
```

### Containers

Individual targets can also specify Docker containers as dependencies. This is
especially useful for running a database for integration tests. For example:

<!-- {% raw %} -->

```yaml
build_targets:
  - name: default
    dependencies:
      containers:
        db:
          image: postgres:12
          environment:
            POSTGRES_USER: myapp
            POSTGRES_PASSWORD: xyzzy
            POSTGRES_DB: myapp
          port_check:
            port: 5432
            timeout: 90
    environment:
      # Use whatever environment variables make sense for your test suite.
      PGHOST: '{{ .Containers.IP "db" }}'
      PGUSER: myapp
      PGPASSWORD: xyzzy
      PGDATABASE: myapp
    commands:
      - ./run_tests.sh
```

Any containers specified will be started with an optional health check before
running any commands in the target. The target's environment variables can
reference the container's IP address with the `{{ .Containers.IP "container_name" }}`
syntax.

<!-- {% endraw %} -->

#### Attributes

- `image`: The name of the Docker image. If no tag is specified (e.g. `postgres`),
  `latest` is assumed. For reproducibility of builds, it is highly recommended
  to specify a tag.

- `command`: The [entrypoint][], as a space-separated string. Defaults to the
  `ENTRYPOINT` specified in the image's Dockerfile.

- `workdir`: The working directory to run inside the container. Defaults to the
  `WORKDIR` specified in the image's Dockerfile.

- `environment`: A map of environment variables inside the container. For
  backwards compatibility, these may also be written as a list of `KEY=VALUE`
  pairs.

- `port_check`: If present, yb will wait until a TCP health check passes before
  continuing with the build. The check is specified by two parameters: `port`
  and `timeout`.

  ```yaml
  build_targets:
    - name: default
      dependencies:
        containers:
          db:
            image: postgres:12
            port_check:
              port: 5432   # container port to check on
              timeout: 90  # number of seconds to wait before giving up
  ```

- `mounts`: Additional [volumes][] on the host to mount in the container. This
  is a list of strings in the format `/container/path:/host/path`. If the host
  path is not absolute, it is interpreted as relative to the .yourbase.yml file.

- `ports`: Ports to publish on the host in the format `HOST:CONTAINER`. This is
  typically only used for the [Executable Target](#executable-target) to expose
  a running dev server.

[entrypoint]: https://docs.docker.com/engine/reference/run/#entrypoint-default-command-to-execute-at-runtime
[volumes]: https://docs.docker.com/storage/volumes/

## Executable Target

The `exec` section specifies a target to run when `yb exec` is invoked. This is
often used to start a local development server for the project. The `exec`
section has the same properties as a target (as described in the
[Build Targets section](#build-targets)), with a few small differences:

- Executable targets do not support `build_after`.
- The `environment` attribute is a map of environment names to environment
  variable maps. The `default` environment is used if the `yb exec --environment`
  flag is not specified.

```yaml
exec:
  container:
    ports:
      - 8080:8080
  dependencies:
    runtime:
      - python:3.9.2
  environment:
    default:
      DJANGO_SETTINGS_MODULE: mysite.settings
  commands:
    - python manage.py runserver
```

## CI Instructions

The CI section allows you to define what to build and when by using a
combination of build targets and conditions. In order for the CI system to
properly build your project, you must define the `build_targets` and
`ci` sections.

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

- `name`: The name of the CI build itself (arbitrary string)
- `build_target`: The name of the build target to run. Must match one of the
  names in the `build_targets` section.
- `when`: (Optional) CI build conditions for this target. You can use the
  following variables and combine them using simple boolean logic using
  [PyPred syntax][]:
  - `branch`: The name of the branch being built (e.g. `main`).
  - `action`: Either `commit` or `pull_request`
  - `tagged`: Boolean value, either `true` or `false`

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
