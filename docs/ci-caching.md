---
parent: CI and CLI Documentation
nav_order: 6
---

# CI Acceleration and Caching

The [YourBase CI service][] automatically accelerates builds based on their
inferred dependencies. For example, if you change your README, the YourBase
CI service will skip any unit tests, but it won't skip your Markdown linter
check. The YourBase CI service will also persist build artifacts from
previous runs, so tools like `go build` or `npm install` will run faster
after their first build.

Here's a few tips for correct, fast builds on YourBase CI:

- **Put files that can be reused between builds inside your project directory
  or under `$HOME`.** Other files may be preserved, but this is not guaranteed.
  Even when using the CLI, `$HOME` will not map to your real home directory, so
  you can safely stash files there without affecting other build targets.
- **Break up `commands` based on component.** The CI service unit of
  acceleration is a command, so either a command will be run or it won't. If you
  run a script in your target, individual commands in that script will not be
  accelerated.
- **Each of a build target's `commands` should be [idempotent][].** For example,
  copying a file is idempotent but moving a file is not because the source file
  would no longer exist after the command. If your commands are not idempotent,
  your builds may have incorrect results when a command gets skipped.
- **Don't assume that the cached files will be from the most recent build.**
  To ensure the fastest build, the YourBase CI service may use cached results
  from an older build for a variety of reasons. We're always tweaking our
  algorithms for the best results, so the exact behavior will change over time.

If something goes wrong, you can always trigger a cold build to ensure the
highest fidelity results.

[idempotent]: https://en.wikipedia.org/wiki/Idempotence#Computer_science_meaning
[YourBase CI service]: https://app.yourbase.io
