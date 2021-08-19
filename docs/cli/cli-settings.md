---
parent: CI and CLI
nav_order: 5
---

# yb settings reference

## Environment

<!-- {% raw %} -->

<dl markdown="0">
   <dt><code>NETRC</code></dt>
   <dd>
      If set to a non-empty value, then yb will copy the named
      <a href="https://everything.curl.dev/usingcurl/netrc">netrc file</a>
      into the build environment. Otherwise, yb will use <code>$HOME/.netrc</code>
      (if present).
   </dd>

   <dt><code>NO_COLOR</code></dt>
   <dd>
      If set to a non-empty value, then yb will not output ANSI escape codes
      with its logs. The <code>NO_COLOR</code> environment variable is
      propagated to the build environment, but yb makes no attempt to remove
      ANSI escape codes from subprocess output.
   </dd>

   <dt><code>PATH</code></dt>
   <dd>
      When running builds on the host (the default), yb will propagate
      <code>PATH</code> into the build environment.
   </dd>

   <dt><code>YB_CACHE_DIR</code></dt>
   <dd>
      If set to a non-empty value, sets the location of cached files.
      The default is <code>$XDG_CACHE_HOME/yb</code>.
   </dd>

   <dt><code>YB_CONTAINER_*_IP</code></dt>
   <dd>
      If a target has a container dependency <code>foo</code> and
      <code>YB_CONTAINER_FOO_IP</code> is set, then yb will make no attempt to
      start the container and makes the variable value available as
      <code>{{ .Containers.IP "foo" }}</code>.
   </dd>

   <dt><code>YB_WORKSPACES_ROOT</code></dt>
   <dd>
      If set to a non-empty value, sets the location of build environment home
      directories. The default is <code>$YB_CACHE_DIR/workspaces</code>.
   </dd>
</dl>

<!-- {% endraw %} -->

yb obeys the environment variables laid out in the [XDG Base Directory Specification][].

[XDG Base Directory Specification]: https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html

## Settings File

The settings file is located at `$XDG_CONFIG_DIRS/yb/settings.ini`. Settings set
in earlier directories take precedence over settings in later directories.

```ini
[defaults]
# Verbosity level. Valid values are "info", "debug", "warning", and "error".
# Default is "info".
log-level = info

[user]
# YourBase API token. Automatically set by `yb login`.
api_key = xyzzy
```
