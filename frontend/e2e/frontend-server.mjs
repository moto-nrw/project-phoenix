import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { spawn } from "node:child_process";

const HERE = dirname(fileURLToPath(import.meta.url));
const FRONTEND_DIR = resolve(HERE, "..");
const REPO_ROOT = resolve(FRONTEND_DIR, "..");
const MANIFEST_PATH = resolve(REPO_ROOT, "backend", ".e2e-manifest.json");

let frontendServer;
let shuttingDown = false;

function requireString(value, label) {
  if (typeof value === "string" && value.length > 0) {
    return value;
  }
  throw new Error(`${label} must be a non-empty string`);
}

function requireNumber(value, label) {
  if (typeof value === "number" && Number.isFinite(value)) {
    return value;
  }
  throw new Error(`${label} must be a finite number`);
}

function loadRuntime() {
  const parsed = JSON.parse(readFileSync(MANIFEST_PATH, "utf-8"));
  const runtime = parsed?.runtime;
  const primaryTenant = parsed?.tenants?.primary;

  if (!runtime || typeof runtime !== "object") {
    throw new Error(`missing runtime in ${MANIFEST_PATH}`);
  }
  if (!primaryTenant || typeof primaryTenant !== "object") {
    throw new Error(`missing primary tenant in ${MANIFEST_PATH}`);
  }

  return {
    backendUrl: requireString(runtime.backend_url, "manifest.runtime.backend_url"),
    tenantDomain: requireString(
      runtime.tenant_domain,
      "manifest.runtime.tenant_domain",
    ),
    frontendPort: requireNumber(
      runtime.frontend_port,
      "manifest.runtime.frontend_port",
    ),
    operatorHostname: requireString(
      runtime.operator_hostname,
      "manifest.runtime.operator_hostname",
    ),
    primarySlug: requireString(primaryTenant.slug, "manifest.tenants.primary.slug"),
  };
}

async function stopFrontend() {
  if (!frontendServer || frontendServer.exitCode !== null) {
    return;
  }

  await new Promise((resolvePromise) => {
    let settled = false;
    const resolveOnce = () => {
      if (settled) {
        return;
      }
      settled = true;
      resolvePromise();
    };

    frontendServer.once("exit", () => {
      clearTimeout(timer);
      resolveOnce();
    });

    const timer = setTimeout(() => {
      if (frontendServer && frontendServer.exitCode === null) {
        frontendServer.kill("SIGKILL");
        return;
      }
      resolveOnce();
    }, 10_000);

    frontendServer.kill("SIGTERM");
  });
}

async function shutdown(exitCode, errorMessage) {
  if (shuttingDown) {
    return;
  }
  shuttingDown = true;

  if (errorMessage) {
    console.error(`[E2E Frontend] ${errorMessage}`);
  }

  await stopFrontend();
  process.exit(exitCode);
}

process.on("SIGTERM", () => {
  void shutdown(0);
});

process.on("SIGINT", () => {
  void shutdown(0);
});

try {
  const runtime = loadRuntime();
  const origin = `http://${runtime.primarySlug}.${runtime.tenantDomain}:${runtime.frontendPort}`;

  frontendServer = spawn(
    "pnpm",
    ["run", "dev", "--port", String(runtime.frontendPort)],
    {
      cwd: FRONTEND_DIR,
      env: {
        ...process.env,
        SKIP_ENV_VALIDATION: "true",
        PORT: String(runtime.frontendPort),
        API_URL: runtime.backendUrl,
        NEXT_PUBLIC_API_URL: runtime.backendUrl,
        TENANT_DOMAIN: runtime.tenantDomain,
        NEXT_PUBLIC_TENANT_DOMAIN: runtime.tenantDomain,
        NEXT_PUBLIC_OPERATOR_HOSTNAME: runtime.operatorHostname,
        NEXTAUTH_URL: origin,
        NEXTAUTH_SECRET:
          process.env.NEXTAUTH_SECRET ?? "project-phoenix-e2e-nextauth-secret",
        AUTH_TRUST_HOST: process.env.AUTH_TRUST_HOST ?? "true",
      },
      stdio: "inherit",
    },
  );

  frontendServer.on("error", (error) => {
    if (shuttingDown) {
      return;
    }
    void shutdown(1, `frontend dev server failed to start: ${error.message}`);
  });

  frontendServer.on("exit", (code, signal) => {
    if (shuttingDown) {
      return;
    }

    const reason =
      signal !== null ? `signal ${signal}` : `exit code ${code ?? 1}`;
    void shutdown(1, `frontend dev server exited early with ${reason}`);
  });
} catch (error) {
  void shutdown(
    1,
    error instanceof Error ? error.message : String(error),
  );
}
