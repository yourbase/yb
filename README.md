# YourBase CLI 


[![YourBase Community Slack](https://img.shields.io/badge/slack-@yourbase/community-blue.svg?logo=slack)](https://slack.yourbase.io)

`yb build all-the-things!`

A build tool that makes working on projects much more delightful. Stop worrying
about dependencies and keep your CI build process in-sync with your local
development process.

It is kind of like docker, but with composable build-packs, configurable with YAML
and that can be used in CIs or in remote builds.

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
  to offload the work. (Currently in-progress, requires a YourBase account)

![magic!](http://www.reactiongifs.com/r/mgc.gif)

# How to use it

1. Download and install `yb` from https://dl.equinox.io/yourbase/yb/stable - alternatively, build the code in this repository using `go build` with a recent version of go. 
2. Clone a package from GitHub 
3. Write a simple build manifest (more below)
4. Build awesome things!

# Documentation 

See http://docs.yourbase.io.

# Self-update

To update to a new *stable* version of yb, use:

`yb update`

You can pick other channels by using `-channel`:

`yb update -channel=development`

# Contributing 

We welcome contributions to this CLI, please see the CONTRIBUTING file for more
information. 

# License 

This project is licensed under the Apache 2.0 license, please see the LICENSE
file for more information.

