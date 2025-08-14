-- depends_on: 000005_create_user_profiles_table, 000006_create_user_roles_join_table

CREATE TABLE app.admin_actions (
    id SERIAL PRIMARY KEY,
    admin_user_id INT NOT NULL REFERENCES auth.users(id),
    target_user_id INT NOT NULL REFERENCES auth.users(id),
    action_description TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);