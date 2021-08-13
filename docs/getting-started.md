---
parent: CI and CLI Documentation
nav_order: 2
---

# Getting Started

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

For more information, see the [complete YAML configuration syntax
reference](configuration.md).

## Run your first local build

If you created a `default` target, you can build it with:

```sh
yb build
```

If you've changed your target's name or added a new one, for example `foo`, run:

```sh
yb build foo
```

## Run your first remote build

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

## Triggering Builds from GitHub pushes

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

## Inspecting the Build Environment

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


