"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useParams } from "next/navigation";
import { useSession } from "next-auth/react";
import useSWR, { unstable_serialize, useSWRConfig } from "swr";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { Button } from "~/components/ui/button";
import { Alert } from "~/components/ui/alert";
import { EmptyState } from "~/components/ui/empty-state";
import { BackButton } from "~/components/ui/back-button";
import { MessageComposer } from "~/components/messaging/message-composer";
import { ChatBubble, ChatEventCard } from "~/components/messaging/chat-bubble";
import { RequestStatusBadge } from "~/components/messaging/request-status-badge";
import { useChatViewportLock } from "~/lib/hooks/use-chat-viewport-lock";
import { useMessagesActivity } from "~/lib/hooks/use-messages-activity";
import { useTenant, useTenantSlugSafe } from "~/lib/tenant-context";
import { useTenantRouter } from "~/lib/tenant-router";
import {
  type InboxThread,
  type Message,
  type ThreadDetail,
  fetchThread,
  postMessage,
  relationshipLabel,
} from "~/lib/parent-messages-api";
import { getApiErrorMessage } from "~/lib/api-error-message";
import { staffRequestStatusLabel } from "~/lib/messaging-status";
import { hasPermission, isAdmin } from "~/lib/auth-utils";
import { createLogger } from "~/lib/logger";
import { formatChatDateTime } from "~/lib/date-helpers";
import { ThreadSkeleton, ThreadMessagesSkeleton } from "./page-skeleton";

const logger = createLogger({ component: "MessageThreadPage" });

export function isMessageSnapshotUnavailable(
  isLoading: boolean,
  isValidating: boolean,
  messageCount: number,
  loadError: unknown,
): boolean {
  return (
    Boolean(loadError) || ((isLoading || isValidating) && messageCount === 0)
  );
}

function MessageThreadContent() {
  const params = useParams();
  const threadId = params.threadId as string;
  const router = useTenantRouter();
  const { data: session } = useSession();
  // The Änderungsanfragen queue is gated on users:update (backend + page guard),
  // scoped per child in the service. So the deep-link on a "request created" pill
  // shows for any staffer who may edit children; one who lacks users:update sees
  // the plain info pill.
  const canReviewRequests =
    isAdmin(session ?? null) || hasPermission(session ?? null, "users:update");
  const { tenant } = useTenant();
  // Tenant-prefix every SWR key on this page so a tenant switch (multi-tab /
  // switch-tenant) can never render the previous school's cached thread under
  // the new tenant until revalidation 403/404s. Mirrors the inbox key, which is
  // also tenant-prefixed (frontend convention: useTenantSlugSafe for cache-key
  // prefixing).
  const tenantSlug = useTenantSlugSafe();
  // When the school has parent-OGS messaging turned off, replying hits a
  // backend 403 (PostMessage → requireEnabled). The inbox already hides the
  // "Neue Nachricht" entry on the same flag; mirror it here so an existing
  // thread renders read-only instead of dead-ending staff on send. History
  // stays visible.
  const messagingEnabled = tenant?.messagingEnabled === true;

  const { cache } = useSWRConfig();

  // Seed the header instantly from the inbox SWR cache (subject, guardian,
  // child) so opening a chat shows its structure immediately instead of a
  // full-page skeleton; the messages then fill in.
  const seed = useMemo<ThreadDetail | undefined>(() => {
    // The inbox is keyed ["messages-inbox", onlyUnread], so the seed must search
    // every filter combination that may be cached.
    for (const unread of [false, true]) {
      const entry = cache.get(
        unstable_serialize([`${tenantSlug ?? ""}:messages-inbox`, unread]),
      ) as { data?: InboxThread[] } | undefined;
      const row = entry?.data?.find((t) => t.thread_id === threadId);
      if (row) {
        return {
          thread_id: row.thread_id,
          student_id: row.student_id,
          student_name: row.student_name,
          guardian_name: row.guardian_name,
          relationship_type: row.relationship_type,
          messages: [],
        };
      }
    }
    return undefined;
  }, [cache, threadId, tenantSlug]);

  const {
    data: thread,
    error: loadError,
    isLoading,
    isValidating,
    mutate,
  } = useSWR(
    [`${tenantSlug ?? ""}:message-thread`, threadId],
    () => fetchThread(threadId),
    {
      revalidateOnFocus: false,
      fallbackData: seed,
      onError: (err: unknown) =>
        logger.error("thread_load_failed", {
          error: err instanceof Error ? err.message : String(err),
          thread_id: threadId,
        }),
    },
  );

  const messages: Message[] = thread?.messages ?? [];
  // The inbox seed is passed as fallbackData with messages: [], so isLoading is
  // already false on first render while the real fetchThread is still in flight.
  // Gate on isValidating too, or a thread WITH history flashes "Noch keine
  // Nachrichten…" until the GET resolves. A background revalidation of a thread
  // that already has messages keeps messages.length > 0, so this stays false.
  const messagesLoading = (isLoading || isValidating) && messages.length === 0;
  const snapshotUnavailable = isMessageSnapshotUnavailable(
    isLoading,
    isValidating,
    messages.length,
    loadError,
  );

  // Per-ROW open/closed tracking for "request_created" pills. A pill offers the
  // "Anfrage bearbeiten" action only while its request is still open; once
  // decided (bestätigt / abgelehnt / zurückgezogen) a "request_status" pill for
  // the SAME request row arrives, carrying the identical ref_table + ref_id.
  // Keying by ref_table:ref_id (not request_type) is required for multi-row
  // master-data submissions: those emit one request_created pill per changed
  // field, all of request_type "master_data", each a separate request row.
  // Deciding one row must NOT collapse the still-open sibling rows' actions.
  const refKey = (m: Message): string | null =>
    m.ref_table && m.ref_id ? `${m.ref_table}:${m.ref_id}` : null;
  const decidedRefs = new Set<string>();
  for (const m of messages) {
    if (m.kind === "event" && m.event_type === "request_status") {
      const key = refKey(m);
      if (key) decidedRefs.add(key);
    }
  }

  const requestStillOpen = (message: Message): boolean => {
    if (message.event_type !== "request_created") {
      return false;
    }
    const key = refKey(message);
    // No row reference (legacy pill): default to open — the action only
    // deep-links to the queue, so showing it is harmless while hiding a live
    // request is the real regression.
    return key === null || !decidedRefs.has(key);
  };

  const [draft, setDraft] = useState("");
  const [isSending, setIsSending] = useState(false);
  const [sendError, setSendError] = useState<string | null>(null);
  const scrollRef = useRef<HTMLDivElement | null>(null);

  // Parent sent a message or staff replied → revalidate this thread. The fan-out
  // is tenant-wide, so only reload when the event names THIS thread (thread_id is
  // retained in the sanitized staff broadcast); unrelated threads are skipped.
  // Debounced so a burst of messages in the open thread collapses into one
  // fetchThread (each does list + read-receipt join + MarkReadUpTo).
  // refetchOnFocus: this view's only refresh path is SSE (revalidateOnFocus is off
  // above), so if the connection dropped while the tab slept a message could have
  // been missed entirely — refetch on return to heal in lockstep with the badge.
  const refreshThread = useCallback(() => void mutate(), [mutate]);
  useMessagesActivity({
    onMatch: refreshThread,
    threadId,
    debounceMs: 500,
    refetchOnFocus: true,
  });

  // Loading the thread marks it read server-side, so nudge the sidebar unread
  // badge to refetch. Gate on !isValidating (NOT !isLoading): the inbox seed is
  // passed as fallbackData, so `data` is defined on first render and isLoading is
  // already false while the real GET is still in flight — firing then would refetch
  // the badge against the STILL-stale (lit) count before the read cursor advanced.
  // isValidating stays true until the GET resolves, so this fires only once the
  // real read has happened (mirrors the messagesLoading gate above).
  useEffect(() => {
    if (thread && !isValidating && typeof window !== "undefined") {
      window.dispatchEvent(new CustomEvent("messages-unread-refresh"));
    }
  }, [thread, isValidating]);

  // Keep the newest message in view when the list changes.
  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [thread]);

  const handleSend = async () => {
    const body = draft.trim();
    if (!body || isSending) return;
    if (snapshotUnavailable) {
      setSendError("Bitte warten Sie, bis der Nachrichtenverlauf geladen ist.");
      return;
    }
    setIsSending(true);
    setSendError(null);
    try {
      const updated = await postMessage(threadId, body, messages.at(-1)?.id);
      // Show the sent message immediately. The postMessage response now carries
      // the rebuilt "current → requested" diff inline on every still-open request
      // (like the GET path), so the optimistic replace keeps the review card's
      // comparison intact even before the revalidate lands — staff can't end up
      // confirming without the preview. Revalidate stays as a freshness backstop.
      await mutate((prev) => (prev ? { ...prev, messages: updated } : prev), {
        revalidate: true,
      });
      setDraft("");
    } catch (err) {
      logger.error("thread_message_send_failed", {
        error: err instanceof Error ? err.message : String(err),
        thread_id: threadId,
      });
      setSendError(
        getApiErrorMessage(
          err,
          "senden",
          "Nachricht",
          "Nachricht konnte nicht gesendet werden.",
        ),
      );
    } finally {
      setIsSending(false);
    }
  };

  // Pin the chat to the viewport and lock page scroll (only the message list
  // scrolls). Measured once the thread renders so the layout is final.
  const containerRef = useChatViewportLock<HTMLDivElement>(
    Boolean(thread) && !isLoading,
  );

  // Skeleton only for a direct deep-link with no cached seed. The back nav
  // and card frame are real chrome and render immediately regardless; only
  // the header (guardian name, "Zum Kinderprofil") and message list are
  // data-bound and skeletonize. The composer stays real — its structural
  // frame doesn't depend on thread data, and `disabled` already covers the
  // loading window via `snapshotUnavailable`.
  const showSkeleton = !thread && isLoading;

  if (!showSkeleton && !thread) {
    return (
      <div className="w-full space-y-4">
        <BackButton referrer="/messages" />
        <Alert
          type="error"
          message={
            loadError
              ? "Nachrichtenverlauf konnte nicht geladen werden."
              : "Unterhaltung nicht gefunden."
          }
        />
      </div>
    );
  }

  return (
    <div
      ref={containerRef}
      className="flex min-h-[20rem] w-full flex-col overflow-hidden"
    >
      <BackButton referrer="/messages" />

      <div className="moto-content-surface flex min-h-0 flex-1 flex-col rounded-2xl border p-4 backdrop-blur-sm sm:p-6">
        {showSkeleton ? (
          <ThreadSkeleton />
        ) : (
          <>
            <div className="mb-4 flex items-start justify-between gap-3">
              {/* Kopf der Unterhaltung in PageIntro-Optik (Kicker, Titel,
                  Unterzeile). Eine zweite Kopfkarte darüber ist hier nicht
                  möglich: die Chat-Karte ist an das Sichtfenster gekoppelt
                  (useChatViewportLock) und trägt den Kopf selbst. */}
              <div className="min-w-0">
                <p className="text-moto-blue text-xs font-semibold tracking-wide uppercase">
                  Nachrichten
                </p>
                <h1 className="mt-1 truncate text-xl leading-tight font-semibold tracking-tight text-gray-900 sm:text-2xl">
                  {thread?.guardian_name}
                </h1>
                <p className="mt-1 truncate text-sm leading-6 text-gray-600">
                  {thread ? relationshipLabel(thread.relationship_type) : ""}{" "}
                  von {thread?.student_name}
                </p>
              </div>
              <Button
                type="button"
                variant="outline"
                size="md"
                onClick={() =>
                  thread &&
                  router.push(`/students/${thread.student_id}?from=/messages`)
                }
                className="flex-shrink-0"
              >
                <MotoConceptIcon
                  concept="children"
                  size={18}
                  className="mr-1.5"
                />
                Zum Kinderprofil
              </Button>
            </div>

            <div
              ref={scrollRef}
              className="min-h-0 flex-1 space-y-3 overflow-y-auto pr-1"
            >
              {messagesLoading ? (
                <ThreadMessagesSkeleton />
              ) : messages.length > 0 ? (
                messages.map((message) =>
                  message.kind === "request" ? (
                    <RequestHistoryCard key={message.id} message={message} />
                  ) : message.kind === "event" ? (
                    <ChatEventCard
                      key={message.id}
                      body={message.body}
                      createdAt={message.created_at}
                      action={
                        canReviewRequests && requestStillOpen(message)
                          ? {
                              label: "Anfrage bearbeiten",
                              onClick: () => router.push("/anfragen"),
                            }
                          : undefined
                      }
                    />
                  ) : (
                    <ChatBubble
                      key={message.id}
                      body={message.body}
                      own={message.sender_kind === "staff"}
                      senderName={message.sender_name}
                      createdAt={message.created_at}
                      readReceiptLabel={
                        message.sender_kind === "staff" &&
                        message.read_by_guardian
                          ? "Gelesen"
                          : undefined
                      }
                    />
                  ),
                )
              ) : loadError ? (
                // The header seed (fallbackData) keeps `thread` truthy with an empty
                // messages array, so a failed history fetch would otherwise render the
                // "no messages" empty state for a thread that actually has history.
                // Surface the load failure instead of a misleading empty conversation.
                <Alert
                  type="error"
                  message="Nachrichtenverlauf konnte nicht geladen werden."
                />
              ) : (
                <EmptyState
                  title="Noch keine Nachrichten"
                  description="In dieser Unterhaltung wurde noch nichts geschrieben."
                />
              )}
            </div>
          </>
        )}

        {sendError && (
          <div className="mt-3">
            <Alert type="error" message={sendError} />
          </div>
        )}

        <div className="mt-4">
          {messagingEnabled ? (
            <MessageComposer
              value={draft}
              onChange={setDraft}
              onSend={() => void handleSend()}
              sending={isSending}
              disabled={snapshotUnavailable}
              placeholder="Nachricht an die Eltern schreiben…"
            />
          ) : (
            <Alert
              type="info"
              message="Die Eltern-Nachrichten sind für diese Schule ausgeschaltet. Sie können den Verlauf weiterhin lesen, aber nicht antworten."
            />
          )}
        </div>
      </div>
    </div>
  );
}

// Historical change-request card (read-only). Since #1803 care-schedule change
// requests are decided on the Änderungsanfragen (change-requests) admin page,
// not inline in the chat — so past kind="request" rows render as a plain card
// (title/body + status + timestamp) with no confirm/reject buttons and no diff
// (messages no longer carry one). Live request activity now arrives as
// non-interactive "event" pills (ChatEventCard) with a German body.
function RequestHistoryCard({ message }: Readonly<{ message: Message }>) {
  return (
    <div className="rounded-2xl border border-gray-200 bg-white p-4 shadow-sm">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-sm font-semibold text-gray-900">{message.body}</p>
          <p className="mt-1 text-xs text-gray-500">
            {message.sender_name} • {formatChatDateTime(message.created_at)}
          </p>
        </div>
        <RequestStatusBadge
          label={staffRequestStatusLabel(message.request_status)}
        />
      </div>
    </div>
  );
}

export default function MessageThreadPage() {
  return <MessageThreadContent />;
}
