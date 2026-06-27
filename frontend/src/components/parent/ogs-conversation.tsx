"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { Alert } from "~/components/ui/alert";
import { MessageComposer } from "~/components/messaging/message-composer";
import { ChatBubble } from "~/components/messaging/chat-bubble";
import { useChatViewportLock } from "~/lib/hooks/use-chat-viewport-lock";
import { getApiErrorMessage } from "~/components/ui/modal-utils";
import {
  type ThreadView,
  getChildConversation,
  postChildMessage,
} from "~/lib/parent-api";
import { useChildCare } from "~/components/parent/child-care";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "OgsConversation" });

/**
 * The parent <-> OGS chat for one child. The parent is talking to "the OGS
 * [Schulname]" — the child is only context, never the counterpart. Used both
 * inline on the messages landing page (single-child parents go straight here)
 * and on the per-child route (multi-child parents, with a back link + the
 * child shown for disambiguation).
 */
export function OgsConversation({
  studentId,
  showBack = false,
  showChild = false,
}: {
  readonly studentId: string;
  readonly showBack?: boolean;
  readonly showChild?: boolean;
}) {
  const [thread, setThread] = useState<ThreadView | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [draft, setDraft] = useState("");
  const care = useChildCare(studentId);
  const [sending, setSending] = useState(false);
  const [sendError, setSendError] = useState<string | null>(null);
  const scrollRef = useRef<HTMLDivElement | null>(null);
  // Pin the chat to the viewport and lock page scroll (only the list scrolls).
  const containerRef = useChatViewportLock<HTMLDivElement>(!loading);

  // Latest-wins guard shared by EVERY setThread path (refresh, send). SSE fires
  // one parent-conversation-refresh per message and those refetches can overlap a
  // just-sent write, so without a token an older in-flight snapshot resolving
  // late would clobber a fresher one.
  //
  // READS (refresh) claim their token at START — latest-started read wins. The
  // SEND claims it right BEFORE applying its result, AFTER the await: the value
  // it returns is the authoritative post-commit thread (it includes the
  // just-sent message), so it must always win. Claiming the send's token at start
  // let a parent-conversation-refresh that began during the send window take a
  // higher token and apply a PRE-commit snapshot, dropping the send's result and
  // making the sent message vanish until the next event. mountedRef additionally
  // blocks setState after unmount.
  const applySeqRef = useRef(0);
  const mountedRef = useRef(true);
  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);
  const applyThread = useCallback((seq: number, view: ThreadView): boolean => {
    if (!mountedRef.current || seq !== applySeqRef.current) return false;
    setThread(view);
    return true;
  }, []);

  // Reading the conversation marks it read server-side, so no separate
  // mark-read call is needed.
  const refresh = useCallback(async () => {
    const seq = ++applySeqRef.current;
    try {
      const view = await getChildConversation(studentId);
      if (applyThread(seq, view)) setLoadError(null);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unknown error";
      logger.warn("ogs_conversation_load_failed", { error: message });
      if (mountedRef.current && seq === applySeqRef.current) {
        setLoadError(message);
      }
    }
  }, [studentId, applyThread]);

  useEffect(() => {
    let active = true;
    void (async () => {
      setLoading(true);
      await refresh();
      if (active) setLoading(false);
    })();
    return () => {
      active = false;
    };
  }, [studentId, refresh]);

  // Real-time: the portal-wide ParentRealtimeBridge owns the SINGLE SSE
  // connection to /api/parent/sse/events and dispatches `parent-conversation-
  // refresh` (carrying the affected studentId) per parent_message. Listen to
  // that window event instead of opening a SECOND EventSource to the same
  // endpoint — the duplicate doubled SSE connections + backend goroutines and
  // fired every event twice. A multi-child guardian gets one event per child,
  // so skip the refetch when it names a DIFFERENT child.
  useEffect(() => {
    const onRefresh = (event: Event) => {
      const detail = (event as CustomEvent<{ studentId?: string | null }>)
        .detail;
      if (detail?.studentId && detail.studentId !== studentId) return;
      void refresh();
    };
    window.addEventListener("parent-conversation-refresh", onRefresh);
    return () =>
      window.removeEventListener("parent-conversation-refresh", onRefresh);
  }, [refresh, studentId]);

  const messages = thread?.messages ?? [];
  const counterpart = thread?.counterpart_name ?? "OGS";

  // Keep the conversation scrolled to the newest message.
  useEffect(() => {
    const el = scrollRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [thread]);

  // Opening/refreshing the conversation marks it read server-side, so nudge the
  // sidebar unread badge to refetch.
  useEffect(() => {
    if (thread && typeof window !== "undefined") {
      window.dispatchEvent(new CustomEvent("parent-messages-unread-refresh"));
    }
  }, [thread]);

  const handleSend = useCallback(async () => {
    const body = draft.trim();
    if (!body || sending) return;
    setSending(true);
    setSendError(null);
    try {
      const view = await postChildMessage(studentId, body);
      // Claim the token AFTER the POST resolves so this authoritative result wins.
      applyThread(++applySeqRef.current, view);
      setDraft("");
    } catch (err) {
      logger.warn("ogs_message_send_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setSendError(
        getApiErrorMessage(
          err,
          "senden",
          "Nachrichten",
          "Die Nachricht konnte nicht gesendet werden.",
        ),
      );
    } finally {
      setSending(false);
    }
  }, [draft, sending, studentId, applyThread]);

  return (
    <div
      ref={containerRef}
      className="mx-auto flex min-h-[20rem] w-full max-w-5xl flex-col gap-3 overflow-hidden"
    >
      {showBack ? <BackBar /> : null}

      <section className="flex min-h-0 flex-1 flex-col rounded-2xl border border-gray-200 bg-white shadow-sm">
        <div className="border-b border-gray-100 px-5 py-4 sm:px-6">
          <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
            Austausch mit der OGS
          </p>
          <h1 className="mt-0.5 text-lg font-semibold break-words text-gray-900">
            {counterpart}
          </h1>
          {showChild && thread?.student_name ? (
            <p className="text-sm text-gray-500">zu {thread.student_name}</p>
          ) : null}
        </div>

        <div
          ref={scrollRef}
          className="flex-1 space-y-3 overflow-y-auto px-5 py-4 sm:px-6"
        >
          {loading ? (
            <ThreadSkeleton />
          ) : loadError ? (
            <Alert
              type="error"
              message="Die Nachrichten konnten nicht geladen werden."
            />
          ) : messages.length === 0 ? (
            <EmptyThread />
          ) : (
            messages.map((message) => (
              <ChatBubble
                key={message.id}
                body={message.body}
                own={message.sender_kind === "guardian"}
                senderName={message.sender_name}
                createdAt={message.created_at}
                readReceiptLabel={
                  message.sender_kind === "guardian" && message.read_by_staff
                    ? "OGS hat gelesen"
                    : undefined
                }
              />
            ))
          )}
        </div>

        <div className="border-t border-gray-100 px-5 py-4 sm:px-6">
          {sendError ? (
            <div className="mb-3">
              <Alert type="error" message={sendError} />
            </div>
          ) : null}
          {/* Only show the free-text composer when the school has parent notes
              enabled AND this relationship grants notes.write (both folded into
              care.features.notes_enabled). A pickup-only/emergency guardian, or
              a school with messaging off, gets a read-only history instead of a
              composer that always 403s on send. */}
          {care.features.notes_enabled ? (
            <MessageComposer
              value={draft}
              onChange={setDraft}
              onSend={() => void handleSend()}
              sending={sending}
              placeholder="Nachricht an die OGS schreiben…"
            />
          ) : (
            <p className="rounded-lg bg-gray-50 px-4 py-3 text-sm text-gray-500">
              Das Schreiben von Nachrichten ist für dieses Kind nicht aktiviert.
              Sie können den Verlauf weiterhin lesen.
            </p>
          )}
        </div>
      </section>
    </div>
  );
}

function EmptyThread() {
  return (
    <div className="flex h-full flex-col items-center justify-center py-10 text-center">
      <h2 className="text-sm font-semibold text-gray-900">
        Noch keine Nachrichten
      </h2>
      <p className="mt-1 text-sm leading-6 text-gray-600">
        Schreiben Sie die erste Nachricht an die OGS.
      </p>
    </div>
  );
}

function ThreadSkeleton() {
  return (
    <div className="space-y-3">
      <div className="h-12 w-2/3 animate-pulse rounded-2xl bg-gray-100" />
      <div className="ml-auto h-12 w-1/2 animate-pulse rounded-2xl bg-gray-100" />
      <div className="h-12 w-3/5 animate-pulse rounded-2xl bg-gray-100" />
    </div>
  );
}

function BackBar() {
  return (
    <Link
      href="/parents/messages"
      className="inline-flex h-8 w-fit items-center gap-2 rounded-lg px-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-50 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
    >
      <ArrowLeft className="h-4 w-4" aria-hidden="true" />
      Zurück
    </Link>
  );
}
