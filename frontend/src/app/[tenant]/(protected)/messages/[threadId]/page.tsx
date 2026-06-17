"use client";

import {
  Suspense,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useParams } from "next/navigation";
import useSWR, { unstable_serialize, useSWRConfig } from "swr";
import { ArrowLeft, User } from "lucide-react";
import { Button } from "~/components/ui/button";
import { Alert } from "~/components/ui/alert";
import { Loading } from "~/components/ui/loading";
import { BackButton } from "~/components/ui/back-button";
import { useTenantRouter } from "~/lib/tenant-router";
import { useSSE } from "~/lib/hooks/use-sse";
import {
  type InboxThread,
  type Message,
  type ThreadDetail,
  fetchThread,
  postMessage,
  relationshipLabel,
} from "~/lib/parent-messages-api";
import { getApiErrorMessage } from "~/components/ui/modal-utils";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "MessageThreadPage" });

function formatDateTime(iso: string): string {
  try {
    return new Intl.DateTimeFormat("de-DE", {
      day: "2-digit",
      month: "2-digit",
      year: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    }).format(new Date(iso));
  } catch {
    return iso;
  }
}

function MessageThreadContent() {
  const params = useParams();
  const threadId = params.threadId as string;
  const router = useTenantRouter();

  const { cache } = useSWRConfig();

  // Seed the header instantly from the inbox SWR cache (subject, guardian,
  // child) so opening a chat shows its structure immediately instead of a
  // full-page skeleton; the messages then fill in.
  const seed = useMemo<ThreadDetail | undefined>(() => {
    for (const unread of [false, true]) {
      const entry = cache.get(
        unstable_serialize(["messages-inbox", unread]),
      ) as { data?: InboxThread[] } | undefined;
      const row = entry?.data?.find((t) => t.thread_id === threadId);
      if (row) {
        return {
          thread_id: row.thread_id,
          subject: row.subject,
          student_id: row.student_id,
          student_name: row.student_name,
          guardian_name: row.guardian_name,
          relationship_type: row.relationship_type,
          messages: [],
        };
      }
    }
    return undefined;
  }, [cache, threadId]);

  const {
    data: thread,
    error: loadError,
    isLoading,
    mutate,
  } = useSWR(["message-thread", threadId], () => fetchThread(threadId), {
    revalidateOnFocus: false,
    fallbackData: seed,
    onError: (err: unknown) =>
      logger.error("thread_load_failed", {
        error: err instanceof Error ? err.message : String(err),
        thread_id: threadId,
      }),
  });

  const messages: Message[] = thread?.messages ?? [];
  const messagesLoading = isLoading && messages.length === 0;

  const [draft, setDraft] = useState("");
  const [isSending, setIsSending] = useState(false);
  const [sendError, setSendError] = useState<string | null>(null);
  const scrollRef = useRef<HTMLDivElement | null>(null);

  // SSE: parent sent a message or staff replied → revalidate the thread.
  const handleSSEEvent = useCallback(
    (event: { type: string }) => {
      if (event.type === "parent_message") {
        void mutate();
      }
    },
    [mutate],
  );

  useSSE("/api/sse/events", { onMessage: handleSSEEvent });

  // Loading the thread marks it read server-side, so nudge the sidebar unread
  // badge to refetch. Guarding on !isLoading skips the instant inbox seed
  // (which doesn't touch the read cursor) and fires only after the real GET.
  useEffect(() => {
    if (thread && !isLoading && typeof window !== "undefined") {
      window.dispatchEvent(new CustomEvent("messages-unread-refresh"));
    }
  }, [thread, isLoading]);

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
      await mutate((prev) => (prev ? { ...prev, messages: updated } : prev), {
        revalidate: false,
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

  // Full-page loader only for a direct deep-link with no cached seed.
  if (!thread && isLoading) {
    return <Loading fullPage={false} />;
  }

  if (!thread) {
    return (
      <div className="-mt-1.5 w-full">
        <BackButton referrer="/messages" />
        <button
          type="button"
          onClick={() => router.push("/messages")}
          className="mb-4 hidden items-center gap-1 text-sm text-gray-500 hover:text-gray-900 md:flex"
        >
          <ArrowLeft className="h-4 w-4" /> Zurück zu den Nachrichten
        </button>
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
    <div className="-mt-1.5 w-full">
      <BackButton referrer="/messages" />
      <button
        type="button"
        onClick={() => router.push("/messages")}
        className="mb-4 hidden items-center gap-1 text-sm text-gray-500 hover:text-gray-900 md:flex"
      >
        <ArrowLeft className="h-4 w-4" /> Zurück zu den Nachrichten
      </button>

      <div className="moto-content-surface rounded-2xl border p-4 backdrop-blur-sm sm:p-6">
        <div className="mb-4 flex items-start justify-between gap-3">
          <div className="min-w-0">
            <h1 className="truncate text-lg font-semibold text-gray-900 sm:text-xl">
              {thread.subject}
            </h1>
            <p className="mt-0.5 truncate text-sm text-gray-500">
              {thread.guardian_name} ·{" "}
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
          className="max-h-[60vh] min-h-[8rem] space-y-3 overflow-y-auto pr-1"
        >
          {messagesLoading ? (
            <p className="py-6 text-center text-sm text-gray-400">
              Verlauf wird geladen…
            </p>
          ) : messages.length === 0 ? (
            <p className="py-6 text-center text-sm text-gray-500">
              Noch keine Nachrichten in dieser Unterhaltung.
            </p>
          ) : (
            messages.map((message) => {
              const isStaff = message.sender_kind === "staff";
              return (
                <div
                  key={message.id}
                  className={`flex flex-col ${
                    isStaff ? "items-end" : "items-start"
                  }`}
                >
                  <div
                    className={`max-w-[85%] rounded-2xl px-3 py-2 text-sm whitespace-pre-wrap ${
                      isStaff
                        ? "bg-[#83CD2D] text-white"
                        : "bg-gray-100 text-gray-900"
                    }`}
                  >
                    {message.body}
                  </div>
                  <p className="mt-1 px-1 text-xs text-gray-500">
                    {message.sender_name} • {formatDateTime(message.created_at)}
                  </p>
                </div>
              );
            })
          )}
        </div>

        {sendError && (
          <div className="mt-3">
            <Alert type="error" message={sendError} />
          </div>
        )}

        <div className="mt-4 flex flex-col gap-2 sm:flex-row sm:items-end">
          <textarea
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            placeholder="Nachricht an die Eltern schreiben..."
            rows={2}
            disabled={isSending}
            className="moto-content-surface w-full resize-none rounded-lg border border-gray-300 px-4 py-3 text-sm text-gray-900 focus:border-[#83CD2D] focus:ring-1 focus:ring-[#83CD2D] focus:outline-none disabled:opacity-60"
          />
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={() => void handleSend()}
            isLoading={isSending}
            loadingText="Senden..."
            disabled={isSending || draft.trim().length === 0}
            className="sm:flex-shrink-0"
          >
            Senden
          </Button>
        </div>
      </div>
    </div>
  );
}

export default function MessageThreadPage() {
  return (
    <Suspense fallback={<Loading fullPage={false} />}>
      <MessageThreadContent />
    </Suspense>
  );
}
