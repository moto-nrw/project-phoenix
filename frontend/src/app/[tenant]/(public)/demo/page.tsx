"use client";

import { useEffect, useRef, useState } from "react";
import { signIn } from "next-auth/react";
import { DemoSetupScreen } from "~/components/demo/demo-setup-screen";
import { Button, ButtonLink } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { Loading } from "~/components/ui/loading";
import {
  DEMO_ENTRY_OPENING,
  DEMO_ENTRY_PROBLEMS,
  DEMO_WEBSITE_URL,
  type DemoEntryPhase,
  type DemoSetupProgress,
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
// Redeeming starts the school's simulation (#3464).
export default function DemoEntryPage() {
  const [phase, setPhase] = useState<DemoEntryPhase>("opening");
  const [setup, setSetup] = useState<DemoSetupProgress>({ step: 0 });
  // Every try enters anew; "Noch einmal versuchen" starts the next one.
  const [attempt, setAttempt] = useState(0);
  const tokenRef = useRef<string | null>(null);

  useEffect(() => {
    const run = { cancelled: false };

    async function enter(token: string): Promise<DemoEntryPhase | "entered"> {
      const waited = await waitForDemoSchool(token, run, (progress) => {
        setSetup(progress);
        setPhase("preparing");
      });
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
    // strict mode, another try) must find the token again.
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
  }, [attempt]);

  if (phase === "opening") return <Loading message={DEMO_ENTRY_OPENING} />;
  if (phase === "preparing") return <DemoSetupScreen {...setup} />;
  const problem = DEMO_ENTRY_PROBLEMS[phase];
  return (
    <main className="flex min-h-dvh items-center justify-center px-4">
      <EmptyState
        title={problem.title}
        description={problem.description}
        action={
          phase === "failed" ? (
            <Button
              type="button"
              onClick={() => {
                setPhase("opening");
                setAttempt((current) => current + 1);
              }}
            >
              Noch einmal versuchen
            </Button>
          ) : (
            <ButtonLink href={DEMO_WEBSITE_URL}>
              Neuen Link anfordern
            </ButtonLink>
          )
        }
      />
    </main>
  );
}
