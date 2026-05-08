import { existsSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { spawn, spawnSync } from "node:child_process";

const HERE = dirname(fileURLToPath(import.meta.url));
const FRONTEND_DIR = resolve(HERE, "..");
const REPO_ROOT = resolve(FRONTEND_DIR, "..");
const COMPOSE_FILE = resolve(REPO_ROOT, "docker-compose.example.yml");
const ROOT_ENV_FILE = resolve(REPO_ROOT, ".env");
const MANIFEST_PATH = resolve(REPO_ROOT, "backend", ".e2e-manifest.json");

let frontendServer;
let shuttingDown = false;

function requireFile(path, hint) {
  if (existsSync(path)) {
    return;
  }

  throw new Error(`missing required file: ${path}\n${hint}`);
}

function composeArgs(args) {
  return ["compose", "-f", COMPOSE_FILE, "--profile", "e2e", ...args];
}

function runChecked(command, args, options = {}) {
  const result = spawnSync(command, args, {
    cwd: options.cwd ?? REPO_ROOT,
    env: process.env,
    stdio: options.stdio ?? "inherit",
    encoding: options.encoding ?? "utf-8",
  });

  if (result.error) {
    throw result.error;
  }

  const exitCode = result.status ?? 1;
  if (exitCode !== 0) {
    throw new Error(`${command} ${args.join(" ")} exited with status ${exitCode}`);
  }

  return result;
}

function prepareWorld() {
  requireFile(
    ROOT_ENV_FILE,
    "Create it from .env.example before running the canonical E2E harness.",
  );
  requireFile(
    COMPOSE_FILE,
    "The canonical E2E harness runs directly against docker-compose.example.yml.",
  );

  console.log("[E2E Harness] Bringing up isolated backend stack...");
  runChecked("docker", [
    ...composeArgs([
      "up",
      "-d",
      "--wait",
      "--force-recreate",
      "postgres-test",
      "server-e2e",
    ]),
  ]);

  console.log("[E2E Harness] Preparing canonical Go-owned E2E world...");
  runChecked("docker", [
    ...composeArgs([
      "exec",
      "-T",
      "server-e2e",
      "go",
      "run",
      "main.go",
      "e2e",
      "prepare",
    ]),
  ]);

  const manifest = runChecked(
    "docker",
    [
      ...composeArgs([
        "exec",
        "-T",
        "server-e2e",
        "cat",
        "/app/.e2e-manifest.json",
      ]),
    ],
    {
      stdio: "pipe",
    },
  );

  writeFileSync(MANIFEST_PATH, manifest.stdout);
  return JSON.parse(readFileSync(MANIFEST_PATH, "utf-8"));
}

function startFrontend(runtime) {
  console.log("[E2E Harness] Starting isolated frontend...");
  frontendServer = spawn(
    "pnpm",
    ["run", "dev", "--port", String(runtime.frontend_port)],
    {
      cwd: FRONTEND_DIR,
      env: {
        ...process.env,
        PORT: String(runtime.frontend_port),
        NEXT_PUBLIC_API_URL: runtime.backend_url,
        API_URL: runtime.backend_url,
        TENANT_DOMAIN: runtime.tenant_domain,
        NEXT_PUBLIC_TENANT_DOMAIN: runtime.tenant_domain,
        NEXT_PUBLIC_OPERATOR_HOSTNAME: runtime.operator_hostname,
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
      signal !== null
        ? `signal ${signal}`
        : `exit code ${code ?? 1}`;
    void shutdown(1, `frontend dev server exited early with ${reason}`);
  });
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

async function teardownBackend() {
  try {
    runChecked("docker", [
      ...composeArgs(["down", "--volumes", "--remove-orphans"]),
    ]);
  } catch (error) {
    console.warn(
      `[E2E Harness] docker compose down failed: ${error instanceof Error ? error.message : String(error)}`,
    );
  }
}

async function shutdown(exitCode, errorMessage) {
  if (shuttingDown) {
    return;
  }
  shuttingDown = true;

  if (errorMessage) {
    console.error(`[E2E Harness] ${errorMessage}`);
  }

  await stopFrontend();
  await teardownBackend();
  process.exit(exitCode);
}

process.on("SIGTERM", () => {
  void shutdown(0);
});

process.on("SIGINT", () => {
  void shutdown(0);
});

try {
  const manifest = prepareWorld();
  startFrontend(manifest.runtime);
} catch (error) {
  void shutdown(
    1,
    error instanceof Error ? error.message : String(error),
  );
}
