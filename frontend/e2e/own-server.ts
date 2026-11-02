import { execFileSync } from "node:child_process";
import { existsSync, realpathSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

// Locally Playwright reuses whatever already listens on the frontend port.
// Worktrees without their own port lease all default to 3000, so a run could
// test another checkout's server and pass (journal, round 1). Refuse when the
// listener is a frontend started from a different checkout. Without lsof, or
// for listeners that are not a checkout (a container proxy), nothing is
// decided here.
const ownFrontend = realpathSync(fileURLToPath(new URL("..", import.meta.url)));

function lsof(args: string[]): string {
  try {
    return execFileSync("lsof", args, {
      encoding: "utf8",
      stdio: ["ignore", "pipe", "ignore"],
    });
  } catch {
    return ""; // No listener (exit 1) or no lsof.
  }
}

export function assertPortServesThisCheckout(port: number): void {
  if (process.env.CI) return;
  const pids = lsof(["-nP", `-iTCP:${port}`, "-sTCP:LISTEN", "-t"])
    .split(/\s+/)
    .filter(Boolean);
  for (const pid of pids) {
    const cwd = /^n(.+)$/m.exec(
      lsof(["-a", "-p", pid, "-d", "cwd", "-Fn"]),
    )?.[1];
    if (!cwd || !existsSync(join(cwd, "playwright.config.ts"))) continue;
    if (realpathSync(cwd) !== ownFrontend) {
      throw new Error(
        `Port ${port} serves the frontend of ${cwd}, not this checkout (${ownFrontend}). ` +
          "Stop that server, or start this worktree's frontend with scripts/dev-native.sh and run with its PORT.",
      );
    }
  }
}
