-- Application login (meltkit pkg/auth). is_admin gates user administration,
-- disabled_at refuses a user on every authentication path.
ALTER TABLE users ADD COLUMN is_admin BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN disabled_at TIMESTAMPTZ;

-- The first human login becomes the admin: the row GetPrimaryUser returns,
-- which tagged tailnet nodes resolved to. 'local' (user 1) is the dev and MCP
-- stdio user.
UPDATE users SET is_admin = true
WHERE id = (
    SELECT id FROM users
    WHERE login LIKE '%@%'
    ORDER BY created_at ASC
    LIMIT 1
);
