-- depends_on: 000003_create_roles_table

CREATE TABLE auth.user_roles (
    user_id INT NOT NULL REFERENCES auth.users(id),
    role_id INT NOT NULL REFERENCES auth.roles(id),
    PRIMARY KEY (user_id, role_id)
);