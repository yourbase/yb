---
parent: CI and CLI Documentation
nav_order: 3
---

# List of Build Packs

yb comes with optimized installers for many programming languages. These are
made available for targets in the [package configuration](configuration.md).

| Build Pack   | Description                                | Example            |
| ------------ | ------------------------------------------ | ------------------ |
| `anaconda2`  | Python 2 via [Miniconda][]                 | `anaconda2:2.7.11` |
| `anaconda3`  | Python 3 via [Miniconda][]                 | `anaconda3:3.9.2`  |
| `android`    | [Android SDK][]                            | `android:latest` or a specific version like `android:4333796` |
| `androidndk` | [Android NDK][]                            | `androidndk:r21e`  |
| `ant`        | [Apache Ant][]                             | `ant:1.10.9` \*    |
| `dart`       | [Dart SDK][]                               | `dart:2.12.2`      |
| `flutter`    | [Flutter SDK][]                            | `flutter:2.0.3`    |
| `glide`      | [Glide][] dependency manager for Go        | `glide:0.13.3`     |
| `go`         | [Go][]                                     | `go:1.16.2`        |
| `gradle`     | [Gradle][]                                 | `gradle:6.8.3` \*  |
| `heroku`     | The [Heroku CLI][]                         | Only `heroku:latest` is allowed |
| `java`       | [OpenJDK][] binaries from [AdoptOpenJDK][] | `java:16+36`       |
| `maven`      | [Apache Maven][]                           | `maven:3.6.3` \*   |
| `node`       | [Node.js][] with bundled npm               | `node:14.16.0`     |
| `protoc`     | [Protocol Buffer compiler][]               | `protoc:3.15.6`    |
| `python`     | Python via [Miniconda][]                   | `python:3.9.2` or `python:2.7.11` |
| `r`          | [R][] programming language                 | `r:4.0.3`          |
| `ruby`       | [Ruby][]                                   | `ruby:2.6.3`       |
| `rust`       | [Rust][]                                   | `rust:1.51.0`      |
| `yarn`       | [Yarn][] package manager for JavaScript    | `yarn:1.22.10`     |

\* When using the `ant`, `gradle`, or `maven` build packs, you must also specify
the `java` build pack.

[AdoptOpenJDK]: https://adoptopenjdk.net/
[Android NDK]: https://developer.android.com/ndk/downloads
[Android SDK]: https://developer.android.com/studio/releases/sdk-tools
[Apache Ant]: https://ant.apache.org/
[Apache Maven]: https://maven.apache.org/download.cgi
[Dart SDK]: https://dart.dev/get-dart
[Flutter SDK]: https://flutter.dev/docs/get-started/install
[Glide]: https://github.com/Masterminds/glide
[Go]: https://golang.org/dl/
[Gradle]: https://gradle.org/install/
[Heroku CLI]: https://devcenter.heroku.com/articles/heroku-cli
[Miniconda]: https://docs.conda.io/en/latest/miniconda.html
[Node.js]: https://nodejs.org/
[OpenJDK]: https://openjdk.java.net/
[Protocol Buffer compiler]: https://github.com/protocolbuffers/protobuf/releases
[R]: https://www.r-project.org/
[Ruby]: https://www.ruby-lang.org/en/downloads/
[Rust]: https://github.com/rust-lang/rust/blob/master/RELEASES.md
[Yarn]: https://github.com/yarnpkg/yarn/releases
