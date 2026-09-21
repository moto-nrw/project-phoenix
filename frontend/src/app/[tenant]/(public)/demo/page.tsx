"use client";

import { useEffect, useState } from "react";
import { signIn } from "next-auth/react";
import { ButtonLink } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { Loading } from "~/components/ui/loading";
import {
  DEMO_WEBSITE_URL,
  fetchDemoAccessStatus,
  redeemDemoAccess,
  takeDemoTokenFromFragment,
} from "~/lib/demo-access";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "DemoEntryPage" });

const POLL_INTERVAL_MS = 2000;
// Two minutes; a demo school that is not ready by then will not become so.
const MAX_POLLS = 60;

type Phase = "opening" | "preparing" | "invalid" | "failed";

const wait = (ms: number) =>
  new Promise<void>((resolve) => setTimeout(resolve, ms));

// Entry page of the public demo (#3462): takes the token from the URL
// fragment, waits until the demo school can be entered, redeems the token by
// POST and signs in with the issued token pair, then opens the start page.
export default function DemoEntryPage() {
  const [phase, setPhase] = useState<Phase>("opening");

  useEffect(() => {
    const run = { cancelled: false };

    async function enter(token: string): Promise<Phase | "entered"> {
      for (let attempt = 0; !run.cancelled; attempt++) {
        const status = await fetchDemoAccessStatus(token);
        if (status === "invalid") return "invalid";
        if (status === "ready") break;
        if (attempt >= MAX_POLLS) return "failed";
        setPhase("preparing");
        await wait(POLL_INTERVAL_MS);
      }
      if (run.cancelled) return "opening";
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

    const token = takeDemoTokenFromFragment();
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

  if (phase === "opening") return <Loading message="Demo wird geöffnet …" />;
  if (phase === "preparing") {
    return (
      <Loading message="Ihre Demo wird vorbereitet. Einen Moment bitte." />
    );
  }

  const backToWebsite = (
    <ButtonLink href={DEMO_WEBSITE_URL}>Neuen Link anfordern</ButtonLink>
  );
  return (
    <main className="flex min-h-dvh items-center justify-center px-4">
      {phase === "invalid" ? (
        <EmptyState
          title="Dieser Link funktioniert nicht mehr"
          description="Ein Demo-Link gilt 14 Tage. Auf unserer Website bekommen Sie sofort einen neuen."
          action={backToWebsite}
        />
      ) : (
        <EmptyState
          title="Das hat leider nicht geklappt"
          description="Es liegt nicht an Ihnen. Bitte öffnen Sie den Link noch einmal. Oder fordern Sie einen neuen an."
          action={backToWebsite}
        />
      )}
    </main>
  );
}
