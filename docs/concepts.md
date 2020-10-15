# yb Concept Reference

This document describes the semantics and terms used by yb. It is aimed at
developers and advanced users of yb, and can be seen as a light-weight
specification of a _build runner_ &mdash; the yb CLI or any other system that
runs a YourBase build.

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD",
"SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be
interpreted as described in [RFC 2119][].

[RFC 2119]: https://tools.ietf.org/html/rfc2119

## Targets

A _target_ is a buildable thing &mdash; it could be a service, a library,
a binary, etc. Targets can depend on zero or more other targets, creating a
[directed acyclic graph][].

A target has five _phases_, each of which has a list of one or more _commands_
to run. A command is a program invocation, like `gcc -o foo foo.c` or
`npm install`. These phases are all optional, but at least one **MUST** be
specified per target. The phases of a target are:

-  **Build** **MUST** be run when the target is depended on and before any of
   the other phases in the same target. This phase can optionally declare output
   files, which **SHALL** be accessible to other targets that depend on this
   target and any other phases in the same target.
-  **Test** _SHOULD_ run the target's test suite. Common commands in this stage
   would include `go test ./...` or `npm test`.
-  **Execute** **MUST** be run when the target is used as a resource for
   another target (discussed later in this section).
-  **Release** _SHOULD_ create release output files, like tarballs or binary
   packages. This phase can optionally declare output files, which **SHALL** be
   accessible to the deploy phase of the same target.
-  **Deploy** _SHOULD_ deploy the release output.

The build, test, and release phases _SHOULD_ be safe to be run multiple
times. Automated builds (like in CI) _MAY_ retry these phases without prompting
the user. Commands in these phases _MAY_ also be skipped or reordered if the
build runner detects that doing so won't alter the results of the phase.
Commands in the execute and deploy phases **MUST NOT** be skipped or retried.

A _resource_ is a process that runs concurrently with a phase's commands.
This can be either another target's execute phase or it may be a Docker
container.

**TODO:** Include example of target YAML.

[directed acyclic graph]: https://en.wikipedia.org/wiki/Directed_acyclic_graph

### Commands

Each phase **SHALL** run in a separate, isolated environment. For example,
a phase may run in a Docker container or a chroot jail. A phase _MAY_ run on
a different host from the build runner. Commands executed in the phase **MUST**
have the following environment variables set:

-  `HOME` **MUST** be set to the path of an readable and writable directory.
   This _SHOULD NOT_ be the same as the user's actual `HOME` directory to
   keep builds reproducible.
-  `LOGNAME` and `USER` **MUST** be set to the name of the UNIX user running the
   command (not the build runner).
-  `PATH` **MUST NOT** be empty.
-  `TZ` **MUST** be set to `UTC`.

By default, commands **SHALL** run with a directory that has readable copies of
files in the package as defined in the [Package section](#Package).

### Buildpacks

A _buildpack_ sets environment variables and installs files before a phase's
commands are executed. These typically provide compilers or other
programming language tools.

Example:

```
build_targets:
- name: default
  dependencies:
    buildpacks:
      - go:1.15
      - python:3.7
```

## Package

A _package_ is a collection of related files and targets. A package is a
directory that contains a .yourbase.yml file directly inside it. Files can only
belong to a single package. A package _MAY_ contain subpackages in subdirectories,
but files within those subdirectories belong to the subpackage, not the parent
package.
