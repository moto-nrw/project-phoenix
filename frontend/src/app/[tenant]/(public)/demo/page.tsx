"use client";

import { useEffect, useRef, useState } from "react";
import { signIn } from "next-auth/react";
import {
  DemoEntryMessage,
  type DemoEntryPhase,
} from "~/components/demo/demo-entry-message";
import {
  redeemDemoAccess,
  takeDemoTokenFromFragment,
  waitForDemoSchool,
} from "~/lib/demo-access";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "DemoEntryPage" });

// Entry page of the public demo (#3462): takes the token from the URL
// fragment, waits until the demo school can be entered, redeems the token by
// POST and signs in with the issued token pair, then opens the start page.
// A new demo school does not exist while it is seeded, so visitors wait on
// the main domain (`/demo`) and arrive here once it is ready (#3463).
export default function DemoEntryPage() {
  const [phase, setPhase] = useState<DemoEntryPhase>("opening");
  const tokenRef = useRef<string | null>(null);

  useEffect(() => {
    const run = { cancelled: false };

    async function enter(token: string): Promise<DemoEntryPhase | "entered"> {
      const waited = await waitForDemoSchool(token, run, () =>
        setPhase("preparing"),
      );
      if (waited.phase === "cancelled") return "opening";
      if (waited.phase !== "ready") return waited.phase;
      const tokens = await redeemDemoAccess(token);
      if (!tokens) return "invalid";
      const result = await signIn("credentials", {
        redirect: false,
        internalRefresh: true,
        token: tokens.access_token,
        refreshToken: tokens.refresh_token,
      });
      return result?.error ? "failed" : "entered";
    }

    // The fragment is read once and removed; a repeated effect run (React
    // strict mode) must find the token again.
    tokenRef.current ??= takeDemoTokenFromFragment();
    const token = tokenRef.current;
    if (!token) {
      setPhase("invalid");
      return;
    }
    enter(token)
      .then((outcome) => {
        if (run.cancelled) return;
        if (outcome === "entered") globalThis.location.assign("/");
        else setPhase(outcome);
      })
      .catch((error: unknown) => {
        logger.error("demo_entry_failed", {
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
