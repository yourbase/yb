---
layout: default
title: Accelerate tests in CI
nav_order: 4
parent: Advanced usage
grand_parent: Test acceleration
permalink: /test-acceleration/advanced-usage/accelerate-tests-in-ci
---

# Integrate with CI
YourBase Test Acceleration uses a [dependency graph](../how-it-works.md#dependency-graph) to accelerate your tests. When you run tests on your local machine, with YourBase Test Acceleration enabled, by default, your dependency graph is stored locally—thus provides only for local acceleration.

But the true power of YourBase Test Acceleration comes in when tests can be accelerated in your CI environment. This requires the [dependency graph](../how-it-works.md#dependency-graph) to be accessible by your CI environment—referred to as [Shared Dependency Graph](../how-it-works.md#shared-dependency-graph) from now on.

The following section guides you to accelerate tests in CI.

## Steps to integrate with CI

### Step 1: Set up shared dependency graph
YourBase Test Acceleration currently supports storing [shared dependency graphs](../how-it-works.md#shared-dependency-graph) only in AWS S3 buckets. The following sections help you set up your project to use a shared dependency graph in your CI environment.

1. Set YOURBASE_REMOTE_CACHE
Set [YOURBASE_REMOTE_CACHE](../reference/configuration-options.md#yourbase_remote_cache)  in your environment to a valid S3 bucket location.

    ```sh
    YOURBASE_REMOTE_CACHE=s3://<bucketname>[/key/prefix]
    
    # where <bucketname> is an S3 bucket that your machine(s) has Get/Put/List access to.
    ```

2. Set the AWS credentials to be used by YourBase Test Acceleration.

   By default, YourBase Test Acceleration uses the system AWS credentials as specified in the environment variables: [AWS_ACCESS_KEY_ID](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html#envvars-list) and [AWS_SECRET_ACCESS_KEY](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html#envvars-list). 
   
   Alternately, you can specify different credentials to be used exclusively by YourBase Test Acceleration by setting the following environment variables:
   
   ```sh
   export YOURBASE_AWS_ACCESS_KEY_ID=<key>
   export YOURBASE_AWS_SECRET_ACCESS_KEY=<key>
   ```
       
   _Note: If YourBase Test Acceleration specific environment variables - [YOURBASE_AWS_ACCESS_KEY_ID](../reference/configuration-options.md#yourbase_aws_access_key_id) and [YOURBASE_AWS_SECRET_ACCESS_KEY](../reference/configuration-options.md#yourbase_aws_secret_access_key) are set, YourBase Test Acceleration uses them instead of the system credentials._

### Step 2: Install YourBase Test Acceleration in your CI environment
Add YourBase Test Acceleration to your project via `requirements.txt` or whatever other mechanism you use to install your dependencies in your CI environment.

### Step 3: Run tests
Run tests as usual.

## Conclusion
The above steps will set up YourBase Test Acceleration to synchronize dependency graphs against the specified storage location when your tests run on your CI environment.

_Note:_
_A dependency graph is synchronized with the specified remote storage location only when:_
- _Only when it's generated from a successful build, and_
- _Only if it's created from a clean working tree, and_
- _Only if it's created from committed code changes. A dependency graph that is created from uncommitted code changes is stored only locally, i.e., it can’t be synchronized against a remote location._