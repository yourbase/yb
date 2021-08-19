---
layout: default
title: Deploying
nav_order: 4
parent: Test acceleration
permalink: /test-acceleration/deploying
---

# Deploying

{: .no_toc }

This section contains guides to deploy the YourBase Test Acceleration library. 

## Table of contents

{: .no_toc }

1. TOC 
{ :toc }

## Rollout recommendation
To safely accelerate tests on production, we recommend that you first run YourBase Test Acceleration in its Observation Mode [link to Advanced usage → Observation mode], and manually verify the output logged [link to Advanced usage → Observation mode → Checking output].

We propose the rollout phases to look like the following:

### Phase 1: Test locally: On your development branch

{: .no_toc }

This phase will synchronize your code’s dependency graph on your local machine storage. Hence, after completing this phase, you’ll see the tests on your branch running faster only on your local machine.

1. Install YourBase Test Acceleration locally [link to Installation]
2. Run YourBase Test Acceleration in Observation Mode [link to Advanced usage → Observation mode]
3. Ensure that YourBase Test Acceleration accelerates your tests correctly [link to Advanced usage → Observation mode → Checking output], or resolve any issues that arise
4. Disable Observation Mode [link to Reference → Configuration → Observation mode]
5. Run YourBase Test Acceleration

### Phase 2: Test in CI

This phase will synchronize your code’s dependency graph on remote storage for use by CI. Hence, after completing this phase, you’ll see the tests on your branch running faster on your CI as well.

In your CI environment:

1. Set up the Shared Dependency Graph for use by your CI [link to Advanced usage → Accelerate Using Shared Dependency Graph] 
2. Install YourBase Test Acceleration to your project via requirements.txt or whatever other mechanism you use to install your dependencies in your CI environment
3. Enable Observation Mode [link to Advanced usage → Observation mode]
4. Set up the following configuration variables for your CI environment:
    - YOURBASE_LICENSE_KEY [link to configuration options section]
    - YOURBASE_ACCEPT_TOS [link to configuration options section]
5. Run YourBase Test Acceleration for a subset of tests
6. Ensure that YourBase Test Acceleration accelerates these tests correctly [link to Advanced usage → Observation mode → Checking output], or resolve any issues that arise
7. Roll out to the remainder of tests in increments, resolving any potential issues that arise
8. Disable Observation Mode [link to Reference → Configuration → Observation mode]
9. Run YourBase Test Acceleration for all your tests in CI

### Phase 3: Enable local test acceleration across your development team, starting with a group of beta users. 

After this phase, you’ll see tests run faster for developers across the team because of them sharing their dependency graph.

1. Set up the Shared Dependency Graph for use by your local machine [link to Advanced usage → Accelerate Using Shared Dependency Graph] 
2. Install YourBase Test Acceleration to your project via requirements.txt or whatever other mechanism you use to install your dependencies in your local environment
3. Set up the following configuration variables for your local environment: 
   - YOURBASE_LICENSE_KEY [link to configuration options section]
   - YOURBASE_ACCEPT_TOS` [link to configuration options section]

---

## Logs
Our standard logs are all prefixed with [YB]. We print minimal logs unless explicitly requested using debug mode [Link to References → Configuration → YOURBASE_DEBUG].

To obtain more detailed debugging information, set the `YOURBASE_DEBUG` environment variable [link to Configuration Options section].

---

## Disable YourBase Test Acceleration
If for any reason, you need to disable YourBase Test Acceleration, you can simply set the environment variable `YOURBASE_DISABLE` to true:

```bash
export YOURBASE_DISABLE=true
```

To uninstall the package, see uninstall instructions [Link to Uninstall]