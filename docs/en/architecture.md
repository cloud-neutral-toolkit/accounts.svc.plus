# Architecture

Use this page as the English entry point into the shared bilingual architecture documents.

## Current Architecture In One View

`accounts.svc.plus` is a Gin-based Go service that combines:

- identity and session management,
- admin control-plane operations,
- agent and Xray configuration control,
- and usage / billing read models.

The actual startup chain is centered in `cmd/accountsvc/main.go`, where the service wires the primary `store.Store`, the GORM-backed admin DB, optional mailer and token service, the agent registry, and the optional Xray periodic syncer before calling `api.RegisterRoutes`.

## Read In This Order

1. [Architecture overview](../architecture/overview.md)
   This is the main runtime story: startup, request flow, session ownership, agent reporting, and Xray config generation.
2. [Components](../architecture/components.md)
   Use this when you need the ownership map by package and dependency direction.
3. [Design decisions](../architecture/design-decisions.md)
   Use this when you need the current tradeoffs, not just the shape of the system.

## Key Architecture Themes

- Session-first control plane with optional JWT middleware.
- `store.Store` as the core business persistence abstraction.
- Selective GORM usage for admin settings, homepage video, sandbox binding, and tenant / XWorkmate models.
- Agent status and sandbox bindings projected into `agentserver.Registry`.
- Xray config generated from database state through `xrayconfig.Generator` and optionally converged by `PeriodicSyncer`.

## Related Pages

- [Design](design.md)
- [Developer Guide](developer-guide.md)
- [API overview](../api/overview.md)
