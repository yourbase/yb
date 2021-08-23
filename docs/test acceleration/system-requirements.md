---
layout: default
title: System requirements
nav_order: 1
parent: Test acceleration
has_children: false
permalink: /test-acceleration/system-requirements
---

# System requirements

YourBase Test Acceleration works seamlessly with the CI system of your choice and is agnostic to where you host your application—be it cloud, on-premise, or offline.

---

## Supported version control tools
The library can accelerate tests only for codebases that are version-controlled using [Git](https://git-scm.com/). 

## Supported platforms
- Linux
- MacOS: All except those with the M1 chip

## Supported languages & testing frameworks
- Python 2.7+ and Python 3.5+ that use the following testing frameworks:
  - [pytest](https://docs.pytest.org/en/6.2.x/)
  - [unittest](https://docs.python.org/3/library/unittest.html)

_Note: Any web frameworks, such as [Django](https://www.djangoproject.com/), that are built atop the above testing frameworks, are also supported._

## Compatible code coverage tools
The library supports integration with the following code coverage reporting tools without changing your workflow, for example:
- [Coverage](https://coverage.readthedocs.io/en/coverage-5.5/)
- [pytest-cov](https://pypi.org/project/pytest-cov/)
- [SonarQube](https://www.sonarqube.org/)
- [Codecov](https://about.codecov.io/)

Reach out to support@yourbase.io if you have questions about support for your favorite coverage tool.