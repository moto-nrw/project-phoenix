"use client";

import { useEffect, useRef, useState } from "react";
import { signIn } from "next-auth/react";
import { DemoRoleChoice } from "~/components/demo/demo-role-choice";
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
  type DemoRole,
  type DemoSetupProgress,
  redeemDemoAccess,
  saveDemoVisit,
  takeDemoLinkFromFragment,
  waitForDemoSchool,
} from "~/lib/demo-access";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "DemoEntryPage" });

/** "choosing": the school is ready and the link brought no demo role. */
type EntryPhase = DemoEntryPhase | "choosing";

/**
 * Redeems the token in the role and signs in with the issued token pair.
 * The banner reports the entry after the start page has loaded (#3467).
 */
async function enterAs(
  token: string,
  role: DemoRole,
): Promise<"entered" | "invalid" | "failed"> {
  const session = await redeemDemoAccess(token, role);
  if (!session) return "invalid";
  const result = await signIn("credentials", {
    redirect: false,
    internalRefresh: true,
    token: session.access_token,
    refreshToken: session.refresh_token,
  });
  if (result?.error) return "failed";
  saveDemoVisit({ ...session.visit, pending: "demo_entered" });
  return "entered";
}

// Entry page of the public demo (#3462): takes the token from the URL
// fragment, waits until the demo school can be entered, redeems the token by
// POST and signs in with the issued token pair, then opens the start page.
// A new demo school does not exist while it is seeded, so visitors wait on
// the main domain (`/demo`) and arrive here once it is ready (#3463).
// Redeeming starts the school's simulation (#3464). A link without a demo
// role shows the role cards first (#3467).
export default function DemoEntryPage() {
  const [phase, setPhase] = useState<EntryPhase>("opening");
  const [setup, setSetup] = useState<DemoSetupProgress>({ step: 0 });
  // Every try enters anew; "Noch einmal versuchen" starts the next one.
  const [attempt, setAttempt] = useState(0);
  const linkRef = useRef<DemoLink | null | undefined>(undefined);

  const finish = (outcome: EntryPhase | "entered") => {
    if (outcome === "entered") globalThis.location.assign("/");
    else setPhase(outcome);
  };

  const fail = (error: unknown) => {
    logger.error("demo_entry_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    setPhase("failed");
  };

  useEffect(() => {
    const run = { cancelled: false };

    async function enter(link: DemoLink): Promise<EntryPhase | "entered"> {
      const waited = await waitForDemoSchool(link.token, run, (progress) => {
        setSetup(progress);
        setPhase("preparing");
      });
      if (waited.phase === "cancelled") return "opening";
      if (waited.phase !== "ready") return waited.phase;
      if (!link.role) return "choosing";
      return enterAs(link.token, link.role);
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
        if (!run.cancelled) finish(outcome);
      })
      .catch((error: unknown) => {
        if (!run.cancelled) fail(error);
      });
    return () => {
      run.cancelled = true;
    };
  }, [attempt]);

  const choose = (role: DemoRole) => {
    const link = linkRef.current;
    if (!link) return;
    // A retry after a failed entry skips the cards.
    linkRef.current = { ...link, role };
    setPhase("opening");
    enterAs(link.token, role).then(finish).catch(fail);
  };

  if (phase === "opening") return <Loading message={DEMO_ENTRY_OPENING} />;
  if (phase === "preparing") return <DemoSetupScreen {...setup} />;
  if (phase === "choosing") return <DemoRoleChoice onChoose={choose} />;
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
