---
layout: default
title: Security
nav_order: 7
parent: Test acceleration
has_children: false
permalink: /test-acceleration/security
---

# Security
YourBase Test Acceleration library is secure. Under no circumstance do your code or your dependency graphs ever touch YourBase Test Acceleration servers. Only metadata about your usage of the library would ever be shared with YourBase Test Acceleration.

By default, YourBase Test Acceleration collects anonymized data about how the YourBase Test Acceleration library is behaving to help resolve technical issues and develop the product with people's use--cases in mind. The information we collect via telemetry is documented at https://yourbase.io/data-usage. 

If YourBase Test Acceleration changes the type of data that it collects, it will always inform you at runtime. 

You can opt-out of sending any crash data or usage statistics to the YourBase Test Acceleration team, by setting the environment variable YOURBASE_TELEMETRY to false.
