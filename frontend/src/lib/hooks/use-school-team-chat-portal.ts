"use client";

import { useMemo } from "react";
import { useRouter } from "next/navigation";
import type { TeamChatPortal } from "~/lib/team-chat-portal";
import { schoolStaffMessagesApi } from "~/lib/school-staff-messages-api";
import { schoolPath } from "~/lib/school-url";

/** Route of the school-portal inbox, before schoolPath rewrites it for the host. */
export const SCHOOL_MESSAGES_ROUTE = "/school/nachrichten";

/**
 * The school-portal binding of the shared Team-Chat surfaces (#2208). No
 * tenant context here: the school session is bound to one school, and the
 * feature flag is not cached client-side — the backend's stable
 * `staff_messaging_disabled` code decides the off-state.
 */
export function useSchoolTeamChatPortal(): TeamChatPortal {
  const router = useRouter();
  return useMemo(
    () => ({
      kind: "school",
      api: schoolStaffMessagesApi,
      cacheScope: "school",
      inboxHref: SCHOOL_MESSAGES_ROUTE,
      threadHref: (threadId) => `${SCHOOL_MESSAGES_ROUTE}/${threadId}`,
      navigate: (href) => router.push(schoolPath(href)),
      flagSaysEnabled: undefined,
      title: "Nachrichten",
      emptyDescription:
        "Hier stehen Ihre Unterhaltungen mit der OGS. Sie schreiben an die Leitung oder an eine Betreuungskraft Ihrer Schule. Eltern sehen davon nichts.",
      recipientHint:
        "Sie schreiben an Personen der OGS Ihrer Schule. Die Nachricht kommt in deren Team-Chat an.",
    }),
    [router],
  );
}
