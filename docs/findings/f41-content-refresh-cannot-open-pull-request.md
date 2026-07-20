# F41: Scheduled content refresh cannot open its generated pull request

- Severity: Low
- Category: Content automation
- Status: Confirmed

## Problem

The inspected scheduled workflow fetched and committed refreshed content and pushed its branch, but then failed because GitHub Actions is not permitted to create pull requests. The automation performs repository work that no reviewer is notified to inspect or merge, while the disconnected content packages continue to drift during normal development.

## Files affected

- `.github/workflows/refresh-content.yml:47-61`

## Proposed solution

Enable GitHub Actions pull-request creation for the repository or use a least-privilege GitHub App token with only the required branch and pull-request permissions. Update the pull-request action to a supported runtime and add an early permission preflight so the workflow fails before fetching and committing when it cannot complete delivery. Test the workflow with a dry-run or staging repository and verify that a successful refresh opens or updates exactly one reviewable pull request.
