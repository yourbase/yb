# yb Concepts

## Package

A _package_ is a collection of related files and targets. A package is a
directory that contains a .yourbase.yml file directly inside it. Files can only
belong to a single package. A package may contain subpackages in subdirectories,
but files within those subdirectories belong to the subpackage, not the parent
package.

## Target

A _target_ is a buildable thing &mdash; it could be a service, a library,
a binary, etc. A target has four _phases_ (all optional, but at least one must
be present):

-  Build
-  Test
-  Execute
-  Release

Each phase, if present, has a list of one or more commands to run. The build
phase can declare output files. Output files will be accessible to other targets
that directly depend on this target.

The build and test phases are assumed to be safe to be run multiple times:
automated builds (like in CI) may retry these phases without prompting the user.
Commands in these phases may also be skipped or reordered if the build tool
detects that doing so won't alter the results of the build. Commands in the
execute and release phases will not be skipped nor will they be retried.

## Contexts

A _context_ is an environment in which a target is built, tested, released,
or executed. Each phase of a target is executed in a separate context. A context
may run on the same host as the build tool, or it may run on an entirely
different host.

Programs in the context are guaranteed to have access to all the files in the
package and the following environment variables:

-  `HOME`
-  `LOGNAME`
-  `PATH`
-  `TZ=UTC`
-  `USER`

## Buildpacks

A _buildpack_ installs development tools into a context. It sets up environment
variables and files before a phase is executed.

## Resources

A _resource_ is a process that runs concurrently with the build. This may be
another target's execute phase or it may be a Docker container.
