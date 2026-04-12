#!/usr/bin/env bash
set -euo pipefail

IMAGE_REF="${1:?image_ref is required}"
BASE_URL="${2:-https://accounts.svc.plus}"

image_ref="$(printf '%s' "${IMAGE_REF}" | tr -d '\n' | xargs)"
if [[ -z "${image_ref}" ]]; then
  echo "image_ref is required" >&2
  exit 1
fi

image_no_digest="${image_ref%@*}"
tag="${image_no_digest##*:}"
if [[ "${image_no_digest}" == "${tag}" ]]; then
  tag=""
fi

commit=""
version="${tag}"

if [[ "${tag}" =~ ^[0-9a-f]{7,40}$ ]]; then
  commit="${tag}"
elif [[ "${tag}" =~ ^sha-([0-9a-f]{7,40})$ ]]; then
  commit="${BASH_REMATCH[1]}"
fi

ping_json="$(
  curl \
    --silent \
    --show-error \
    --fail \
    --location \
    --max-time 20 \
    "${BASE_URL}/api/ping"
)"

PING_JSON="${ping_json}" python3 - "${image_ref}" "${tag}" "${commit}" "${version}" <<'PY'
import json
import os
import sys

image_ref, tag, commit, version = sys.argv[1:5]
payload = json.loads(os.environ["PING_JSON"])

if payload.get("status") != "ok":
    raise SystemExit("ping status not ok")

if payload.get("image") != image_ref:
    raise SystemExit(f"expected image {image_ref!r}, got {payload.get('image')!r}")

if tag and payload.get("tag") != tag:
    raise SystemExit(f"expected tag {tag!r}, got {payload.get('tag')!r}")

if commit and payload.get("commit") != commit:
    raise SystemExit(f"expected commit {commit!r}, got {payload.get('commit')!r}")

if version and payload.get("version") != version:
    raise SystemExit(f"expected version {version!r}, got {payload.get('version')!r}")
PY
