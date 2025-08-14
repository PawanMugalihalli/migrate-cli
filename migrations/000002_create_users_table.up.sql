-- depends_on: 000001_create_schemas

CREATE TABLE auth.users (
    id SERIAL PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
