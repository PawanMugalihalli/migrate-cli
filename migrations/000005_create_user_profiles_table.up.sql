-- depends_on: 000002_create_users_table,000006_create_user_roles_join_table

CREATE TABLE app.user_profiles (
    user_id INT PRIMARY KEY REFERENCES auth.users(id),
    first_name TEXT,
    last_name TEXT
);