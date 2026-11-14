"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { AggregatedRequestList } from "~/components/students/aggregated-request-list";
import { Button } from "~/components/ui/button";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { InsetPanel } from "~/components/ui/inset-panel";
import { Modal } from "~/components/ui/modal";
import { MessageAttachments } from "~/components/ui/message-attachments";
import {
  PARENT_MESSAGE_ATTACHMENTS,
  fetchMessageAttachmentFile,
} from "~/lib/message-attachments";
import { ListSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import { SectionCard } from "~/components/ui/section-card";
import { SectionLabel } from "~/components/ui/section-label";
import { useApiErrorDisplay, useApiLoadError } from "~/contexts/ToastContext";
import { formatTime } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import type { ParentAttention } from "~/lib/parent-attention";
import {
  confirmThreadForTeam,
  fetchStudentThreads,
  fetchThread,
  relationshipLabel,
  type InboxThread,
  type Message,
} from "~/lib/parent-messages-api";
import { useTenantRouter } from "~/lib/tenant-router";

const logger = createLogger({ component: "ParentAttentionDialog" });

/** Eine ungelesene Unterhaltung, wie der Dialog sie beim Öffnen gelesen hat. */
interface UnreadConversation {
  readonly thread: InboxThread;
  /** Die neuen Nachrichten der Eltern, älteste zuerst. */
  readonly messages: readonly Message[];
  /** Die letzte Nachricht, die der Dialog zeigt: bis hier gilt „gelesen“. */
  readonly lastShownId: string | null;
}

/** The same counterpart classification as the backend, without stale count slicing. */
export function parentMessagesInSnapshot(
  messages: readonly Message[],
): Message[] {
  return messages.filter(
    (message) =>
      message.event_type !== "request_created" &&
      (message.sender_kind === "guardian" ||
        (message.sender_kind === "system" &&
          message.event_actor_kind === "guardian")),
  );
}

function messageText(message: Message): string {
  if (message.body.trim() !== "") return message.body;
  return message.attachments?.length ? "Foto oder Datei" : "Ohne Text";
}

function ConversationCard({
  conversation,
  confirmed,
  confirming,
  onConfirm,
  onAnswer,
}: Readonly<{
  conversation: UnreadConversation;
  confirmed: boolean;
  confirming: boolean;
  onConfirm: () => void;
  onAnswer: () => void;
}>) {
  const { thread, messages } = conversation;
  return (
    <SectionCard
      title={`${thread.guardian_name} (${relationshipLabel(thread.relationship_type)})`}
    >
      <div className="space-y-2">
        {messages.map((message) => (
          <InsetPanel key={message.id} className="space-y-1 text-sm">
            <p className="text-xs text-gray-500">
              {formatTime(message.created_at)} Uhr
            </p>
            <p className="whitespace-pre-line text-gray-900">
              {messageText(message)}
            </p>
            {message.attachments?.length ? (
              <MessageAttachments
                attachments={message.attachments}
                downloadUrl={PARENT_MESSAGE_ATTACHMENTS.downloadUrl}
                loadFile={fetchMessageAttachmentFile}
              />
            ) : null}
          </InsetPanel>
        ))}
        <div className="flex flex-wrap gap-2 pt-1">
          {confirmed ? (
            <p
              className="text-moto-green-strong text-sm font-medium"
              role="status"
            >
              Für das Team als gelesen bestätigt.
            </p>
          ) : conversation.lastShownId !== null ? (
            <Button
              type="button"
              variant="success"
              size="compact"
              isLoading={confirming}
              loadingText="Wird bestätigt …"
              onClick={onConfirm}
              tooltip="Das ganze Team sieht die Nachricht danach nicht mehr als neu."
            >
              Als gelesen bestätigen
            </Button>
          ) : null}
          <Button
            type="button"
            variant="surface"
            size="compact"
            onClick={onAnswer}
            tooltip="Öffnet die Unterhaltung, dort können Sie antworten."
          >
            Antworten
          </Button>
        </div>
      </div>
    </SectionCard>
  );
}

function MessagesSection({ studentId }: Readonly<{ studentId: string }>) {
  const router = useTenantRouter();
  const { error, show: showLoadError, clear } = useApiLoadError();
  const { show: showActionError } = useApiErrorDisplay();
  const [conversations, setConversations] = useState<
    UnreadConversation[] | null
  >(null);
  const [confirmed, setConfirmed] = useState<ReadonlySet<string>>(
    () => new Set(),
  );
  const [confirming, setConfirming] = useState<string | null>(null);
  const loadRef = useRef<(threads?: readonly InboxThread[]) => Promise<void>>(
    async () => undefined,
  );

  const load = useCallback(
    async (retryThreads?: readonly InboxThread[]) => {
      clear();
      try {
        const threads =
          retryThreads ??
          (await fetchStudentThreads(studentId)).filter(
            (thread) => thread.unread_count > 0,
          );
        const results = await Promise.allSettled(
          threads.map(async (thread) => {
            // The endpoint marks this exact message snapshot as personally read.
            // Show all its parent-side content, including older messages, so a
            // stale list count or a concurrent arrival can never hide a read row.
            const detail = await fetchThread(thread.thread_id);
            const messages = parentMessagesInSnapshot(detail.messages);
            return {
              thread,
              messages,
              lastShownId: messages.at(-1)?.id ?? null,
            };
          }),
        );
        const loaded: UnreadConversation[] = [];
        const failed: InboxThread[] = [];
        let failure: unknown;
        results.forEach((result, index) => {
          if (result.status === "fulfilled") loaded.push(result.value);
          else {
            const thread = threads[index];
            if (thread) failed.push(thread);
            failure = result.reason;
          }
        });
        setConversations((previous) => {
          const merged = new Map(
            (previous ?? []).map((conversation) => [
              conversation.thread.thread_id,
              conversation,
            ]),
          );
          for (const conversation of loaded)
            merged.set(conversation.thread.thread_id, conversation);
          return [...merged.values()];
        });
        if (loaded.length > 0)
          window.dispatchEvent(new CustomEvent("messages-unread-refresh"));
        if (failed.length > 0) {
          logger.warn("parent_attention_messages_load_failed", {
            error: failure instanceof Error ? failure.message : String(failure),
          });
          void showLoadError(failure, {
            object: "die Nachrichten",
            retry: () => loadRef.current(failed),
          });
        }
      } catch (err: unknown) {
        logger.warn("parent_attention_messages_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setConversations((previous) => previous ?? []);
        void showLoadError(err, {
          object: "die Nachrichten",
          retry: () => loadRef.current(retryThreads),
        });
      }
    },
    [clear, showLoadError, studentId],
  );

  useEffect(() => {
    loadRef.current = load;
    void load();
  }, [load]);

  const confirm = useCallback(
    async (conversation: UnreadConversation) => {
      if (conversation.lastShownId === null) return;
      const threadId = conversation.thread.thread_id;
      setConfirming(threadId);
      try {
        await confirmThreadForTeam(threadId, conversation.lastShownId);
        setConfirmed((current) => new Set(current).add(threadId));
        window.dispatchEvent(new CustomEvent("messages-unread-refresh"));
      } catch (err: unknown) {
        logger.warn("parent_attention_confirm_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        await showActionError(err, { object: "die Bestätigung" });
      } finally {
        setConfirming(null);
      }
    },
    [showActionError],
  );

  if (conversations === null) {
    return (
      <SkeletonRegion label="Nachrichten werden geladen">
        <ListSkeleton rows={2} avatar={false} />
      </SkeletonRegion>
    );
  }
  return (
    <div className="space-y-3">
      <LoadErrorAlert error={error} />
      {conversations.length === 0 && !error ? (
        <p className="text-sm text-gray-600">Keine neuen Nachrichten.</p>
      ) : null}
      {conversations.map((conversation) => (
        <ConversationCard
          key={conversation.thread.thread_id}
          conversation={conversation}
          confirmed={confirmed.has(conversation.thread.thread_id)}
          confirming={confirming === conversation.thread.thread_id}
          onConfirm={() => void confirm(conversation)}
          onAnswer={() =>
            router.push(`/messages/${conversation.thread.thread_id}`)
          }
        />
      ))}
    </div>
  );
}

/**
 * Neue Nachrichten und offene Anfragen der Eltern eines Kindes, direkt von der
 * Kinderkachel geöffnet (#6486). Nachrichten bestätigt die Person für das
 * ganze Team als gelesen; Anfragen entscheidet sie wie auf der Anfragen-Seite.
 */
export function ParentAttentionDialog({
  studentId,
  studentName,
  attention,
  isOpen,
  onClose,
}: Readonly<{
  studentId: string;
  studentName: string;
  /** Die Zahlen beim Öffnen; sie entscheiden, welche Teile der Dialog zeigt. */
  attention: ParentAttention;
  isOpen: boolean;
  onClose: () => void;
}>) {
  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={`Neu von den Eltern: ${studentName}`}
      size="xl"
      mobileSheet
    >
      <div className="space-y-6">
        {attention.messages > 0 ? (
          <section className="space-y-3" aria-label="Nachrichten">
            <SectionLabel>Nachrichten</SectionLabel>
            <MessagesSection key={studentId} studentId={studentId} />
          </section>
        ) : null}
        {attention.requests > 0 ? (
          <section className="space-y-3" aria-label="Offene Anfragen">
            <SectionLabel>Offene Anfragen</SectionLabel>
            <AggregatedRequestList
              view="open"
              singleChild
              filters={{
                search: "",
                studentId,
                types: [],
                statuses: [],
                includeEnrollment: false,
                includeCareWithdrawals: false,
              }}
            />
          </section>
        ) : null}
      </div>
    </Modal>
  );
}
