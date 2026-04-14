# Developer Guide

Use this page as the English navigation layer for the shared bilingual engineering references.

## Start Here

1. [Code structure reference](../development/code-structure.md)
2. [API overview](../api/overview.md)
3. [Authentication and authorization](../api/auth.md)
4. [Endpoint matrix](../api/endpoints.md)
5. [Error conventions](../api/errors.md)
6. [Testing baseline](../development/testing.md)

## What Each Detailed Page Answers

- [Code structure reference](../development/code-structure.md)
  Which package owns what, which exported types matter, which non-exported owners carry runtime behavior, and how the core packages connect.
- [API overview](../api/overview.md)
  How route families are organized, where handlers live, and how authentication layers stack.
- [Authentication and authorization](../api/auth.md)
  Request and response fields for session login, MFA, OAuth exchange, JWT refresh, password reset, and XWorkmate secret flows.
- [Endpoint matrix](../api/endpoints.md)
  Method, path, owner file, auth mode, request parameters, response shape, and dependency wiring for the current route set.
- [Testing baseline](../development/testing.md)
  The verification baseline for docs and code alignment, centered on `go test ./...`.

## Validation Baseline

The documentation was aligned against current source, then validated with:

```bash
go test ./...
```

If you update routes, type signatures, or package ownership, update the detailed pages above in the same change.

## Related Pages

- [Architecture](architecture.md)
- [Design](design.md)
- [Development setup](../development/dev-setup.md)
- [Contributing](../development/contributing.md)
