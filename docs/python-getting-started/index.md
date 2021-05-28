# Getting Started with Python Test Acceleration

YourBase Test Acceleration is a plug-in that automatically determines which
tests need to run based on which changes were made to code.

All it takes to get started is a `pip install yourbase`.

## Accelerate Tests Locally

YourBase intelligent test selection for Python comes in the form of a library
that can be installed from PyPI using `pip` or `poetry` with a single command.
The first time you run your tests with YourBase, a *Dependency Graph* will be
automatically built that maps which files and functions your tests depend on.
After making a change and re-running your tests, YourBase will load the optimal
dependency graph and use that information to intelligently select which tests
need to be run based on the changes that have been made to your source code.

YourBase has support for both `pytest` and `unittest` - you can see examples of
both below, depending on the test framework you use. For further information you
can also refer to the [PyPI documentation page][].

[PyPI documentation page]: https://pypi.org/project/yourbase/

### pytest

YourBase is bundled with hooks for the pytest framework; to use it you run
`pip install yourbase` or add `yourbase` to your `requirements.txt` before
running `pip install -r requirements.txt` (or equivalent, `poetry` will also
work just fine). This installs the YourBase test selection library, which
includes a pytest plugin that will be automatically discovered by pytest without
any additional configuration. Once it has been installed, the plugin will be
loaded for any invocation of pytest that uses the python installation.

For those who want to get started with an existing project, you can follow along
with the below example, using the `unleash-client-python` project to demonstrate
how things work.

1. Checkout the project:
   `git clone https://github.com/Unleash/unleash-client-python`

   <img src="pytest01.png">

2. (Optional) Create a virtual environment for this demonstration if you want to
   use a clean Python environment (macOS / Linux can use `python -m venv .venv`
   and activate it with `source .venv/bin/activate` ; Windows users can use
   `python -m venv .venv` and then activate it with `source .venv/scripts/activate`)

3. Install the YourBase Python library using `pip install yourbase`

4. Run the Unleash client's tests using `pytest tests` - as you will see, this
   executes all of the tests and builds an initial dependency graph.

   <img src="pytest02.png">

5. Run `pytest tests` again (making no code changes to the project) - this time
   you will see that no code has changed and all the tests will be skipped, as
   the outcome will be the same as before.

   <img src="pytest03.png">

6. Make a minor change to a source file; in our example we have modified the
   function `test_create_feature_true` in the `tests/unit_tests/test_features.py`
   file.

7. Run `pytest tests` again, this time you will see a message telling you which
   functions have been altered and how many tests were affected. If you modified
   the same function as above then your output will closely match the output
   below in which only one test gets re-run, while 167 of them are skipped,
   bringing our test time down from 2 minutes to 1 second.

   <img src="pytest04.png">

8. Feel free to experiment with other functions in the source tree and see what
   happens, then go ahead and use `yourbase` in your own project!

### unittest

YourBase comes bundled with hooks that will automatically detect when `unittest`
is being used and automatically take care of loading itself without any
additional action on your part. These hooks intercept the existing unittest
setup and teardown handlers in order to provide test acceleration.
Run `pip install yourbase` or add `yourbase` to your `requirements.txt` before
running `pip install -r requirements.txt` in order to install the test selection
library.

For those wanting to get started with an example project, the following example
will get you up and running with minimal fuss by using the `python-unidiff`
project (which uses `unittest` to run its tests).

1. Check out the project: `git clone https://github.com/matiasb/python-unidiff.git`

   <img src="unittest01.png">

2. (Optional) Create a virtual environment for this demonstration if you want to
   use a clean Python environment (macOS / Linux can use `python -m venv .venv`
   and activate it with `source .venv/bin/activate` ; Windows users can use
   `python -m venv .venv` and then activate it with `source .venv/scripts/activate`)

3. Install the YourBase Python library using `pip install yourbase`

4. Update `tests/__init__.py`

   ```python
   import unittest
   import yourbase

   yourbase.attach(unittest)
   ```

5. Run `python -m unittest discover`

   <img src="unittest02.png">

6. Run `python -m unittest discover` a second time with no changes to the
   `python-unidiff` code files; observe that all tests will be skipped because
   there have been no code changes.

   <img src="unittest03.png">

7. Make a small change to the project - in our case we made a small change to
   `test_preserve_dos_line_endings` inside the `TestUnidiffParser` class located
   in `tests/test_parser.py` - you can add a comment, a print statement or
   something else small (or even break the test if you like).

8. Run `pytest tests` after having made the change; here you will see that our
   modification resulted in one test being affected and as such 37 out of the 38
   tests were skipped completely.

   <img src="unittest04.png">

9. Feel free to experiment with other functions in the source tree and see what
   happens, then go ahead and use `yourbase` in your own project!

## Accelerate Tests in CI

To use YourBase in your CI, you will need to set up a few things first in order
to enable shared Dependency Graph storage. Once you have done that, you can add
`yourbase` to your project (via `requirements.txt` or whatever mechanism you use
to install your dependencies) and then use YourBase to accelerate the builds in
your CI system.

### About the Shared Dependency Graph

To get acceleration gains across team members, you can store the Dependency
Graph in the cloud and share it with others working on the same code-base.
YourBase will use information from your project's commit history to determine
the optimal Dependency Graph for each build. As a result, when you've submitted
code that is the same as code already tested by your colleagues, YourBase will
be able to skip those tests anywhere that has access to the shared graph
storage.

Currently, YourBase supports [S3 buckets][] for graph storage, so you will
need to create a new (or have an existing) bucket available for storing your
team's graphs. The graphs are separated by unique project, so you can choose to
use one bucket per project or one bucket for all of your projects. In order to
access the shared storage, anywhere that you will use YourBase needs to have
credentials that have read, write and list permissions to the relevant S3
bucket.

[S3 buckets]: https://docs.aws.amazon.com/AmazonS3/latest/userguide/UsingBucket.html

### S3 Configuration for CI

YourBase needs the information required to access your S3 bucket; as mentioned
before you will need to configure credentials (via environment, EC2 role,
configuration file, etc.) in a way that the default configuration can access
them. In addition to that you will need to export the S3 bucket information;
below is an example of how to do this — more information can be found on the
[PyPI documentation page][].

1. Get the name of the bucket you want to use, for example
   `acmecorp-yourbase-graphs`
2. Export the `YOURBASE_REMOTE_CACHE` variable to be `s3://your-bucket-name`.
   For example: `export YOURBASE_REMOTE_CACHE="s3://acmecorp-yourbase-graphs"`
3. Configure your AWS credentials - you can do this one of two ways:
    1. The standard AWS way (config file, or environment variable, etc), or
    2. Using a YourBase-specific pair of environment variables (in case you want
       to have a separate set of credentials just for the graph access), which
       are `YOURBASE_AWS_ACCESS_KEY_ID` and `YOURBASE_AWS_SECRET_ACCESS_KEY`

## Support for Parallelized Tests

If you've already conducted your CI to parallelize tests, YourBase has you
covered. YourBase can be configured to work with tests run in cohorts.

## Product Usage Data

By default, YourBase tracks how many tests are run and how many are skipped with
each build. YourBase also tracks the length of the tests. You can opt out of
data sharing by setting an environment variable.
