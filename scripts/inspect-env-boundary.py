#!/usr/bin/env python3
"""Read dotenv from stdin; report names and non-secret DSN properties only."""

import sys
from urllib.parse import parse_qs, urlsplit


def main():
    values = {}
    for line in sys.stdin:
        if line.strip() and not line.startswith("#") and "=" in line:
            key, value = line.rstrip("\n").split("=", 1)
            values[key] = value
    print("Variable names: " + ", ".join(sorted(values)))
    print("Empty variable names: " + ", ".join(sorted(key for key, value in values.items() if not value)))
    for key in ("DB_DSN", "TEST_DB_DSN"):
        if key not in values:
            continue
        try:
            dsn = urlsplit(values[key])
            query = parse_qs(dsn.query)
            print(key, {
                "postgres_role": dsn.username == "postgres",
                "application_role": dsn.username == "phoenix_auth",
                "compose_host": dsn.hostname == "postgres",
                "default_port": dsn.port in (None, 5432),
                "postgres_database": dsn.path == "/postgres",
                "tls_required": query.get("sslmode") == ["require"],
                "tls_disabled": query.get("sslmode") == ["disable"],
                "query_keys": sorted(query),
            })
        except ValueError:
            print(key + ": invalid URL (value suppressed)")


if __name__ == "__main__":
    main()
