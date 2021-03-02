# Build Environment Reference

**WIP specification. This information should eventually live in end-user
reference documentation.**

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD",
"SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be
interpreted as described in [RFC 2119][].

[RFC 2119]: https://tools.ietf.org/html/rfc2119

## Environment Variables

Each target **SHALL** run in a separate, isolated environment. For example,
a target may run in a Docker container or a chroot jail. A target _MAY_ run on
a different host from the build runner. At a minimum, the following environment
variables **MUST** be set for commands running in POSIX environments:

-  `HOME` **MUST** be set to the path of an readable and writable directory.
   This _SHOULD NOT_ be the same as the user's actual `HOME` directory to
   keep builds reproducible.
-  `LOGNAME` and `USER` **MUST** be set to the name of the POSIX user running
   the command (not the runner of `yb`).
-  `PATH` **MUST NOT** be empty.
-  `TZ` **MUST** be set to `UTC0`.
-  `LANG` _SHOULD_ be set to `C.UTF-8` if the environment supports it.
   Otherwise, `LANG` **MUST** be set to `C`.
-  One of `LC_ALL` or `LC_CTYPE` **MUST** be set to C-like locale category
   whose `charmap` is UTF-8. If `LC_CTYPE` is set, `LC_ALL` **MUST NOT** be set.

### Examples of Locale Settings

For Linux systems:

```
LANG=C.UTF-8
LC_ALL=C.UTF-8
```

For macOS systems:

```
LANG=C
LC_CTYPE=UTF-8
```

### Further Reading

-  [POSIX.1-2017 Environment Variables](https://pubs.opengroup.org/onlinepubs/9699919799/basedefs/V1_chap08.html)
   describes the meaning of the standard environment variables.
-  [PEP 538](https://www.python.org/dev/peps/pep-0538/) documents Python's
   process for bootstrapping a UTF-8 locale with rationale and platform-specific
   caveats.

## Expected Userspace

The build environment that a target's commands run in **MUST** include the
standard [POSIX utilities][]. yb also depends on the following utilities being
available:

-  `python` on non-Linux to fill in `readlink --canonicalize-existing` behavior
-  `readlink` on Linux
-  `tar`
-  `unzip`

[POSIX utilities]: https://pubs.opengroup.org/onlinepubs/9699919799/idx/utilities.html
