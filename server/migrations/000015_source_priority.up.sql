-- Per-user, per-category source priority configuration.
-- Category "_default" is the global fallback.
CREATE TABLE source_priority (
    user_id  INTEGER NOT NULL,
    category TEXT    NOT NULL,
    sources  TEXT[]  NOT NULL,
    PRIMARY KEY (user_id, category)
);
