---
layout: default
title: Unittest
nav_order: 2
parent: Getting started
grand_parent: Test acceleration
permalink: /test-acceleration/getting-started/unittest
---

# Try in unittest
{:.no_toc}

This section notes how to run YourBase Test Acceleration in your own project that uses the [unittest testing framework](https://docs.python.org/3/library/unittest.html). For a deeper walkthrough of the library using a sample project, [see here](pytest.md).

<details open markdown="block">
  <summary>
    Table of contents
  </summary>
  {: .text-delta }
1. TOC
{:toc}
</details>

---

## Prerequisites
Make sure that, on your machine or on your virtual environment:
- Tests are running successfully with [unittest](https://docs.python.org/3/library/unittest.html) before installing YourBase Test Acceleration.
- [YourBase Test Acceleration is installed](../install.md).
- [Git is installed](https://git-scm.com/book/en/v2/Getting-Started-Installing-Git).

---

## Example usage in your project

### Step 1: Attach yourbase to unittest.
{:.no_toc}

In your `tests/__init__.py file`, copy-paste the following:

```python
import unittest
import yourbase

yourbase.attach(unittest)
```

### Step 2: Run your project’s tests
{:.no_toc}

For example, to run all the tests for the project, use:
```sh
python -m unittest discover
```

_Note: If your tests are going to take a while to run, you can run just a subset of your tests. Running a subset of tests will create a dependency graph just for those tests, so you can see YourBase Test Acceleration in action more quickly._


### Step 3: Re-run your tests
{:.no_toc}

Without making a code change, run your tests again using the same command as in [step #2](#step-2-run-your-projects-tests). You'll see that no tests are run. Here, since no code was changed, YourBase Test acceleration ensures that no tests are run.

### Step 4: Make a code change in any one of your tests
{:.no_toc}

You can simply add a print statement like below:

```python
print(“Checking YourBase Test Acceleration after a code-change...”)
``` 


### Step 5: Run your tests again
{:.no_toc}

Use the same command as in [step #2](#step-2-run-your-projects-tests) to run your tests. Here, YourBase Test acceleration ensures that only the one test that you modified is run.
