# Accounts Service Plus Documentation

This documentation set now covers the current engineering reality of `accounts.svc.plus` instead of placeholder summaries. The detailed pages under `docs/architecture/*`, `docs/api/*`, and `docs/development/*` are shared bilingual pages, while `docs/en/*` remains the English entry layer.

## What Is Covered

- System design from `cmd/accountsvc/main.go` through API routing, store access, agent registry, and Xray sync.
- Package and type ownership for `api`, `internal/store`, `internal/auth`, `internal/service`, `internal/xrayconfig`, `internal/agentmode`, `internal/agentserver`, and `internal/agentproto`.
- HTTP contracts including request fields, response fields, auth mode, owner handler file, and error conventions.

## Recommended Reading Paths

### For architecture readers

1. [Architecture](architecture.md)
2. [Detailed startup and runtime flow](../architecture/overview.md)
3. [Component ownership map](../architecture/components.md)
4. [Design decisions](../architecture/design-decisions.md)

### For API and integration readers

1. [Developer Guide](developer-guide.md)
2. [API overview](../api/overview.md)
3. [Authentication and authorization](../api/auth.md)
4. [Endpoint matrix](../api/endpoints.md)
5. [Error conventions](../api/errors.md)

### For codebase readers

1. [Developer Guide](developer-guide.md)
2. [Code structure reference](../development/code-structure.md)
3. [Testing baseline](../development/testing.md)

## Canonical Entry Pages

- [Architecture](architecture.md)
- [Design](design.md)
- [Deployment](deployment.md)
- [User Guide](user-guide.md)
- [Developer Guide](developer-guide.md)
- [Vibe Coding Reference](vibe-coding-reference.md)

## Notes

- Detailed subsystem pages are intentionally shared instead of duplicated into `docs/en/api/*` or `docs/en/architecture/*`.
- The validation baseline for the current docs set is `go test ./...`; see [testing](../development/testing.md).
