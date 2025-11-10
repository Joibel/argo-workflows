# Helm Chart Link Automation

This workflow automatically adds Helm chart installation information to Argo Workflows GitHub releases.

## Overview

When the argo-helm repository publishes a new Helm chart for an Argo Workflows version, this workflow updates the corresponding GitHub release page with:
- Link to the chart on Artifact Hub
- Chart repository URL
- Installation instructions
- Chart version information

## Trigger Methods

### 1. Manual Trigger (Workflow Dispatch)

You can manually trigger the workflow from the GitHub Actions UI:

1. Go to **Actions** → **Add Helm Chart Link to Release**
2. Click **Run workflow**
3. Enter the Argo Workflows version (e.g., `3.5.0` or `v3.5.0`)
4. Optionally enter the Helm chart version if different from the app version
5. Click **Run workflow**

### 2. Automated Trigger (Repository Dispatch)

The argo-helm repository can automatically trigger this workflow when publishing a new chart.

#### Setting up in argo-helm

Add the following step to the argo-helm chart publishing workflow (typically in `.github/workflows/publish.yml`):

```yaml
- name: Notify argo-workflows repository
  if: startsWith(github.ref, 'refs/heads/main') && contains(steps.publish.outputs.chart, 'argo-workflows')
  run: |
    # Extract versions from Chart.yaml
    CHART_VERSION=$(yq eval '.version' charts/argo-workflows/Chart.yaml)
    APP_VERSION=$(yq eval '.appVersion' charts/argo-workflows/Chart.yaml)

    # Trigger workflow in argo-workflows repository
    curl -X POST \
      -H "Accept: application/vnd.github+json" \
      -H "Authorization: Bearer ${{ secrets.ARGO_WORKFLOWS_DISPATCH_TOKEN }}" \
      -H "X-GitHub-Api-Version: 2022-11-28" \
      https://api.github.com/repos/argoproj/argo-workflows/dispatches \
      -d "{\"event_type\":\"helm-chart-published\",\"client_payload\":{\"version\":\"${APP_VERSION}\",\"chart_version\":\"${CHART_VERSION}\"}}"
```

**Required Secret**: `ARGO_WORKFLOWS_DISPATCH_TOKEN` - A GitHub Personal Access Token or App token with `repo` scope for the argo-workflows repository.

### 3. Manual API Call

You can also trigger the workflow using the GitHub API directly:

```bash
curl -X POST \
  -H "Accept: application/vnd.github+json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  https://api.github.com/repos/argoproj/argo-workflows/dispatches \
  -d '{"event_type":"helm-chart-published","client_payload":{"version":"3.5.0","chart_version":"0.45.0"}}'
```

Or using the `gh` CLI:

```bash
gh workflow run add-helm-chart-link.yaml \
  --repo argoproj/argo-workflows \
  --field version=3.5.0 \
  --field helm_chart_version=0.45.0
```

## How It Works

1. **Version Detection**: Extracts the version from either manual input or webhook payload
2. **Release Verification**: Checks that the GitHub release exists for the specified version
3. **Chart Verification**: Verifies the Helm chart has been published to the chart repository
4. **Release Update**: Appends a Helm Chart section to the release notes with:
   - Chart version
   - Links to Artifact Hub and chart repository
   - Installation instructions
   - Link to argo-helm repository

## Example Output

The workflow adds a section like this to the release notes:

```markdown
---

## Helm Chart

The Helm chart for Argo Workflows `3.5.0` is available:

- **Chart Version**: `0.45.0`
- **Artifact Hub**: https://artifacthub.io/packages/helm/argo/argo-workflows/0.45.0
- **Chart Repository**: https://argoproj.github.io/argo-helm

### Installation

\`\`\`bash
# Add the Argo Helm repository
helm repo add argo https://argoproj.github.io/argo-helm
helm repo update

# Install or upgrade Argo Workflows
helm install argo-workflows argo/argo-workflows \
  --version 0.45.0 \
  --namespace argo \
  --create-namespace
\`\`\`

For more information, see the [argo-helm repository](https://github.com/argoproj/argo-helm/tree/main/charts/argo-workflows).
```

## Permissions

The workflow requires:
- `contents: write` permission to update releases

For repository_dispatch triggers from external repositories:
- The triggering repository needs a token with `repo` scope for argo-workflows

## Troubleshooting

### Release not found
- Ensure the release tag exists in the argo-workflows repository
- The workflow expects tags in the format `vX.Y.Z` (e.g., `v3.5.0`)

### Helm chart not found
- The workflow will show a warning if the chart isn't published yet
- Verify the chart version exists at https://argoproj.github.io/argo-helm/index.yaml
- The workflow will still update the release even if the chart isn't found (useful for pre-publishing the link)

### Link already exists
- The workflow skips updating if a "## Helm Chart" section already exists in the release notes
- To update an existing section, manually edit the release first to remove the old section

## Testing

To test the workflow:

1. Create a test release or use an existing one
2. Manually trigger the workflow with the release version
3. Check the GitHub Actions run for any errors
4. Verify the release notes were updated correctly

## Maintenance

The workflow uses:
- `actions/checkout@v4`
- GitHub CLI (`gh`) for release management
- `curl` and `grep` for chart verification

No external actions or dependencies are required beyond standard Ubuntu runner tools.
