---
layout: default
title: Install
nav_order: 2
parent: Test acceleration
has_children: false
permalink: /test-acceleration/install
---

# Install
{:.no_toc}

## Table of contents
{:.no_toc}

1. TOC 
{:toc}

## Prerequisites
- YourBase Test Acceleration library supports your technical stack and infrastructure [link to the System Requirements section].

## Installation

YourBase Test Acceleration library can be installed with either pip or poetry package managers.

### Using pip
If you use pip, you can install YourBase Test Acceleration with:

```pip install yourbase```

To check if YourBase Test Acceleration is installed, run the following command in a shell prompt:

```pip show yourbase```

If the installation is successful, you should see an output similar to what’s shown below, having a Version key that displays a valid version of YourBase Test Acceleration.
 
```bash
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


### Using poetry
If you use poetry, you can install YourBase Test Acceleration with:

```bash
poetry add yourbase
```

To check if YourBase Test Acceleration is installed, run the following command in a shell prompt:

```bash
poetry show yourbase
```

If the installation is successful, you should see an output similar to what’s shown below, having a Version key that displays a valid version of YourBase Test Acceleration.

```bash
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

## Installation recommendations

### Use virtual environment

{: .no_toc }

We recommend that you install YourBase Test Acceleration in a clean virtual environment. See here to learn how to set up a Python virtual environment.

## Uninstall
If for any reason, you want to completely remove YourBase Test Acceleration, simply uninstall the package:

Using pip: 

```pip uninstall yourbase```

Or, using poetry:

```poetry remove yourbase```



