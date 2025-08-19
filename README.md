# Messaging System Backend

A secure, real-time messaging backend built with Go (Gin). Supports user auth, DMs, group chats, message editing with handling of concurrency issues, and chat summarization using LLMs.

---

## Features

- JWT-based authentication
- Direct messages (DMs)
- Group chat with admin/member roles
- Chat summarization using LLMs
- Edit messages (DM and group)
- View chat previews and history
- Dockerized for easy setup

---

## Getting Started

### 1. Clone the repository

git clone https://github.com/PawanMugalihalli/MessagingSystemBackend.git  
cd MessagingSystemBackend






# Migrate CLI

A powerful, dependency-aware command-line tool for managing database schema migrations in Go, built to handle complex dependency graphs and provide a safe, predictable workflow.

---

## Features

- **Dependency Management**: Uses a topological sort to ensure migrations are always applied in the correct order, even with complex, non-sequential dependencies.

- **Safe Rollbacks**: A fully dependency-aware `goto` command correctly calculates the rollback order by reversing the application order, preventing an inconsistent database state.

- **SQL & Go Migrations**: Supports both standard `.sql` files and programmatic migrations written in `.go`, allowing for complex data seeding or logic.

- **High Performance**: Features concurrent file parsing for fast startup and intelligent in-memory caching to minimize redundant work within a single command.

- **Robust & Safe**: Uses a `dirty` flag to prevent new migrations from running if a previous one failed, ensuring the integrity of your schema.

---

**Usage and Use Cases**

This guide explains when and why to use each command.


**up**

Use Case: Applying new changes. This is the most common command to bring your database to the latest version.

-> docker compose run migrate -action=up -db=""

**goto**

Use Case: The power tool for precise version control. Use goto for complex rollbacks or up-migrations or for setting a database to a specific state.

-> docker compose run migrate -action=goto -version= -db=""

**down**

Use Case: A quick "undo". Use this to revert the last migration you applied.

-> docker compose run migrate -action=down -db=""

**status**

Use Case: Inspecting the database state. Run status to see a clear list of all applied migrations and check for failures

-> docker compose run migrate -action=status -db=""

**create**

Use Case: Starting a new schema change. Run this to generate the necessary .up and .down migration files.

-> docker compose run migrate -action=create -name=

**validate**

Use Case: Dry-running the entire migration files one-by-one to check if the dependency provided by the user is right or not.

-> docker compose run migrate -action=validate -db=

**validategraph**

Use Case: It validates the dependency graph provided by the user with the set of rules provided by the application to follow.

-> docker compsose run migrate -action=validategraph -db=

