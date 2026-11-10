## Release

To build a new release:

- Verify the test status is green on the [GitHub Actions](https://github.com/ConSol-Monitoring/snclient/actions/workflows/builds.yml) page.
- If the changelog has no `next:` entry, add one.

  (For example, see `git show b2e1b020ed462196670068034fdee87ee33814ac`.)

  Review `git log`. Add any missing changes to the changelog.

- Run `make release`. Choose a new version.
  Unless the release has breaking changes, usually increment the minor number.
- Review `git log -1` and `git diff HEAD` to check that everything looks correct.

  For example, make sure that the changelog contains the current version tag.

- Push the release commit with `git push` and `git push --tags`

- Monitor the GitHub Actions workflow that builds the release packages on the [GitHub Actions](https://github.com/ConSol-Monitoring/snclient/actions/workflows/builds.yml) page.

  The workflow for the git tag also creates a draft release.
  When the workflow finishes, open the [releases page](https://github.com/ConSol-Monitoring/snclient/releases).
  Edit the draft for the latest tag. Scroll down and publish the release.
