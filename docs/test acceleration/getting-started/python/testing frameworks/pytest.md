---
layout: default
title: Pytest
nav_order: 1
parent: Python testing frameworks
grand_parent: Python testing frameworks
permalink: /test-acceleration/getting-started/python/pytest
---

# Try in pytest

This page will walk you through how to run YourBase Test Acceleration with the pytest testing framework. The quickest way to get started with it is to use it in the sample project provided.

YourBase Test Acceleration’s pytest hooks ensure that, by default:
- It runs automatically on every invocation of pytest
- It runs without any additional configuration
- It runs irrespective of how you invoke the tests to run—be it via Makefile, Docker, or anything else—without any other setup.

---

## Prerequisites
Make sure that, on your machine or on your virtual environment, you have:
- Tests running successfully with pytest before installing YourBase Test Acceleration
- YourBase Test Acceleration installed [link to the installation section of Python]
- You have git installed

---

## Example usage in a sample project

### Step 1: Set up the specified sample project on your machine as shown below:

Open a new shell prompt, and checkout this sample project from git

```git clone https://github.com/Unleash/unleash-client-python```

From your shell prompt, navigate to the directory where you checked out the project

```cd unleash-client-python```
 
Run its `requirements.txt` file to install the required packages. 

```pip install -r requirements.txt```

### Step 2: Execute a command to run all the tests of the project

```pytest tests```

Let’s look at the output.

All messages that are prefixed with “[YB]” are logged by the YourBase Test Acceleration library. Note, that one of these messages says “No function-level dependency graph found; all tests will be run and traced.”—this is a cold run. During a cold run, YourBase Test Acceleration will build a dependency graph based on the tests that are executed. This dependency graph contains relationships between individual tests and the code they call.

### Step 3: Without making any code change, execute the command to run all the tests again

```pytest tests```

Let’s look at the messages logged by YourBase Test Acceleration. One of the messages says: “[YB] No code changed. Running only new tests, if any.”

Since you ran the tests without changing any code, YourBase Test Acceleration skipped all the tests to finish the run much more quickly than the last time—compare the time taken by both runs.

### Step 4: Make a code change and execute the command to run all tests again as shown below:

Open ```tests/unit_tests/test_features.py``` in your text editor

```vim tests/unit_tests/test_features.py```

Add the following print statement in the beginning of the method ```test_create_feature_true```

```print(“Checking YourBase Test Acceleration after a code-change...”)```

Run the tests again using:

```pytest tests```

Let’s look at the output again. 

If you modified the same function as above then your output will closely match the output above. 

Look for the logs traced by YourBase Test Acceleration. You’ll see a message telling you how many functions have been altered and how many tests were affected. If you’re following the steps as is, you should see:

```
[YB] 1 function differs from the dependency graph
[YB] ~ tests/unit_tests/test_features.py#test_create_feature_true
[YB] Function-level dependency graph found 1 test affected
```

You can see form the logs that the existing dependency graph [link to How it works → Dependency graph section] was used to decide that only one test was affected by your code-change and that test was the only one that was run, while the remaining tests were skipped.

---

## Conclusion
You just ran YourBase Test Acceleration on a project that uses the pytest testing framework. And now, you’re ready to use YourBase Test Acceleration in your own project. 

Note that if your tests are going to take a while to run, you can run just a subset of your tests. Running a subset of tests will create a dependency graph just for those tests, so you can see YourBase Test Acceleration in action more quickly.