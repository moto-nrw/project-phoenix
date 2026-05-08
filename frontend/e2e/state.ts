import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const HERE = dirname(fileURLToPath(import.meta.url));
const FRONTEND_DIR = resolve(HERE, "..");
const REPO_ROOT = resolve(FRONTEND_DIR, "..");

export const E2E_STATE_PATH = resolve(REPO_ROOT, "backend", ".e2e-state.json");

let cachedState: unknown;

function loadState(): unknown {
  try {
    return JSON.parse(readFileSync(E2E_STATE_PATH, "utf-8")) as unknown;
  } catch (err) {
    throw new Error(
      `Could not read ${E2E_STATE_PATH}. Run \`go run . e2e run\` or \`go run . e2e prepare\` from backend/ first.\n` +
        `Underlying error: ${err instanceof Error ? err.message : String(err)}`,
      { cause: err },
    );
  }
}

export function getE2EState(): unknown {
  if (cachedState === undefined) {
    cachedState = loadState();
  }
  return cachedState;
}
