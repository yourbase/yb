---
layout: default
title: System requirements
nav_order: 1
parent: Test acceleration
has_children: false
permalink: /test-acceleration/system-requirements
---

# System requirements

YourBase Test Acceleration library works seamlessly with the CI system of your choice and is agnostic to where you host your application—be it cloud, on-premise, or offline.

### Supported version control tools
The library can currently accelerate tests for codebases that are version-controlled using Git. 

### Supported platforms
- Linux
- MacOS: All except those with the M1 chip

### Supported languages & testing frameworks
- Python 2.7+ and Python 3.5+ that use the following testing frameworks:
  - pytest
  - unittest

  Note: Any web frameworks, such as Django, that are built atop these, are also supported.

### Compatible code coverage tools
The library supports integration with the following code coverage reporting tools without changing your workflow, for example:
- <a href="https://coverage.readthedocs.io/">Coverage</a> [Link to Advanced usage section that guides on how to configure it for proper coverage reports] 
- <a href="https://pypi.org/project/pytest-cov/">pytest-cov</a>
- <a href="https://www.sonarqube.org/">SonarQube</a>
- <a href="https://about.codecov.io/">Codecov</a> 

Reach out to support@yourbase.io if you have questions about support for your favorite coverage tool.