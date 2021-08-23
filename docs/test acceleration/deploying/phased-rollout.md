---
layout: default
title: Phased rollout
nav_order: 1
parent: Deploying
grand_parent: Test acceleration
permalink: /test-acceleration/rollout
---

# Rollout recommendation
{:.no_toc}

<details open markdown="block">
  <summary>
    Table of contents
  </summary>
  {: .text-delta }
1. TOC
{:toc}
</details>

To safely accelerate tests on production, we recommend that you first run YourBase Test Acceleration in its [Observation Mode](advanced-usage/verify-results.md), and manually [verify the output logged](advanced-usage/verify-results.md#verification-steps).

We propose the rollout to be done in the following 3 phases:

## Phase 1: Test locally
This phase will synchronize your code’s dependency graph on your local machine storage. Hence, after completing this phase, you’ll see the tests on your branch running faster only on your local machine.

In your development branch:

1. [Install YourBase Test Acceleration](install.md) locally. 
2. Run YourBase Test Acceleration in [Observation Mode](advanced-usage/verify-results.md).
3. Ensure that YourBase Test Acceleration [accelerates your tests correctly](advanced-usage/verify-results.md#verification-steps), or resolve any issues that arise.
4. [Disable Observation Mode](reference/configuration-options.md#yourbase_observation_mode).
5. Run your tests. 

---

## Phase 2: Test in CI
This phase will synchronize your code’s dependency graph on remote storage for use by CI. Hence, after completing this phase, you’ll see the tests on your branch running faster on your CI as well.

### Step 1. Configure Remote Cache 
{: .no_toc }

In your CI environment:
   1. Set up a [Shared Dependency Graph for use in CI](advanced-usage/accelerate-tests-in-ci.md).
   
      - Tip: It may be easier to set up and debug the remote cache from your local environment before configuring it in the CI.
   
   2. Set up the following configuration variables for your CI environment:
      - [YOURBASE_LICENSE_KEY](reference/configuration-options.md#yourbase_license_key)
      - [YOURBASE_ACCEPT_TOS](reference/configuration-options.md#yourbase_accept_tos)

### Step 2. Install in test branch
{: .no_toc }

In your test branch, do the following:
   1. Install YourBase Test Acceleration to your project via `requirements.txt` or whatever other mechanism you use to install your dependencies in your CI environment.
   2. [Enable Observation Mode](reference/configuration-options.md#yourbase_observation_mode).
   3. Run your tests as usual.
   4. Ensure that [YourBase Test Acceleration accelerates these tests correctly](advanced-usage/verify-results.md#verification-steps), or resolve any issues that arise.
   5. [Disable Observation Mode](reference/configuration-options.md#yourbase_observation_mode).
   6. Run your tests as usual.


### Step 3: Install in main branch
{: .no_toc }

In your main branch, before enabling Yourbase Test Acceleration for full production, we recommend executing the following steps for a subset of builds, for example in a canary environment or as a percentage experiment:

   1. [Install YourBase Test Acceleration](../install.md) to your project via `requirements.txt` or whatever other mechanism you use to install your dependencies in your CI environment.
   2. Set `YOURBASE_DISABLE=true` and ensure CI continues to run as expected. 
   3. [Enable Observation Mode](reference/configuration-options.md#yourbase_observation_mode).
   4. Set `YOURBASE_DISABLE=false`. 
   5. Run your tests as usual. 
   6. Ensure that [YourBase Test Acceleration accelerates these tests correctly](advanced-usage/verify-results.md#verification-steps), or resolve any issues that arise. 
   7. [Disable Observation Mode](reference/configuration-options.md#yourbase_observation_mode).
   8. Run your tests as usual.


_Note: Once YourBase Test Acceleration is launched to production, we recommend continuing to run the full test suite occasionally, for example, in advance of major releases._

---

## Phase 3: Enable local test acceleration across your development team
After this phase, you’ll see tests run faster for developers across the team because of them sharing their dependency graph.

Do the following starting with a group of beta users, slowly rolling out to the entire team: 
1. Set up the [shared Dependency Graph for use by your local machine](advanced-usage/accelerate-tests-across-developers.md).
2. [Install YourBase Test Acceleration](../install.md) to your project via `requirements.txt` or whatever other mechanism you use to install your dependencies in your local environment
3. Set up the following configuration variables for your local environment: 
   - [YOURBASE_LICENSE_KEY](reference/configuration-options.md#yourbase_license_key)
   - [YOURBASE_ACCEPT_TOS](reference/configuration-options.md#yourbase_accept_tos)
