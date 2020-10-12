# yb Release Notes

The format is based on [Keep a Changelog][], and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

[Keep a Changelog]: https://keepachangelog.com/en/1.0.0/
[Unreleased]: https://github.com/yourbase/yb/compare/v0.4.0...HEAD

## [0.4.0][] - 2020-10-12

Version 0.4 removes some broken or ill-conceived functionality from yb and
changes where yb stores files to obey the [XDG Base Directory specification][].

[0.4.0]: https://github.com/yourbase/yb/releases/tag/v0.4.0
[XDG Base Directory specification]: https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html

### Changed

-  The build cache is now created under `$XDG_CACHE_HOME/yb`
   (usually `$HOME/.cache/yb`) rather than `$HOME/.yourbase`.
   You can safely remove `$HOME/.yourbase` to reclaim disk space.

### Removed

-  Workspaces. At the moment, we're focusing on single packages and will
   reintroduce the concept when we have a better grasp on how dependencies will
   work inside yb.
-  The build log streaming feature has been removed, since it has been broken
   for some time. However, we hope to reintroduce it in a future version.
-  Removed the `yb update` command. Users can now stay up-to-date with either
   the APT or Homebrew repositories.

### Fixed

-  yb now respects the `XDG_CONFIG_HOME` and `XDG_CONFIG_DIRS` environment
   variables when reading configuration files.
-  Update to latest version of [Narwhal](https://github.com/yourbase/narwhal),
   which contains many fixes for Docker interactions.
-  `yb build` will now exit with a non-zero status code if more than one
   argument is given. Previously, it would silently ignore such arguments.
-  The download span names in the `yb build` trace now include the URL rather
   than the unhelpful `%s`.

## [0.3.2][] - 2020-10-07

Version 0.3.2 fixes issues with the Python buildpack.

[0.3.2]: https://github.com/yourbase/yb/releases/tag/v0.3.2

### Fixed

-  Fixed an incorrect URL for Miniconda in the Python buildpack.
-  HTTP downloads in yb no longer ignore the status code and will abort for
   any non-200 status code.

## [0.3.1][] - 2020-10-06

Version 0.3.1 fixes an issue with error handling during builds.

[0.3.1]: https://github.com/yourbase/yb/releases/tag/v0.3.1

### Fixed

-  Fixed a regression where if a dependent target fails, it did not stop
   the build.

## [0.3.0][] - 2020-10-05

Version 0.3 is the first release with our new release automation.

[0.3.0]: https://github.com/yourbase/yb/releases/tag/v0.3.0

### Added

-  Add `yb token` command

### Changed

-  We are no longer using Equinox for releases. See the
   [README](https://github.com/yourbase/yb/blob/main/README.md) for installation
   instructions.
-  Release binaries are now smaller due to debug symbol stripping.
-  Release binaries are now built as [position-independent executables][].
-  The output of timing information at the end of a build has changed formatting
   slightly to accommodate more sophisticated breakdowns in the future.

[position-independent executables]: https://en.wikipedia.org/wiki/Position-independent_code

### Fixed

-  Fixes to OpenJDK and Anaconda buildpacks
   ([#170](https://github.com/yourbase/yb/pull/170))
