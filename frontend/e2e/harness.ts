import { existsSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { spawn, spawnSync, type ChildProcess } from "node:child_process";
import { setTimeout as delay } from "node:timers/promises";
import {
  E2E_DEFAULT_BACKEND_URL,
  E2E_DEFAULT_FRONTEND_PORT,
  E2E_DEFAULT_TENANT_DOMAIN,
} from "./contract.generated";

const HERE = dirname(fileURLToPath(import.meta.url));
const FRONTEND_DIR = resolve(HERE, "..");
const REPO_ROOT = resolve(FRONTEND_DIR, "..");
const COMPOSE_FILE = resolve(REPO_ROOT, "docker-compose.example.yml");
const ROOT_ENV_FILE = resolve(REPO_ROOT, ".env");
const MANIFEST_PATH = resolve(REPO_ROOT, "backend", ".e2e-manifest.json");
const FRONTEND_URL = `http://localhost:${E2E_DEFAULT_FRONTEND_PORT}`;

function requireFile(path: string, hint: string): void {
  if (existsSync(path)) {
    return;
  }

  throw new Error(`missing required file: ${path}\n${hint}`);
}

function composeArgs(args: string[]): string[] {
  return ["compose", "-f", COMPOSE_FILE, "--profile", "e2e", ...args];
}

function run(
  command: string,
  args: string[],
  options: {
    cwd?: string;
    allowFailure?: boolean;
  } = {},
): number {
  const { cwd = REPO_ROOT, allowFailure = false } = options;
  const result = spawnSync(command, args, {
    cwd,
    env: process.env,
    stdio: "inherit",
  });

  if (result.error) {
    throw result.error;
  }

  const exitCode = result.status ?? 1;
  if (exitCode !== 0 && !allowFailure) {
    throw new Error(
      `${command} ${args.join(" ")} exited with status ${exitCode}`,
    );
  }

  return exitCode;
}

function syncManifestPermissions(): void {
  if (!existsSync(MANIFEST_PATH)) {
    throw new Error(
      `expected E2E manifest at ${MANIFEST_PATH} after prepare completed`,
    );
  }

  if (
    typeof process.getuid === "function" &&
    typeof process.getgid === "function"
  ) {
    const uid = process.getuid();
    const gid = process.getgid();
    run("docker", [
      ...composeArgs([
        "exec",
        "-T",
        "server-e2e",
        "sh",
        "-lc",
        `chown ${uid}:${gid} /app/.e2e-manifest.json && chmod 600 /app/.e2e-manifest.json`,
      ]),
    ]);
    return;
  }

  run("docker", [
    ...composeArgs([
      "exec",
      "-T",
      "server-e2e",
      "chmod",
      "644",
      "/app/.e2e-manifest.json",
    ]),
  ]);
}

export function prepareHarness(): void {
  requireFile(
    ROOT_ENV_FILE,
    "Create it from .env.example before running the canonical E2E harness.",
  );
  requireFile(
    COMPOSE_FILE,
    "The canonical E2E harness runs directly against docker-compose.example.yml.",
  );

  console.log(
    "Bringing up isolated E2E backend (postgres-test + server-e2e)...",
  );
  run("docker", [
    ...composeArgs([
      "up",
      "-d",
      "--wait",
      "--force-recreate",
      "postgres-test",
      "server-e2e",
    ]),
  ]);

  console.log("Preparing canonical Go-owned E2E world...");
  run("docker", [
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

  syncManifestPermissions();
}

function startFrontendServer(): ChildProcess {
  console.log("Starting isolated E2E frontend...");
  return spawn(
    "pnpm",
    ["run", "dev", "--port", String(E2E_DEFAULT_FRONTEND_PORT)],
    {
      cwd: FRONTEND_DIR,
      env: {
        ...process.env,
        PORT: String(E2E_DEFAULT_FRONTEND_PORT),
        NEXT_PUBLIC_API_URL: E2E_DEFAULT_BACKEND_URL,
        API_URL: E2E_DEFAULT_BACKEND_URL,
        TENANT_DOMAIN: E2E_DEFAULT_TENANT_DOMAIN,
        NEXT_PUBLIC_TENANT_DOMAIN: E2E_DEFAULT_TENANT_DOMAIN,
        NEXT_PUBLIC_OPERATOR_HOSTNAME: `operator.${E2E_DEFAULT_TENANT_DOMAIN}:${E2E_DEFAULT_FRONTEND_PORT}`,
      },
      stdio: "inherit",
      detached: process.platform !== "win32",
    },
  );
}

async function waitForFrontendReady(server: ChildProcess): Promise<void> {
  const deadline = Date.now() + 120_000;

  while (Date.now() < deadline) {
    if (server.exitCode !== null) {
      throw new Error(
        `frontend dev server exited early with status ${server.exitCode}`,
      );
    }

    try {
      const response = await fetch(FRONTEND_URL, {
        signal: AbortSignal.timeout(5_000),
      });
      if (response.ok || response.status < 500) {
        return;
      }
    } catch {
      // Cold Next.js startup will refuse connections for a while; keep polling.
    }

    await delay(1_000);
  }

  throw new Error(
    `frontend dev server did not become ready at ${FRONTEND_URL}`,
  );
}

async function stopFrontendServer(server: ChildProcess): Promise<void> {
  if (server.exitCode !== null) {
    return;
  }

  try {
    if (process.platform === "win32") {
      server.kill("SIGTERM");
    } else if (server.pid) {
      process.kill(-server.pid, "SIGTERM");
    } else {
      server.kill("SIGTERM");
    }
  } catch (error) {
    if (!(error instanceof Error) || !error.message.includes("ESRCH")) {
      throw error;
    }
  }

  const deadline = Date.now() + 10_000;
  while (server.exitCode === null && Date.now() < deadline) {
    await delay(250);
  }

  if (server.exitCode === null) {
    try {
      if (process.platform === "win32") {
        server.kill("SIGKILL");
      } else if (server.pid) {
        process.kill(-server.pid, "SIGKILL");
      } else {
        server.kill("SIGKILL");
      }
    } catch (error) {
      if (!(error instanceof Error) || !error.message.includes("ESRCH")) {
        throw error;
      }
    }
  }
}

export async function startHarness(): Promise<() => Promise<void>> {
  prepareHarness();
  const server = startFrontendServer();

  try {
    await waitForFrontendReady(server);
  } catch (error) {
    await stopFrontendServer(server);
    teardownHarness();
    throw error;
  }

  return async () => {
    await stopFrontendServer(server);
    teardownHarness();
  };
}

export function teardownHarness(): void {
  console.log("Tearing down isolated E2E backend...");
  const exitCode = run(
    "docker",
    [...composeArgs(["down", "--volumes", "--remove-orphans"])],
    {
      allowFailure: true,
    },
  );

  if (exitCode !== 0) {
    console.warn(
      `warning: docker compose down exited with status ${exitCode} during E2E teardown`,
    );
  }
}
