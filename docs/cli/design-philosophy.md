---
parent: CI and CLI
nav_order: 7
---

# Design Philosophy

The YourBase ecosystem is designed around two primary goals:

-  We should adhere to the principle of least surprise
-  Our tools should provide consistent behavior and structure without forcing the user to go to extreme measures

By design, integrating the yb tooling into your project should require
minimal adaptation on the part of a team. We want to provide you with the
structure to adapt your process over time while getting immediate value for
minimal effort.

What this means is that you do not need to re-write your entire build process
to work with our tooling. If you use NPM and NodeJS, you should (and are)
able to simply add to your .yourbase.yml file a few commands and be up and
running. Once you see the value that the tool brings you can begin to adopt
(or build) buildpacks in order to encourage better repeatability across
systems.

## Core Concepts

YourBase is both a command-line tool and a build service. The primary goal of
the tooling is to provide developers with a consistent experience throughout
the development and release cycle. By eliminating the differences between how
software is built locally as compared to how it is built by the build
service, we remove the hassle of having to manage separate, infrequently
used, configuration that describes traditional CI processes.

There are a few primary building blocks used by YourBase:

-  Dependencies
-  Build targets
-  Build service

### Dependencies

One of the major design goals of YB, as a tool, is to eliminate the need to
manage tooling and dependencies for every language and separately from your
build process. This is achieved by modeling the various languages and build
systems one might use, such as Go, Node.JS, or Ruby as what we refer to as
“build-packs”; by providing building blocks that can be combined to compose
your projects tools, YB helps developers to be more productive by worrying
less about installing and configuring tools required to work on a project.

#### Build packs

Build packs are programmatically described ways of installing, configuring,
and setting up dependencies that your project has. Each build pack is
responsible for managing the lifecycle of a single tool and can be combined
with others to provide a rich multi-tool environment on a per-project basis.
This has a number of design principles:

-  Containment
-  Composability
-  Convenience
-  Consistency
-  Correctness

#### Containment

Each build pack is designed to contain its installation to its own versioned
space that is commonly accessible to all projects while isolating its
per-workspace resources in such a way that they do not impact the
installation nor other workspaces that are using hat tool. For those familiar
with Python’s virtual environments this may feel somewhat familiar.

#### Composability

Build packs should be able to be combined with one another to provide a
multi-tool / language environment. For example, there are build packs for
Maven, Gradle, the OpenJDK and the Oracle JDK. One should be able to leverage
build packs for Maven and the Oracle JDK but then replace the Oracle
dependency with the OpenJDK simply by changing the dependency list.

#### Convenience

One of the primary challenges we face with a wide variety of build tools is
the effort involved in finding, installing, and configuring each tool. Tools
have a variety of required environment variables, cache locations, download
locations, and so on. Build packs take care of all of this so that in order
to start using NodeJS 10.15, for example, a developer only needs to add the
dependency and it will be fetched, configured and ready to use immediately.

In addition to making it much simpler to begin using a new tool or language,
it means that the on-boarding process for new developers is greatly
simplified.

#### Consistency

By isolating each tool and working in a containers, per-project work area,
there is a consistent experience for all developers working across a variety
of projects. Each build pack understands platform-specific nuances and
mediates them. And, because the YourBase Build Service uses the exact same
CLI, all of the setup and configuration done during CI is exactly the same as
locally.

#### Correctness

Because each build pack is written in Go, maintainers are able to write tests
and verify that they work across a variety of platforms and environments.

### Build targets

Beyond the management of dependencies, YB manages build targets. Similar to
what one might expect from Make or other tools, each build target is a set of
steps required to build some result.

Build targets provide the primary building block for local and CI system
operations. In addition to commands to run, build targets can describe other
dependencies, such as containers to run during the build process, environment
variables, or dependencies on other targets being run first.

### Build service

The YB tool describes and manages build and testing in a way that is
different from other tools. The build description is only composed of three
pieces of information: the name of the build, the build target to execute,
and when the build service should build that target. In doing so, YB runs
builds in the exact same way they would be run locally. No more wondering if
it will pass in the “CI” system if it builds correctly locally.
