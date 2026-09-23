"use client";

import { useEffect, useRef, useState } from "react";
import { DemoRoleChoice } from "~/components/demo/demo-role-choice";
import { DemoSetupScreen } from "~/components/demo/demo-setup-screen";
import { DemoOpening, DemoProblem } from "~/components/demo/demo-shell";
import {
  type DemoEntryPhase,
  type DemoLink,
  type DemoRole,
  type DemoSetupProgress,
  demoEntryEvent,
  isParentDemoRole,
  parentsDemoEntryUrl,
  redeemDemoAccess,
  startDemoSession,
  takeDemoLinkFromFragment,
  waitForDemoSchool,
} from "~/lib/demo-access";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "DemoEntryPage" });

/** "choosing": the school is ready and the link brought no demo role. */
type EntryPhase = DemoEntryPhase | "choosing";

/**
 * Redeems the token in the role and signs in with the issued token pair.
 * The banner reports the entry after the start page has loaded (#3467), or
 * the switch when the banner of the parents app sent the visitor (#3468).
 * The role parent is the parents app's: the link goes on to its entry page.
 */
async function enterAs(
  link: DemoLink,
  role: DemoRole,
): Promise<"entered" | "left" | "invalid" | "failed"> {
  if (isParentDemoRole(role)) {
    globalThis.location.assign(parentsDemoEntryUrl(link));
    return "left";
  }
  const session = await redeemDemoAccess(link.token, role);
  if (!session) return "invalid";
  return (await startDemoSession(session, demoEntryEvent(link)))
    ? "entered"
    : "failed";
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

  const finish = (outcome: EntryPhase | "entered" | "left") => {
    if (outcome === "entered") globalThis.location.assign("/");
    else if (outcome !== "left") setPhase(outcome);
  };

  const fail = (error: unknown) => {
    logger.error("demo_entry_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    setPhase("failed");
  };

  useEffect(() => {
    const run = { cancelled: false };

    async function enter(
      link: DemoLink,
    ): Promise<EntryPhase | "entered" | "left"> {
      const waited = await waitForDemoSchool(link.token, run, (progress) => {
        setSetup(progress);
        setPhase("preparing");
      });
      if (waited.phase === "cancelled") return "opening";
      if (waited.phase !== "ready") return waited.phase;
      if (!link.role) return "choosing";
      return enterAs(link, link.role);
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
    enterAs(link, role).then(finish).catch(fail);
  };

  if (phase === "opening") return <DemoOpening />;
  if (phase === "preparing") return <DemoSetupScreen {...setup} />;
  if (phase === "choosing") return <DemoRoleChoice onChoose={choose} />;
  return (
    <DemoProblem
      phase={phase}
      onRetry={() => {
        setPhase("opening");
        setAttempt((current) => current + 1);
      }}
    />
  );
}
