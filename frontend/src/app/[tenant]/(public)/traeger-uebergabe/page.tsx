"use client";

import { signIn } from "next-auth/react";
import { useEffect, useRef, useState } from "react";
import { MotoBrand } from "~/components/auth/moto-brand";
import { ButtonLink } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { Loading } from "~/components/ui/loading";
import { SectionCard } from "~/components/ui/section-card";
import { createLogger } from "~/lib/logger";
import { clearSessionCache } from "~/lib/session-cache";
import {
  TRAEGER_HANDOFF_BACK,
  TRAEGER_HANDOFF_FAILED_TEXT,
  TRAEGER_HANDOFF_FAILED_TITLE,
  TRAEGER_HANDOFF_OPENING,
  takeTraegerHandoffToken,
  traegerPortalUrl,
} from "~/lib/traeger-handoff";

const logger = createLogger({ component: "TraegerHandoffPage" });

/**
 * Entry page of the Absprung from the Träger portal (#4654). The Träger
 * portal opens it on the school's host with a short-lived hand-off token in
 * the fragment; the page redeems it into this school's session, without a
 * sign-in, and opens the start page. A used or expired link leads back to
 * the Träger portal.
 */
export default function TraegerHandoffPage() {
  const [failed, setFailed] = useState(false);
  const redemption = useRef<Promise<
    Awaited<ReturnType<typeof signIn>> | undefined
  > | null>(null);

  useEffect(() => {
    // The token works once: the fragment is read once and redeemed once. A
    // repeated effect run (React strict mode) waits for the same answer.
    if (redemption.current === null) {
      const token = takeTraegerHandoffToken();
      redemption.current = token
        ? signIn("credentials", { redirect: false, traegerHandoff: token })
        : Promise.resolve(undefined);
    }
    let cancelled = false;
    redemption.current
      .then((result) => {
        if (cancelled) return;
        if (!result || result.error) {
          if (result) logger.warn("traeger_handoff_refused", {});
          setFailed(true);
          return;
        }
        clearSessionCache();
        globalThis.location.assign("/");
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        logger.error("traeger_handoff_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
        setFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <main className="moto-dotted-background moto-dotted-background--fullscreen flex min-h-dvh flex-col items-center justify-center px-4 py-10">
      {/* `relative` lifts the content above the pattern's ::before layer. */}
      <div className="relative flex w-full max-w-md flex-col items-center gap-8">
        <MotoBrand />
        {failed ? (
          <SectionCard className="w-full">
            <EmptyState
              className="py-4"
              title={TRAEGER_HANDOFF_FAILED_TITLE}
              description={TRAEGER_HANDOFF_FAILED_TEXT}
              action={
                <ButtonLink href={traegerPortalUrl()}>
                  {TRAEGER_HANDOFF_BACK}
                </ButtonLink>
              }
            />
          </SectionCard>
        ) : (
          <Loading message={TRAEGER_HANDOFF_OPENING} fullPage={false} />
        )}
      </div>
    </main>
  );
}
