"use client";

import * as barrier from "~/lib/session-end-barrier";
import { installSessionEndBarrier } from "~/lib/session-end-state";

// Installed when the module loads, before the pages fetch; only in the
// browser, where the fetch observer applies it.
if (typeof window !== "undefined") installSessionEndBarrier(barrier);

/**
 * Brings the session-end barrier (#5399) to the pages that use a session:
 * the tenant shell's protected layout and the providers of the other
 * portals. Draws nothing.
 */
export function SessionEndBarrier() {
  return null;
}
