"use client";

import { use, useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { Button } from "~/components/ui/button";
import { Alert } from "~/components/ui/alert";
import { getApiErrorMessage } from "~/components/ui/modal-utils";
import { useSSE } from "~/lib/hooks/use-sse";
import type { SSEEvent } from "~/lib/sse-types";
import {
  type ParentMessage,
  type ThreadView,
  getMessageThread,
  markThreadRead,
  postThreadMessage,
} from "~/lib/parent-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ParentMessageThreadPage" });

function formatTime(iso: string): string {
  return new Intl.DateTimeFormat("de-DE", {
    day: "2-digit",
    month: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(iso));
}

export default function ParentMessageThreadPage({
  params,
}: {
  readonly params: Promise<{ threadId: string }>;
}) {
  const { threadId } = use(params);
  return <MessageThread threadId={threadId} />;
}

function MessageThread({ threadId }: Readonly<{ threadId: string }>) {
  const [thread, setThread] = useState<ThreadView | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);
  const [sendError, setSendError] = useState<string | null>(null);
  const scrollRef = useRef<HTMLDivElement | null>(null);

  const refresh = useCallback(async () => {
    try {
      const view = await getMessageThread(threadId);
      setThread(view);
      setLoadError(null);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unknown error";
      logger.warn("parent_message_thread_load_failed", { error: message });
      setLoadError(message);
    }
  }, [threadId]);

  // Initial load: fetch the thread and clear the unread badge.
  useEffect(() => {
    let active = true;
    void (async () => {
      setLoading(true);
      await refresh();
      if (active) setLoading(false);
      try {
        await markThreadRead(threadId);
      } catch (err) {
        logger.warn("parent_message_mark_read_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      }
    })();
    return () => {
      active = false;
    };
  }, [threadId, refresh]);

  // Real-time: refetch the thread when the backend signals a new message.
  // The parent-scoped SSE channel (/api/parent/sse/events) delivers the
  // parent_message trigger for the tenants of the guardian's children.
  const handleSSE = useCallback(
    (event: SSEEvent) => {
      if (event.type === "parent_message") {
        void refresh();
      }
    },
    [refresh],
  );
  useSSE("/api/parent/sse/events", { onMessage: handleSSE });

  const messages = thread?.messages ?? [];

  // Keep the conversation scrolled to the newest message.
  useEffect(() => {
    const el = scrollRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [thread]);

  const handleSend = useCallback(async () => {
    const body = draft.trim();
    if (!body || sending) return;
    setSending(true);
    setSendError(null);
    try {
      const view = await postThreadMessage(threadId, body);
      setThread(view);
      setDraft("");
    } catch (err) {
      logger.warn("parent_message_send_failed", {
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
  }, [draft, sending, threadId]);

  const composerDisabled = sending || draft.trim().length === 0;

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-4">
      <BackBar />

      <section className="flex min-h-[24rem] flex-col rounded-2xl border border-gray-200 bg-white shadow-sm">
        <div className="border-b border-gray-100 px-5 py-4 sm:px-6">
          <h1 className="text-lg font-semibold break-words text-gray-900">
            {thread?.subject ?? "Nachrichten"}
          </h1>
          {thread?.student_name ? (
            <p className="text-sm text-gray-500">{thread.student_name}</p>
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
              <MessageBubble key={message.id} message={message} />
            ))
          )}
        </div>

        <div className="border-t border-gray-100 px-5 py-4 sm:px-6">
          {sendError ? (
            <div className="mb-3">
              <Alert type="error" message={sendError} />
            </div>
          ) : null}
          <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
            <label htmlFor="message-body" className="sr-only">
              Nachricht
            </label>
            <textarea
              id="message-body"
              name="message-body"
              rows={2}
              value={draft}
              onChange={(event) => setDraft(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
                  event.preventDefault();
                  void handleSend();
                }
              }}
              disabled={sending}
              placeholder="Nachricht an die OGS schreiben…"
              className="block w-full resize-none rounded-lg border-0 bg-white px-4 py-3 text-base text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset placeholder:text-gray-400 focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400 disabled:bg-gray-50 disabled:text-gray-500 disabled:ring-gray-200"
            />
            <Button
              type="button"
              variant="primary"
              size="md"
              onClick={() => void handleSend()}
              disabled={composerDisabled}
              className="shrink-0"
            >
              {sending ? "Senden…" : "Senden"}
            </Button>
          </div>
        </div>
      </section>
    </div>
  );
}

function MessageBubble({ message }: Readonly<{ message: ParentMessage }>) {
  const isOwn = message.sender_kind === "guardian";
  const alignment = isOwn ? "items-end" : "items-start";
  const bubble = isOwn
    ? "bg-[#83CD2D] text-white"
    : "bg-gray-100 text-gray-900";
  return (
    <div className={`flex flex-col ${alignment}`}>
      <div
        className={`max-w-[85%] rounded-2xl px-4 py-2 text-sm leading-5 break-words whitespace-pre-wrap ${bubble}`}
      >
        {message.body}
      </div>
      <p className="mt-1 px-1 text-xs text-gray-400">
        {message.sender_name} · {formatTime(message.created_at)}
      </p>
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
