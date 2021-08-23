---
layout: default
title: Integrate code coverage tools
nav_order: 3
parent: Advanced usage
grand_parent: Test acceleration
permalink: /test-acceleration/advanced-usage/integrate-code-coverage-tools
---

# Integrate code coverage tools
{: .no_toc }

Since YourBase Test Acceleration is designed to avoid test runs that do not need to be executed, the percentage covered as reported by your code coverage tool will likely be lower than what is actually covered as a result. 

This decrease in the coverage reported by your tool will affect you if your CI is configured to fail a build on the basis of code coverage percentage.

The following sections lists ways to circumvent this problem.

## Configure your code coverage tool

You can circumvent this by making configuration changes in your code coverage tool, so that YourBase Test Acceleration can fill missing coverage data from previous runs. 
However, this feature of YourBase Test Acceleration works only on specific versions of the supported code coverage tools.  The following sections provide details on this.



---

## Configure your builds

YourBase Test Acceleration supports the following code coverage tools.
- TOC
{:toc}

## Coverage
YourBase Test Acceleration is compatible with all versions of Coverage out-of-the-box. To ensure that coverage reports account for skipped tests as well:
1. Use Coverage 5.5+ 
2. Set the following in your `.coveragerc` file:
```python
[run]
relative_files = true
```

Note: Prior to Coverage 5.5, coverage reports will only include the tests run by YourBase Test Acceleration. Tests that were skipped by YourBase Test Acceleration will be omitted from coverage reporting, thereby decreasing your coverage percentage.
