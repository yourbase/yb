# YourBase Ruby Acceleration

Tests are important. For large monoliths, they're also a major source of drag on velocity.

YourBase is a tool that traces your tests to determine which files they depend on. It uses this information to determine which tests must run, because they are new or have changed dependencies. Tests without changed dependencies are skipped.

YourBase works with Ruby versions >= 2.3 and RSpec 3+.

## Getting Started
1. Add `gem 'yourbase-rspec', '~> 0.5.6',` to your Gemfile. If you have a `:test` group, add it there.
2. `bundle install` from the command line.
3. If you are not in a Rails project, you will also need to `require 'yourbase-rspec` your spec_helper (or at the top of the spec file you want to run).

Once you have a token, simply follow the steps below:
```sh
bundle add yourbase-rspec --git "https://${YOURBASE_DOWNLOAD_TOKEN?}:x-oauth-basic@github.com/yourbase/yourbase-rspec-skipper-engine.git" && bundle install
```

> Note: After installing yourbase-rspec, if you are not using Rails you must add
"require 'yourbase-rspec'" to in your spec folder.

```sh
# Add require 'yourbase-rspec' for non Rails projects.
echo "require 'yourbase-rspec'" >> spec/yourbase_spec.rb
```
## First run

> Note: If you are using Spring, please run `spring stop` before starting your tests.

Run your tests with the same command you typically use. You should see a rocket ship at the beginning the RSpec test section.

```plain
🚀
```

The first time you run your tests with `yourbase-rspec` will take the typical amount of time as it records tracing data to map dependencies (a "cold build"). If you run the same test again without changing any code, you should see everything skipped!  Subsequent runs will only run examples that are new or depend on changed files. 

After the run finishes, running again will skip all tests. Modifying a dependency will run only tests whose code paths touched the changed code. 

You're YourBased! 🚀

## RSpec Output

YourBase adds to the RSpec output to give you information while about if examples are being run or skipped, and why. 

The default RSpec output from the `ProgressFormatter` (`.....*..... F........`) will print a `.` in yellow (or the color you have set for `:pending`) when an example is skipped.  

The `DocumentationFormatter` will add the reason an example group is being run in green (or the color your have set for `:success`), and the reason an example group is being skipped in yellow (or the color you have set for `:pending`). 

The summary line will show how show how many examples were skipped! 🚀  
```plain
1 examples, 0 failures, 1 skipped with YourBase🚀
```

## Parallelization and  Sharding
The `yourbase-rspec` gem supports your workflows for both parallelization (running tests in more than one process at a time on the same machine) and sharding (running tests on more than one machine). Dependency histories are keyed off of the code state (git hash), and all tracing data derived from an identical code state is grouped for future use.  

The environment variables `YOURBASE_ACTIVE_COHORT` and `YOURBASE_COHORT COUNT` control which tests **might** run. Tests that are in the active cohort will be checked against dependency changes, and _tests that are not in the active cohort will be automatically skipped_.  

YourBase cohorts are assigned based on consistent hashing of the example group name AND the number of cohorts. An example group that is in cohort `1` will always be in cohort `1` unless either the `YOURBASE_COHORT_COUNT` OR example group name are changed.  

The `YOURBASE_ACTIVE_COHORT` is 1-indexed (it starts at 1, not 0). If you are sharding with YourBase cohorts, and you set `YOURBASE_COHORT_COUNT=2`, then one of your shard should have `YOURBASE_ACTIVE_COHORT=1` and the other should have `YOURBASE_ACTIVE_COHORT=2`.  

Unless the value of `YOURBASE_COHORT_COUNT` is set and is greater than 1, cohorts are turned off.  

## Observation Mode
The yourbase-rspec gem includes an "observation mode" which allows you to test drive the gem without actually skipping any tests.  

In “observation mode” all [command-line specified] examples will be run, but `yourbase-rspec` will monitor if our test selection would have skipped any examples that ultimately failed. At the end,  it will print out the names of any example group that would have been incorrectly skipped, or it will confirm that none were.  

To access observation mode, set `YOURBASE_OBSERVATION_MODE=true` in the environment, and run your specs. The documentation formatter isn’t required, but it will print the reasons why YourBase would select to run or skip a given example group.  

`$ YOURBASE_OBSERVATION_MODE=true bundle exec rspec --format documentation`

Instead of a single rocketship, you’ll see the following at the top of the rspec output for observation mode:
`:rocket: YourBase test selection is in observation mode. All example groups will be run. :rocket:`

And then at the bottom, below the RSpec summary, you should see this: 
`🚀 YourBase observation mode: all "skipped" example groups passed successfully! 🚀`

If you instead see: `🚀 YourBase observation mode: some "skipped" example groups contained failures 🛸` followed by one or more example group names, it means that yourbase-spec would have skipped at least one test that failed when it was actually run. We hope you'll never see this, and we hope that you'll email us if you do, at: <support@yourbase.io>

