"use client";

import { useEffect, useRef, useState } from "react";
import {
  DemoEntryMessage,
  type DemoEntryPhase,
} from "~/components/demo/demo-entry-message";
import {
  takeDemoTokenFromFragment,
  waitForDemoSchool,
} from "~/lib/demo-access";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "DemoWaitingRoom" });

// Waiting room of the public demo (#3463). Every visitor gets a demo school
// of their own, and its subdomain answers only once the school is seeded. So
// the link from the website leads here, on the main domain. The page waits
// until the school is ready and then hands the token on to the school's own
// entry page, again in the URL fragment, where it is redeemed.
export default function DemoWaitingRoomPage() {
  const [phase, setPhase] = useState<DemoEntryPhase>("opening");
  const tokenRef = useRef<string | null>(null);

  useEffect(() => {
    const run = { cancelled: false };
    // The fragment is read once and removed; a repeated effect run (React
    // strict mode) must find the token again.
    tokenRef.current ??= takeDemoTokenFromFragment();
    const token = tokenRef.current;
    if (!token) {
      setPhase("invalid");
      return;
    }
    waitForDemoSchool(token, run, () => setPhase("preparing"))
      .then((waited) => {
        if (run.cancelled || waited.phase === "cancelled") return;
        if (waited.phase !== "ready") {
          setPhase(waited.phase);
          return;
        }
        if (!waited.schoolUrl?.startsWith("http")) {
          throw new Error("ready demo school without an address");
        }
        globalThis.location.assign(
          `${waited.schoolUrl}/demo#token=${encodeURIComponent(token)}`,
        );
      })
      .catch((error: unknown) => {
        logger.error("demo_waiting_room_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
        if (!run.cancelled) setPhase("failed");
      });
    return () => {
      run.cancelled = true;
    };
  }, []);

  return <DemoEntryMessage phase={phase} />;
}
