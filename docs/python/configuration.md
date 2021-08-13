---
nav_order: 2
---
# Python Test Acceleration Configuration

Python Test Acceleration works out of the box, but can be configured to take
advantage of your specific environment using the environment variables listed
below.

Have configuration needs not addressed here? Give us a holler at
[hi@yourbase.io][hi].

- Common Settings
  - [`YOURBASE_ACCEPT_TOS`](#yourbase_accept_tos)
  - [`YOURBASE_ACTIVE_COHORT`](#yourbase_active_cohort)
  - [`YOURBASE_COHORT_COUNT`](#yourbase_cohort_count)
  - [`YOURBASE_LICENSE_KEY`](#yourbase_license_key)
  - [`YOURBASE_OBSERVATION_MODE`](#yourbase_observation_mode)
  - [`YOURBASE_REMOTE_CACHE`](#yourbase_remote_cache)
- Uncommon settings
  - [`YOURBASE_AWS_ACCESS_KEY_ID`](#yourbase_aws_access_key_id)
  - [`YOURBASE_AWS_SECRET_ACCESS_KEY`](#yourbase_aws_secret_access_key)
  - [`YOURBASE_DEBUG`](#yourbase_debug)
  - [`YOURBASE_DISABLE`](#yourbase_disable)
  - [`YOURBASE_IGNORE_LOCAL_CACHE`](#yourbase_ignore_local_cache)
  - [`YOURBASE_SYNC_DIRTY`](#yourbase_sync_dirty)
  - [`YOURBASE_TELEMETRY`](#yourbase_telemetry)
  - [`YOURBASE_TIMID`](#yourbase_timid)
  - [`YOURBASE_WORKDIR`](#yourbase_workdir)

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

When set alongside [`YOURBASE_COHORT_COUNT`](#yourbase_cohort_count), tells
YourBase the cohort ID to run. Used for sharded or otherwise parallelized test
suites.

See [`YOURBASE_COHORT_COUNT`](#yourbase_cohort_count) for more information.

### `YOURBASE_COHORT_COUNT`

- **Type:** integer
- **Default:** 1

When set alongside [`YOURBASE_ACTIVE_COHORT`](#yourbase_active_cohort), tells
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
aren't in the current cohort. This selection is consistent between runs; given
the same cohort ID and count, tests will be selected to the same shard for life.

### `YOURBASE_LICENSE_KEY`

- **Type:** opaque string
- **Default:** _(unset)_

When set to a valid license key, YourBase acceleration will be unlocked for use
after the end of the free trial. Email [hi@yourbase.io][hi] to obtain a license
key.

### `YOURBASE_OBSERVATION_MODE`

- **Type:** bool-ish (`0`, `false`, `off`, `1`, `true`, `on`)
- **Default:** off

When on, YourBase will not skip tests. Instead, it will record the duration and
outcome of each test it believes can be skipped.

If any of these tests fail, this is a bug in YourBase's tracing or acceleration.
YourBase will complain loudly and output details; please report these to
[bugs@yourbase.io][].

Otherwise, the total amount of time that could have been saved is output to
stdout.

[bugs@yourbase.io]: mailto:bugs@yourbase.io

### `YOURBASE_REMOTE_CACHE`

- **Type:** uri
- **Default:** _(unset)_

When set, YourBase will synchronize dependency graphs with the given remote
location. This setting is recommended for use when using YourBase in CI, as the
filesystem will not be a dependable store for dependency graphs.

Dependency graphs generated from dirty working trees will not be synchronized,
as they can poison the cache. See [`YOURBASE_SYNC_DIRTY`](#yourbase_sync_dirty)
to override this behavior.

Currently, the only supported protocol is `s3`.

#### `s3`

Sets an S3 bucket as a remote cache. `Get`, `Put`, and `List` permissions are
needed.

[System credentials][aws-sys-creds] for AWS will be used if present. To use
different credentials than the system credentials, see
[`YOURBASE_AWS_ACCESS_KEY_ID`](#yourbase_aws_access_key_id) and
[`YOURBASE_AWS_SECRET_ACCESS_KEY`](#yourbase_aws_secret_access_key).

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

- **Type:** AWS access key ID
- **Default:** _(unset)_

When set alongside
[`YOURBASE_AWS_SECRET_ACCESS_KEY`](#yourbase_aws_secret_access_key), forces
YourBase to use these credentials over system credentials when interacting with
AWS.

These environment variables are recommended for use if your system credentials
are fudged for the sake of your tests.

### `YOURBASE_AWS_SECRET_ACCESS_KEY`

- **Type:** AWS secret access key
- **Default:** _(unset)_

When set alongside [`YOURBASE_AWS_ACCESS_KEY_ID`](#yourbase_aws_access_key_id),
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

When on, YourBase will not read or write dependency graphs on the filesystem. If
[`YOURBASE_REMOTE_CACHE`](#yourbase_remote_cache) is set, it will still be used
as normal.

This setting can be used if the local cache is expected to be poisoned. This can
happen if cohorting is used against a local cache.

### `YOURBASE_SYNC_DIRTY`

- **Type:** bool-ish (`0`, `false`, `off`, `1`, `true`, `on`)
- **Default:** off

When on, YourBase will [synchronize graphs](#yourbase_remote_cache) even if the
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

Telemetry data **never** includes your code.

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
