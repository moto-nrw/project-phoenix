"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { ArrowRight, MessageSquare, Plus } from "lucide-react";
import { Button } from "~/components/ui/button";
import { NewThreadModal } from "~/components/parent/new-thread-modal";
import { type ThreadSummary, listMessageThreads } from "~/lib/parent-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ParentMessagesPage" });

function formatTimestamp(iso: string | undefined): string {
  if (!iso) return "";
  return new Intl.DateTimeFormat("de-DE", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(iso));
}

function previewLine(thread: ThreadSummary): string {
  if (!thread.last_message_body) return "";
  const prefix =
    thread.last_sender_kind === "staff"
      ? "OGS: "
      : thread.last_sender_kind === "guardian"
        ? "Sie: "
        : "";
  return `${prefix}${thread.last_message_body}`;
}

export default function ParentMessagesPage() {
  const [threads, setThreads] = useState<ThreadSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [newThreadOpen, setNewThreadOpen] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setThreads(await listMessageThreads());
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unknown error";
      logger.warn("parent_messages_load_failed", { error: message });
      setError(message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  if (loading) {
    return <ParentMessagesSkeleton />;
  }

  if (error) {
    return (
      <div className="mx-auto max-w-7xl">
        <div className="rounded-2xl border border-[#FF3130]/20 bg-[#FF3130]/10 p-5 text-sm text-[#CC2626] shadow-sm">
          Die Nachrichten konnten nicht geladen werden.
        </div>
      </div>
    );
  }

  return (
    <div className="mx-auto w-full max-w-7xl space-y-6">
      <section className="overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm">
        <div className="flex flex-col gap-4 p-5 sm:flex-row sm:items-end sm:justify-between sm:p-6 lg:p-8">
          <div className="min-w-0">
            <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
              Austausch mit der OGS
            </p>
            <h1 className="mt-1 text-3xl font-semibold text-gray-900 sm:text-4xl">
              Nachrichten
            </h1>
            <p className="mt-3 max-w-3xl text-sm leading-6 text-gray-600 sm:text-base">
              Schreiben Sie der OGS Ihres Kindes und lesen Sie die Antworten des
              Teams.
            </p>
          </div>
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={() => setNewThreadOpen(true)}
            className="shrink-0 self-start sm:self-auto"
          >
            <Plus className="h-4 w-4" aria-hidden="true" />
            Neue Nachricht
          </Button>
        </div>
      </section>

      <section className="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm sm:p-6">
        {threads.length === 0 ? (
          <EmptyMessages onNew={() => setNewThreadOpen(true)} />
        ) : (
          <ul className="divide-y divide-gray-100">
            {threads.map((thread) => (
              <ThreadRow key={thread.thread_id} thread={thread} />
            ))}
          </ul>
        )}
      </section>

      <NewThreadModal
        isOpen={newThreadOpen}
        onClose={() => setNewThreadOpen(false)}
      />
    </div>
  );
}

function ThreadRow({ thread }: Readonly<{ thread: ThreadSummary }>) {
  const timestamp = formatTimestamp(thread.last_message_at);
  const preview = previewLine(thread);
  const unread = thread.unread > 0;
  return (
    <li>
      <Link
        href={`/parents/messages/${thread.thread_id}`}
        className="group flex items-start gap-4 px-1 py-4 transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none sm:rounded-xl sm:px-3"
      >
        <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-[#83CD2D]/15 text-[#4A7A15]">
          <MessageSquare className="h-5 w-5" aria-hidden="true" />
        </span>
        <div className="min-w-0 flex-1">
          <div className="flex min-w-0 items-start justify-between gap-2">
            <h2
              className={`min-w-0 truncate text-base text-gray-900 ${
                unread ? "font-bold" : "font-semibold"
              }`}
            >
              {thread.subject}
            </h2>
            <div className="flex shrink-0 items-center gap-2">
              {unread ? (
                <span
                  className="inline-flex h-6 min-w-6 items-center justify-center rounded-full bg-red-500 px-2 text-xs font-semibold text-white"
                  aria-label={`${thread.unread} ungelesene Nachrichten`}
                >
                  {thread.unread}
                </span>
              ) : null}
              {timestamp ? (
                <span className="text-xs text-gray-400">{timestamp}</span>
              ) : null}
            </div>
          </div>
          <p className="truncate text-sm text-gray-600">
            {thread.student_name}
          </p>
          {preview ? (
            <p
              className={`mt-1 truncate text-sm ${
                unread ? "text-gray-700" : "text-gray-500"
              }`}
            >
              {preview}
            </p>
          ) : null}
        </div>
        <ArrowRight
          className="mt-1 hidden h-4 w-4 shrink-0 text-gray-400 transition-colors group-hover:text-gray-700 sm:block"
          aria-hidden="true"
        />
      </Link>
    </li>
  );
}

function EmptyMessages({ onNew }: Readonly<{ onNew: () => void }>) {
  return (
    <div className="rounded-xl border border-dashed border-gray-300 bg-gray-50 p-8 text-center">
      <span className="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-white text-gray-500 shadow-sm">
        <MessageSquare className="h-5 w-5" aria-hidden="true" />
      </span>
      <h2 className="mt-3 text-sm font-semibold text-gray-900">
        Sie haben noch keine Nachrichten
      </h2>
      <p className="mt-1 text-sm leading-6 text-gray-600">
        Beginnen Sie eine neue Unterhaltung.
      </p>
      <div className="mt-4 flex justify-center">
        <Button type="button" variant="primary" size="md" onClick={onNew}>
          <Plus className="h-4 w-4" aria-hidden="true" />
          Neue Nachricht
        </Button>
      </div>
    </div>
  );
}

function ParentMessagesSkeleton() {
  return (
    <div className="mx-auto w-full max-w-7xl space-y-6">
      <div className="h-48 animate-pulse rounded-2xl border border-gray-200 bg-white shadow-sm" />
      <div className="space-y-3 rounded-2xl border border-gray-200 bg-white p-5 shadow-sm sm:p-6">
        {[0, 1, 2, 3].map((item) => (
          <div
            key={item}
            className="h-16 animate-pulse rounded-xl bg-gray-100"
          />
        ))}
      </div>
    </div>
  );
}
