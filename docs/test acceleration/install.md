---
layout: default
title: Installation
nav_order: 2
parent: Test acceleration
has_children: false
permalink: /test-acceleration/install
---

# Install
{:.no_toc}

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
Make sure that the YourBase Test Acceleration library [supports your technical stack and infrastructure](system-requirements.md).

---

## Installation

YourBase Test Acceleration library can be installed with either pip or poetry package managers.


### Using pip
If you use [pip](https://pip.pypa.io/en/stable/), you can install YourBase Test Acceleration with:

```sh
pip install yourbase
```

To check if YourBase Test Acceleration is installed, run the following command in a shell prompt:

```sh
pip show yourbase
```

If the installation is successful, you should see an output similar to what’s shown below, having a Version key that displays a valid version of YourBase Test Acceleration.
 
```sh
Name: yourbase
Version: 5.2.4
Summary: Skip tests based on tracing data
Home-page: https://yourbase.io
Author: YourBase Test Acceleration
Author-email: python@yourbase.io
License: UNKNOWN
Location: /Path/Python/3.8/lib/python/site-packages
Requires: six, boto3, python-dateutil, coverage, pastel, requests
Required-by:
```

---

### Using poetry
If you use [poetry](https://python-poetry.org/docs/), you can install YourBase Test Acceleration with:

```sh
poetry add yourbase
```

To check if YourBase Test Acceleration is installed, run the following command in a shell prompt:

```sh
poetry show yourbase
```

If the installation is successful, you should see an output similar to what’s shown below, having a Version key that displays a valid version of YourBase Test Acceleration.

```sh
Name: yourbase
Version: 5.2.4
Summary: Skip tests based on tracing data
Home-page: https://yourbase.io
Author: YourBase Test Acceleration
Author-email: python@yourbase.io
License: UNKNOWN
Location: /Path/Python/3.8/lib/python/site-packages
Requires: six, boto3, python-dateutil, coverage, pastel, requests
Required-by:
```

---

## Recommendations

### Use virtual environment
{: .no_toc }

We recommend that you install YourBase Test Acceleration in a clean virtual environment. [Learn to set up a Python virtual environment here](https://docs.python.org/3/tutorial/venv.html).

---

## Uninstall
If for any reason, you want to completely remove YourBase Test Acceleration, simply uninstall the package:

Using [pip](https://pip.pypa.io/en/stable/): 

```sh
pip uninstall yourbase
```

Or, using [poetry](https://python-poetry.org/docs/):

```sh
poetry remove yourbase
```



