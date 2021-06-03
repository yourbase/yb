# Python Test Acceleration Configuration

Python Test Acceleration works out of the box, but can be configured to take
advantage of your specific environment.

Have configuration needs not addressed here? Give us a holler at
[hi@yourbase.io][hi].

- Common Settings
  - [`YOURBASE_ACCEPT_TOS`](#YOURBASE_ACCEPT_TOS)
  - [`YOURBASE_ACTIVE_COHORT`](#YOURBASE_ACTIVE_COHORT)
  - [`YOURBASE_COHORT_COUNT`](#YOURBASE_COHORT_COUNT)
  - [`YOURBASE_LICENSE_KEY`](#YOURBASE_LICENSE_KEY)
  - [`YOURBASE_OBSERVATION_MODE`](#YOURBASE_OBSERVATION_MODE)
  - [`YOURBASE_REMOTE_CACHE`](#YOURBASE_REMOTE_CACHE)
- Uncommon settings
  - [`YOURBASE_AWS_ACCESS_KEY_ID`](#YOURBASE_AWS_ACCESS_KEY_ID)
  - [`YOURBASE_AWS_SECRET_ACCESS_KEY`](#YOURBASE_AWS_SECRET_ACCESS_KEY)
  - [`YOURBASE_DEBUG`](#YOURBASE_DEBUG)
  - [`YOURBASE_DISABLE`](#YOURBASE_DISABLE)
  - [`YOURBASE_IGNORE_LOCAL_CACHE`](#YOURBASE_IGNORE_LOCAL_CACHE)
  - [`YOURBASE_SYNC_DIRTY`](#YOURBASE_SYNC_DIRTY)
  - [`YOURBASE_TELEMETRY`](#YOURBASE_TELEMETRY)
  - [`YOURBASE_TIMID`](#YOURBASE_TIMID)
  - [`YOURBASE_WORKDIR`](#YOURBASE_WORKDIR)

## Common Settings

### `YOURBASE_ACCEPT_TOS`

- **Type:** bool-ish (`0`, `false`, `off`, `1`, `true`, `on`)
- **Default:** off

When set, YourBase will consider the terms of service permanently accepted for
your organization, and will not output terms of service agreement prompts or
info messages.

### `YOURBASE_ACTIVE_COHORT`

- **Type:** integer in the range `[1, $YOURBASE_COHORT_COUNT]`
- **Default:** 1

When set alongside [`YOURBASE_COHORT_COUNT`](#YOURBASE_COHORT_COUNT), tells
YourBase the cohort ID to run. Used for sharded or otherwise parallelized test
suites.

See [`YOURBASE_COHORT_COUNT`](#YOURBASE_COHORT_COUNT) for more information.

### `YOURBASE_COHORT_COUNT`

- **Type:** integer
- **Default:** 1

When set alongside [`YOURBASE_ACTIVE_COHORT`](#YOURBASE_ACTIVE_COHORT), tells
YourBase how many cohorts tests should be split into. Used for sharded or
otherwise parallelized test suites.

This pair of settings lets YourBase work with your existing sharding or
parallelization setup. You likely already have a test splitting solution in
place whose job is to slice up your pool of tests so that each shard or process
runs their fair share of tests.

However, most test splitting solutions are not sticky; tests "jump" shards as
the pool changes. The effectiveness of YourBase acceleration scales with test
stickiness, so these solutions are not recommended for use with YourBase.

As a convenience, YourBase provides a built-in sticky test splitting
implementation. To use it, first remove your existing test splitting tools. It
should appear as if each shard will run the entire test suite.

Then, set `YOURBASE_ACTIVE_COHORT` to the ID of the current shard or process
(starting from 1), and set `YOURBASE_COHORT_COUNT` to the total number of shards
or processes. Under the hood, YourBase will just-in-time deselect tests that
aren't in the current cohort. Each test will stick to its assigned shard for
life.

### `YOURBASE_LICENSE_KEY`

- **Type:** opaque string
- **Default:** _(unset)_

When set to a valid license key, YourBase acceleration will be unlocked for use
after the end of the free trial. Email [hi@yourbase.io][hi] to obtain a license
key.

### `YOURBASE_OBSERVATION_MODE`

- **Type:** bool-ish (`0`, `false`, `off`, `1`, `true`, `on`)
- **Default:** off

When on, YourBase will internally trace, time, and catalogue your tests as
normal and use this information to make acceleration decisions; however it will
not act on these decisions, so your test suite will run as if YourBase were not
enabled.

At the end of the run, YourBase will perform a self-check by cross-referencing
its acceleration decisions with test results. If the self-check passes, it will
report potential time savings for this test run based on timing data for the
would-be-skipped tests.

If the self-check fails, YourBase will report details of the failure to stderr.
We encourage you to reach out to [bugs@yourbase.io](bugs-email) if this happens.

[bugs-email]: mailto:bugs@yourbase.io

### `YOURBASE_REMOTE_CACHE`

- **Type:** uri
- **Default:** _(unset)_

When set, YourBase will synchronize dependency graphs with the given remote
location. This setting is recommended for use when using YourBase in CI, as the
filesystem will not be a dependable store for dependency graphs.

Dependency graphs generated from dirty working trees will not be synchronized,
as they can poison the cache. See [`YOURBASE_SYNC_DIRTY`](#YOURBASE_SYNC_DIRTY)
to override this behavior.

Currently, the only supported protocol is `s3`.

#### `s3`

[System credentials][aws-sys-creds] for AWS will be used if present. To use
different credentials than the system credentials, see
[`YOURBASE_AWS_ACCESS_KEY_ID`](#YOURBASE_AWS_ACCESS_KEY_ID) and
[`YOURBASE_AWS_SECRET_ACCESS_KEY`](#YOURBASE_AWS_SECRET_ACCESS_KEY).

[aws-sys-creds]: https://docs.aws.amazon.com/general/latest/gr/aws-access-keys-best-practices.html

##### Example

```sh
# Without a key prefix
export YOURBASE_REMOTE_CACHE=s3://my-bucket-name

# With a key prefix
export YOURBASE_REMOTE_CACHE=s3://my-bucket-name/my/key/prefix
```

## Uncommon settings

### `YOURBASE_AWS_ACCESS_KEY_ID`

When set alongside
[`YOURBASE_AWS_SECRET_ACCESS_KEY`](#YOURBASE_AWS_SECRET_ACCESS_KEY), forces
YourBase to use these credentials over system credentials when interacting with
AWS.

These environment variables are recommended for use if your system credentials
are fudged for the sake of your tests.

### `YOURBASE_AWS_SECRET_ACCESS_KEY`

When set alongside [`YOURBASE_AWS_ACCESS_KEY_ID`](#YOURBASE_AWS_ACCESS_KEY_ID),
forces YourBase to use these credentials over system credentials when
interacting with AWS.

These environment variables are recommended for use if your system credentials
are fudged for the sake of your tests.

### `YOURBASE_DEBUG`

- **Type:** bool-ish (`0`, `false`, `off`, `1`, `true`, `on`)
- **Default:** off

When on, YourBase will report significantly more internal information to stdout,
stderr, and the file returned by this expression:

```sh
echo ${XDG_STATE_HOME-~/.local/state}/yourbase/python.log
```

### `YOURBASE_DISABLE`

- **Type:** bool-ish (`0`, `false`, `off`, `1`, `true`, `on`)
- **Default:** off

When on, YourBase will not load.

Enabling this setting then manually attaching to a test framework using
`yourbase.attach` produces undefined behavior.

### `YOURBASE_IGNORE_LOCAL_CACHE`

- **Type:** bool-ish (`0`, `false`, `off`, `1`, `true`, `on`)
- **Default:** off

When on, YourBase will not look in the filesystem for a dependency graph. If
[`YOURBASE_REMOTE_CACHE`](#YOURBASE_REMOTE_CACHE) is set, it will still be used
as normal.

This setting can be used if the local cache is expected to be poisoned. This can
happen if cohorting is used against a local cache.

### `YOURBASE_SYNC_DIRTY`

- **Type:** bool-ish (`0`, `false`, `off`, `1`, `true`, `on`)
- **Default:** off

When on, YourBase will [synchronize graphs](#YOURBASE_REMOTE_CACHE) even if the
Git working tree is dirty.

This setting is not recommended for use if you plan to use YourBase on developer
machines, as it will poison the remote cache.

If you only plan to use YourBase in CI, this setting can help you overcome
situations where you need your working tree to be dirty while building, and you
know the dirtiness will not affect the dependency graph.

If that situation does not apply to you, do not enable this setting.

### `YOURBASE_TELEMETRY`

- **Type:** bool-ish (`0`, `false`, `off`, `1`, `true`, `on`)
- **Default:** on

When on, YourBase will send anonymized telemetry data to `api.yourbase.io` over
HTTPS for the purposes of improving the product.

Telemetry data never includes your code.

### `YOURBASE_TIMID`

- **Type:** bool-ish (`0`, `false`, `off`, `1`, `true`, `on`)
- **Default:** off

When on, YourBase will use a slower tracing algorithm that is less prone to
conflicts with other packages than the default. Only enable this if you
experience issues with the default algorithm.

### `YOURBASE_WORKDIR`

- **Type:** absolute or relative path
- **Default:** `.`

The directory YourBase should treat as the project directory. Only code in this
directory, or one of its descendants, is guaranteed to be traced. You usually do
not need to change this.

[hi]: mailto:hi@yourbase.io
