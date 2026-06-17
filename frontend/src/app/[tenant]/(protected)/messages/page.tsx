"use client";

import { Suspense, useCallback, useMemo, useState } from "react";
import useSWR from "swr";
import { MessageCircle } from "lucide-react";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import { Button } from "~/components/ui/button";
import { Loading } from "~/components/ui/loading";
import { useTenantRouter } from "~/lib/tenant-router";
import { useSSE } from "~/lib/hooks/use-sse";
import {
  type InboxThread,
  fetchInbox,
  relationshipLabel,
} from "~/lib/parent-messages-api";
import { NewMessageModal } from "./new-message-modal";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "MessagesInboxPage" });

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

function MessagesInboxContent() {
  const router = useTenantRouter();

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
  } = useSWR(["messages-inbox", onlyUnread], () => fetchInbox(onlyUnread), {
    revalidateOnFocus: false,
    keepPreviousData: true,
    onError: (err: unknown) =>
      logger.error("inbox_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      }),
  });

  // SSE: parent sent a message or staff replied → revalidate the inbox.
  const handleSSEEvent = useCallback(
    (event: { type: string }) => {
      if (event.type === "parent_message") {
        void mutate();
      }
    },
    [mutate],
  );

  useSSE("/api/sse/events", { onMessage: handleSSEEvent });

  const filteredThreads = useMemo(() => {
    const list: InboxThread[] = threads ?? [];
    if (!searchTerm) return list;
    const term = searchTerm.toLowerCase();
    return list.filter(
      (thread) =>
        thread.guardian_name.toLowerCase().includes(term) ||
        thread.student_name.toLowerCase().includes(term) ||
        thread.subject.toLowerCase().includes(term),
    );
  }, [threads, searchTerm]);

  // Skeleton only on the very first load (no cached data yet).
  if (isLoading && !threads) {
    return <Loading fullPage={false} />;
  }

  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch
        title="Nachrichten"
        badge={{
          icon: <MessageCircle className="h-5 w-5 text-gray-600" />,
          count: filteredThreads.length,
        }}
        search={{
          value: searchTerm,
          onChange: setSearchTerm,
          placeholder: "Person, Kind oder Betreff suchen...",
        }}
      />

      <div className="mb-4 flex items-center justify-end gap-2">
        <Button
          type="button"
          variant={onlyUnread ? "primary" : "outline"}
          size="md"
          onClick={() => setOnlyUnread((prev) => !prev)}
        >
          Nur ungelesen
        </Button>
        <Button
          type="button"
          variant="primary"
          size="md"
          onClick={() => setComposeOpen(true)}
        >
          Neue Nachricht
        </Button>
      </div>

      {error && (
        <div className="mb-4 rounded-lg border border-red-200 bg-red-50 p-4 text-red-800">
          Nachrichten konnten nicht geladen werden.
        </div>
      )}

      {filteredThreads.length === 0 ? (
        <div className="py-12 text-center">
          <div className="flex flex-col items-center gap-4">
            <MessageCircle className="h-12 w-12 text-gray-400" />
            <div>
              <h3 className="text-lg font-medium text-gray-900">
                Noch keine Nachrichten
              </h3>
              <p className="text-gray-600">
                Hier erscheinen Unterhaltungen mit den Eltern. Über `Neue
                Nachricht` können Sie selbst eine beginnen.
              </p>
            </div>
            <Button
              type="button"
              variant="primary"
              size="md"
              onClick={() => setComposeOpen(true)}
            >
              Neue Nachricht
            </Button>
          </div>
        </div>
      ) : (
        <ul className="space-y-3">
          {filteredThreads.map((thread) => {
            const hasUnread = thread.unread_count > 0;
            const needsReply =
              hasUnread && thread.last_sender_kind === "guardian";
            const navigate = () => router.push(`/messages/${thread.thread_id}`);

            return (
              <li key={thread.thread_id}>
                <div
                  role="button"
                  tabIndex={0}
                  onClick={navigate}
                  onKeyDown={(e: React.KeyboardEvent) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      navigate();
                    }
                  }}
                  className="moto-content-surface moto-hover-elevated w-full cursor-pointer rounded-2xl border border-gray-200 bg-white p-4 text-left shadow-sm focus-visible:ring-2 focus-visible:ring-gray-300 focus-visible:ring-offset-2 focus-visible:outline-none sm:p-5"
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
                        {hasUnread && (
                          <span className="inline-flex h-5 min-w-5 items-center justify-center rounded-full bg-red-500 px-1.5 text-xs font-bold text-white">
                            {thread.unread_count}
                          </span>
                        )}
                        {needsReply && (
                          <span className="inline-flex items-center rounded-full bg-[#F78C10]/10 px-2 py-0.5 text-xs font-medium text-[#F78C10]">
                            Antwort nötig
                          </span>
                        )}
                      </div>
                      <p className="mt-1 truncate text-sm font-medium text-gray-800">
                        {thread.subject}
                      </p>
                      {thread.last_message_body && (
                        <p className="mt-0.5 truncate text-sm text-gray-600">
                          {thread.last_sender_kind === "staff" && (
                            <span className="text-gray-500">Sie: </span>
                          )}
                          {thread.last_message_body}
                        </p>
                      )}
                    </div>
                    {thread.last_message_at && (
                      <span className="flex-shrink-0 text-xs whitespace-nowrap text-gray-400">
                        {formatDateTime(thread.last_message_at)}
                      </span>
                    )}
                  </div>
                </div>
              </li>
            );
          })}
        </ul>
      )}

      {composeOpen && (
        <NewMessageModal
          onClose={() => setComposeOpen(false)}
          onSent={(thread) => router.push(`/messages/${thread.thread_id}`)}
        />
      )}
    </div>
  );
}

export default function MessagesPage() {
  return (
    <Suspense fallback={<Loading fullPage={false} />}>
      <MessagesInboxContent />
    </Suspense>
  );
}
