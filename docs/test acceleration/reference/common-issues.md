---
layout: default
title: Common issues
nav_order: 2
parent: Reference
grand_parent: Test acceleration
permalink: /test-acceleration/common-issues
---

# Common issues
{: .no_toc }

This is a list of commonly encountered problems, known issues, and their solutions.

## Table of contents
{: .no_toc .text-delta }

- TOC
{:toc}

## Reduced code coverage percentage
YourBase Test Acceleration is designed to avoid test runs that do not need to be executed. The percentage covered may appear lower than what is actually covered as a result. If your CI is configured to fail a build based on the percentage covered, you may need to reconfigure this feature. YourBase Test Acceleration has future plans to fill missing coverage data with data from previous runs. [link to code coverage]

## Unittest setUp or tearDown overrides lead to incorrect test skipping

If you are using unittest and define your own setUp/tearDown functions, be sure they call super before performing other actions:

```python
class MyTestClass:
   def setUp(self):
      super(self.__class__, self).setUp()
      # ...

   def tearDown(self):
      super(self.__class__, self).tearDown()
      # ...
```

If you are not defining your own setUp and tearDown functions, you do not need to do this.

## __Sqlite3 module not found
If you run into errors about the _sqlite3 module not being found, follow the below steps:

1. Install <a href="https://www.sqlite.org/quickstart.html">sqlite3</a>

2. Rebuild and reinstall the Python version you are using. If you use `pyenv`, this will look something like:

```bash
pyenv install --force <PYTHON_VERSION>
```

If the above step doesn't work, try:
```bash
PYTHON_CONFIGURE_OPTS="--enable-loadable-sqlite-extensions"
pyenv install --force <PYTHON_VERSION>
```

## Incompatibility with Apple machines having the M1 chip

YourBase Test Acceleration does not currently support Apple Machines running the M1 chip. Support is on our roadmap. If this is causing an issue for you, please reach out to us at hi@yourbase.io.

## Conflict with proxy objects

Python objects that opaquely wrap other objects by overriding Python builtins like __name__ and __class__ can cause tracing issues in YourBase Test Acceleration that may manifest as errors from within those proxy objects. If you experience these issues, you can set to use a slower tracing algorithm that should avoid these errors. 

```bash
export YOURBASE_TIMID=true
```

Tracing overhead is dramatically increased using this flag, so we don't recommend setting this if you are not experiencing issues.

## Conflict with plugin pytest-xdist

The YourBase Test Acceleration Test Selection and pytest-xdist [link] plugins have similar goals, reducing the overall test execution time of tests, but take different approaches to solving the problem. As such, there are conflicts when both plugins are enabled. When using the YourBase Test Acceleration Test Selection plugin, please uninstall pytest-xdist or execute pytest-xdist with `NUMCPUS=0`
