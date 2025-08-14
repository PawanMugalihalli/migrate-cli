-- depends_on: 000001_create_schemas

CREATE TABLE auth.roles (
    id SERIAL PRIMARY KEY,
    name TEXT UNIQUE NOT NULL
);