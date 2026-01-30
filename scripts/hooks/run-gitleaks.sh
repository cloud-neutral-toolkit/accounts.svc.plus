#!/bin/bash

# run-gitleaks.sh - Wrapper for gitleaks detection modes
# Usage: ./run-gitleaks.sh [staged|full]

MODE=${1:-staged}
GITLEAKS_BIN=${GITLEAKS_BIN:-gitleaks}
GITLEAKS_CONFIG=${GITLEAKS_CONFIG:-config/gitleaks.toml}
GITLEAKS_REPORT_DIR=${GITLEAKS_REPORT_DIR:-.git/gitleaks}
REPORT_PATH="$GITLEAKS_REPORT_DIR/report.json"

# Check if gitleaks is installed
if ! command -v "$GITLEAKS_BIN" &> /dev/null; then
    echo "Error: gitleaks is not installed or not in PATH."
    echo "Install it from: https://github.com/gitleaks/gitleaks/releases"
    exit 1
fi

# Create report directory
mkdir -p "$GITLEAKS_REPORT_DIR"

if [ "$MODE" == "staged" ]; then
    echo "Running Gitleaks protect on staged changes (respecting .gitignore)..."
    # 'protect --staged' is the recommended way to scan staged changes
    "$GITLEAKS_BIN" protect -v --staged --redact --config "$GITLEAKS_CONFIG" --report-path "$REPORT_PATH"
    EXIT_CODE=$?
else
    echo "Running full Gitleaks detection on commits..."
    # 'detect' scans the git history (commits). In-progress worktree files
    # that are .gitignored will NOT be scanned unless they were previously committed.
    # To ignore specific files/folders always, use config/gitleaks.toml [allowlist]
    "$GITLEAKS_BIN" detect -v --redact --config "$GITLEAKS_CONFIG" --report-path "$REPORT_PATH"
    EXIT_CODE=$?
fi

if [ $EXIT_CODE -ne 0 ]; then
    echo ""
    echo "⚠️ Potential secrets detected!"
    echo "Report generated at: $REPORT_PATH"
    echo ""
    echo "Please review the findings and refer to the security playbook:"
    echo "skills/git.secret-incident-response.v1.md"
    exit $EXIT_CODE
else
    echo "✅ No secrets detected."
    # Clean up empty report
    [ -f "$REPORT_PATH" ] && [ ! -s "$REPORT_PATH" ] && rm "$REPORT_PATH"
    exit 0
fi
