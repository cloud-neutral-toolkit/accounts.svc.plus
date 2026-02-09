# Runbook: P1 Dev Bug Fix - Rotating UUID Sync (2026-02-06)

## Incident Description
**Severity**: P1 (High)
**Issue**: Sandbox/Demo users reported inability to connect via VLESS after the hourly UUID rotation.
**Root Cause**: The Agent-Server configuration synchronization was incorrectly using the user's permanent `uuid` field instead of the rotating `proxy_uuid`.

## Remediation Steps

### Backend (`accounts.svc.plus`)
- Modified `internal/xrayconfig/source_gorm.go` to select `proxy_uuid` for Xray client generation.
- Updated database queries and row mapping to ensure the rotating ID is propagated to all nodes.
- Verified fix with unit tests in `internal/xrayconfig/source_gorm_test.go`.

### Frontend (`console.svc.plus`)
- Standardized node resolution logic in `VlessQrCard.tsx` and `agent.tsx`.
- Ensured that Sandbox guest accounts correctly prioritize the root-bound node while maintaining isolation from standard user logic.

## Verification
- **Automated**: Go tests for Xray sync passed.
- **Manual**: UI successfully displays the root-bound node as the primary connection point for Sandbox users.

## Maintenance
- Any future changes to the `User` model's identity fields must check the `xrayconfig` source logic for consistency.
