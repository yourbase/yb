---
layout: default
title: Verify test selection
nav_order: 2
parent: Advanced usage
grand_parent: Test acceleration
permalink: /test-acceleration/verify
---

# Verify test selection
{: .no_toc }
To test-drive YourBase Test Acceleration’s test selection without actually skipping any tests, you can run it under the “Observation Mode”.

When “Observation Mode” is set, in addition to running all the tests, YourBase Test Acceleration will also monitor if its test selection would have skipped any tests that would have failed. As its output, it will print out the names of any tests that would have been incorrectly skipped. 

---

Table of contents
{: .text-delta }
1. TOC
{:toc}

---

## Verification steps
You can follow the below steps to verify: 

1. [Install YourBase Test Acceleration](../install.md).
2. Enable [Observation Mode](../reference/configuration-options.md/#yourbase_observation_mode) in your environment. For example, if you use a bash shell to set environment variables, you can set is as follows:
   ```sh
   export YOURBASE_OBSERVATION_MODE=true
   ```
   
   Else if you use an environment specific configuration file, set [YOURBASE_OBSERVATION_MODE](../reference/configuration-options.md/#yourbase_observation_mode) in that file.

3. Run ALL your tests as usual.
4. Check your logs manually:
   - If YourBase Test Acceleration is accelerating tests correctly, it will log the total amount of time that could have been saved, to stdout or your log file.
   - Else if, it's accelerating tests incorrectly, i.e. it's skipping one or more tests that would have failed, then it'll complain about this loudly by outputting the details of the errors in your log file or your shell-prompt. _Note that if this happens, it means that there's a bug in YourBase Test Acceleration’s tracing or acceleration._ Please report these to bugs@yourbase.io.
5. Ensure that you disable [YOURBASE_OBSERVATION_MODE](../reference/configuration-options.md/#yourbase_observation_mode) only after YourBase Test Acceleration accelerates tests correctly.

---

## Recommendation
We strongly recommend that you run YourBase Test Acceleration in the "Observation "Mode" every time before rolling out code-changes to your production.