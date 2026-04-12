---
name: release-traceability
description: Use when changing CI/CD, image tagging, deployment inputs, runtime version reporting, or validate logic for accounts.svc.plus. Enforces a single-source-of-truth release chain from GITHUB_SHA-based image build output through deploy and /api/ping validation.
license: Internal use only
metadata:
  owner: cloud-neutral-toolkit
  distribution: local-repo
  package-format: .skill
---

# Release Traceability

Use this skill for any work that touches:

- `.github/workflows/pipeline.yml`
- image tag generation
- deploy inputs and playbook contracts
- `/api/ping` version reporting
- `scripts/github-actions/validate-deploy.sh`
- cross-repo release handoff into deployment automation

## Goal

Preserve a single, traceable release chain so every delivery can answer:

- Which commit was built?
- Which image was deployed?
- Does the running service report that exact image and commit?

## Canonical Contract

1. `GITHUB_SHA` is the only commit source for release identity.
2. Build image tags must use the full commit SHA, typically `sha-<40 hex>`.
3. `service_image_ref` is the only authoritative build artifact identifier.
4. Deploy must consume `service_image_ref` directly.
5. Any `repo` / `tag` values used during deploy must be derived from `service_image_ref`, never provided independently.
6. The running container must receive `IMAGE=<full image_ref>`.
7. `/api/ping` must derive `image`, `tag`, `commit`, and `version` from `IMAGE`.
8. Validate must accept only `image_ref` and compare remote output against values parsed from it.

## Build Rules

- Generate tags only from full `GITHUB_SHA`.
- Do not introduce manual `image_tag`, `commit_id`, `version_tag`, or similar override inputs.
- Prefer one build output:
  - `service_image_ref`
- Optional outputs such as `service_image_tag` and `service_image_commit` are allowed only if parsed from `service_image_ref`.
- `latest` may exist as an auxiliary tag, but it must never be used as the authoritative release identifier.

## Deploy Rules

- Deploy must use the build job output, not a separately selected image.
- Default deploy flow must pull and run a prebuilt image.
- Do not add or preserve target-host build behavior:
  - `docker build`
  - `podman build`
  - `docker buildx build`
  - `gcloud builds submit`
- If an external repo or playbook is part of the release path, it must also accept `IMAGE_REF` / `ACCOUNTS_IMAGE_REF` and treat it as the single source of truth.
- If legacy deploy systems still need `repo` and `tag`, derive them in the deploy job from `service_image_ref`.

## Runtime Rules

- `/api/ping` must not trust a separate `COMMIT_ID` environment variable.
- `commit` must come from parsing the image tag in `IMAGE`.
- Support both:
  - raw hex tags like `<40 hex>`
  - SHA-prefixed tags like `sha-<40 hex>`
- If `IMAGE` is missing, return diagnostically useful empty values rather than inventing a commit.

## Validate Rules

- `scripts/github-actions/validate-deploy.sh` must accept only:
  - `image_ref`
  - optional base URL
- The script must parse `tag`, `commit`, and `version` from `image_ref`.
- Minimum required checks:
  - remote `image == image_ref`
  - remote `tag == parsed tag`
  - remote `commit == parsed commit`
  - remote `version == parsed version`

## Required Checks Before Completion

- Confirm the workflow still emits a full-SHA `service_image_ref`.
- Confirm deploy consumes that artifact and does not re-select the image.
- Confirm the runtime container receives `IMAGE=<service_image_ref>`.
- Confirm `/api/ping` reports values consistent with `service_image_ref`.
- Confirm validate compares against values parsed from the same `service_image_ref`.
- Confirm no target-host image build commands were introduced into the default deploy path.

## Anti-Patterns

- Passing `commit_id` separately into deploy or validate
- Letting deploy choose a different image than build produced
- Using `latest` as the release truth source
- Rebuilding images on the target host
- Returning a commit from any source other than the runtime `IMAGE`
