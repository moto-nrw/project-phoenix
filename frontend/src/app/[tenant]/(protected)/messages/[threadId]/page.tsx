"use client";

import {
  isCurrentSWRValue,
  stampSWRValue,
  dataGenerationOf,
} from "~/lib/swr/generation";
import { messageKeys } from "~/lib/swr/portal-keys";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { useSWRCacheKey } from "~/lib/swr";
import { useSWRConfig } from "~/lib/swr/cache";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { Button } from "~/components/ui/button";
import { Alert } from "~/components/ui/alert";
import { EmptyState } from "~/components/ui/empty-state";
import { formErrorDetail, formErrorMessage } from "~/components/ui/form-error";
import {
  FormErrorAlert,
  LoadErrorAlert,
  errorAlertActions,
} from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import NavigationLink from "~/components/ui/navigation-link";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { MessageComposer } from "~/components/messaging/message-composer";
import { ChatBubble, ChatEventCard } from "~/components/messaging/chat-bubble";
import { PickupRequestDetailModal } from "~/components/messaging/pickup-request-detail-modal";
import { RequestStatusBadge } from "~/components/messaging/request-status-badge";
import { useChatViewportLock } from "~/lib/hooks/use-chat-viewport-lock";
import { useMessagesActivity } from "~/lib/hooks/use-messages-activity";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { useApiErrorDisplay, useApiFormError } from "~/contexts/ToastContext";
import { useTenant } from "~/lib/tenant-context";
import { useTenantAwarePath } from "~/lib/tenant-path";
import { useTenantRouter } from "~/lib/tenant-router";
import {
  type InboxThread,
  type Message,
  type ThreadDetail,
  fetchThread,
  markThreadUnread,
  postMessage,
  readersLabel,
  relationshipLabel,
} from "~/lib/parent-messages-api";
import {
  LEADERSHIP_AUDIENCE_LABEL,
  pickupRequestRef,
  requestPillCoveredByReview,
  staffRequestStatusLabel,
} from "~/lib/messaging-status";
import type { StudentRequestReviewCoverage } from "~/lib/change-request-access";
import { fetchStudentRequestReviewCoverage } from "~/lib/change-requests-api";
import { useSWRAuth } from "~/lib/swr";
import {
  hasEffectiveAdminScope,
  hasPermission,
  isAdmin,
} from "~/lib/auth-utils";
import { createLogger } from "~/lib/logger";
import { formatChatDateTime } from "~/lib/date-helpers";
import { ThreadSkeleton, ThreadMessagesSkeleton } from "./page-skeleton";

const logger = createLogger({ component: "MessageThreadPage" });

const MARK_UNREAD_LABEL = "Als ungelesen markieren";

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

/** Rückweg jeder Unterhaltung: der Posteingang der Elternnachrichten. */
const MESSAGES_BACK = {
  back: true,
  backHref: "/messages",
  backLabel: "Zurück zu Nachrichten",
} as const;

function MessageThreadContent() {
  const params = useParams();
  const threadId = params.threadId as string;
  const router = useTenantRouter();
  const { data: session } = useSession();
  // The Änderungsanfragen queue is gated on users:update (backend + page guard),
  // scoped per child in the service. One who lacks users:update sees the plain
  // info pill.
  const canReviewRequests =
    isAdmin(session ?? null) || hasPermission(session ?? null, "users:update");
  // Admin role and admin wildcard review school-wide in the backend policy.
  const schoolWideReviewer = hasEffectiveAdminScope(session ?? null);
  const { tenant } = useTenant();
  // Tenant-prefix every SWR key on this page so a tenant switch (multi-tab /
  // switch-tenant) can never render the previous school's cached thread under
  // the new tenant until revalidation 403/404s. Mirrors the inbox key, which is
  // also tenant-prefixed (frontend convention: useTenantSlugSafe for cache-key
  // prefixing).
  // When the school has parent-OGS messaging turned off, replying hits a
  // backend 403 (PostMessage → requireEnabled). The inbox already hides the
  // "Neue Nachricht" entry on the same flag; mirror it here so an existing
  // thread renders read-only instead of dead-ending staff on send. History
  // stays visible.
  const messagingEnabled = tenant?.messagingEnabled === true;

  const { cache } = useSWRConfig();
  const cacheKey = useSWRCacheKey();
  const leavingAsUnreadRef = useRef(false);
  const pendingThreadLoadsRef = useRef(new Set<Promise<ThreadDetail>>());

  // Seed the header instantly from the inbox SWR cache (subject, guardian,
  // child) so opening a chat shows its structure immediately instead of a
  // full-page skeleton; the messages then fill in.
  const seed = useMemo<ThreadDetail | undefined>(() => {
    // The inbox is keyed ["messages-inbox", onlyUnread], so the seed must search
    // every filter combination that may be cached.
    for (const unread of [false, true]) {
      const key = cacheKey(messageKeys.messagesInbox(unread));
      if (key === null) continue;
      const entry = cache.get(key) as { data?: InboxThread[] } | undefined;
      if (!entry?.data || isCurrentSWRValue(key, entry.data) !== true) continue;
      const row = entry.data.find((t) => t.thread_id === threadId);
      if (row) {
        const seeded: ThreadDetail = {
          thread_id: row.thread_id,
          student_id: row.student_id,
          student_name: row.student_name,
          guardian_name: row.guardian_name,
          relationship_type: row.relationship_type,
          audience: row.audience,
          messages: [],
        };
        const targetKey = cacheKey(messageKeys.messageThread(threadId));
        if (targetKey !== null)
          stampSWRValue(targetKey, seeded, dataGenerationOf(key));
        return seeded;
      }
    }
    return undefined;
  }, [cache, threadId, cacheKey]);

  const {
    data: thread,
    error: loadError,
    isLoading,
    isValidating,
    mutate,
  } = useSWRAuth(
    messageKeys.messageThread(threadId),
    async () => {
      const load = fetchThread(threadId);
      pendingThreadLoadsRef.current.add(load);
      try {
        return await load;
      } finally {
        pendingThreadLoadsRef.current.delete(load);
      }
    },
    {
      keepPreviousData: false,
      revalidateOnFocus: false,
      isPaused: () => leavingAsUnreadRef.current,
      fallbackData: seed,
      onError: (err: unknown) =>
        logger.error("thread_load_failed", {
          error: err instanceof Error ? err.message : String(err),
          thread_id: threadId,
        }),
    },
  );

  // The thread is readable school-wide, the request detail and queue only
  // within the review scope (#3886). Ask the backend whether that scope
  // reaches this child before offering a pill action; administrators are
  // school-wide and need no round trip. Until the answer is in, and on any
  // failure, the pills stay plain info.
  const studentId = thread?.student_id;
  const { data: fetchedCoverage } = useSWRAuth<StudentRequestReviewCoverage>(
    canReviewRequests && !schoolWideReviewer && studentId && session?.user.id
      ? `request-review-coverage:${session.user.id}:${studentId}`
      : null,
    () => fetchStudentRequestReviewCoverage(studentId ?? ""),
    {
      // The policy can change while this thread stays open. Refresh on return
      // to remove actions that are no longer covered before they lead to a 403.
      revalidateOnFocus: true,
      shouldRetryOnError: false,
      onError: (err: unknown) =>
        logger.warn("request_review_coverage_failed", {
          error: err instanceof Error ? err.message : String(err),
          thread_id: threadId,
        }),
    },
  );
  const reviewCoverage: StudentRequestReviewCoverage | null = schoolWideReviewer
    ? { requests: true, absences: true }
    : (fetchedCoverage ?? null);

  // Katalogtext, Wiederholen und Vorgangskennung für einen Ladefehler
  // (#2517), dort, wo der Verlauf fehlt.
  const shownLoadError = useSwrLoadError(loadError, "die Unterhaltung", mutate);

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

  // A pickup-change pill (created or decided) opens exactly the request row it
  // references, in a read-only view, for as long as the row exists (#3135).
  // Other request pills keep the deep-link to the queue while still open.
  // Either action needs the review scope to reach the child (#3886).
  const [openRequestId, setOpenRequestId] = useState<string | null>(null);
  const eventAction = (
    message: Message,
  ): { label: string; onClick: () => void } | undefined => {
    if (!canReviewRequests) return undefined;
    if (!requestPillCoveredByReview(message, reviewCoverage)) return undefined;
    const pickupRef = pickupRequestRef(message);
    if (pickupRef) {
      return {
        label: "Anfrage ansehen",
        onClick: () => setOpenRequestId(pickupRef),
      };
    }
    if (requestStillOpen(message)) {
      return {
        label: "Anfrage bearbeiten",
        onClick: () => router.push("/anfragen"),
      };
    }
    return undefined;
  };

  const [draft, setDraft] = useState("");
  const [isSending, setIsSending] = useState(false);
  // Ein Sendefehler steht direkt über dem Eingabefeld (#2517).
  const sendError = useApiFormError();
  const latestSendRef = useRef<() => void>(() => undefined);
  const scrollRef = useRef<HTMLDivElement | null>(null);

  // Parent sent a message or staff replied → revalidate this thread. The fan-out
  // is tenant-wide, so only reload when the event names THIS thread (thread_id is
  // retained in the sanitized staff broadcast); unrelated threads are skipped.
  // Debounced so a burst of messages in the open thread collapses into one
  // fetchThread (each does list + read-receipt join + MarkReadUpTo).
  // refetchOnFocus: this view's only refresh path is SSE (revalidateOnFocus is off
  // above), so if the connection dropped while the tab slept a message could have
  // been missed entirely — refetch on return to heal in lockstep with the badge.
  // After "Als ungelesen markieren" the page is on its way to the inbox. A
  // refetch now would open the thread again and end the mark at once, so the
  // SSE refresh stands down while leaving.
  const refreshThread = useCallback(() => {
    if (!leavingAsUnreadRef.current) void mutate();
  }, [mutate]);
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
      sendError.invalid(
        "Bitte warten Sie, bis der Nachrichtenverlauf geladen ist.",
      );
      return;
    }
    setIsSending(true);
    sendError.clear();
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
      void sendError.show(err, {
        object: "die Nachricht",
        retry: () => latestSendRef.current(),
      });
    } finally {
      setIsSending(false);
    }
  };
  // „Wiederholen“ sendet den Text, der dann im Feld steht.
  useLayoutEffect(() => {
    latestSendRef.current = () => void handleSend();
  });

  // Marks the conversation unread for the whole team and returns to the inbox.
  // Staying here is not an option: this page reloads the thread on every SSE
  // event, and loading it would mark it read again right away.
  // Eine Aktion ohne Formular: der Fehler kommt als Meldung mit Wiederholen.
  const { show: showActionError } = useApiErrorDisplay();
  const latestMarkUnreadRef = useRef<() => void>(() => undefined);
  const handleMarkUnread = async () => {
    if (leavingAsUnreadRef.current) return;
    leavingAsUnreadRef.current = true;
    try {
      // A GET already in flight can mark the thread read after the POST.
      // Let every started read finish before creating the new unread mark.
      await Promise.allSettled([...pendingThreadLoadsRef.current]);
      await markThreadUnread(threadId);
      window.dispatchEvent(new CustomEvent("messages-unread-refresh"));
      router.push("/messages");
    } catch (err) {
      leavingAsUnreadRef.current = false;
      logger.error("thread_mark_unread_failed", {
        error: err instanceof Error ? err.message : String(err),
        thread_id: threadId,
      });
      void showActionError(err, {
        object: "das Markieren als ungelesen",
        retry: () => latestMarkUnreadRef.current(),
      });
    }
  };
  useLayoutEffect(() => {
    latestMarkUnreadRef.current = () => void handleMarkUnread();
  });

  // Pin the chat to the viewport and lock page scroll (only the message list
  // scrolls). Measured once the thread renders so the layout is final.
  const containerRef = useChatViewportLock<HTMLDivElement>(
    Boolean(thread) && !isLoading,
  );

  // Skeleton only for a direct deep-link with no cached seed. The back link
  // and card frame are real chrome and render immediately regardless; only
  // the header (guardian name, "Zum Kinderprofil") and message list are
  // data-bound and skeletonize. The composer stays real — its structural
  // frame doesn't depend on thread data, and `disabled` already covers the
  // loading window via `snapshotUnavailable`.
  // Bis der Katalogtext eines Ladefehlers da ist, bleibt das Skelett stehen.
  const showSkeleton =
    !thread &&
    (isLoading ||
      (Boolean(loadError) && formErrorMessage(shownLoadError) === null));

  // Statuszeile unter dem Titel: Beziehung, Kind und die Zahl der geladenen
  // Nachrichten.
  // Eine Leitungs-Unterhaltung trägt das vorne in der Statuszeile (#5200).
  const statusLine = thread
    ? `${thread.audience === "leadership" ? `${LEADERSHIP_AUDIENCE_LABEL} · ` : ""}${relationshipLabel(thread.relationship_type)} von ${thread.student_name} · ${
        messagesLoading
          ? "Nachrichten werden geladen"
          : `${messages.length} ${messages.length === 1 ? "Nachricht" : "Nachrichten"}`
      }`
    : null;

  if (!showSkeleton && !thread) {
    return (
      <TenantPage
        title="Unterhaltung"
        {...MESSAGES_BACK}
        error={
          loadError
            ? {
                message: formErrorMessage(shownLoadError) ?? "",
                action: errorAlertActions(formErrorDetail(shownLoadError)),
              }
            : "Unterhaltung nicht gefunden."
        }
      />
    );
  }

  return (
    <TenantPage
      title={thread?.guardian_name ?? "Unterhaltung"}
      {...MESSAGES_BACK}
      stats={statusLine}
      statsLoading={showSkeleton}
      actions={
        thread ? (
          <>
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={() =>
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
            <OverflowMenu
              ariaLabel="Weitere Aktionen"
              items={[
                {
                  label: MARK_UNREAD_LABEL,
                  onClick: () => void handleMarkUnread(),
                },
              ]}
            />
          </>
        ) : null
      }
    >
      <div
        ref={containerRef}
        className="flex min-h-[20rem] w-full flex-col overflow-hidden"
      >
        {/* `moto-scroll-surface`: die Höhe setzt `useChatViewportLock`, nicht
            die Wachs-Regel des Seitenrumpfs; nur die Liste scrollt (#3328). */}
        <SectionCard
          className="moto-scroll-surface flex min-h-0 flex-1 flex-col"
          bodyClassName="flex min-h-0 flex-1 flex-col"
        >
          {showSkeleton ? (
            <ThreadSkeleton />
          ) : (
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
                      action={eventAction(message)}
                    />
                  ) : (
                    <div key={message.id}>
                      {message.group_thread_id && (
                        <GroupThreadReference
                          id={message.group_thread_id}
                          title={message.group_thread_title}
                        />
                      )}
                      <ChatBubble
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
                    </div>
                  ),
                )
              ) : loadError ? (
                // The header seed (fallbackData) keeps `thread` truthy with an empty
                // messages array, so a failed history fetch would otherwise render the
                // "no messages" empty state for a thread that actually has history.
                // Surface the load failure instead of a misleading empty conversation.
                <LoadErrorAlert error={shownLoadError} />
              ) : (
                <EmptyState
                  title="Noch keine Nachrichten"
                  description="In dieser Unterhaltung wurde noch nichts geschrieben."
                />
              )}
            </div>
          )}

          <FormErrorAlert message={sendError.error} className="mt-3" />

          {/* Wer mitliest, steht direkt über dem Schreibfeld (#5200). Die
              Angabe kommt vom Backend, das Frontend rät sie nie. */}
          <div className="mt-4">
            {thread?.readers ? (
              <p
                className="mb-2 text-sm text-gray-500"
                data-testid="thread-readers"
              >
                {readersLabel(thread.readers)}
              </p>
            ) : null}
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
        </SectionCard>
      </div>

      <PickupRequestDetailModal
        requestId={openRequestId}
        onClose={() => setOpenRequestId(null)}
        onOpenQueue={() => router.push("/anfragen")}
      />
    </TenantPage>
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
    <SectionCard>
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
    </SectionCard>
  );
}

// A guardian's answer to a Gruppennachricht (#4784) names the thread it
// answers, so the OGS reads it in context. The other families never see it.
function GroupThreadReference({
  id,
  title,
}: Readonly<{ id: string; title?: string }>) {
  const tenantPath = useTenantAwarePath();

  return (
    <p className="mb-1 px-1 text-xs text-gray-500">
      Antwort auf Gruppennachricht:{" "}
      <NavigationLink
        href={tenantPath(`/messages/groups/${encodeURIComponent(id)}`)}
        className="font-medium text-gray-700 underline underline-offset-2 hover:text-gray-900"
      >
        {title ?? "Gruppennachricht"}
      </NavigationLink>
    </p>
  );
}

export default function MessageThreadPage() {
  return <MessageThreadContent />;
}
