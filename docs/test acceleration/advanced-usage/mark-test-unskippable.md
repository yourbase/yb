---
layout: default
title: Mark a test as unskippable
nav_order: 1
parent: Advanced usage
grand_parent: Test acceleration
permalink: /test-acceleration/mark-test-unskippable
---

# Mark a test as unskippable
{: .no_toc }
This feature is supported in the following testing frameworks:
- TOC
{:toc}

## pytest
If you're using [pytest](https://docs.pytest.org/en/6.2.x/), you can tell YourBase Test Acceleration to never skip a specific test using decorators as shown in the below example:

```python
import pytest

# decorator to never skip this test
@pytest.mark.do_not_accelerate
def test_function():
   # ...
```

The decorator `@pytest.mark.do_not_accelerate` ensures that the `test_function()` is never skipped, even where there are no code changes in its dependencies.


## unittest
If you're using [unittest](https://docs.python.org/3/library/unittest.html), you can tell YourBase Test Acceleration to never skip specific tests using decorators as shown in the below example:

```python
import yourbase.plugins.unittest as yourbase

# decorator to never skip this test
@yourbase.do_not_accelerate
class TestClass(unittest.TestCase):
   def test_function():
      # ...
```