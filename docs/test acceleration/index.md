---
layout: default
title: Test acceleration
nav_order: 2
has_children: true
has_toc: false
permalink: /test-acceleration
---

# Introduction
{:.no_toc}
YourBase Test Acceleration is a library for shortening your test run-times by up to 90%. It hooks into your testing framework to intelligently select which tests should be run for a given code change.

---

## How does YourBase Test Acceleration reduce test times?
At the core of YourBase Test Acceleration is the YourBase Dependency Graph that maps which files and functions each of your tests depends on. Every time you run your tests, YourBase Test Acceleration library will load the optimal dependency graph to select and run only the tests that pertain to your code changes, and safely avoid running any unrelated tests. As a result, your builds execute only the optimal fraction of your total tests and finish much faster.

---

## Is it compatible with your tech-stack and infrastructure?
The library currently supports [testing frameworks for Python](system-requirements#supported-languages--testing-frameworks). Check the [complete list of system requirements here](system-requirements).

---

## Which type of tests does it accelerate?
The library supports unit tests and integration tests that call code from within the application. Since the library traces dependencies from within the test-runner process,  distributed dependencies like network or database calls are not fully traced and accelerated.

---

## Will it provide benefits at your scale? 
The library has the most benefits for use-cases where test runs are taking over 10 minutes. [Our customers have reduced their test run times by up to 90%](https://yourbase.io/case-studies/), for example, where one customer was able to skip over 99% of the 11,000+ tests in the suite using our library.

---

## Is it secure?
Yes. Under no circumstance do your code or your dependency graphs ever touch YourBase Test Acceleration servers. Only metadata about your usage of the library would ever be shared with YourBase Test Acceleration. [Learn more about security here](security.md).

---

## Is it stable?
The library is currently in Beta. While we're confident in the reliability of our offering, we're making improvements all the time. If you identify any issues, please let us know at support@yourbase.io and we will look into them quickly.
