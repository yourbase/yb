<h1><img alt="YourBase" src="docs/images/Logo-Horiz-On-White@1x.jpg" width="384" height="112"></h1>

[![YourBase Community Slack](https://img.shields.io/badge/slack-@yourbase/community-blue.svg?logo=slack)](https://slack.yourbase.io)

`yb build all-the-things!`

YourBase is a build tool that makes working on projects much more delightful. Stop worrying
about dependencies, keep your CI build process in-sync with your local
development process and onboard new team members with ease.

The primary features of the YB tooling are:

* *Consistent local and CI tooling* How a project is built as part of the
  CI/CD process should not be any different than how it is built on a
  developer's machine. Keeping tooling in-sync means predictable results and 
  a better developer experience for everyone. (Although local builds are not 
  fully isolated yet)

* *Accelerated on-boarding* Many projects have long sets of instructions that 
  are required for a developer to get started. With YB, the experience is as 
  simple as getting source code and running `yb build` - batteries included!

* *Programmatic dependency management* No need to have developers manually
  install and manage different versions of Go, Node, Java, etc. By describing
  these tools in codified build-packs, these can be installed and configured 
  automatically on a per-project basis. Manage containers and other runtime 
  dependencies programmatically and in a consistent manner. 

* *Remote builds* Run your work and tests in the cloud just like you would 
  locally or as part of CI! Stream the results back to your machine in real-time
  to offload the work.

![magic!](http://www.reactiongifs.com/r/mgc.gif)

## Installation

yb uses Docker for isolating the build environment. You can install Docker here:
https://hub.docker.com/search/?type=edition&offering=community

If you're using WSL on Windows, you will need to use [Docker in WSL2](https://code.visualstudio.com/blogs/2020/03/02/docker-in-wsl2).

### macOS

You can install `yb` using [Homebrew][]:

```sh
brew install yourbase/yourbase/yb
```

or you can download a binary from the [latest GitHub release][] and place it
in your `PATH`.

[Homebrew]: https://brew.sh/
[latest GitHub release]: https://github.com/yourbase/yb/releases/latest

### Linux (and WSL2)

On Debian-based distributions (including Ubuntu), you can use our APT
repository:

```sh
# Import the signing key
sudo curl -fsSLo /etc/apt/trusted.gpg.d/yourbase.asc https://apt.yourbase.io/signing-key.asc

# Add the YourBase APT repository to the list of sources
echo "deb [arch=amd64 signed-by=/etc/apt/trusted.gpg.d/yourbase.asc] https://apt.yourbase.io stable main" | sudo tee /etc/apt/sources.list.d/yourbase.list

# Update the package list and install yb
sudo apt-get update && sudo apt-get install yb
```

For Red Hat, Fedora, or CentOS, you can install the RPM from the
[latest GitHub release][]. For other distributions, copy the Linux binary from
the release into your `PATH`.

## Getting Started

To use yb, you need a `.yourbase.yml` file at the top of your project directory.
You can run `yb init` to generate one:

```shell
cd path/to/my/project
yb init
```

The YourBase configuration was designed from the ground up to be a fast,
composable and easy to use method to declare build targets. Targets defined
in the configuration are built in an isolated and uniform container, so local
builds are similar to remote builds.

The YourBase YAML replaces both legacy CI configs and container build
configuration formats like Dockerfile. It uses containers underneath and you can
still run things on the side like MySQL, Postgres, Redis or any service from a
container image.

For more examples and a complete reference to the YAML configuration syntax,
see https://docs.yourbase.io/configuration/yourbase_yaml.html

### Run your first local build

If you created a `default` target, you can build it with:

```sh
yb build
```

If you've changed your target's name or added a new one, for example `foo`, run:

```sh
yb build foo
```

### Run your first remote build

To use remote builds, first you have to sign-in to YourBase.io.
Run this to get a sign-in URL:

```sh
yb login
```

Then try to run a remote build. To build the `default` target, just do:

```sh
yb remotebuild
```

or if you have a different target in your config:

```sh
yb remotebuild <target>
```

You can watch the build/test stream both locally and in the build dashboard at https://app.yourbase.io

### Triggering Builds from GitHub pushes

You can also build code after every push to a GitHub repository. Install the
[GitHub application](https://github.com/apps/yourbase), then push a change to
your repository. After you do, you should see a new build in the
[YourBase dashboard](https://app.yourbase.io).

By default, the YourBase Accelerated CI runs all build targets whenever there
is a GitHub push event. You can change it to only build after certain events or
targets by specifying conditions for build service builds:

```yaml
ci:
  builds:
    - name: tests
      build_target: default
      when: branch is 'master' OR action is 'pull_request'
```

### Inspecting the Build Environment

If you want to run tools inside your local build environment, you can use
`yb run`. This will use the exact same version of the tools as what you
specified in your `.yourbase.yml` file.

```shell
yb run python myscript.py
```

You can even pull up a shell and run commands interactively:

```shell
yb run bash
```

`yb run` will use the target named `default` if you do not specify a target.
If you want to use a different target:

```shell
yb run --target=foo bash
```

## Documentation

More documentation available at http://docs.yourbase.io

## Contributing 

We welcome contributions to this project! Please see the [contributor's guide][]
for more information. 

[contributor's guide]: CONTRIBUTING.md

## License 

This project is licensed under an [Apache 2.0 license](LICENSE.md).

