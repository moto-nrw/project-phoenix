#!/usr/bin/env python3
"""Create the encrypted demo environment once, with fresh application secrets.

SMTP settings come from staging. Plaintext passes through memory and pipes only.
"""

from pathlib import Path
import secrets
import subprocess


ROOT = Path(__file__).resolve().parents[1]


def main():
    target = ROOT / "environments/demo.sops.env"
    if target.exists():
        raise SystemExit("Demo configuration already exists; use sops edit to change it")
    source = subprocess.run(
        ["sops", "decrypt", "environments/staging.sops.env"],
        cwd=ROOT, capture_output=True, text=True, check=True,
    ).stdout
    values = dict(line.split("=", 1) for line in source.splitlines()
                  if "=" in line and not line.startswith("#"))
    for key in ("POSTGRES_PASSWORD", "PHOENIX_AUTH_PASSWORD", "PHOENIX_DEMO_PASSWORD", "AUTH_JWT_SECRET",
                "NEXTAUTH_SECRET", "METRICS_BEARER_TOKEN", "ADMIN_PASSWORD", "OPERATOR_PASSWORD"):
        values[key] = secrets.token_hex(32) + "aA1!"
    values.update({
        "DB_DSN": "postgres://postgres:" + values["POSTGRES_PASSWORD"] + "@postgres:5432/postgres?sslmode=disable",
        "DEMO_DB_DSN": "postgres://phoenix_demo@postgres:5432/postgres?sslmode=disable",
        "APP_ENV": "demo", "NEXT_PUBLIC_APP_ENV": "demo", "LOG_LEVEL": "info", "DB_DEBUG": "false",
        "NEXT_PUBLIC_API_URL": "https://api.demo.moto-app.de", "NEXTAUTH_URL": "https://demo.moto-app.de",
        "TENANT_DOMAIN": "demo.moto-app.de", "NEXT_PUBLIC_TENANT_DOMAIN": "demo.moto-app.de",
        "NEXT_PUBLIC_OPERATOR_HOSTNAME": "operator.demo.moto-app.de",
        "NEXT_PUBLIC_PARENTS_HOSTNAME": "eltern.demo.moto-app.de",
        "NEXT_PUBLIC_SCHOOL_HOSTNAME": "schule.demo.moto-app.de",
        "FRONTEND_URL": "https://demo.moto-app.de", "PARENTS_URL": "https://eltern.demo.moto-app.de",
        "SCHOOL_URL": "https://schule.demo.moto-app.de", "EMAIL_FROM_NAME": "moto Demo",
        "ADMIN_EMAIL": "admin@demo.moto-app.de", "OPERATOR_EMAIL": "operator@demo.moto-app.de",
        # The demo seeder writes the PIN into security.ogs_device_pin, which
        # accepts exactly four digits.
        "OPERATOR_DISPLAY_NAME": "Demo Operator", "OGS_DEVICE_PIN": f"{secrets.randbelow(10000):04d}",
        "SECURITY_LOGGING_ENABLED": "true", "RATE_LIMIT_ENABLED": "true", "SKIP_ENV_VALIDATION": "false",
        # The demo request form posts from the marketing website, so its origins
        # belong here too. The encrypted file is authoritative once it exists;
        # keep this list in sync with it.
        "CORS_ALLOWED_ORIGINS": "https://demo.moto-app.de,https://*.demo.moto-app.de,"
                                "https://moto-ogs.de,https://www.moto-ogs.de,https://staging.moto-ogs.de,"
                                "https://moto.nrw,https://www.moto.nrw",
        # SENTRY_DSN stays the staging value: backend events of all environments go
        # to one project and filter apart by APP_ENV.
        "NEXT_PUBLIC_SENTRY_DSN": "",
        "NEXT_PUBLIC_SENTRY_ENVIRONMENT": "demo", "POSTHOG_API_KEY": "", "NEXT_PUBLIC_POSTHOG_KEY": "",
        "VAPID_PUBLIC_KEY": "", "VAPID_PRIVATE_KEY": "", "VAPID_SUBSCRIBER": "",
    })
    result = subprocess.run(
        ["sops", "encrypt", "--input-type", "dotenv", "--output-type", "dotenv",
         "--filename-override", "environments/demo.sops.env", "--output", str(target), "/dev/stdin"],
        cwd=ROOT, input="\n".join(key + "=" + value for key, value in values.items()) + "\n",
        capture_output=True, text=True,
    )
    if result.returncode:
        raise SystemExit("SOPS encryption failed; no secret values displayed")
    print("Created encrypted demo configuration with independent application secrets")


if __name__ == "__main__":
    main()
