-- How many health_metrics rows the client of a metrics checkpoint has stored.
-- The iOS app shows it per category; a count over the hypertable per request
-- would scan every chunk.
--
-- The ingest path adds the rows an insert created. NULL means not counted yet:
-- RecountIngestCheckpoints fills it from health_metrics, in time windows. Rows
-- deleted or inserted outside the ingest path are not tracked; after such a
-- change, set row_count to NULL and the next sync-state request recounts.
ALTER TABLE ingest_checkpoints ADD COLUMN row_count BIGINT;
