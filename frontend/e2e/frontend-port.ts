// Port of the frontend under test. `next dev` and `next start` bind PORT and
// default to 3000; `scripts/dev-native.sh frontend` and wt-managed shells set
// PORT to the worktree's leased FRONTEND_HOST_PORT. Playwright waits for,
// reuses and visits exactly this port, so a worktree never tests another
// checkout's server on 3000. CI leaves PORT unset for the E2E step.
function readFrontendPort(): number {
  const raw = process.env.PORT;
  if (raw === undefined || raw === "") return 3000;
  const port = Number(raw);
  if (!/^\d+$/.test(raw) || port < 1024 || port > 65535) {
    throw new Error("PORT must be a port between 1024 and 65535");
  }
  return port;
}

export const frontendPort = readFrontendPort();
