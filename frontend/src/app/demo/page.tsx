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
  demoLinkFragment,
  takeDemoLinkFromFragment,
  waitForDemoSchool,
} from "~/lib/demo-access";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "DemoWaitingRoom" });

// Waiting room of the public demo (#3463). Every visitor gets a demo school
// of their own, and its subdomain answers only once the school is seeded. So
// the link from the website leads here, on the main domain. The page names
// the OGS being set up with its progress lines (#3464), waits until the
// school is ready and then hands the token on to the school's own entry page,
// again in the URL fragment, where it is redeemed.
export default function DemoWaitingRoomPage() {
  const [phase, setPhase] = useState<DemoEntryPhase>("opening");
  const [setup, setSetup] = useState<DemoSetupProgress>({ step: 0 });
  // Every try waits anew; "Noch einmal versuchen" starts the next one.
  const [attempt, setAttempt] = useState(0);
  const linkRef = useRef<DemoLink | null>(null);

  useEffect(() => {
    const run = { cancelled: false };
    // The fragment is read once and removed; a repeated effect run (React
    // strict mode, another try) must find the link again.
    linkRef.current ??= takeDemoLinkFromFragment();
    const link = linkRef.current;
    if (!link) {
      setPhase("invalid");
      return;
    }
    waitForDemoSchool(link.token, run, (progress) => {
      setSetup(progress);
      setPhase("preparing");
    })
      .then((waited) => {
        if (run.cancelled || waited.phase === "cancelled") return;
        if (waited.phase !== "ready") {
          setPhase(waited.phase);
          return;
        }
        if (!waited.schoolUrl?.startsWith("http")) {
          throw new Error("ready demo school without an address");
        }
        // A preselected role (#3467) travels on, so the school's entry page
        // skips its role cards.
        globalThis.location.assign(
          `${waited.schoolUrl}/demo${demoLinkFragment(link)}`,
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
