/**
 * One tenant logout per tab at a time (#5126).
 *
 * The school switch (`reloginAtTenant`) and the menu logout
 * (`TeacherShellProvider`) both end the session. They run through
 * `endSessionOnce`, so a double click, „Wiederholen“ during a running logout
 * or both entry points at once start one logout: later calls get the running
 * logout's promise and its result. A settled logout frees the slot.
 *
 * Everyone awaiting the same logout receives the same failure. Only the first
 * caller that claims it shows it (`claimSessionEndFailure`), so one failure
 * leads to one message; the logout modal closes for a claimed failure.
 */

let running: Promise<void> | null = null;
const claimedFailures = new WeakSet<object>();

/** Runs `run` unless a logout of this tab is already running; then joins it. */
export function endSessionOnce(run: () => Promise<void>): Promise<void> {
  if (running) return running;
  const logout = (async () => run())().finally(() => {
    if (running === logout) running = null;
  });
  running = logout;
  return logout;
}

/**
 * Frees the slot when the barrier ends a session end without the page
 * leaving. The running logout's requests were aborted, so it settles on its
 * own; a later settle does not clear a newer logout's slot.
 */
export function releaseSessionEnd(): void {
  running = null;
}

/** True for the first caller of a failure: that caller shows it, no one else. */
export function claimSessionEndFailure(error: unknown): boolean {
  if (typeof error !== "object" || error === null) return true;
  if (claimedFailures.has(error)) return false;
  claimedFailures.add(error);
  return true;
}

/** Whether a caller already showed this logout failure. */
export function sessionEndFailureClaimed(error: unknown): boolean {
  return (
    typeof error === "object" && error !== null && claimedFailures.has(error)
  );
}
