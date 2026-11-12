"use client";

import { useEffect } from "react";
import { useSession } from "next-auth/react";

import { hasPermission, isAdmin } from "~/lib/auth-utils";
import { CHANGE_REQUESTS_REFRESH_EVENT } from "~/lib/change-requests-refresh";
import { useChangeRequestAccess } from "~/lib/hooks/use-change-request-access";
import {
  countParentAttention,
  fetchAllOpenRequests,
  type ParentAttention,
} from "~/lib/parent-attention";
import { fetchInboxWithFilters } from "~/lib/parent-messages-api";
import { useSWRAuth } from "~/lib/swr";
import { messageKeys } from "~/lib/swr/portal-keys";
import { useTenantSafe } from "~/lib/tenant-context";

/** Ereignisse, nach denen die Zahlen auf den Kinderkacheln neu geladen werden. */
const REFRESH_EVENTS = [
  "messages-unread-refresh",
  CHANGE_REQUESTS_REFRESH_EVENT,
] as const;

// Viele Kacheln lesen denselben Schlüssel. Ein Ereignis lädt ihn einmal neu,
// nicht einmal je Kachel: je Schlüssel ruft nur der erste Leser `mutate`.
const refreshers = new Map<string, Set<() => void>>();

function refreshAll(): void {
  for (const readers of refreshers.values()) {
    readers.values().next().value?.();
  }
}

function subscribeRefresh(key: string, refresh: () => void): () => void {
  if (refreshers.size === 0) {
    for (const event of REFRESH_EVENTS) {
      window.addEventListener(event, refreshAll);
    }
  }
  const readers = refreshers.get(key) ?? new Set();
  readers.add(refresh);
  refreshers.set(key, readers);
  return () => {
    readers.delete(refresh);
    if (readers.size === 0) refreshers.delete(key);
    if (refreshers.size === 0) {
      for (const event of REFRESH_EVENTS) {
        window.removeEventListener(event, refreshAll);
      }
    }
  };
}

/**
 * Zeigt die Schule neue Elternnachrichten auf der Kinderkachel (#6486)? Nur
 * ein Blick in die Schulangaben, ohne Abruf: eine Kachel ruft den Zähler erst
 * auf, wenn die Schule ihn eingeschaltet hat.
 */
export function useParentAttentionEnabled(): boolean {
  return useTenantSafe()?.tenant?.parentMessageCardBadgeEnabled === true;
}

/**
 * Neue Nachrichten und offene Anfragen der Eltern je Kind (#6486). Alle
 * Kacheln einer Seite teilen einen Abruf. Nachrichten liest, wer
 * Elternnachrichten lesen darf (users:read); Anfragen zählen nur für Personen,
 * die sie auch entscheiden dürfen.
 */
function useParentAttentionMap(): ReadonlyMap<string, ParentAttention> | null {
  const enabled = useParentAttentionEnabled();
  const { data: session } = useSession();
  const readsMessages =
    isAdmin(session) || hasPermission(session, "users:read");
  const { canReviewParentRequests, isLoading: accessLoading } =
    useChangeRequestAccess();
  // Nachrichten und Anfragen hängen an getrennten Rechten: wer nur Anfragen
  // entscheidet, sieht trotzdem deren Zahl.
  const key =
    enabled && !accessLoading && (readsMessages || canReviewParentRequests)
      ? messageKeys.parentAttention(readsMessages, canReviewParentRequests)
      : null;
  const { data, mutate } = useSWRAuth(
    key,
    async () => {
      const [threads, requests] = await Promise.all([
        readsMessages
          ? fetchInboxWithFilters({ onlyUnread: true })
          : Promise.resolve([]),
        canReviewParentRequests ? fetchAllOpenRequests() : Promise.resolve([]),
      ]);
      return countParentAttention(threads, requests);
    },
    { revalidateOnFocus: true, keepPreviousData: true },
  );

  useEffect(() => {
    if (key === null) return;
    return subscribeRefresh(key, () => void mutate());
  }, [key, mutate]);

  return data ?? null;
}

/** Die Zahlen eines Kindes, oder null ohne neue Nachrichten und Anfragen. */
export function useStudentParentAttention(
  studentId: string,
): ParentAttention | null {
  const attention = useParentAttentionMap()?.get(studentId);
  return attention && attention.total > 0 ? attention : null;
}
