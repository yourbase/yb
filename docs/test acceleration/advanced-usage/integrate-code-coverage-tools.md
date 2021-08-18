---
layout: default
title: Verify results
nav_order: 3
parent: Advanced usage
grand_parent: Test acceleration
permalink: /test-acceleration/advanced-usage/integrate-code-coverage-tools
---

# Integrate code coverage tools

{: .no_toc }

YourBase Test Acceleration supports the following code coverage tools.
- Coverage
{:toc}

Note that since YourBase Test Acceleration is designed to avoid test runs that do not need to be executed, the percentage covered may appear lower than what is actually covered as a result. You can circumvent this by making configuration changes in your code coverage tool, so that YourBase Test Acceleration can fill missing coverage data from previous runs. 
However, this feature of YourBase Test Acceleration works only on specific versions of code coverage tools.  The following sections provide details on this.

Note: If your CI is configured to fail a build based on the percentage covered, you may need to reconfigure it.

## Coverage
YourBase Test Acceleration is compatible with all versions of Coverage out-of-the-box. To ensure that coverage reports account for skipped tests as well:
1. Use Coverage 5.5+ 
2. Set the following in your `.coveragerc` file:
```
[run]
relative_files = true
```

Note: Prior to Coverage 5.5, coverage reports will only include the tests run by YourBase Test Acceleration. Tests that were skipped by YourBase Test Acceleration will be omitted from coverage reporting, thereby decreasing your coverage percentage.
