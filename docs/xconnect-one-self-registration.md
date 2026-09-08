# XConnect One self-registration

Accounts is the formal XConnect Zero control plane and remains the only source
of networks, owner assignment, device records, credentials, signed
configuration, policy and acknowledgement state. This API does not modify
Gateway enrollment or the existing invitation flow.

`POST /api/overlay/v1/registrations` accepts a One identity declaration for an
existing public network ID. The caller supplies a device ID, optional display
name and hostname, platform (`linux`, `darwin`, or `windows`) and a WireGuard
public key. Owner and role are never accepted from the caller: the service
copies the owner from the stored network and fixes the requested role to One.
The public key and fingerprint identify a declaration; neither proves hardware
or person identity.

The creation response contains a one-time 256-bit registration token and a
15-minute absolute expiry. Accounts stores only its SHA-256 digest. Pending
registration neither creates a device nor grants an address, peer, credential,
enrollment token, configuration, route or access to the VPN. Pending requests
are bounded per network and per requested identity in database transactions.
The independent bounds are 20 still-live pending registrations, 20 creation
attempts in every rolling 15-minute window, and 100 creation attempts in every
rolling 24-hour window. Creation counters include rejected, expired and
consumed records, so changing terminal state cannot evade the budget.
Registration history is retained as owner-scoped audit data; this API never
deletes users, networks or devices.

An active account user with `xconnect.zero.manage` can list only registrations
for their owned networks. Approval requires an explicit matching `network_id`;
approval only changes `pending` to `approved`. Rejection also changes only
state. A stale approval or rejection returns a conflict so the Portal refreshes.
Cross-owner and mismatched-network requests return not found.

Listing is bounded to the newest 100 records and returns `has_more` plus an
opaque `next_cursor` when another page is available. A record is visible only
when its stored owner snapshot and the network's current owner both match the
caller. A network transfer therefore reveals no prior registration metadata to
the new owner and leaves none visible to the old owner.

The device polls `POST /api/overlay/v1/registrations/{registration_id}/exchange`
with `Authorization: Bearer <registration_token>`. Pending returns 202. After
approval, a single transaction consumes the registration and creates the One
device, credential and enrollment session, then returns the existing
`ExchangeResponse`; normal signed sync and ACK follow. Rejected and consumed
registrations return 409; expired registrations return 410. Invalid or
mismatched tokens return 401 without revealing state. Responses that carry or
depend on a registration token use `Cache-Control: no-store`.

Expiry is absolute for pending and approved registrations. Rejected and
consumed records remain terminal conflicts (409) even after their expiry time;
only pending or approved records can transition to expired (410).

## Deployment

The durable table is introduced by
`sql/migrations/2026090802_overlay_registrations.up.sql`. Release deployments
must run `migratectl migrate --dir sql/migrations` (or the repository's
`scripts/migrate-db.sh`) before serving registration traffic. Accounts startup
also includes the record in its GORM `AutoMigrate` set for compatible local/UAT
bootstrap, but the versioned migration remains the production schema authority.
