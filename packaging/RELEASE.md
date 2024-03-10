## Release

A new release can be build by following these steps:

- Verify the test status is green on the [github actions](https://github.com/ConSol-Monitoring/snclient/actions/workflows/builds.yml) page.
- If not already there, add a `next:` entry to the changelog.

  (ex.: as in git show b2e1b020ed462196670068034fdee87ee33814ac)

  Then have a look at `git log` and add missing changes.

- Run `make release` and choose a new version. Usually just increment the
  minor number unless there are breaking changes.
- Check `git log -1` and `git diff HEAD` if things look good.

  Ex. the changelog should contain the current version tag now.

- Push the release commit with `git push` and `git push --tags`

- Watch the github action build the release packages on the [github actions](https://github.com/ConSol-Monitoring/snclient/actions/workflows/builds.yml) page.

  The one with the git tag will also create a draft release. So when the action
  is ready, go to the [releases page](https://github.com/ConSol-Monitoring/snclient/releases)
  and edit the last tag, scroll down and publish the release.
