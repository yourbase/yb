# YourBase Ruby Acceleration

Tests are important. For large monoliths, they're also a major source of drag on velocity.

YourBase is a tool that traces your tests to determine which functions each test depends on. It can later use this information to determine which tests do not need to run because their code paths have not changed. These tests are skipped automatically.

YourBase works with Ruby versions >= 2.3

## Getting Started
No configuration, setup, or babysitting required. To get started, you need the YourBase gem and a download token.  To request a token, please visit [YourBase.io](https://yourbase.io/download)

Once you have a token, simply follow the steps below:
```sh
bundle add yourbase-rspec --git "https://${YOURBASE_DOWNLOAD_TOKEN?}:x-oauth-basic@github.com/yourbase/yourbase-rspec-skipper-engine.git" && bundle install
```

## First run
After installing yourbase-rspec, if you are not using Rails you must add
"require 'yourbase-rspec'" in your spec folder.

```sh
# Add require for non Rails projects
echo "require 'yourbase-rspec'" >> spec/yourbase_spec.rb
```

Run your tests with the same command you typically use. You should see a rocket ship at the beginning the RSpec test section.

```plain
🚀
```

The first run will be cold, so if you just want to see YourBase in action and your tests are going to take a while, you can run a subset of tests. Tracing data for the subset will be used correctly even if you later run all tests.

After the run finishes, running again will skip all tests. Modifying a dependency will run only tests whose code paths touched the changed code. You're YourBased! 🚀

## RSpec Output

YourBase enhances the output so that you can clearly see the results of the Gem.

For an accelerated run, you will see the number of skipped tested added to your
RSpec summary line:
```plain
1 examples, 0 failures, 1 skipped with YourBase🚀
```

YourBase adds additional details when using the RSpec formatter option with the
progress and documentation formatters.

## Cohorting / Sharding
YourBase supports sharding your tests without negatively affecting tracing or acceleration. It uses consistent hashing to split tests into cohorts that stay the same between runs, even as the test pool grows or shrinks.

1) Set YB_COHORT_COUNT to the number of cohorts your tests should be split into. This should be the same among all shards.
1) Set YB_TEST_COHORT to the cohort ID this run should identify as, starting with 1. This should be different among all shards.
Without these set, YourBase assumes a value of "1" for both, meaning one shard will run one cohort; that cohort will contain all tests.

Note that tests are only guaranteed to remain in the same cohort as long as
YB_COHORT_COUNT doesn't change.

## Observation Mode
The yourbase-rspec gem includes an "observation mode" which will run all [command-line specified] examples. "Observation mode" allows you to test drive the plugin without actually skipping any tests. 

To enable observation mode, set YOURBASE_OBSERVATION_MODE to true in the environment. The documentation formatter isn't strictly required, but it will print the reasons why YourBase would run or skip a given example group.

```sh
YOURBASE_OBSERVATION_MODE=true bundle exec rspec --format documentation
```

Instead of a single rocketship, you'll see the following at the top of the rspec output for observation mode:

```plain
🚀 YourBase test selection is in observation mode. All example groups will be run. 🚀
```

