#!/usr/bin/env python3
"""Verify deployed service environment boundaries without displaying values."""

import argparse
import json
import os
import re
from pathlib import Path
import subprocess
import sys
import tempfile
from urllib.parse import parse_qs, urlsplit


ROOT = Path(__file__).resolve().parents[1]
OPTIONAL_VALUES = {
    "EMAIL_SMTP_USER", "EMAIL_SMTP_PASSWORD", "NEXT_PUBLIC_POSTHOG_KEY",
    "NEXT_PUBLIC_POSTHOG_HOST", "NEXT_PUBLIC_SENTRY_DSN", "NEXT_PUBLIC_SENTRY_ENVIRONMENT",
    "SENTRY_DSN", "SENTRY_ENVIRONMENT", "POSTHOG_API_KEY", "POSTHOG_HOST",
    "VAPID_PUBLIC_KEY", "VAPID_PRIVATE_KEY", "VAPID_SUBSCRIBER",
}


def compose_config(path, interpolate, env_file=ROOT / ".env.example"):
    command = ["docker", "compose", "--profile", "maintenance", "--env-file", str(env_file),
               "-f", str(path), "config", "--format", "json", "--no-env-resolution"]
    if not interpolate:
        command.append("--no-interpolate")
    # Keep Docker connectivity, but never inherit shell application credentials.
    environment = {key: os.environ[key] for key in (
        "PATH", "HOME", "DOCKER_HOST", "DOCKER_CONTEXT", "DOCKER_CONFIG",
        "XDG_RUNTIME_DIR", "SSH_AUTH_SOCK",
    ) if key in os.environ}
    result = subprocess.run(command, capture_output=True, text=True, env=environment)
    if result.returncode:
        missing = sorted(set(re.findall(r"([A-Z][A-Z0-9_]*) is required", result.stderr)))
        raise ValueError("Compose validation failed; missing names: " + ", ".join(missing))
    return json.loads(result.stdout)


def check(path, target, expected):
    raw = compose_config(path, False)
    resolved = compose_config(path, True)
    services = raw["services"]
    for name, service in services.items():
        if service.get("env_file"):
            raise ValueError(name + ": whole-file environment injection is forbidden")
    if set(services) != set(expected):
        raise ValueError("Unexpected service set")
    for name, allowed in expected.items():
        service = services[name]
        actual = set(service.get("environment", {}))
        if actual != set(allowed):
            raise ValueError(name + ": allowlist mismatch; names: " +
                             ", ".join(sorted(actual.symmetric_difference(allowed))))
        if any(".env" in str(volume.get("source", "")) for volume in service.get("volumes", [])):
            raise ValueError(name + ": dotenv mounts are forbidden")
        for key, value in service["environment"].items():
            if key in ("DB_DSN", "API_URL", "PORT", "HOSTNAME") and name != "migrate":
                continue
            operator = "?" if key in OPTIONAL_VALUES else ":?"
            if value != "${" + key + operator + key + " is required}":
                raise ValueError(name + ": missing fail-fast binding for " + key)
        print(target + " " + name + ": " + ", ".join(sorted(actual)))
    dsn = urlsplit(resolved["services"]["server"]["environment"]["DB_DSN"])
    if (dsn.scheme != "postgres" or dsn.username != "phoenix_auth" or
            dsn.password is not None or dsn.hostname != "postgres" or
            dsn.port != 5432 or dsn.path != "/postgres" or
            parse_qs(dsn.query) != {"sslmode": ["require" if target == "production" else "disable"]}):
        raise ValueError("server: DSN must contain only the application role and expected endpoint")
    if services["migrate"].get("profiles") != ["maintenance"]:
        raise ValueError("migrate: must be an explicit maintenance job")
    if services["migrate"]["command"] != ["./main", "migrate"]:
        raise ValueError("migrate: unexpected command")
    if services["migrate"]["image"] != services["server"]["image"]:
        raise ValueError("migrate: must use the serving backend image")


def check_missing_config(path, directory):
    lines = (ROOT / ".env.example").read_text().splitlines()
    for key in ("NEXTAUTH_SECRET", "PHOENIX_AUTH_PASSWORD", "POSTGRES_PASSWORD", "DB_DSN"):
        fixture = Path(directory) / "missing.env"
        fixture.write_text("\n".join(line for line in lines if not line.startswith(key + "=")) + "\n")
        try:
            compose_config(path, True, fixture)
        except ValueError as error:
            if key not in str(error):
                raise ValueError("Missing configuration failed without naming " + key) from None
        else:
            raise ValueError("Missing configuration was accepted: " + key)
    print("Missing required configuration: rejected with key names")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--revision", help="Check a historical Compose revision against the current policy")
    args = parser.parse_args()
    expected = json.loads((ROOT / "environments/runtime-env-allowlist.json").read_text())
    try:
        with tempfile.TemporaryDirectory(prefix="phoenix-env-check-") as directory:
            for target in ("staging", "production"):
                relative = "environments/" + target + ".compose.yml"
                path = ROOT / relative
                if args.revision:
                    result = subprocess.run(["git", "show", args.revision + ":" + relative],
                                            cwd=ROOT, capture_output=True, text=True, check=True)
                    path = Path(directory) / (target + ".yml")
                    path.write_text(result.stdout)
                check(path, target, expected)
                check_missing_config(path, directory)
        print("Runtime environment boundaries: PASS")
    except (ValueError, KeyError, subprocess.CalledProcessError) as error:
        print("Runtime environment boundaries: FAIL: " + str(error), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
