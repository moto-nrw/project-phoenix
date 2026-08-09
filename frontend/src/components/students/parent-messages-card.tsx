"use client";

import { useCallback, useState } from "react";
import useSWR from "swr";
import { Button } from "~/components/ui/button";
import { ConceptSectionHeader } from "~/components/ui/concept-section-header";
import { UnreadBadge } from "~/components/messaging/unread-badge";
import { useTenant, useTenantSlugSafe } from "~/lib/tenant-context";
import { useTenantRouter } from "~/lib/tenant-router";
import {
  fetchStudentThreads,
  relationshipLabel,
} from "~/lib/parent-messages-api";
import { NewMessageModal } from "~/components/messaging/new-message-modal";
import { useMessagesActivity } from "~/lib/hooks/use-messages-activity";
import { createLogger } from "~/lib/logger";
import { formatChatDateTime } from "~/lib/date-helpers";

const logger = createLogger({ component: "ParentMessagesCard" });

/**
 * Compact overview of the parent-OGS message threads for one child, shown on
 * the child detail page. Lists the child's threads (guardian + subject +
 * unread pill + last activity) — each opens the chat window — and offers a
 * "Neue Nachricht" button that starts a thread pre-selected to this child.
 *
 * Threads are scoped server-side to this child.
 */
export function ParentMessagesCard({
  studentId,
  studentName,
}: {
  readonly studentId: string;
  readonly studentName?: string;
}) {
  const router = useTenantRouter();
  const { tenant } = useTenant();
  // Tenant-prefix the SWR key so a cross-tenant mutate() in a multi-tab / tenant-
  // switch scenario cannot touch another tenant's cached threads (frontend
  // convention: useTenantSlugSafe is for SWR cache-key prefixing).
  const tenantSlug = useTenantSlugSafe();
  // Hide compose when the school has messaging off (would 403 on the backend);
  // existing threads stay readable.
  const messagingEnabled = tenant?.messagingEnabled === true;
  const [composeOpen, setComposeOpen] = useState(false);

  // SWR + SSE so the card's unread pill and last-activity stay live (matching
  // the inbox page) instead of going stale until the page is remounted.
  const { data: threads = [], mutate } = useSWR(
    [`${tenantSlug ?? ""}:student-threads`, studentId],
    () => fetchStudentThreads(studentId),
    {
      revalidateOnFocus: false,
      onError: (err: unknown) =>
        logger.warn("parent_messages_load_failed", {
          error: err instanceof Error ? err.message : String(err),
          student_id: studentId,
        }),
    },
  );

  // The event fans out tenant-wide; revalidate when it names THIS child or
  // carries no student id (the staff broadcast strips student_id — see
  // realtime/events.go staffSafeParentMessage — so a null id must refetch).
  // Debounced like the inbox: during the morning rush every staffer with a
  // child-detail page open would otherwise re-run the full student-threads
  // projection once per parent message anywhere in the tenant.
  // Refetch-only (re-runs the student-threads projection, never advances a read
  // cursor), so fire even in a background tab — marksRead: false skips the
  // hidden-tab deferral that exists only for read-advancing chat views.
  const refreshMessages = useCallback(() => void mutate(), [mutate]);
  useMessagesActivity({
    onMatch: refreshMessages,
    studentId,
    debounceMs: 500,
    marksRead: false,
  });

  return (
    <div className="moto-content-surface rounded-2xl border p-4 backdrop-blur-sm sm:p-6">
      <ConceptSectionHeader
        className="mb-4"
        title="Nachrichten mit den Eltern"
        concept="parentConversations"
        actions={
          messagingEnabled ? (
            <Button
              type="button"
              variant="primary"
              size="md"
              onClick={() => setComposeOpen(true)}
              className="flex-shrink-0"
            >
              Neue Nachricht
            </Button>
          ) : undefined
        }
      />

      {threads.length === 0 ? (
        <p className="py-6 text-center text-sm text-gray-500">
          {messagingEnabled
            ? "Noch keine Unterhaltungen. Schreiben Sie den Eltern die erste Nachricht."
            : "Der Nachrichtenaustausch mit den Eltern ist für diese Schule deaktiviert."}
        </p>
      ) : (
        <ul className="space-y-2">
          {threads.map((thread) => {
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
                      <UnreadBadge count={thread.unread_count} />
                    </span>
                    {thread.last_message_body && (
                      <span className="mt-0.5 block truncate text-sm text-gray-700">
                        {thread.last_sender_kind === "staff" && (
                          <span className="text-gray-500">Sie: </span>
                        )}
                        {thread.last_message_body}
                      </span>
                    )}
                  </span>
                  {thread.last_message_at && (
                    <span className="flex-shrink-0 text-xs whitespace-nowrap text-gray-400">
                      {formatChatDateTime(thread.last_message_at)}
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
          onOpened={(thread) => {
            // Navigate straight to the new thread. We don't refetch the list
            // here: this card unmounts on navigation, so a post-navigation
            // setThreads would touch an unmounted component, and the list
            // refetches via the mount effect when the user returns.
            router.push(`/messages/${thread.thread_id}`);
          }}
        />
      )}
    </div>
  );
}
