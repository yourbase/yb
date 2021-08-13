---
parent: CI and CLI Documentation
nav_order: 1
---

# Installation

## macOS

You can install `yb` using [Homebrew][]:

```sh
brew install yourbase/yourbase/yb
```

or you can download a binary from the [latest GitHub release][] and place it
in your `PATH`.

[Homebrew]: https://brew.sh/
[latest GitHub release]: https://github.com/yourbase/yb/releases/latest

## Linux (and WSL2)

On Debian-based distributions (including Ubuntu), you can use our APT
repository:

```sh
# Import the signing key
sudo curl -fsSLo /etc/apt/trusted.gpg.d/yourbase.asc https://apt.yourbase.io/signing-key.asc

# Add the YourBase APT repository to the list of sources
echo "deb [arch=amd64 signed-by=/etc/apt/trusted.gpg.d/yourbase.asc] https://apt.yourbase.io stable main" | sudo tee /etc/apt/sources.list.d/yourbase.list

# Update the package list and install yb
sudo apt-get update && sudo apt-get install yb
```

For Red Hat, Fedora, or CentOS, you can install the RPM from the
[latest GitHub release][]. For other distributions, copy the Linux binary from
the release into your `PATH`.

## Docker (Optional)

yb can optionally use Docker for isolating the build environment.
You can install Docker here: https://hub.docker.com/search/?type=edition&offering=community

If you're using WSL on Windows, you will need to use [Docker in WSL2](https://code.visualstudio.com/blogs/2020/03/02/docker-in-wsl2).
