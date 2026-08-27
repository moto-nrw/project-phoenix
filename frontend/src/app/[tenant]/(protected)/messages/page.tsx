"use client";

import { useCallback, useMemo, useState } from "react";
import useSWR from "swr";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { TenantPage } from "~/components/ui/tenant-page";
import { Button } from "~/components/ui/button";
import { Alert } from "~/components/ui/alert";
import { UnreadBadge } from "~/components/messaging/unread-badge";
import { useTenant, useTenantSlugSafe } from "~/lib/tenant-context";
import { useTenantRouter } from "~/lib/tenant-router";
import {
  type InboxThread,
  fetchInboxWithFilters,
  relationshipLabel,
} from "~/lib/parent-messages-api";
import { NewMessageModal } from "~/components/messaging/new-message-modal";
import { useMessagesActivity } from "~/lib/hooks/use-messages-activity";
import { createLogger } from "~/lib/logger";
import { formatChatDateTime } from "~/lib/date-helpers";
import { MessagesSkeleton } from "./page-skeleton";

const logger = createLogger({ component: "MessagesInboxPage" });

function MessagesInboxContent() {
  const router = useTenantRouter();
  const { tenant } = useTenant();
  // Tenant-prefix the SWR key so a tenant switch (multi-tab / switch-tenant) can
  // never render the previous school's cached threads from this key (frontend
  // convention: useTenantSlugSafe is for SWR cache-key prefixing).
  const tenantSlug = useTenantSlugSafe();
  // Hide the compose entry points when the school has parent-OGS messaging
  // turned off — composing would otherwise hit a backend 403 dead-end. Existing
  // threads stay readable.
  const messagingEnabled = tenant?.messagingEnabled === true;

  const [searchTerm, setSearchTerm] = useState("");
  const [onlyUnread, setOnlyUnread] = useState(false);
  const [composeOpen, setComposeOpen] = useState(false);

  // SWR caches the inbox per filter across navigation, so returning from a
  // chat shows the list instantly and revalidates in the background — no
  // skeleton flash. keepPreviousData avoids a flash when toggling the filter.
  const {
    data: threads,
    error,
    isLoading,
    mutate,
  } = useSWR(
    [`${tenantSlug ?? ""}:messages-inbox`, onlyUnread],
    () => fetchInboxWithFilters({ onlyUnread }),
    {
      revalidateOnFocus: false,
      keepPreviousData: true,
      onError: (err: unknown) =>
        logger.error("inbox_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        }),
    },
  );

  // Parent sent a message or staff replied → revalidate the inbox. The backend
  // wakes EVERY tenant staffer per message and this inbox query is heavy (joins
  // + correlated unread subqueries), so debounce: a burst collapses into a
  // single revalidation instead of one refetch per event.
  // Refetch-only (revalidates the list, never advances a read cursor), so fire
  // even in a background tab — marksRead: false skips the hidden-tab deferral.
  const refreshInbox = useCallback(() => void mutate(), [mutate]);
  useMessagesActivity({
    onMatch: refreshInbox,
    debounceMs: 500,
    marksRead: false,
  });

  const filteredThreads = useMemo(() => {
    const list: InboxThread[] = threads ?? [];
    if (!searchTerm) return list;
    const term = searchTerm.toLowerCase();
    return list.filter(
      (thread) =>
        thread.guardian_name.toLowerCase().includes(term) ||
        thread.student_name.toLowerCase().includes(term),
    );
  }, [threads, searchTerm]);

  // Skeleton only on the very first load (no cached data yet). The header,
  // filter bar, and compose entry point are real chrome and render
  // immediately regardless — only the thread list skeletonizes.
  const showSkeleton = isLoading && !threads;

  // Statuszeile unter dem Seitentitel, allein aus der geladenen Inbox.
  const threadList: InboxThread[] = threads ?? [];
  const unreadThreads = threadList.filter(
    (thread) => thread.unread_count > 0,
  ).length;
  const inboxSummary = `${threadList.length} ${threadList.length === 1 ? "Unterhaltung" : "Unterhaltungen"} · ${unreadThreads} ungelesen`;

  const composeButton = messagingEnabled ? (
    <Button
      type="button"
      variant="primary"
      size="md"
      onClick={() => setComposeOpen(true)}
    >
      Neue Nachricht
    </Button>
  ) : undefined;

  // Der Fehler ersetzt den Inhalt nur, wenn nichts anzuzeigen ist. Liegt noch
  // eine (möglicherweise veraltete) Liste vor, bleibt sie stehen und der
  // Hinweis steht darüber.
  const hasThreads = filteredThreads.length > 0;
  const loadFailed = Boolean(error);
  // Fehler- und Leerzustand ersetzen den Inhalt des Gerüsts. Solange das
  // Verfassen-Fenster offen ist, muss der Inhalt gerendert werden, denn dort
  // hängt das Fenster.
  const bodyReplaced = !showSkeleton && !hasThreads && !composeOpen;

  return (
    <TenantPage
      title="Nachrichten"
      stats={inboxSummary}
      statsLoading={showSkeleton}
      actions={composeButton}
      search={{
        value: searchTerm,
        onChange: setSearchTerm,
        placeholder: "Person oder Kind suchen…",
      }}
      filters={[
        {
          id: "unread",
          type: "dropdown",
          label: "Nachrichten filtern",
          value: onlyUnread ? "unread" : "all",
          onChange: (next) => setOnlyUnread(next === "unread"),
          options: [
            { value: "all", label: "Alle Nachrichten" },
            { value: "unread", label: "Nur ungelesen" },
          ],
        },
      ]}
      error={
        bodyReplaced && loadFailed
          ? "Nachrichten konnten nicht geladen werden."
          : null
      }
      empty={
        bodyReplaced && !loadFailed
          ? {
              icon: <MotoConceptIcon concept="parentConversations" size={48} />,
              title: "Noch keine Nachrichten",
              description: messagingEnabled
                ? "Hier erscheinen Unterhaltungen mit den Eltern. Über „Neue Nachricht“ können Sie selbst eine beginnen."
                : "Der Nachrichtenaustausch mit den Eltern ist für diese Schule ausgeschaltet.",
              action: composeButton,
            }
          : null
      }
    >
      {showSkeleton ? (
        <MessagesSkeleton />
      ) : (
        <>
          {loadFailed && (
            <Alert
              type="error"
              message="Nachrichten konnten nicht geladen werden."
            />
          )}

          <ul className="space-y-3">
            {filteredThreads.map((thread) => {
              const navigate = () =>
                router.push(`/messages/${thread.thread_id}`);

              return (
                <li key={thread.thread_id}>
                  <button
                    type="button"
                    onClick={navigate}
                    className="moto-content-surface moto-hover-elevated block w-full cursor-pointer rounded-2xl border border-gray-200 bg-white p-4 text-left shadow-sm focus-visible:ring-2 focus-visible:ring-gray-300 focus-visible:ring-offset-2 focus-visible:outline-none sm:p-5"
                  >
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0 flex-1">
                        <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
                          <h3 className="truncate text-base font-semibold text-gray-900">
                            {thread.guardian_name}
                          </h3>
                          <span className="truncate text-sm text-gray-500">
                            {relationshipLabel(thread.relationship_type)} von{" "}
                            {thread.student_name}
                          </span>
                          <UnreadBadge count={thread.unread_count} />
                        </div>
                        {thread.last_message_body && (
                          <p className="mt-1 truncate text-sm text-gray-600">
                            {thread.last_sender_kind === "staff" && (
                              <span className="text-gray-500">Sie: </span>
                            )}
                            {thread.last_message_body}
                          </p>
                        )}
                      </div>
                      {thread.last_message_at && (
                        <span className="flex-shrink-0 text-xs whitespace-nowrap text-gray-400">
                          {formatChatDateTime(thread.last_message_at)}
                        </span>
                      )}
                    </div>
                  </button>
                </li>
              );
            })}
          </ul>
        </>
      )}

      {composeOpen && (
        <NewMessageModal
          onClose={() => setComposeOpen(false)}
          onOpened={(thread) => router.push(`/messages/${thread.thread_id}`)}
        />
      )}
    </TenantPage>
  );
}

export default function MessagesPage() {
  return <MessagesInboxContent />;
}
