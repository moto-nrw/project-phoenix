"use client";

import { useMessageSendAttempt } from "~/lib/swr/optimistic";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { useLocale, useTranslations } from "next-intl";
import { ChatBubble } from "~/components/messaging/chat-bubble";
import { MessageComposer } from "~/components/messaging/message-composer";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import NavigationLink from "~/components/ui/navigation-link";
import { SectionCard } from "~/components/ui/section-card";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { Skeleton } from "~/components/ui/skeleton";
import {
  ParentPage,
  ParentPageHeader,
  ParentPageSkeleton,
} from "~/components/parent/parent-page";
import { useApiFormError, useApiLoadError } from "~/contexts/ToastContext";
import {
  type GroupThreadChild,
  type GroupThreadView as GroupThreadData,
  getChildFeatures,
  getGroupThread,
  postChildMessage,
} from "~/lib/parent-api";
import { createLogger } from "~/lib/logger";
import {
  childConversationPath,
  parentReadersLine,
} from "~/lib/messaging-status";
import { parentPath } from "~/lib/parent-url";

const logger = createLogger({ component: "ParentGroupThreadView" });

// Opening the thread marked it read server-side; the sidebar badge refetches.
function nudgeUnreadBadge(): void {
  if (typeof window !== "undefined") {
    window.dispatchEvent(new CustomEvent("parent-messages-unread-refresh"));
  }
}

function childName(child: GroupThreadChild): string {
  return `${child.first_name} ${child.last_name}`.trim();
}

/**
 * Eine Gruppennachricht der OGS (#4784). Die OGS schreibt an die Gruppe,
 * Klasse oder AG; die Eltern lesen hier nur, was sie für ihre eigenen Kinder
 * erreicht hat. Eine Antwort geht nur an die OGS: sie landet im Chat des
 * Kindes mit der OGS, kein anderes Elternteil sieht sie.
 */
export function ParentGroupThreadView({
  groupThreadId,
}: Readonly<{ groupThreadId: string }>) {
  const t = useTranslations("parentGroupThreads");
  const tm = useTranslations("parentMessages");
  const locale = useLocale();
  const [thread, setThread] = useState<GroupThreadData | null>(null);
  const [loading, setLoading] = useState(true);
  // Children of the thread the guardian may write for (notes.write).
  const [writable, setWritable] = useState<GroupThreadChild[] | null>(null);
  const [selected, setSelected] = useState<string | null>(null);
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);
  const [sentFor, setSentFor] = useState<string | null>(null);
  const {
    error: loadError,
    show: showLoadError,
    clear: clearLoadError,
  } = useApiLoadError();
  const {
    error: sendError,
    show: showSendError,
    clear: clearSendError,
  } = useApiFormError();
  const mountedRef = useRef(true);
  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  const load = useCallback(async () => {
    try {
      const view = await getGroupThread(groupThreadId);
      if (!mountedRef.current) return;
      setThread(view);
      clearLoadError();
      nudgeUnreadBadge();
      // The composer follows the 1:1 chat's rule: only children whose
      // relationship grants writing (child features notes_enabled).
      const features = await Promise.all(
        view.children.map((child) =>
          getChildFeatures(child.student_id).then(
            (value) => value.notes_enabled,
            (err: unknown) => {
              // A child whose features do not load is left out of the
              // picker: offering it would end in a refused answer.
              logger.warn("group_thread_child_features_failed", {
                error: err instanceof Error ? err.message : String(err),
              });
              return false;
            },
          ),
        ),
      );
      if (!mountedRef.current) return;
      const allowed = view.children.filter((_, index) => features[index]);
      setWritable(allowed);
      setSelected((current) =>
        current && allowed.some((child) => child.student_id === current)
          ? current
          : (allowed[0]?.student_id ?? null),
      );
    } catch (err) {
      logger.warn("group_thread_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      if (mountedRef.current) {
        void showLoadError(err, {
          object: t("errorObjectThread"),
          retry: () => void loadRef.current(),
        });
      }
    } finally {
      if (mountedRef.current) setLoading(false);
    }
  }, [groupThreadId, clearLoadError, showLoadError, t]);
  const loadRef = useRef(load);
  useLayoutEffect(() => {
    loadRef.current = load;
  });

  useEffect(() => {
    void load();
  }, [load]);
  useEffect(() => {
    const refresh = () => void loadRef.current();
    window.addEventListener("focus", refresh);
    return () => window.removeEventListener("focus", refresh);
  }, []);

  const attempt = useMessageSendAttempt(
    `parent-group-reply:${groupThreadId}:${selected ?? ""}`,
  );
  const handleSend = useCallback(async () => {
    const body = draft.trim();
    if (!body || sending || !selected) return;
    const clientMessageId = attempt.begin(body);
    setSending(true);
    clearSendError();
    try {
      await postChildMessage(
        selected,
        body,
        groupThreadId,
        "team",
        clientMessageId,
      );
      attempt.complete(clientMessageId);
      if (!mountedRef.current) return;
      setDraft("");
      setSentFor(selected);
    } catch (err) {
      logger.warn("group_thread_answer_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showSendError(err, {
        object: t("errorObjectAnswer"),
        retry: () => void sendRef.current(),
      });
    } finally {
      if (mountedRef.current) setSending(false);
    }
  }, [
    draft,
    sending,
    selected,
    groupThreadId,
    attempt,
    clearSendError,
    showSendError,
    t,
  ]);
  const sendRef = useRef(handleSend);
  useLayoutEffect(() => {
    sendRef.current = handleSend;
  });

  if (loading) return <ParentPageSkeleton rows={2} />;

  if (!thread) {
    return (
      <ParentPage>
        <ParentPageHeader
          title={tm("title")}
          backHref={parentPath("/parents/messages")}
          backLabel={tm("back")}
        />
        <LoadErrorAlert error={loadError} />
      </ParentPage>
    );
  }

  const counterpart = thread.school_name
    ? tm("ogsTeam", { school: thread.school_name })
    : thread.counterpart_name;
  const readersLine = thread.readers
    ? parentReadersLine(thread.readers, tm)
    : null;
  const chatChild = sentFor ?? selected ?? thread.children[0]?.student_id;
  const chatLink = chatChild ? (
    <NavigationLink
      href={parentPath(childConversationPath(chatChild))}
      className="text-moto-blue text-sm font-semibold underline-offset-2 hover:underline"
    >
      {t("toChat")}
    </NavigationLink>
  ) : null;

  return (
    <ParentPage>
      <ParentPageHeader
        kicker={thread.target_label}
        title={thread.title}
        description={`${counterpart} · ${t("forChildren", {
          names: thread.children.map(childName).join(", "),
        })}`}
        backHref={parentPath("/parents/messages")}
        backLabel={tm("back")}
      />
      {loadError && <LoadErrorAlert error={loadError} />}

      <SectionCard>
        <div className="space-y-2.5" data-testid="group-thread-messages">
          {thread.messages.map((message) => (
            <ChatBubble
              key={message.id}
              body={message.body}
              own={false}
              senderName={counterpart}
              createdAt={message.created_at}
              locale={locale}
              tone="parent"
            />
          ))}
        </div>
      </SectionCard>

      {/* Danach steht, wer auf OGS-Seite die Antwort liest (#5200). */}
      <SectionCard
        title={t("replyTitle")}
        description={
          readersLine ? `${t("privacyNote")} ${readersLine}` : t("privacyNote")
        }
      >
        {sentFor ? (
          <div className="space-y-2" role="status">
            <p className="text-sm font-semibold text-gray-900">
              {t("sentConfirmation")}
            </p>
            {chatLink}
          </div>
        ) : thread.closed ? (
          <div className="space-y-2">
            <p className="text-sm leading-6 text-gray-600">{t("closedNote")}</p>
            {chatLink}
          </div>
        ) : writable === null ? (
          <Skeleton className="h-16 w-full rounded-lg" />
        ) : writable.length === 0 ? (
          <p className="rounded-xl bg-gray-50 px-4 py-3 text-sm text-gray-600">
            {t("writingDisabled")}
          </p>
        ) : (
          <div className="space-y-3">
            {writable.length > 1 && (
              <div className="space-y-1.5">
                <p className="text-sm font-medium text-gray-700">
                  {t("replyChildLabel")}
                </p>
                <SegmentedControl
                  items={writable.map((child) => ({
                    value: child.student_id,
                    label: childName(child),
                  }))}
                  value={selected}
                  onChange={setSelected}
                  ariaLabel={t("replyChildLabel")}
                />
              </div>
            )}
            <FormErrorAlert message={sendError} />
            <MessageComposer
              value={draft}
              onChange={setDraft}
              onSend={() => void handleSend()}
              sending={sending}
              placeholder={t("replyPlaceholder")}
              tone="parent"
              sendLabel={tm("send")}
              sendingLabel={tm("sending")}
              fieldLabel={t("replyFieldLabel")}
            />
          </div>
        )}
      </SectionCard>
    </ParentPage>
  );
}
