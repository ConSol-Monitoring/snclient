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

## Winget

The release workflow generates the WinGet manifests.
It validates the manifests with `winget validate`.
It attaches `snclient-<version>-winget-manifests.zip` to the draft release.
Review the generated manifests before you publish the release.
The first submission is manual.
Generate the v1.01.0 manifests from the three release MSIs.
The generated files are in `packaging/winget/<package-version>/`.
Copy the three YAML files to `microsoft/winget-pkgs`.
Use the path `manifests/c/ConSol/SNClient/<package-version>/`.
Open one pull request for one package version.
For later releases, copy the three YAML files from the release zip.

After the first package is merged, enable automation.
Create a fork of `microsoft/winget-pkgs` under the fork owner.
Set the `WINGET_FORK_OWNER` repository variable to the fork owner.
Add a classic PAT with `public_repo` as the `WINGET_TOKEN` secret.
The PAT owner must have write access to the fork.
The workflow uses the GitHub API and does not clone the fork.
Keep `WINGET_AUTOPUBLISH` false until the first package is merged.
Set `WINGET_AUTOPUBLISH` to true after the merge.
The publish workflow creates the next winget-pkgs pull request.

Run `make winget-manifests` to generate the manifests locally.
Pass the hash and ProductCode variables from `buildtools/winget_manifests.sh`.
On Windows, run `pwsh buildtools/winget_manifests.ps1` with the three MSI paths.
The PowerShell script reads the ProductCode and ProductVersion from the MSIs.
