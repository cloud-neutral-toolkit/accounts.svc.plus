#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  setup.sh <repo_name_or_dir> [--repo <git_url>] [--ref <git_ref>] [--dir <path>]

Examples:
  # Remote install:
  # curl -fsSL "https://raw.githubusercontent.com/cloud-neutral-toolkit/<repo>/main/scripts/setup.sh?$(date +%s)" | bash -s -- <repo>
  #
  # Local:
  # bash scripts/setup.sh <repo>

Notes:
  - Safe: no secrets written; no destructive actions.
  - If .env does not exist, it copies .env.example -> .env (placeholder only).
EOF
}

log() { printf '[setup] %s\n' "$*"; }

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    log "missing required command: $1"
    exit 1
  fi
}

if [[ "${1:-}" == "" || "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

NAME="$1"
shift

REPO_URL=""
REF="main"
DIR="$NAME"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo) REPO_URL="${2:-}"; shift 2 ;;
    --ref) REF="${2:-}"; shift 2 ;;
    --dir) DIR="${2:-}"; shift 2 ;;
    *) log "unknown arg: $1"; usage; exit 2 ;;
  esac
done

if [[ -z "${REPO_URL}" ]]; then
  REPO_URL="https://github.com/cloud-neutral-toolkit/${NAME}.git"
fi

need_cmd git
need_cmd curl

if [[ -e "${DIR}" && ! -d "${DIR}" ]]; then
  log "path exists and is not a directory: ${DIR}"
  exit 2
fi

if [[ ! -d "${DIR}" ]]; then
  log "cloning ${REPO_URL} (ref=${REF}) -> ${DIR}"
  git clone --depth 1 --branch "${REF}" "${REPO_URL}" "${DIR}"
else
  if [[ ! -d "${DIR}/.git" ]]; then
    log "directory exists but is not a git repo: ${DIR}"
    exit 2
  fi
  log "repo directory already exists: ${DIR}"
fi

cd "${DIR}"

did_any=false

if [[ -f "package.json" ]]; then
  need_cmd node
  if command -v corepack >/dev/null 2>&1; then
    corepack enable >/dev/null 2>&1 || true
  fi
  if command -v yarn >/dev/null 2>&1; then
    log "installing JS dependencies (yarn install)"
    yarn install
    did_any=true
  else
    log "yarn not found; install yarn (or enable corepack) then re-run"
    exit 1
  fi
fi

if [[ -f "go.mod" ]]; then
  need_cmd go
  log "downloading Go dependencies (go mod download)"
  go mod download
  did_any=true
fi

if [[ "${did_any}" == "false" ]]; then
  log "no supported project type detected (package.json/go.mod)."
  log "setup script completed without installing deps."
fi

if [[ ! -f ".env" && -f ".env.example" ]]; then
  log "creating .env from .env.example (placeholder only)"
  cp .env.example .env
fi

if [[ -f "scripts/post-setup.sh" ]]; then
  log "running scripts/post-setup.sh"
  bash scripts/post-setup.sh
fi

log "setup complete"
log "next steps:"
if [[ -f "go.mod" ]]; then
  log "  cp .env.example .env"
  log "  make dev"
fi

