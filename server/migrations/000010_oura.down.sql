DELETE FROM metric_allowlist WHERE category = 'oura';
DROP TABLE IF EXISTS oura_sync_state;
DROP TABLE IF EXISTS oura_tokens;
