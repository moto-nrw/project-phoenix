#!/usr/bin/env python3
"""Rehearse deployment boundaries with fake data in a unique Compose project."""

import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import uuid


ROOT = Path(__file__).resolve().parents[1]


def main():
    project = "phoenix-env-rehearsal-" + uuid.uuid4().hex[:10]
    spec = importlib.util.spec_from_file_location("boundary", ROOT / "scripts/check-runtime-env.py")
    boundary = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(boundary)
    config = boundary.compose_config(ROOT / "environments/staging.compose.yml", True)
    config.pop("name", None)
    config.pop("volumes", None)
    services = config["services"]
    for name, service in services.items():
        for key in ("ports", "volumes", "logging", "restart"):
            service.pop(key, None)
        if name in ("server", "migrate"):
            service["image"] = project + "-server"
            service["build"] = {"context": str(ROOT / "backend"), "dockerfile": "Dockerfile"}
        if "APP_ENV" in service.get("environment", {}):
            service["environment"]["APP_ENV"] = "development"
    services["postgres"]["tmpfs"] = ["/var/lib/postgresql/data"]
    services["postgres"]["environment"]["POSTGRES_PASSWORD"] = "rehearsal-postgres-fixture"
    services["migrate"]["environment"].update({
        "DB_DSN": "postgres://postgres:rehearsal-postgres-fixture@postgres:5432/postgres?sslmode=disable",
        "ADMIN_EMAIL": "admin@example.invalid", "ADMIN_PASSWORD": "Rehearsal-admin-fixture-42!",
        "OPERATOR_EMAIL": "operator@example.invalid", "OPERATOR_PASSWORD": "Rehearsal-operator-fixture-42!",
        "OPERATOR_DISPLAY_NAME": "Rehearsal Operator",
    })
    services["server"]["environment"]["POSTHOG_API_KEY"] = ""
    services["frontend"]["environment"]["NEXT_PUBLIC_POSTHOG_KEY"] = ""
    frontend_args = {key: value for key, value in services["frontend"]["environment"].items()
                     if key.startswith("NEXT_PUBLIC_") or key in ("API_URL", "TENANT_DOMAIN")}
    services["frontend"]["image"] = project + "-frontend"
    services["frontend"]["build"] = {
        "context": str(ROOT / "frontend"), "dockerfile": "Dockerfile.prod", "args": frontend_args,
    }
    services["mailpit"] = {"image": "axllent/mailpit:v1.30", "networks": ["staging"]}
    with tempfile.TemporaryDirectory(prefix=project) as temporary:
        directory = Path(temporary)
        compose_file = directory / "compose.json"
        compose_file.write_text(json.dumps(config))
        environment = dict(os.environ, COMPOSE_FILE=str(compose_file), COMPOSE_PROJECT_NAME=project)
        log = directory / "rehearsal.log"

        def run(arguments, *, data=None):
            result = subprocess.run(arguments, env=environment, cwd=directory,
                                    input=data, capture_output=True)
            with log.open("ab") as output:
                output.write(result.stdout)
                output.write(result.stderr)
            if result.returncode:
                # All inputs are fixtures, but report only the failing command's operation.
                raise RuntimeError("failed operation: " + arguments[0] + " " + arguments[1])
            return result.stdout

        def compose(*arguments, data=None):
            return run(["docker", "compose", *arguments], data=data)

        try:
            print(project + ": build production images", flush=True)
            compose("build", "server", "frontend")
            print("Start disposable PostgreSQL; run migration job", flush=True)
            compose("up", "-d", "--wait", "postgres", "mailpit")
            compose("run", "--rm", "migrate")
            print("Start serving backend and frontend with deployment allowlists", flush=True)
            compose("up", "-d", "--wait", "--wait-timeout", "120", "server", "frontend")
            for name in ("postgres", "server", "frontend"):
                output = compose("exec", "-T", name, "env").decode()
                keys = sorted(line.split("=", 1)[0] for line in output.splitlines() if "=" in line)
                print(name + " runtime names: " + ", ".join(keys), flush=True)
                forbidden = {"AUTH_JWT_SECRET", "DB_DSN", "POSTGRES_PASSWORD", "PHOENIX_AUTH_PASSWORD"}
                if name == "frontend" and forbidden.intersection(keys):
                    raise RuntimeError("frontend inherited forbidden credentials")
                if name == "server" and "POSTGRES_PASSWORD" in keys:
                    raise RuntimeError("server inherited superuser password")
            password = services["server"]["environment"]["PHOENIX_AUTH_PASSWORD"]
            role_sql = """SELECT current_user = 'phoenix_auth' AND NOT rolsuper AND NOT rolbypassrls
                FROM pg_roles WHERE rolname = current_user;
                BEGIN; SET LOCAL ROLE phoenix_tenant; SELECT current_user = 'phoenix_tenant'; ROLLBACK;
                BEGIN; SET LOCAL ROLE phoenix_admin; SELECT current_user = 'phoenix_admin'; ROLLBACK;"""
            role_output = compose("exec", "-T", "postgres", "env", "PGPASSWORD=" + password,
                                  "psql", "-h", "postgres", "-U", "phoenix_auth", "-d", "postgres",
                                  "-At", "-v", "ON_ERROR_STOP=1", "-c", role_sql)
            if role_output.decode().splitlines().count("t") != 3:
                raise RuntimeError("application connection/tenant/admin role checks failed")
            print("Application connection and tenant/admin transactions: PASS", flush=True)
            compose("stop", "server", "frontend")
            marker = "CREATE TABLE public.env_rehearsal (value text); INSERT INTO public.env_rehearsal VALUES ('fixture');"
            compose("exec", "-T", "postgres", "psql", "-U", "postgres", "-v", "ON_ERROR_STOP=1", "-c", marker)
            backup = directory / "backup-rehearsal.dump"
            backup.write_bytes(compose("exec", "-T", "postgres", "pg_dump", "-U", "postgres", "-d", "postgres", "-Fc"))
            globals_file = directory / "globals-rehearsal.sql"
            globals_file.write_bytes(compose("exec", "-T", "postgres", "pg_dumpall", "-U", "postgres", "--globals-only"))
            globals_file.chmod(0o600)
            run(["bash", str(ROOT / "scripts/restore-db.sh"), str(backup)])
            restored = compose("exec", "-T", "postgres", "psql", "-U", "postgres", "-At", "-c",
                               "SELECT value = 'fixture' FROM public.env_rehearsal")
            if restored.strip() != b"t":
                raise RuntimeError("backup marker did not survive restore")
            compose("up", "-d", "--wait", "--wait-timeout", "120", "server", "frontend")
            print("Migration, backup, restore, and post-restore service health: PASS", flush=True)
        except Exception:
            # Keep fixture-only diagnostic logs, never real environment data.
            saved_log = ROOT / ".scratch/runtime-env-rehearsal.log"
            saved_log.parent.mkdir(exist_ok=True)
            saved_log.write_bytes(log.read_bytes() if log.exists() else b"")
            print("Fixture-only diagnostic log: " + str(saved_log), flush=True)
            raise
        finally:
            compose("down", "--volumes", "--remove-orphans")


if __name__ == "__main__":
    main()
