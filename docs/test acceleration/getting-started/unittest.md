---
layout: default
title: Unittest
nav_order: 2
parent: Getting started
grand_parent: Test acceleration
permalink: /test-acceleration/getting-started/python/unittest
---

# Try in unittest
This section notes how to run YourBase Test Acceleration in projects that use the unittest testing framework. For a deeper walkthrough with a sample open source project that uses pytest, see <link to pytest section>.

---

## Prerequisites
Make sure that, on your machine or on your virtual environment, you have:
- Tests running successfully with unittest before installing YourBase Test Acceleration
- YourBase Test Acceleration installed [link to the installation section of Python]
- You have git installed
 
---

## Example usage in your project

### Step 1: Attach yourbase to unittest.

For example, in your `tests/__init__.py file`, copy-paste the following:

```python
import unittest
import yourbase

yourbase.attach(unittest)
```

### Step 2: Run your project’s tests

For example, to run all the tests for the project, use:
```bash
python -m unittest discover
```

Note: If your tests are going to take a while to run, you can run just a subset of your tests. Running a subset of tests will create a dependency graph just for those tests, so you can see YourBase Test Acceleration in action more quickly.


###Step 3: Re-run your tests

Without making a code change, run your tests again using the same command as in step #2. Here, YourBase Test acceleration ensures that no tests are run since no code was changed.

### Step 4: Make a code change in any one of your tests. 

You can simply add a print statement like below:

```python
print(“Checking YourBase Test Acceleration after a code-change...”)
``` 


### Step 5: Run your tests again using the same command as in step #2. 

Here, YourBase Test acceleration ensures that only the one test that you modified is run.
