---
layout: default
title: Accelerate tests in CI
nav_order: 4
parent: Advanced usage
grand_parent: Test acceleration
permalink: /test-acceleration/advanced-usage/accelerate-tests-in-ci
---

# Accelerate tests in CI
YourBase Test Acceleration harnesses its dependency graph to accelerate your tests. When you run tests on your local machine, with YourBase Test Acceleration enabled, by default, your dependency graph is stored locally—thus providing only local acceleration.

But the true power of YourBase Test Acceleration comes in when tests can be accelerated in your CI environment. This requires the dependency graph to be accessible by your CI environment—referred to as the Shared Dependency Graph from now on.

The following steps guide you to accelerate tests in CI:

## Step 1: Set storage location for shared dependency graph
YourBase Test Acceleration currently supports storing shared graphs in AWS S3 buckets. The following sections help you set up a shared dependency graph for your project.

### Using S3 bucket
Within your project, set the following environment variable to your S3 bucket using the `s3://` syntax. 

#### 1. Set S3 Bucket name

```YOURBASE_REMOTE_CACHE=s3://<bucketname>[/key/prefix]```

where <bucketname> is an S3 bucket that your machine(s) has Get/Put/List access to.

#### 2. Set AWS credentials
You can configure YourBase Test Acceleration to use either the system AWS credentials, or you can also specify different credentials that are specific to YourBase Test Acceleration.

##### Option 1: Use the system AWS credentials
To use your systems’ AWS credentials, export the following in your environment:

```
export AWS_ACCESS_KEY_ID=<key>
export AWS_SECRET_ACCESS_KEY=<key>
```

##### Option 2: Use YourBase Test Acceleration-specific AWS credentials
If you want YourBase Test Acceleration to use different credentials (or if you're setting the system AWS credentials to mock values for your tests or CI), you can set these YourBase Test Acceleration-specific environment variables instead:

```
export YOURBASE_AWS_ACCESS_KEY_ID=<key>
export YOURBASE_AWS_SECRET_ACCESS_KEY=<key>
```

Note:

By default, YourBase Test Acceleration uses the system AWS credentials - `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`. 
However, if YourBase Test Acceleration specific credentials are set - `YOURBASE_AWS_ACCESS_KEY_ID` and `YOURBASE_AWS_SECRET_ACCESS_KEY`, YourBase Test Acceleration uses them instead of the system credentials.

## Step 2: Install YourBase Test Acceleration in your CI environment
Add yourbase to your project via requirements.txt or whatever other mechanism you use to install your dependencies in your CI environment.

## Step 3: Run tests
Run tests as usual.

The above steps will set up YourBase Test Acceleration to synchronize dependency graphs against the specified S3 location when your tests run on your CI environment.

---

## Points to note
- Your code will never be uploaded to the configured bucket. Only the dependency graphs are uploaded to the bucket. Neither your code nor your dependency graphs will touch YourBase Test Acceleration servers.
- A dependency graph that is generated from successful builds and using clean working trees will be synchronized to the specified S3 location. This graph is accessed for future test runs if a local cache is not present. 
- Dependency graphs run against uncommitted code changes will be stored only locally, i.e., they can’t be shared over S3.
- It is safe to share one bucket between multiple repositories, even if the repositories use different languages.
