---
layout: default
title: Verify results
nav_order: 3
parent: Advanced usage
grand_parent: Introduction
permalink: /test-acceleration/advanced-usage/verify-results
---

# Verify results
YourBase Test Acceleration test selection includes an "Observation Mode" which allows you to test-drive its test selection without actually skipping any tests. 

When “Observation Mode” is set, in addition to running all the tests, YourBase Test Acceleration will also monitor if its test selection would have skipped any tests that would have failed. As its output, it will print out the names of any tests that would have been incorrectly skipped. 

To verify results follow the below steps: 

1. Enable Observation Mode
Before running your tests, set `YOURBASE_OBSERVATION_MODE=true` in your environment.

2. Check the output of running YourBase Test Acceleration in Observation mode
   - If YourBase Test Acceleration is accelerating tests correctly, it will log the total amount of time that could have been saved, to stdout or your log file.
   - Else if YourBase Test Acceleration is accelerating tests incorrectly—by skipping a test(s) that would have failed, YourBase Test Acceleration will complain about this loudly and output the details of the errors in your log file or your shell-prompt. Note that if this happens, it means that there is a bug in YourBase Test Acceleration’s tracing or acceleration. Please also report these to bugs@yourbase.io.