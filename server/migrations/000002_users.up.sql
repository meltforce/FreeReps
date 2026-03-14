CREATE TABLE users (
    id           SERIAL PRIMARY KEY,
    login        TEXT NOT NULL UNIQUE,
    display_name TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed user_id=1 for existing data (dev/local mode fallback)
INSERT INTO users (id, login, display_name) VALUES (1, 'local', 'Local Dev User');
SELECT setval('users_id_seq', (SELECT MAX(id) FROM users));
