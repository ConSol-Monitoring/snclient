## Release

A new release can be built by following these steps:

- Verify the test status is green on the [GitHub Actions](https://github.com/ConSol-Monitoring/snclient/actions/workflows/builds.yml) page.
- If not already there, add a `next:` entry to the changelog.

  (For example, see `git show b2e1b020ed462196670068034fdee87ee33814ac`.)

  Then have a look at `git log` and add missing changes.

- Run `make release` and choose a new version. Usually just increment the
  minor number unless there are breaking changes.
- Review `git log -1` and `git diff HEAD` to check that everything looks correct.

  For example, the changelog should contain the current version tag now.

- Push the release commit with `git push` and `git push --tags`

- Monitor the GitHub Actions workflow that builds the release packages on the [GitHub Actions](https://github.com/ConSol-Monitoring/snclient/actions/workflows/builds.yml) page.

  The one with the git tag will also create a draft release. So when the workflow
  finishes, go to the [releases page](https://github.com/ConSol-Monitoring/snclient/releases)
  and edit the draft for the latest tag, scroll down, and publish the release.
