# Google Cloud Run Configuration Guide

## Required Configuration

Before the `deploy.yml` workflow can run successfully, you must configure the following:

### 1. Update `deploy.yml` Environment Variables

Open the workflow file `.github/workflows/deploy.yml` and update the `env` section:
- `PROJECT_ID`: Set to your Google Cloud Project ID.
- `REGION`: Update if you want to use a region other than `asia-northeast1` (e.g., `asia-northeast1` for Tokyo).
- `SERVICE_NAME`: Confirm the Cloud Run service name.
- `REPOSITORY`: Confirm the Artifact Registry repository name.

### 2. Configure Workload Identity Federation (WIF)

You need to replace the placeholders in the `Google Auth` step or use GitHub Secrets (Recommended).

**Recommended Approach:**

1. Go to your GitHub Repository -> **Settings** -> **Secrets and variables** -> **Actions**.
2. Create a new Repository Secret named `WIF_PROVIDER`.
   - Value should be the full provider path, e.g., `projects/123456789/locations/global/workloadIdentityPools/my-pool/providers/my-provider`.
3. Create a new Repository Secret named `WIF_SERVICE_ACCOUNT` with your service account email, e.g., `my-service-account@your-project-id.iam.gserviceaccount.com`.

Then update the workflow file (`.github/workflows/deploy.yml`) to use these secrets:

```yaml
      # 1. 身份验证 (使用 Workload Identity Federation)
      - name: Google Auth
        uses: google-github-actions/auth@v2
        with:
          workload_identity_provider: ${{ secrets.WIF_PROVIDER }}
          service_account: ${{ secrets.WIF_SERVICE_ACCOUNT }}
```

### 3. Grant Permissions

Ensure your Service Account has the following IAM roles in Google Cloud:

- `Artifact Registry Writer` (roles/artifactregistry.writer): To push container images.
- `Cloud Run Admin` (roles/run.admin): To deploy new revisions.
- `Service Account User` (roles/iam.serviceAccountUser): To act as the service account during deployment.
