---
layout: default
title: Integrate code coverage tools
nav_order: 3
parent: Advanced usage
grand_parent: Test acceleration
permalink: /test-acceleration/integrate-code-coverage-tools
---

# Integrate code coverage tools
{: .no_toc }

YourBase Test Acceleration supports integration with the following code coverage reporting tools without changing your workflow, for example:
- [Coverage](https://coverage.readthedocs.io/en/coverage-5.5/)
- [pytest-cov](https://pypi.org/project/pytest-cov/)
- [SonarQube](https://www.sonarqube.org/)
- [Codecov](https://about.codecov.io/)

However, since YourBase Test Acceleration is designed to avoid test runs that do not need to be executed, the test coverage percentage as reported by your code coverage tool will be lower than what is actually covered. 

This may cause your CI builds to fail if they're configured to pass only when the test coverage is above a specified threshold. The following sections lists ways in which you can circumvent this problem.

---

## Option 1: Configure code coverage tool to fill missing coverage data

This feature allows you to configure your code coverage tool such that YourBase Test Acceleration can fill missing coverage data from previous runs.

The feature is available only on the following code coverage tools:

### 1. Coverage 5.5+

To ensure that coverage reports account for skipped tests as well:
1. Use Coverage 5.5+
2. Set the following in your `.coveragerc` file:

```python
[run]
relative_files = true
```

_Note: Prior to Coverage 5.5, coverage reports will only include the tests run by YourBase Test Acceleration. Tests that were skipped by YourBase Test Acceleration will be omitted from coverage reporting, thereby decreasing your coverage percentage._

You can configure your code coverage tool such that YourBase Test Acceleration can fill missing coverage data from previous runs.

---

## Option 2: Reconfigure build-failure threshold

On your CI, you can reduce the build-pass threshold for the percentage of tests covered, so that you can use YourBase Test Acceleration seamlessly.

### Note: We highly recommend that you make use of [Option 1](#option-1-configure-code-coverage-tool-to-fill-missing-coverage-data) instead of [Option 2](#option-2-reconfigure-build-failure-threshold). 