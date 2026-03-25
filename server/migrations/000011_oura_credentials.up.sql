-- Add per-user Oura developer app credentials to the tokens table.
-- These were missing from the initial 000010 migration.
ALTER TABLE oura_tokens ADD COLUMN IF NOT EXISTS client_id TEXT NOT NULL DEFAULT '';
ALTER TABLE oura_tokens ADD COLUMN IF NOT EXISTS client_secret TEXT NOT NULL DEFAULT '';

-- Allow rows to exist with empty access_token (credentials saved but not yet authorized).
ALTER TABLE oura_tokens ALTER COLUMN access_token SET DEFAULT '';
ALTER TABLE oura_tokens ALTER COLUMN refresh_token SET DEFAULT '';
ALTER TABLE oura_tokens ALTER COLUMN expires_at SET DEFAULT '1970-01-01';
