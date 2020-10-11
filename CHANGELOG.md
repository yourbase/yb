# yb Release Notes

The format is based on [Keep a Changelog][], and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

[Keep a Changelog]: https://keepachangelog.com/en/1.0.0/
[Unreleased]: https://github.com/yourbase/yb/compare/v0.3.2...HEAD

## [Unreleased][]

### Changed

-  The build cache is now created under `$XDG_CACHE_HOME/yb`
   (usually `$HOME/.cache/yb`) rather than `$HOME/.yourbase`.
   You can safely remove `$HOME/.yourbase` to reclaim disk space.

### Fixed

-  yb now respects the `XDG_CONFIG_HOME` and `XDG_CONFIG_DIRS` environment
   variables when reading configuration files.
-  Update to latest version of [Narwhal](https://github.com/yourbase/narwhal),
   which contains many fixes for Docker interactions.
-  `yb build` will now exit with a non-zero status code if more than one
   argument is given. Previously, it would silently ignore such arguments.

## [0.3.2][]

Version 0.3.2 fixes issues with the Python buildpack.

[0.3.2]: https://github.com/yourbase/yb/releases/tag/v0.3.2

### Fixed

-  Fixed an incorrect URL for Miniconda in the Python buildpack.
-  HTTP downloads in yb no longer ignore the status code and will abort for
   any non-200 status code.

## [0.3.1][]

Version 0.3.1 fixes an issue with error handling during builds.

[0.3.1]: https://github.com/yourbase/yb/releases/tag/v0.3.1

### Fixed

-  Fixed a regression where if a dependent target fails, it did not stop
   the build.

## [0.3.0][]

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
