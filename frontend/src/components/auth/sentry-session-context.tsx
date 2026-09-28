"use client";

import { useEffect } from "react";
import { useSession } from "next-auth/react";
import {
  applySentrySessionContext,
  sentrySessionContext,
  type SentryPortal,
} from "~/lib/sentry-context";

/**
 * Attaches the signed-in account to the browser's Sentry events (`user.id`,
 * `role`, `school_id`) and clears it at logout. Mounted once inside each
 * portal's SessionProvider; renders nothing. The `portal` tag comes from the
 * host in sentry.client.config.ts.
 */
export function SentrySessionContext({
  portal,
}: {
  readonly portal: SentryPortal;
}) {
  const { data: session, status } = useSession();
  const context = sentrySessionContext(
    portal,
    status === "authenticated" ? session?.user : null,
  );
  const userId = context.user?.id ?? null;
  const { role, schoolId } = context;

  useEffect(() => {
    // While the session loads, keep what is there: a refetch must not strip
    // the context from an error that happens in that moment.
    if (status === "loading") return;
    applySentrySessionContext({
      user: userId === null ? null : { id: userId },
      role,
      schoolId,
    });
  }, [status, userId, role, schoolId]);

  return null;
}
