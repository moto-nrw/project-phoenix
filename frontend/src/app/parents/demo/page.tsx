"use client";

import { useEffect, useRef, useState } from "react";
import { DemoSetupScreen } from "~/components/demo/demo-setup-screen";
import { Button, ButtonLink } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { Loading } from "~/components/ui/loading";
import {
  DEMO_ENTRY_OPENING,
  DEMO_ENTRY_NEW_LINK,
  DEMO_ENTRY_PROBLEMS,
  DEMO_ENTRY_RETRY,
  DEMO_WEBSITE_URL,
  type DemoEntryPhase,
  type DemoLink,
  type DemoSetupProgress,
  demoEntryEvent,
  demoLinkFragment,
  redeemDemoAccess,
  startDemoSession,
  takeDemoLinkFromFragment,
  waitForDemoSchool,
} from "~/lib/demo-access";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ParentDemoEntryPage" });

// Entry page of the parents app in the public demo (#3468). The same link
// that opens the OGS app arrives here with the role parent: from the
// waiting room, from the OGS app's entry page or from the banner. The page
// waits until the demo school can be entered, redeems the token as the
// school's parent of the visitor's name and signs in to the parents app.
// The standing demo school has no such parent; its visitor goes on to the
// OGS app in the role the session really has.
export default function ParentDemoEntryPage() {
  const [phase, setPhase] = useState<DemoEntryPhase>("opening");
  const [setup, setSetup] = useState<DemoSetupProgress>({ step: 0 });
  // Every try enters anew; "Noch einmal versuchen" starts the next one.
  const [attempt, setAttempt] = useState(0);
  const linkRef = useRef<DemoLink | null | undefined>(undefined);

  useEffect(() => {
    const run = { cancelled: false };

    async function enter(link: DemoLink): Promise<DemoEntryPhase | "left"> {
      const waited = await waitForDemoSchool(link.token, run, (progress) => {
        setSetup(progress);
        setPhase("preparing");
      });
      if (waited.phase === "cancelled") return "opening";
      if (waited.phase !== "ready") return waited.phase;
      const session = await redeemDemoAccess(link.token, "parent");
      if (!session) return "invalid";
      if (session.visit.role !== "parent") {
        if (!waited.schoolUrl?.startsWith("http")) {
          throw new Error("ready demo school without an address");
        }
        globalThis.location.assign(
          `${waited.schoolUrl}/demo${demoLinkFragment({ ...link, role: session.visit.role })}`,
        );
        return "left";
      }
      const started = await startDemoSession(
        {
          ...session,
          visit: { ...session.visit, schoolName: waited.schoolName },
        },
        demoEntryEvent(link),
        "parent-credentials",
      );
      if (!started) return "failed";
      globalThis.location.assign("/");
      return "left";
    }

    // The fragment is read once and removed; a repeated effect run (React
    // strict mode, another try) must find the link again.
    if (linkRef.current === undefined) {
      linkRef.current = takeDemoLinkFromFragment();
    }
    const link = linkRef.current;
    if (!link) {
      setPhase("invalid");
      return;
    }
    enter(link)
      .then((outcome) => {
        if (!run.cancelled && outcome !== "left") setPhase(outcome);
      })
      .catch((error: unknown) => {
        logger.error("parent_demo_entry_failed", {
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
              {DEMO_ENTRY_RETRY}
            </Button>
          ) : (
            <ButtonLink href={DEMO_WEBSITE_URL}>
              {DEMO_ENTRY_NEW_LINK}
            </ButtonLink>
          )
        }
      />
    </main>
  );
}
