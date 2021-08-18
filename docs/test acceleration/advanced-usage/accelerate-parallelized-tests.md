---
layout: default
title: Accelerate parallelized tests
nav_order: 6
parent: Advanced usage
grand_parent: Test acceleration
permalink: /test-acceleration/advanced-usage/accelerate-parallelized-tests
---

# Accelerate parallelized tests

YourBase Test Acceleration integrates smoothly with your existing parallelization setup in your CI. On top of your parallelization setup, YourBase Test Acceleration will:
1. Split your tests across shards
2. Accelerate tests running on each shard

## Prerequisites:
- You’re already successfully running tests across multiple shards. 
- You've removed any test splitting solution that you may already have in-place. This is required because YourBase Test Acceleration is effective only when the test-splitting solution is sticky—tests don’t get reshuffled across shards when other tests are added or removed. However, since most test-splitting solutions aren’t sticky, YourBase Test Acceleration provides its own built-in sticky test-splitting solution, which you will learn to use in the following section. 

## Steps to accelerate already-parallelized tests:
1. Remove your existing test splitting tools, if any. It should appear as if each shard will run the entire test suite.
2. Set YOURBASE_COHORT_COUNT [link to References → Configuration → YOURBASE_COHORT_COUNT] to your number of cohorts / shards
3. Set YOURBASE_ACTIVE_COHORT [link to References → Configuration → YOURBASE_ACTIVE_COHORT] to the ID of the current cohort / shard – starting from 1. For example, if you have 5 shards and are running the 4th shard, you’d set:
   
    ```
    YOURBASE_COHORT_COUNT = 5
    YOURBASE_ACTIVE_COHORT = 4
    ```

4. Run your tests as usual.

YourBase Test Acceleration will now just-in-time select and run only the tests that are in the current cohort. This selection will be consistent between runs—given the same cohort ID and total cohorts, a test will always be selected to be run on the same shard for life.

## Pro tip
As an added benefit of YouBase’s sticky splitting of tests, you can schedule a build for each of the sharded pools of tests—to run sharded builds. This works smoothly because YourBase Test Acceleration will merge graphs from multiple shards, for the same commit, which can be used in future builds.
