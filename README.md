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

yb is available for Mac via Homebrew, and Linux/WSL2 via `apt-get`.
Instructions are available at https://docs.yourbase.io/installation.html

## Getting Started

To use yb, you need a `.yourbase.yml` file at the top of your project directory.
You can run `yb init` to generate one:

```shell
cd path/to/my/project
yb init
yb build
```

For a more in-depth tutorial, see https://docs.yourbase.io/getting-started.html

## Documentation

More documentation available in the [docs folder](docs).

## Contributing 

We welcome contributions to this project! Please see the [contributor's guide][]
for more information. 

[contributor's guide]: CONTRIBUTING.md

## License 

This project is licensed under an [Apache 2.0 license](LICENSE.md).
