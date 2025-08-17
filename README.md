# Migrate CLI

A powerful, dependency-aware command-line tool for managing database schema migrations in Go, built to handle complex dependency graphs and provide a safe, predictable workflow.



## Features

-   **Dependency Management**: Uses a topological sort to ensure migrations are always applied in the correct order, even with complex, non-sequential dependencies.
-   **Safe Rollbacks**: A fully dependency-aware `goto` command correctly calculates the rollback order by reversing the application order, preventing an inconsistent database state.
-   **SQL & Go Migrations**: Supports both standard `.sql` files and programmatic migrations written in `.go`, allowing for complex data seeding or logic.
-   **High Performance**: Features concurrent file parsing for fast startup and intelligent in-memory caching to minimize redundant work within a single command.
-   **Robust & Safe**: Uses a `dirty` flag to prevent new migrations from running if a previous one failed, ensuring the integrity of your schema.

---

## Installation

Ensure you have the Go toolchain installed (version 1.23 or newer is recommended).

Run the following command in your terminal to install the tool:
```sh
go install [github.com/PawanMugalihalli/migrate-cli@latest](https://github.com/PawanMugalihalli/migrate-cli@latest)
