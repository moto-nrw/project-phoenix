"use client";

import { isParentMessagingDisabled } from "~/lib/parent-messages-api";

import { messageKeys } from "~/lib/swr/portal-keys";

import { useCallback, useState } from "react";
import { useParams } from "next/navigation";
import { useSWRAuth } from "~/lib/swr";
import { GROUP_THREAD_SEND_REFUSALS } from "~/lib/message-send-refusals";
import { useOptimisticAction } from "~/lib/swr/optimistic";
import { Alert } from "~/components/ui/alert";
import { ConfirmationModal } from "~/components/ui/modal";
import { EmptyState } from "~/components/ui/empty-state";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { formErrorMessage } from "~/components/ui/form-error";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage } from "~/components/ui/tenant-page";
import { ChatBubble } from "~/components/messaging/chat-bubble";
import { MessageComposer } from "~/components/messaging/message-composer";
import { useApiErrorDisplay, useApiFormError } from "~/contexts/ToastContext";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { useMessagesActivity } from "~/lib/hooks/use-messages-activity";
import { createLogger } from "~/lib/logger";
import {
  type GroupThreadDetail,
  closeGroupThread,
  familyReadLabel,
  groupRecipientsLabel,
  fetchGroupThread,
  postGroupMessage,
} from "~/lib/parent-group-threads-api";
import { useTenant } from "~/lib/tenant-context";

const logger = createLogger({ component: "GroupMessagePage" });

const GROUPS_BACK = {
  back: true,
  backHref: "/messages/groups",
  backLabel: "Zurück zu Gruppennachrichten",
} as const;

/**
 * One Gruppennachricht (#4784): the messages the OGS wrote, a field for one
 * more while it is open, and which families have read it. Parents answer in
 * the conversation of their child; their answers do not appear here.
 */
function GroupMessageContent() {
  const params = useParams();
  const threadId = params.groupThreadId as string;
  const { tenant } = useTenant();
  const [disabledThreadId, setDisabledThreadId] = useState<string>();
  const messagingEnabled =
    tenant?.messagingEnabled === true && disabledThreadId !== threadId;

  const {
    data: confirmed,
    error: readerError,
    isLoading,
    mutate,
  } = useSWRAuth<GroupThreadDetail>(
    messageKeys.groupMessage(threadId),
    () => fetchGroupThread(threadId),
    { keepPreviousData: false, revalidateOnFocus: false },
  );
  // Eine weitere Nachricht steht sofort im Verlauf (#5011), markiert, bis
  // die Antwort sie bestätigt oder ihr Fehlschlag sie als nicht gesendet
  // zeigt.
  const sendAction = useOptimisticAction(
    messageKeys.groupMessage(threadId),
    confirmed,
  );
  const data = sendAction.data;
  // Lehnt der Server beim Senden die Gruppennachricht ab, ist sie verworfen
  // (#5011). Die Ablehnung steht dann dort, wo sie fehlt, bis ein neuer
  // Abruf wieder Daten liefert.
  const [refusal, setRefusal] = useState<{
    readonly id: string;
    readonly error: unknown;
  }>();
  const error =
    readerError ??
    (data || refusal?.id !== threadId ? undefined : refusal.error);
  const loadError = useSwrLoadError(error, "die Gruppennachricht", mutate);

  // Group-thread broadcasts deliberately omit the thread id from staff events,
  // so refetch any open group detail. This view does not advance a read cursor.
  const refreshThread = useCallback(() => void mutate(), [mutate]);
  useMessagesActivity({
    onMatch: refreshThread,
    debounceMs: 500,
    marksRead: false,
    refetchOnFocus: true,
  });

  const [draft, setDraft] = useState("");
  const sending = sendAction.pending;
  const sendError = useApiFormError();
  const [confirmClose, setConfirmClose] = useState(false);
  const [closing, setClosing] = useState(false);
  const { show: showActionError } = useApiErrorDisplay();

  const send = async () => {
    const body = draft.trim();
    if (!body || sending || !messagingEnabled) return;
    sendError.clear();
    // Ein neuer Versuch ersetzt die als nicht gesendet markierte Blase.
    sendAction.dismiss();
    setDraft("");
    const outcome = await sendAction.run({
      optimistic: (current, { id }) => ({
        ...current,
        messages: [
          ...current.messages,
          { id, sender_name: "", body, created_at: "" },
        ],
      }),
      request: () => postGroupMessage(threadId, body),
      commit: (next) => void mutate(next, { revalidate: false }),
      keepFailed: true,
      refusal: {
        codes: GROUP_THREAD_SEND_REFUSALS,
        matches: (key) =>
          key === messageKeys.groupMessage(threadId) ||
          key === messageKeys.groupMessages(),
      },
    });
    if (outcome.status !== "failed") return;
    const err = outcome.error;
    // Der Text kommt zurück ins Feld, außer dort steht schon ein neuer.
    setDraft((current) => (current === "" ? body : current));
    logger.error("group_message_send_failed", {
      error: err instanceof Error ? err.message : String(err),
    });
    if (outcome.refused) {
      setRefusal({ id: threadId, error: err });
      return;
    }
    if (isParentMessagingDisabled(err)) {
      setDisabledThreadId(threadId);
      return;
    }
    void sendError.show(err, { object: "die Nachricht" });
  };

  const close = async () => {
    if (closing || !messagingEnabled) return;
    setClosing(true);
    try {
      const next = await closeGroupThread(threadId);
      await mutate(next, { revalidate: false });
      setConfirmClose(false);
    } catch (err) {
      logger.error("group_message_close_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setConfirmClose(false);
      if (isParentMessagingDisabled(err)) {
        setDisabledThreadId(threadId);
        return;
      }
      void showActionError(err, {
        object: "die Gruppennachricht",
        retry: () => setConfirmClose(true),
      });
    } finally {
      setClosing(false);
    }
  };

  const showSkeleton =
    (isLoading && !data) ||
    (Boolean(error) && !data && formErrorMessage(loadError) === null);

  if (!showSkeleton && !data) {
    return (
      <TenantPage
        title="Gruppennachricht"
        {...GROUPS_BACK}
        error={error ? loadError : "Gruppennachricht nicht gefunden."}
      />
    );
  }

  const thread = data?.thread;
  const closed = Boolean(thread?.closed_at);
  const children = data?.children ?? [];
  const stats =
    data && thread
      ? [
          thread.target_label,
          familyReadLabel(data.read_count, data.family_count),
          ...(closed ? ["Geschlossen"] : []),
        ].join(" · ")
      : null;

  return (
    <TenantPage
      title={thread?.title ?? "Gruppennachricht"}
      {...GROUPS_BACK}
      stats={stats}
      statsLoading={showSkeleton}
      loading={showSkeleton}
      overflowMenu={
        thread && !closed && messagingEnabled
          ? [{ label: "Schließen", onClick: () => setConfirmClose(true) }]
          : []
      }
      overlays={
        <ConfirmationModal
          isOpen={confirmClose}
          onClose={() => setConfirmClose(false)}
          onConfirm={() => void close()}
          title="Gruppennachricht schließen?"
          confirmText="Schließen"
          confirmVariant="warning"
          isConfirmLoading={closing}
          loadingText="Wird geschlossen…"
        >
          <p className="text-sm text-gray-700">
            Danach können Eltern nicht mehr antworten. Sie selbst können auch
            nichts mehr dazuschreiben.
          </p>
        </ConfirmationModal>
      }
    >
      <SectionCard title="Nachrichten">
        <div className="space-y-3">
          {(data?.messages ?? []).map((message) => (
            <ChatBubble
              key={message.id}
              body={message.body}
              own
              senderName={message.sender_name}
              createdAt={message.created_at}
              pending={sendAction.stateOf(message.id)}
            />
          ))}
        </div>
        <p className="mt-4 text-sm text-gray-500" data-testid="group-readers">
          {data?.readers ? `${groupRecipientsLabel(data.readers)} ` : null}
          Eltern antworten nur der OGS. Ihre Antworten finden Sie in der
          Unterhaltung mit der Familie.
        </p>
        <FormErrorAlert message={sendError.error} className="mt-3" />
        <div className="mt-4">
          {closed ? (
            <Alert
              type="info"
              message="Diese Gruppennachricht ist geschlossen. Eltern können nicht mehr antworten."
            />
          ) : messagingEnabled ? (
            <MessageComposer
              value={draft}
              onChange={setDraft}
              onSend={() => void send()}
              sending={sending}
              placeholder="Weitere Nachricht an alle Eltern…"
            />
          ) : (
            <Alert
              type="info"
              message="Die Eltern-Nachrichten sind für diese Schule ausgeschaltet. Sie können den Verlauf weiterhin lesen."
            />
          )}
        </div>
      </SectionCard>

      <SectionCard
        title="Gelesen"
        description="Je Kind: Hat ein Elternteil die neueste Nachricht gelesen?"
      >
        {children.length === 0 ? (
          <EmptyState title="Noch keine Familien" />
        ) : (
          <ul
            className="divide-y divide-gray-100"
            data-testid="group-read-list"
          >
            {children.map((child) => (
              <li
                key={child.student_id}
                className="flex items-center justify-between gap-3 py-2"
              >
                <span className="min-w-0 truncate text-sm text-gray-900">
                  {child.first_name} {child.last_name}
                  {child.school_class && (
                    <span className="text-gray-500">
                      {" "}
                      · {child.school_class}
                    </span>
                  )}
                </span>
                <StatusBadge
                  tone={child.read ? "green" : "gray"}
                  label={child.read ? "Gelesen" : "Noch nicht gelesen"}
                  compact
                />
              </li>
            ))}
          </ul>
        )}
      </SectionCard>
    </TenantPage>
  );
}

export default function GroupMessagePage() {
  return <GroupMessageContent />;
}
