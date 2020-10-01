Name:       yb
Version:    %{version}
Release:    1
Summary:    Build tool optimized for local + remote development
License:    ASL 2.0
# TODO(light): Technically, we require these things to build.
# However, we're arranging for these things to be present in the build
# environment through different means.
#BuildRequires: gcc
#BuildRequires: golang >= 1.15
Requires: glibc

%description
YourBase is a build tool that makes working on projects much more
delightful. Stop worrying about dependencies, keep your CI build process
in-sync with your local development process and onboard new team members
with ease.

%install
mkdir -p %{buildroot}/usr/bin/
# TODO(light): This could be under %build.
VERSION=v%{version} release/build.sh %{buildroot}/usr/bin/yb
chmod 755 %{buildroot}/usr/bin/yb

%files
%license LICENSE
/usr/bin/yb
