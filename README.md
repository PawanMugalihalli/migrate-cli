\# Migrate CLI

A powerful, dependency-aware command-line tool for managing database schema migrations in Go, built to handle complex dependency graphs and provide a safe, predictable workflow.

\## Features

\- \*\*Dependency Management\*\*: Uses a topological sort to ensure migrations are always applied in the correct order, even with complex, non-sequential dependencies.

\- \*\*Safe Rollbacks\*\*: A fully dependency-aware \`goto\` command correctly calculates the rollback order by reversing the application order, preventing an inconsistent database state.

\- \*\*SQL & Go Migrations\*\*: Supports both standard \`.sql\` files and programmatic migrations written in \`.go\`, allowing for complex data seeding or logic.

\- \*\*High Performance\*\*: Features concurrent file parsing for fast startup and intelligent in-memory caching to minimize redundant work within a single command.

\- \*\*Robust & Safe\*\*: Uses a \`dirty\` flag to prevent new migrations from running if a previous one failed, ensuring the integrity of your schema.

\---

\## Installation

Ensure you have the Go toolchain installed (version 1.23 or newer is recommended).

Run the following command in your terminal to install the tool:

go install github.com/PawanMugalihalli/migrate-cli@latest

\---

Usage and Use Cases

This guide explains when and why to use each command. All commands require a database connection string passed via the -db flag.

create

Use Case: Starting a new schema change. Run this to generate the necessary .up and .down migration files.

migrate-cli -action=create -name=

up

Use Case: Applying new changes. This is the most common command to bring your database to the latest version.

migrate-cli -action=up -db=""

down

Use Case: A quick "undo" during development. Use this to revert the last migration you applied.

migrate-cli -action=down -db=""

goto

Use Case: The power tool for precise version control. Use goto for complex rollbacks or for setting a database to a specific older state for testing.

migrate-cli -action=goto -version= -db=""

status

Use Case: Inspecting and debugging the database state. Run status to see a clear list of all applied migrations and check for failures

migrate-cli -action=status -db=""

