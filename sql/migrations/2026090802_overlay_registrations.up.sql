-- XConnect One self-registration is an owner-approved identity declaration.
-- Raw registration tokens, device credentials, enrollment sessions and private
-- keys are never stored in this table.
BEGIN;

CREATE TABLE IF NOT EXISTS public.overlay_registrations (
  id TEXT PRIMARY KEY,
  network_id TEXT NOT NULL,
  owner_user_id TEXT NOT NULL,
  device_id TEXT NOT NULL,
  name TEXT NOT NULL DEFAULT '',
  hostname TEXT NOT NULL DEFAULT '',
  platform TEXT NOT NULL,
  wireguard_public_key TEXT NOT NULL,
  wireguard_public_key_fingerprint TEXT NOT NULL,
  token_hash TEXT NOT NULL UNIQUE,
  status TEXT NOT NULL CHECK (status IN ('pending', 'approved', 'rejected', 'expired', 'consumed')),
  expires_at TIMESTAMPTZ NOT NULL,
  approved_at TIMESTAMPTZ,
  rejected_at TIMESTAMPTZ,
  consumed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS overlay_registrations_owner_created_idx
  ON public.overlay_registrations (owner_user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS overlay_registrations_network_pending_idx
  ON public.overlay_registrations (network_id, status, expires_at);
CREATE INDEX IF NOT EXISTS overlay_registrations_network_created_idx
  ON public.overlay_registrations (network_id, created_at);
CREATE INDEX IF NOT EXISTS overlay_registrations_identity_pending_idx
  ON public.overlay_registrations (network_id, device_id, wireguard_public_key_fingerprint, status, expires_at);

COMMIT;
