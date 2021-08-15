---
layout: default
title: Force run specific tests
nav_order: 2
parent: Advanced usage
grand_parent: Introduction
permalink: /test-acceleration/advanced-usage/force-run-specific-tests
---

# Force run specific tests

{: .no_toc }

This feature is supported in the following testing frameworks:
- pytest
{:toc}
- unittest

## pytest
You can tell YourBase Test Acceleration to never skip a specific test using decorators.

```
import pytest

@pytest.mark.do_not_accelerate
def test_function():
   # ...
```

The decorator @pytest.mark.do_not_accelerate ensures that the test test_function() is never skipped by YourBase Test Acceleration, even where there are no code changes in its dependencies.


## unittest
You can tell YourBase Test Acceleration to never skip specific tests using decorators.

```import yourbase.plugins.unittest as yourbase

@yourbase.do_not_accelerate
class TestClass(unittest.TestCase):
   def test_function():
      # ...
```