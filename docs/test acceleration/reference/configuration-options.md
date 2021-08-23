---
layout: default
title: Configuration options
nav_order: 1
parent: Reference
grand_parent: Test acceleration
permalink: /test-acceleration/reference/configuration-options
---

# Configuration options
{: .no_toc }

This page documents the available configuration options. These operate as environment variables that should be set the same way environment variables are set for your deployment.

<details open markdown="block">
  <summary>
    Table of contents
  </summary>
  {: .text-delta }
- TOC
{:toc}
</details>

## YOURBASE_ACCEPT_TOS
`Type`: `bool-ish (0, false, off, 1, true, on)`

`Default`: `off`

When set, YourBase Test Acceleration will consider the terms of service permanently accepted for your organization, and will not output terms of service agreement prompts or info messages. This is helpful, for example, when rolling out YourBase Test Acceleration to CI for an organization.

---

## YOURBASE_ACTIVE_COHORT
`Type`: `integer` in the range `[1, $YOURBASE_COHORT_COUNT]`

`Default`: `1`

When set alongside [YOURBASE_COHORT_COUNT](#yourbase_cohort_count), tells YourBase Test Acceleration the cohort ID to run. Used for sharded or otherwise parallelized test suites.

See [here](../advanced-usage/accelerate-parallelized-tests.md) to learn to use this in your parallelized test-suite. 

--- 

## YOURBASE_COHORT_COUNT
`Type`: `integer`
`Default`: `1`

When set alongside [YOURBASE_ACTIVE_COHORT](#yourbase_active_cohort), tells YourBase Test Acceleration how many cohorts the tests should be split into.

This pair of settings lets YourBase Test Acceleration work with your existing sharding or parallelization setup. See here to [learn to use it in your parallelized test setup](../advanced-usage/accelerate-parallelized-tests.md).

---

## YOURBASE_LICENSE_KEY
`Type`: `opaque string`
`Default`: `(unset)`

When set to a valid license key, YourBase Test Acceleration will be unlocked for use after the end of the free trial. Email hi@yourbase.io to obtain a license key.

---

## YOURBASE_OBSERVATION_MODE
`Type`: `bool-ish (0, false, off, 1, true, on)`
`Default`: `off`

When on, YourBase Test Acceleration will not skip tests, and instead, only record the duration and outcome of each test it believes should be skipped. 

When this setting is off, YourBase Test Acceleration will optimize your test run-time as usual.

It’s useful to turn this setting on when you’re testing YourBase Test Acceleration before taking it live with your codebase. Learn more about [how to use the Observation mode here](../advanced-usage/verify-results.md).

---

## YOURBASE_REMOTE_CACHE
`Type`: `uri`
`Default`: `(unset)`


When set, this synchronizes [dependency graphs](../how-it-works.md#dependency-graph) generated only from clean working trees—dependency graphs generated from dirty working trees will not be synchronized as they can poison the cache. Use [YOURBASE_SYNC_DIRTY](#yourbase_sync_dirty) to override this behavior.

```sh
# Without a key prefix
export YOURBASE_REMOTE_CACHE=s3://my-bucket-name
```

```sh
# With a key prefix
export YOURBASE_REMOTE_CACHE=s3://my-bucket-name/my/key/prefix
```

This setting is recommended for use when using YourBase Test Acceleration in CI, as the filesystem will not be a dependable store for [dependency graphs](../how-it-works.md#dependency-graph). Learn to [set up a shared dependency graph in your CI here](../advanced-usage/accelerate-tests-in-ci.md).

---

## YOURBASE_AWS_ACCESS_KEY_ID
`Type`: `AWS access key ID`
`Default`: `(unset)`

When set alongside [YOURBASE_AWS_SECRET_ACCESS_KEY](#yourbase_aws_secret_access_key), it forces YourBase Test Acceleration to use these credentials over system credentials when interacting with AWS.

These environment variables are recommended for use if your AWS system credentials are fudged for the sake of your tests. Learn to use this [YourBase Test Acceleration specific AWS environment variables here](../advanced-usage/accelerate-tests-in-ci.md#step-1-set-up-shared-dependency-graph).

---

## YOURBASE_AWS_SECRET_ACCESS_KEY
`Type`: `AWS secret access key`
`Default`: `(unset)`

When set alongside [YOURBASE_AWS_ACCESS_KEY_ID](#yourbase_aws_access_key_id), it forces YourBase Test Acceleration to use these credentials over AWS system credentials when interacting with AWS.

These environment variables are recommended for use if your system credentials are mocked for the sake of your tests. Learn to use this [YourBase Test Acceleration specific AWS environment variables here](../advanced-usage/accelerate-tests-in-ci.md#step-1-set-up-shared-dependency-graph)

---

## YOURBASE_DEBUG
`Type`: `bool-ish (0, false, off, 1, true, on)`
`Default`: `off`

When on, YourBase Test Acceleration will report significantly more internal information to stdout, stderr, and XDG (see the file returned by this expression):

```sh
echo ${XDG_STATE_HOME-~/.local/state}/yourbase/python.log
```

This setting is most beneficial when collaborating with the YourBase Test Acceleration team to debug issues.

---

## YOURBASE_DISABLE
`Type`: `bool-ish (0, false, off, 1, true, on)`
`Default`: `off`

When on, YourBase Test Acceleration will not load.

Enabling this setting and then manually attaching to a test framework using `yourbase.attach` produces undefined behavior.

---

## YOURBASE_IGNORE_LOCAL_CACHE
`Type`: `bool-ish (0, false, off, 1, true, on)`
`Default`: `off`

When on, YourBase Test Acceleration will not look in the local filesystem for a [dependency graph](../how-it-works.md#dependency-graph). 

If you’ve set [YOURBASE_REMOTE_CACHE](#yourbase_remote_cache) to a valid location, YourBase Test Acceleration will look up and [synchronize the dependency graph](../how-it-works.md#shared-dependency-graph) with the specified location. 

Else, if [YOURBASE_REMOTE_CACHE](#yourbase_remote_cache) is not set, YourBase Test Acceleration will do a cold run of your tests—it will run all the tests since it won’t be able to find any dependency graph. 

This setting can be used if the local cache is expected to be poisoned. For instance, this can happen if cohorting is used against a local cache.

---

## YOURBASE_SYNC_DIRTY
`Type`: `bool-ish (0, false, off, 1, true, on)`
`Default`: `off`

When on, YourBase Test Acceleration will [synchronize dependency graphs](../how-it-works.md#shared-dependency-graph) even if the Git working tree is dirty. 

This setting is not recommended for use when you run YourBase Test Acceleration locally, as it will poison the remote cache.

This setting is useful when you use YourBase Test Acceleration in CI. There it can help you overcome situations where you need your working tree to be dirty while building, and you know the dirtiness will not affect the dependency graph.
If that situation does not apply to you, do not enable this setting.

---

## YOURBASE_TELEMETRY
`Type`: `bool-ish (0, false, off, 1, true, on)`
`Default`: `on`

When on, YourBase Test Acceleration will send anonymized telemetry data to `api.yourbase.io` over HTTPS for the purposes of improving the product.
Note that, telemetry data never includes your code.

Turn it off, if you want to opt out of sending usage statistics and error reports to YourBase Test Acceleration. 

[Learn more about telemetry data here](../security.md#telemetry).

---

## YOURBASE_TIMID
`Type`: `bool-ish (0, false, off, 1, true, on)`
`Default`: `off`

When on, YourBase Test Acceleration will use a slower tracing algorithm that is less prone to conflicts with other packages than the default. 

We recommend reaching out to support@yourbase.io if you are encountering this scenario, and only enabling this if you experience issues with the default algorithm. 

---

## YOURBASE_WORKDIR
`Type`: `absolute or relative path`
`Default`: `.`

This is the directory that YourBase Test Acceleration treats as the project directory. Only the code in this directory, or one of its descendants, will be traced. 

You usually do not need to change this, as it’s mainly for debugging purposes.
