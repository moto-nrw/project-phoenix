"use client";

import { useEffect, useState } from "react";
import { MessageCircle } from "lucide-react";
import { Button } from "~/components/ui/button";
import { useTenantRouter } from "~/lib/tenant-router";
import {
  type InboxThread,
  fetchInbox,
  relationshipLabel,
} from "~/lib/parent-messages-api";
import { NewMessageModal } from "~/app/[tenant]/(protected)/messages/new-message-modal";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ParentMessagesCard" });

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

/**
 * Compact overview of the parent-OGS message threads for one child, shown on
 * the child detail page. Lists the child's threads (guardian + subject +
 * unread pill + last activity) — each opens the chat window — and offers a
 * "Neue Nachricht" button that starts a thread pre-selected to this child.
 *
 * There is no per-child list endpoint, so the inbox is fetched and filtered
 * by student_id on the client.
 */
export function ParentMessagesCard({
  studentId,
  studentName,
}: {
  readonly studentId: string;
  readonly studentName?: string;
}) {
  const router = useTenantRouter();
  const [threads, setThreads] = useState<InboxThread[]>([]);
  const [composeOpen, setComposeOpen] = useState(false);

  const loadThreads = () => {
    fetchInbox()
      .then((all) => {
        setThreads(all.filter((t) => t.student_id === studentId));
      })
      .catch((err) => {
        logger.warn("parent_messages_load_failed", {
          error: err instanceof Error ? err.message : String(err),
          student_id: studentId,
        });
      });
  };

  useEffect(() => {
    let active = true;
    fetchInbox()
      .then((all) => {
        if (active) setThreads(all.filter((t) => t.student_id === studentId));
      })
      .catch((err) => {
        logger.warn("parent_messages_load_failed", {
          error: err instanceof Error ? err.message : String(err),
          student_id: studentId,
        });
      });
    return () => {
      active = false;
    };
  }, [studentId]);

  return (
    <div className="moto-content-surface rounded-2xl border p-4 backdrop-blur-sm sm:p-6">
      <div className="mb-4 flex items-center justify-between gap-2">
        <div className="flex min-w-0 items-center gap-2 sm:gap-3">
          <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-[#83CD2D]/10 text-[#83CD2D] sm:h-10 sm:w-10">
            <MessageCircle className="h-5 w-5" aria-hidden="true" />
          </div>
          <h2 className="truncate text-base font-semibold text-gray-900 sm:text-lg">
            Nachrichten mit den Eltern
          </h2>
        </div>
        <Button
          type="button"
          variant="primary"
          size="md"
          onClick={() => setComposeOpen(true)}
          className="flex-shrink-0"
        >
          Neue Nachricht
        </Button>
      </div>

      {threads.length === 0 ? (
        <p className="py-6 text-center text-sm text-gray-500">
          Noch keine Unterhaltungen. Schreiben Sie den Eltern die erste
          Nachricht.
        </p>
      ) : (
        <ul className="space-y-2">
          {threads.map((thread) => {
            const hasUnread = thread.unread_count > 0;
            const navigate = () => router.push(`/messages/${thread.thread_id}`);
            return (
              <li key={thread.thread_id}>
                <button
                  type="button"
                  onClick={navigate}
                  className="flex w-full items-start justify-between gap-3 rounded-lg border border-gray-200 px-3 py-2 text-left hover:bg-gray-50"
                >
                  <span className="min-w-0">
                    <span className="flex items-center gap-2">
                      <span className="truncate text-sm font-medium text-gray-900">
                        {thread.guardian_name}
                      </span>
                      <span className="truncate text-xs text-gray-500">
                        {relationshipLabel(thread.relationship_type)}
                      </span>
                      {hasUnread && (
                        <span className="inline-flex h-5 min-w-5 items-center justify-center rounded-full bg-red-500 px-1.5 text-xs font-bold text-white">
                          {thread.unread_count}
                        </span>
                      )}
                    </span>
                    <span className="mt-0.5 block truncate text-sm text-gray-700">
                      {thread.subject}
                    </span>
                  </span>
                  {thread.last_message_at && (
                    <span className="flex-shrink-0 text-xs whitespace-nowrap text-gray-400">
                      {formatDateTime(thread.last_message_at)}
                    </span>
                  )}
                </button>
              </li>
            );
          })}
        </ul>
      )}

      {composeOpen && (
        <NewMessageModal
          presetStudentId={studentId}
          presetStudentName={studentName}
          onClose={() => setComposeOpen(false)}
          onSent={(thread) => {
            loadThreads();
            router.push(`/messages/${thread.thread_id}`);
          }}
        />
      )}
    </div>
  );
}
