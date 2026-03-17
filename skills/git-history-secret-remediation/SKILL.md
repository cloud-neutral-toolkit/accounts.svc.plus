---
name: git-history-secret-remediation
description: Use when a user asks to detect secrets in git commit history, clean tracked sensitive data, rewrite history with git-filter-repo, or verify cleanup with gitleaks. Covers gitleaks detect -v, replacement mapping, path removal, ref inventory, history rewrites, force-push planning, and post-cleanup coordination.
license: Internal use only
metadata:
  owner: cloud-neutral-toolkit
  distribution: clawhub-compatible
  package-format: .skill
---

# Git History Secret Remediation

Use this skill when secrets have already been committed and the task is to inspect, scrub, verify, and coordinate git history cleanup.

Core tools:

- `gitleaks detect -v`
- `git filter-repo`

Bundled scripts:

- `scripts/list_git_refs.sh`
- `scripts/run_gitleaks_history_scan.sh`
- `scripts/backup_git_remotes.py`
- `scripts/restore_git_remotes.py`
- `scripts/run_filter_repo_redaction.sh`
- `scripts/run_history_remediation.sh`

## When To Use

Trigger this skill when the user asks to:

- scan commit history for secrets
- run `gitleaks detect -v`
- remove passwords, API keys, tokens, or private keys from git history
- run `git filter-repo`
- clean up old commits after a leak
- rewrite history and force-push the cleaned repository

## Safety Rules

1. Clean current `HEAD` first, then rewrite history.
2. Rotate real leaked credentials out-of-band. History cleanup is not secret rotation.
3. Prefer empty values or angle-bracket placeholders in tracked samples.
4. Do not use fake secret-looking placeholders such as `` when scanners still match them.
5. Treat history rewrite as destructive:
   - inventory refs first
   - expect force-push
   - warn that teammates must reclone or fully scrub old clones
6. Back up `git remote -v` before rewrite and restore it after rewrite or force-push preparation.

## Workflow

### 1. Inventory refs

At repo root:

```bash
bash skills/git-history-secret-remediation/scripts/list_git_refs.sh /path/to/repo
```

This tells you which branches and tags may need to be force-pushed after rewriting.

### 2. Run the history scan

Use the bundled wrapper:

```bash
bash skills/git-history-secret-remediation/scripts/run_gitleaks_history_scan.sh /path/to/repo
```

Behavior:

- auto-detects `config/gitleaks.toml` when present
- otherwise runs `gitleaks detect -v` with tool defaults

Classify findings into:

- current-file leaks still present in `HEAD`
- history-only leaks from deleted or renamed files

### 3. Sanitize current HEAD

Before rewriting history:

- replace real secrets in tracked sample/config files
- prefer:
  - `""`
  - empty env values
  - `<OPENSSH_PRIVATE_KEY_CONTENT>`
- keep real values only in local `.env` or a secret manager

### 4. Build a replace-text file

Create a temporary mapping file, for example:

```text
real-secret-1==>
real-secret-2==>
OPENSSH_PRIVATE_KEY_BEGIN_LINE==><OPENSSH_PRIVATE_KEY_BEGIN_LINE>
OPENSSH_PRIVATE_KEY_END_LINE==><OPENSSH_PRIVATE_KEY_END_LINE>
```

Notes:

- default replacement can be empty
- use explicit placeholders only when file syntax requires visible text
- if an old placeholder also triggers scanners, run a second rewrite replacing it with an empty string

### 5. Remove history-only artifact files when appropriate

If a file exists only as a leak artifact, prefer removing it from history entirely.

Examples:

- `leaks_github.json`
- obsolete docs that embed private-key examples
- scratch backup files that contain real credentials

### 6. Rewrite history

Use the bundled wrapper:

```bash
bash skills/git-history-secret-remediation/scripts/run_filter_repo_redaction.sh \
  /path/to/repo \
  /tmp/replace-text.txt \
  [path-to-remove...]
```

Behavior:

- backs up `git remote -v` metadata before rewriting
- restores remotes after rewriting if needed
- runs `git filter-repo --force --sensitive-data-removal --no-fetch`
- clears `.git/filter-repo/already_ran` when present
- optionally removes listed paths from history with `--invert-paths`

### 6b. Single-command remediation

If you already know the replacement mapping and the paths to purge, use the orchestrator:

```bash
bash skills/git-history-secret-remediation/scripts/run_history_remediation.sh \
  /path/to/repo \
  /tmp/replace-text.txt \
  [path-to-remove...]
```

Behavior:

- inventories refs
- runs a pre-scan
- rewrites history
- restores remotes
- re-runs `gitleaks`
- exits non-zero until the repo scans clean

### 7. Re-run gitleaks

Repeat until:

- real secrets are gone from all commits
- remaining findings, if any, are only deliberate placeholders you explicitly accept

### 8. Push rewritten refs

For normal repos with all relevant local branches:

```bash
git push --force origin --all
git push --force origin --tags
```

If the remote has important branches not present locally:

- create local tracking branches first
- or do the rewrite in a fresh mirror clone and push from there

Do not assume a normal non-bare clone can safely use `git push --mirror`.

### 9. Post-cleanup coordination

Always tell the user to:

- rotate leaked credentials
- purge or invalidate old access where relevant
- have other clones recloned or scrubbed
- notify repo admins if server-side cache or object cleanup is needed
- use the remote backup JSON when reconstructing remotes after force-push in a fresh clone
