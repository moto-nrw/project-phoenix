"use client";

import { Suspense, useEffect, useMemo, useRef, useState } from "react";
import { useParams } from "next/navigation";
import { useSession } from "next-auth/react";
import useSWR, { unstable_serialize, useSWRConfig } from "swr";
import { ArrowLeft, User } from "lucide-react";
import { Button } from "~/components/ui/button";
import { Alert } from "~/components/ui/alert";
import { Loading } from "~/components/ui/loading";
import { BackButton } from "~/components/ui/back-button";
import { MessageComposer } from "~/components/messaging/message-composer";
import { ChatBubble, ChatEventCard } from "~/components/messaging/chat-bubble";
import { RequestDiffPanel } from "~/components/messaging/request-diff-panel";
import { RequestStatusBadge } from "~/components/messaging/request-status-badge";
import { useChatViewportLock } from "~/lib/hooks/use-chat-viewport-lock";
import { useMessagesActivity } from "~/lib/hooks/use-messages-activity";
import { hasPermission } from "~/lib/auth-utils";
import { useTenant, useTenantSlugSafe } from "~/lib/tenant-context";
import { useTenantRouter } from "~/lib/tenant-router";
import {
  type InboxThread,
  type Message,
  type ThreadDetail,
  confirmRequest,
  fetchThread,
  postMessage,
  rejectRequest,
  relationshipLabel,
} from "~/lib/parent-messages-api";
import { getApiErrorMessage } from "~/lib/api-error-message";
import { staffRequestStatusLabel } from "~/lib/messaging-status";
import { createLogger } from "~/lib/logger";
import { formatChatDateTime } from "~/lib/date-helpers";

const logger = createLogger({ component: "MessageThreadPage" });

function MessageThreadContent() {
  const params = useParams();
  const threadId = params.threadId as string;
  const router = useTenantRouter();
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

  // Deciding a parent request (Bestätigen / Ablehnen) hits the confirm/reject
  // endpoints, which the backend gates on users:update (route middleware) PLUS
  // CanUpdateStudent (service). A staffer with only users:read — e.g. under
  // gdpr.student_data_scope='all_staff', which grants read on every child but no
  // write — would otherwise see both buttons and get a generic 403 on click.
  // Gate the buttons on the same coarse write permission so we never offer an
  // action that cannot succeed. (Per-child CanUpdateStudent still applies
  // server-side; this just removes the obvious read-only case.)
  const { data: session } = useSession();
  const canDecideRequests = hasPermission(session, "users:update");

  const { cache } = useSWRConfig();

  // Seed the header instantly from the inbox SWR cache (subject, guardian,
  // child) so opening a chat shows its structure immediately instead of a
  // full-page skeleton; the messages then fill in.
  const seed = useMemo<ThreadDetail | undefined>(() => {
    // The inbox is keyed ["messages-inbox", onlyUnread, onlyOpenRequests], so the
    // seed must search every filter combination that may be cached.
    for (const unread of [false, true]) {
      for (const openRequests of [false, true]) {
        const entry = cache.get(
          unstable_serialize([
            `${tenantSlug ?? ""}:messages-inbox`,
            unread,
            openRequests,
          ]),
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

  const [draft, setDraft] = useState("");
  const [isSending, setIsSending] = useState(false);
  const [sendError, setSendError] = useState<string | null>(null);
  const [rejectingRequestId, setRejectingRequestId] = useState<string | null>(
    null,
  );
  const [rejectReason, setRejectReason] = useState("");
  const scrollRef = useRef<HTMLDivElement | null>(null);

  // Parent sent a message or staff replied → revalidate this thread. The fan-out
  // is tenant-wide, so only reload when the event names THIS thread (thread_id is
  // retained in the sanitized staff broadcast); unrelated threads are skipped.
  // Debounced so a burst of messages in the open thread collapses into one
  // fetchThread (each does list + read-receipt join + MarkReadUpTo).
  // refetchOnFocus: this view's only refresh path is SSE (revalidateOnFocus is off
  // above), so if the connection dropped while the tab slept a message could have
  // been missed entirely — refetch on return to heal in lockstep with the badge.
  useMessagesActivity({
    onMatch: () => void mutate(),
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
    setIsSending(true);
    setSendError(null);
    try {
      const updated = await postMessage(threadId, body);
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

  const applyRequestAction = async (
    requestId: string,
    action: "confirm" | "reject",
  ) => {
    setSendError(null);
    try {
      const updated =
        action === "confirm"
          ? await confirmRequest(requestId)
          : await rejectRequest(requestId, rejectReason);
      // The action response carries the rebuilt diff inline for any OTHER request
      // left open in the thread (the one just decided is now closed), so the
      // optimistic replace keeps those review cards intact. Revalidate refreshes
      // read state and acts as a freshness backstop.
      await mutate((prev) => (prev ? { ...prev, messages: updated } : prev), {
        revalidate: true,
      });
      setRejectingRequestId(null);
      setRejectReason("");
    } catch (err) {
      logger.error("thread_request_action_failed", {
        error: err instanceof Error ? err.message : String(err),
        thread_id: threadId,
        action,
      });
      setSendError(
        getApiErrorMessage(
          err,
          "aktualisieren",
          "Anfrage",
          "Anfrage konnte nicht aktualisiert werden.",
        ),
      );
    }
  };

  // Pin the chat to the viewport and lock page scroll (only the message list
  // scrolls). Measured once the thread renders so the layout is final.
  const containerRef = useChatViewportLock<HTMLDivElement>(
    Boolean(thread) && !isLoading,
  );

  // Full-page loader only for a direct deep-link with no cached seed.
  if (!thread && isLoading) {
    return <Loading fullPage={false} />;
  }

  if (!thread) {
    return (
      <div className="-mt-1.5 w-full">
        <MessagesBackNav />
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
      className="-mt-1.5 flex min-h-[20rem] w-full flex-col overflow-hidden"
    >
      <MessagesBackNav />

      <div className="moto-content-surface flex min-h-0 flex-1 flex-col rounded-2xl border p-4 backdrop-blur-sm sm:p-6">
        <div className="mb-4 flex items-start justify-between gap-3">
          <div className="min-w-0">
            <h1 className="truncate text-lg font-semibold text-gray-900 sm:text-xl">
              {thread.guardian_name}
            </h1>
            <p className="mt-0.5 truncate text-sm text-gray-500">
              {relationshipLabel(thread.relationship_type)} von{" "}
              {thread.student_name}
            </p>
          </div>
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={() =>
              router.push(`/students/${thread.student_id}?from=/messages`)
            }
            className="flex-shrink-0"
          >
            <User className="mr-1.5 h-4 w-4" /> Zum Kinderprofil
          </Button>
        </div>

        <div
          ref={scrollRef}
          className="min-h-0 flex-1 space-y-3 overflow-y-auto pr-1"
        >
          {messagesLoading ? (
            <p className="py-6 text-center text-sm text-gray-400">
              Verlauf wird geladen...
            </p>
          ) : messages.length > 0 ? (
            messages.map((message) =>
              message.kind === "request" ? (
                <RequestReviewCard
                  key={message.id}
                  message={message}
                  messagingEnabled={messagingEnabled}
                  canDecide={canDecideRequests}
                  rejecting={rejectingRequestId === message.id}
                  rejectReason={rejectReason}
                  onRejectReasonChange={setRejectReason}
                  onConfirm={() =>
                    void applyRequestAction(message.id, "confirm")
                  }
                  onRejectStart={() => {
                    setRejectingRequestId(message.id);
                    setRejectReason("");
                  }}
                  onRejectCancel={() => {
                    setRejectingRequestId(null);
                    setRejectReason("");
                  }}
                  onRejectSubmit={() =>
                    void applyRequestAction(message.id, "reject")
                  }
                />
              ) : message.kind === "event" ? (
                <ChatEventCard
                  key={message.id}
                  body={message.body}
                  createdAt={message.created_at}
                />
              ) : (
                <ChatBubble
                  key={message.id}
                  body={message.body}
                  own={message.sender_kind === "staff"}
                  senderName={message.sender_name}
                  createdAt={message.created_at}
                  readReceiptLabel={
                    message.sender_kind === "staff" && message.read_by_guardian
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
            <p className="py-6 text-center text-sm text-gray-500">
              Noch keine Nachrichten in dieser Unterhaltung.
            </p>
          )}
        </div>

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
              placeholder="Nachricht an die Eltern schreiben..."
            />
          ) : (
            <p className="rounded-lg bg-gray-50 px-4 py-3 text-sm text-gray-500">
              Die Eltern-Nachrichten sind für diese Schule deaktiviert. Sie
              können den Verlauf weiterhin lesen, aber nicht antworten.
            </p>
          )}
        </div>
      </div>
    </div>
  );
}

function RequestReviewCard({
  message,
  messagingEnabled,
  canDecide,
  rejecting,
  rejectReason,
  onRejectReasonChange,
  onConfirm,
  onRejectStart,
  onRejectCancel,
  onRejectSubmit,
}: Readonly<{
  message: Message;
  messagingEnabled: boolean;
  canDecide: boolean;
  rejecting: boolean;
  rejectReason: string;
  onRejectReasonChange: (value: string) => void;
  onConfirm: () => void;
  onRejectStart: () => void;
  onRejectCancel: () => void;
  onRejectSubmit: () => void;
}>) {
  const open = message.request_status === "offen";
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
      <RequestDiffPanel diff={message.diff} heading="Änderungen" />
      {/* Both decisions require users:update + per-child write server-side, so
          hide them entirely from staff who lack that write capability (canDecide)
          — otherwise a read-only staffer clicks and gets a generic 403.
          Bestätigen additionally calls ConfirmRequest, which the backend gates on
          requireEnabled() and 403s once parent messaging is disabled, so it is
          also hidden when messaging is off. Ablehnen is NOT gated on
          requireEnabled (the backend keeps reject available so an outstanding
          request can still be wound down after messaging is turned off), only on
          write — so it stays visible whenever canDecide. */}
      {open && canDecide ? (
        <div className="mt-4 flex flex-wrap gap-2">
          {messagingEnabled ? (
            <Button
              type="button"
              variant="primary"
              size="md"
              onClick={onConfirm}
            >
              Bestätigen
            </Button>
          ) : null}
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={onRejectStart}
          >
            Ablehnen
          </Button>
        </div>
      ) : null}
      {rejecting ? (
        <div className="mt-3 space-y-2">
          <textarea
            value={rejectReason}
            onChange={(event) => onRejectReasonChange(event.target.value)}
            className="min-h-20 w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-gray-400 focus:ring-2 focus:ring-gray-200 focus:outline-none"
            placeholder="Grund für die Ablehnung"
          />
          <div className="flex justify-end gap-2">
            <Button
              type="button"
              variant="ghost"
              size="md"
              onClick={onRejectCancel}
            >
              Abbrechen
            </Button>
            <Button
              type="button"
              variant="danger"
              size="md"
              onClick={onRejectSubmit}
            >
              Ablehnen
            </Button>
          </div>
        </div>
      ) : null}
    </div>
  );
}

// Back navigation for the thread screen, in ONE place (it renders in both the
// not-found and the loaded branches). Mobile uses the kit BackButton
// (md:hidden); desktop gets an inline "Zurück zu den Nachrichten" link because
// the kit has no desktop back component (BackButton is mobile-only by design)
// and this screen has no breadcrumb. The two are responsive-exclusive — only
// one shows at a time.
function MessagesBackNav() {
  const router = useTenantRouter();
  return (
    <>
      <BackButton referrer="/messages" />
      <button
        type="button"
        onClick={() => router.push("/messages")}
        className="mb-4 hidden items-center gap-1 text-sm text-gray-500 hover:text-gray-900 md:flex"
      >
        <ArrowLeft className="h-4 w-4" /> Zurück zu den Nachrichten
      </button>
    </>
  );
}

export default function MessageThreadPage() {
  return (
    <Suspense fallback={<Loading fullPage={false} />}>
      <MessageThreadContent />
    </Suspense>
  );
}
