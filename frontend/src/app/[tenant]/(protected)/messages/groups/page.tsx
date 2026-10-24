"use client";

import { useCallback, useState } from "react";
import useSWR from "swr";
import { Button } from "~/components/ui/button";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { formErrorMessage } from "~/components/ui/form-error";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage } from "~/components/ui/tenant-page";
import { TileCard } from "~/components/ui/tile-card";
import { messagesTabs } from "~/components/messaging/messages-tabs";
import { NewGroupMessageModal } from "~/components/messaging/new-group-message-modal";
import { formatChatDateTime } from "~/lib/date-helpers";
import { useMessagesActivity } from "~/lib/hooks/use-messages-activity";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { createLogger } from "~/lib/logger";
import {
  type GroupThreadSummary,
  familyReadLabel,
  fetchGroupThreads,
} from "~/lib/parent-group-threads-api";
import { useTenant, useTenantSlugSafe } from "~/lib/tenant-context";
import { useTenantRouter } from "~/lib/tenant-router";

const logger = createLogger({ component: "GroupMessagesPage" });

/**
 * Gruppennachrichten (#4784): what the OGS wrote to every family of a group,
 * class or AG, with how many families have read it.
 */
function GroupMessagesContent() {
  const router = useTenantRouter();
  const { tenant } = useTenant();
  const tenantSlug = useTenantSlugSafe();
  const messagingEnabled = tenant?.messagingEnabled === true;
  const [composeOpen, setComposeOpen] = useState(false);

  const { data, error, isLoading, mutate } = useSWR<GroupThreadSummary[]>(
    `${tenantSlug ?? ""}:group-messages`,
    fetchGroupThreads,
    {
      revalidateOnFocus: false,
      keepPreviousData: true,
      onError: (err: unknown) =>
        logger.error("group_messages_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        }),
    },
  );
  // Group-thread changes use the shared staff messaging event. Revalidation
  // only reloads the list; it does not advance a read cursor, even in a
  // background tab.
  const refreshGroupThreads = useCallback(() => void mutate(), [mutate]);
  useMessagesActivity({ onMatch: refreshGroupThreads, marksRead: false });
  const loadError = useSwrLoadError(
    error,
    "die Liste der Gruppennachrichten",
    mutate,
  );

  const threads = data ?? [];
  const showSkeleton =
    (isLoading && !data) ||
    (Boolean(error) && formErrorMessage(loadError) === null);
  const openCount = threads.filter((thread) => !thread.closed_at).length;
  const summary =
    data === undefined
      ? null
      : `${threads.length} ${threads.length === 1 ? "Gruppennachricht" : "Gruppennachrichten"} · ${openCount} offen`;

  const composeButton = messagingEnabled ? (
    <Button
      type="button"
      variant="primary"
      size="md"
      onClick={() => setComposeOpen(true)}
    >
      Neue Gruppennachricht
    </Button>
  ) : undefined;

  const loadFailed = Boolean(error);
  const bodyReplaced = !showSkeleton && threads.length === 0 && !composeOpen;

  return (
    <TenantPage
      title="Nachrichten"
      tabs={messagesTabs("groups")}
      stats={summary}
      statsLoading={showSkeleton}
      actions={composeButton}
      loading={showSkeleton}
      error={bodyReplaced && loadFailed ? loadError : null}
      empty={
        bodyReplaced && !loadFailed
          ? {
              icon: <MotoConceptIcon concept="groups" size={48} />,
              title: "Noch keine Gruppennachrichten",
              description: messagingEnabled
                ? "Mit einer Gruppennachricht schreiben Sie allen Eltern einer Gruppe, Klasse oder AG. Eltern antworten nur Ihnen."
                : "Der Nachrichtenaustausch mit den Eltern ist für diese Schule ausgeschaltet.",
              action: composeButton,
            }
          : null
      }
      overlays={
        composeOpen ? (
          <NewGroupMessageModal
            onClose={() => setComposeOpen(false)}
            onCreated={(detail) => {
              setComposeOpen(false);
              void mutate();
              router.push(`/messages/groups/${detail.thread.id}`);
            }}
          />
        ) : null
      }
    >
      <>
        {loadFailed && <LoadErrorAlert error={loadError} />}
        <ul className="space-y-3">
          {threads.map((thread) => (
            <li key={thread.id}>
              <TileCard
                onClick={() => router.push(`/messages/groups/${thread.id}`)}
              >
                <div className="flex min-w-0 items-center gap-2">
                  <MotoConceptIcon concept="groups" size={20} />
                  <h3 className="truncate text-base font-semibold text-gray-900">
                    {thread.title}
                  </h3>
                  {thread.closed_at && (
                    <StatusBadge tone="gray" label="Geschlossen" compact />
                  )}
                  {thread.last_message_at && (
                    <span className="ml-auto flex-shrink-0 pl-1 text-xs whitespace-nowrap text-gray-400">
                      {formatChatDateTime(thread.last_message_at)}
                    </span>
                  )}
                </div>
                <p className="mt-1 text-sm text-gray-500">
                  {thread.target_label} ·{" "}
                  {familyReadLabel(thread.read_count, thread.family_count)}
                </p>
                {thread.last_message_body && (
                  <p className="mt-1 truncate text-sm text-gray-600">
                    {thread.last_message_body}
                  </p>
                )}
              </TileCard>
            </li>
          ))}
        </ul>
      </>
    </TenantPage>
  );
}

export default function GroupMessagesPage() {
  return <GroupMessagesContent />;
}
