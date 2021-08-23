---
layout: default
title: Pytest
nav_order: 1
parent: Getting started
grand_parent: Test acceleration
permalink: /test-acceleration/getting-started/pytest
---

# Try in pytest
{:.no_toc}

This page will walk you through how to run YourBase Test Acceleration with the [pytest testing framework](https://docs.pytest.org/en/6.2.x/). The quickest way to get started with it is to use it in the sample project provided.

<details open markdown="block">
  <summary>
    Table of contents
  </summary>
  {: .text-delta }
1. TOC
{:toc}
</details>

---

## Introduction
YourBase Test Acceleration’s pytest hooks ensure that, by default:
- It runs automatically on every invocation of pytest.
- It runs without any additional configuration.
- It runs irrespective of how you invoke the tests to run—be it via Makefile, Docker, or anything else—without any other setup.

---

## Prerequisites
Make sure that, on your machine or on your virtual environment:
- Tests are running successfully with [pytest](https://docs.pytest.org/en/6.2.x/) before installing YourBase Test Acceleration.
- [YourBase Test Acceleration is installed](../install.md).
- [Git is installed](https://git-scm.com/book/en/v2/Getting-Started-Installing-Git).

---

## Example usage in a sample project

### Step 1: Set up the specified sample project on your machine as shown below:
{:.no_toc}

Open a new shell prompt, and checkout this sample project from git

```sh
git clone https://github.com/Unleash/unleash-client-python
```

From your shell prompt, navigate to the directory where you checked out the project

```sh
cd unleash-client-python
```
 
Install dependencies 

```sh
pip install -r requirements.txt
```

### Step 2: Execute a command to run all the tests of the project
{:.no_toc}

```sh
pytest tests
```

Let’s look at the output.

All messages that are prefixed with `[YB]` are logged by the YourBase Test Acceleration library. Note, that one of these messages says “No function-level dependency graph found; all tests will be run and traced.”—this is a cold run. During a cold run, YourBase Test Acceleration will build a dependency graph based on the tests that are executed. This dependency graph contains relationships between individual tests and the code they call.

### Step 3: Without making any code change, execute the command to run all the tests again
{:.no_toc}

```sh
pytest tests
```

Let’s look at the messages logged by YourBase Test Acceleration. One of the messages says: `[YB]` No code changed. Running only new tests, if any.”

Since you ran the tests without changing any code, YourBase Test Acceleration skipped all the tests to finish the run much more quickly than the last time—compare the time taken by both runs.

### Step 4: Make a code change and execute the command to run all tests again as shown below:
{:.no_toc}

Open `tests/unit_tests/test_features.py` in your text editor

```sh
vim tests/unit_tests/test_features.py
```

Add the following print statement in the beginning of the method `test_create_feature_true`

```python
print(“Checking YourBase Test Acceleration after a code-change...”)
```

Run the tests again using:

```sh
pytest tests
```

Let’s look at the output again. 

If you modified the same function as above then your output will closely match the output above. 

Look for the logs traced by YourBase Test Acceleration. You’ll see a message telling you how many functions have been altered and how many tests were affected. If you’re following the steps as is, you should see:

```sh
[YB] 1 function differs from the dependency graph
[YB] ~ tests/unit_tests/test_features.py#test_create_feature_true
[YB] Function-level dependency graph found 1 test affected
```

You can see form the logs that the existing dependency graph [link to How it works → Dependency graph section] was used to decide that only one test was affected by your code-change and that test was the only one that was run, while the remaining tests were skipped.

---

## Conclusion
{:.no_toc}

You just ran YourBase Test Acceleration on a project that uses the pytest testing framework. Now, you’re ready to use it in your own project. 

_Note that if your tests are going to take a while to run, you can run just a subset of your tests. Running a subset of tests will create a dependency graph just for those tests, so you can see YourBase Test Acceleration in action more quickly._