"use client";

import { useMessageSendAttempt } from "~/lib/swr/optimistic";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { XIcon } from "@phosphor-icons/react/ssr";
import { useLocale, useTranslations } from "next-intl";
import { ApiError } from "~/lib/api-error";
import { BackLink } from "~/components/ui/back-link";
import NavigationLink from "~/components/ui/navigation-link";
import { parentPath } from "~/lib/parent-url";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { useApiFormError, useApiLoadError } from "~/contexts/ToastContext";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { Skeleton } from "~/components/ui/skeleton";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { MessageComposer } from "~/components/messaging/message-composer";
import { ChatBubble, ChatEventCard } from "~/components/messaging/chat-bubble";
import { RequestStatusBadge } from "~/components/messaging/request-status-badge";
import { useChatViewportLock } from "~/lib/hooks/use-chat-viewport-lock";
import { useLatest } from "~/lib/hooks/use-latest";
import {
  type MessageAudience,
  parentEventI18nDescriptor,
  hasLeadershipReaders,
  parentReadersLine,
  parentRequestStatusI18nKey,
  parentRequestTypeI18nKey,
} from "~/lib/messaging-status";
import {
  type ChildFeatures,
  type ChildToday,
  type ParentMessage,
  type ThreadSummary,
  type ThreadView,
  UNKNOWN_CHILD_TODAY,
  getChildConversation,
  getChildToday,
  listChildThreads,
  postChildMessage,
} from "~/lib/parent-api";
import {
  PickupTimeModal,
  SickNoteModal,
  getOgsActions,
  useChildCare,
  type OgsActionKey,
} from "~/components/parent/child-care";
import { useMessagesActivity } from "~/lib/hooks/use-messages-activity";
import { createLogger } from "~/lib/logger";
import { PARENT_PORTAL_SEND_REFUSALS } from "~/lib/message-send-refusals";
import { useOptimisticAction } from "~/lib/swr/optimistic";
import { parentKeys } from "~/lib/swr/parent-keys";
import { formatChatTime } from "~/lib/date-helpers";
import { childrenConcept } from "~/lib/concepts/children";

const logger = createLogger({ component: "OgsConversation" });

// Nudge the sidebar unread badge to refetch after a read/send that marked the
// thread read server-side. EVERY such path must call this AFTER the read
// resolves, including the SSE-driven refresh: the portal-wide
// ParentRealtimeBridge dispatches its own badge refresh BEFORE the conversation
// GET runs (and defers hidden-tab reads), so that first fetch races ahead of the
// read-cursor advance and can leave the badge counting a message this open chat
// has already marked read. Re-nudging once refresh() resolves corrects the stale
// count; the extra debounced fetch is the price of not trusting the bridge's
// pre-read ordering.
function nudgeUnreadBadge(): void {
  if (typeof window !== "undefined") {
    window.dispatchEvent(new CustomEvent("parent-messages-unread-refresh"));
  }
}

/**
 * The parent <-> OGS chat for one child. The parent is talking to "the OGS
 * [Schulname]" — the child is only context, never the counterpart. Used both
 * inline on the messages landing page (single-child parents go straight here)
 * and on the per-child route (multi-child parents, with a back link + the
 * child shown for disambiguation).
 */
export function OgsConversation({
  studentId,
  showBack = false,
  showChild = false,
  audience: audienceProp,
  onAudienceChange,
}: {
  readonly studentId: string;
  readonly showBack?: boolean;
  readonly showChild?: boolean;
  /**
   * The conversation shown (#5200). Given by the per-child route from
   * `?audience=`, which onAudienceChange updates; without it the component
   * keeps the choice itself and starts with the team.
   */
  readonly audience?: MessageAudience;
  readonly onAudienceChange?: (next: MessageAudience) => void;
}) {
  const t = useTranslations("parentOgsMessaging");
  const tm = useTranslations("parentMessages");
  const locale = useLocale();
  // A child has a team conversation and, once begun, one with the leadership
  // only (#5200). The switch shows one of them and is where the next message
  // goes.
  const [ownAudience, setOwnAudience] = useState<MessageAudience>("team");
  const audience = audienceProp ?? ownAudience;
  // Both conversations of the child: whether a leadership one exists (the
  // switch stays for it after the school switched the offer off) and the
  // unread count of the one not on screen.
  const [summaries, setSummaries] = useState<readonly ThreadSummary[]>([]);
  const [summariesFailed, setSummariesFailed] = useState(false);
  const [loadedThread, setThread] = useState<ThreadView | null>(null);
  // Only the conversation of the chosen audience is shown. A late answer for
  // the other one (a slow load or send from before a switch) never appears
  // under this audience's title and readers.
  const audienceRef = useLatest(audience);

  const confirmedThread =
    loadedThread && (loadedThread.audience ?? "team") === audience
      ? loadedThread
      : null;
  // A sent message shows at once (#5011), marked until the answer confirms
  // it or its failure marks it as not sent. Bound to the session's scope
  // and generation like the SWR readers.
  const conversationKey = parentKeys.conversation(studentId, audience);
  const send = useOptimisticAction(
    conversationKey,
    confirmedThread ?? undefined,
  );
  const thread = send.data ?? null;
  const [loading, setLoading] = useState(true);
  // A failed load shows in the thread with retry (#2518). Only shown while
  // nothing is on screen: a failed background refresh keeps the visible chat.
  const {
    error: loadError,
    show: showLoadError,
    clear: clearLoadError,
  } = useApiLoadError();
  const [draft, setDraft] = useState("");
  const care = useChildCare(studentId);
  const [today, setToday] = useState<ChildToday>(UNKNOWN_CHILD_TODAY);
  const [activeModal, setActiveModal] = useState<OgsActionKey | null>(null);
  const sending = send.pending;
  const {
    error: sendError,
    show: showSendError,
    clear: clearSendError,
  } = useApiFormError();
  const scrollRef = useRef<HTMLDivElement | null>(null);
  // Pin the chat to the viewport and lock page scroll (only the list scrolls).
  const containerRef = useChatViewportLock<HTMLDivElement>(!loading);

  useEffect(() => {
    if (activeModal !== "pickup") return;
    void getChildToday(studentId)
      .then(setToday)
      .catch((err: unknown) => {
        // Deliberately silent: the pickup dialog works without today's
        // schedule and only drops its hint about the usual pickup time.
        logger.warn("ogs_child_today_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setToday(UNKNOWN_CHILD_TODAY);
      });
  }, [activeModal, studentId]);

  // Latest-wins guard shared by EVERY setThread path (refresh, send). SSE fires
  // one parent-conversation-refresh per message and those refetches can overlap a
  // just-sent write, so without a token an older in-flight snapshot resolving
  // late would clobber a fresher one.
  //
  // READS (refresh) claim their token at START — latest-started read wins. The
  // SEND claims it right BEFORE applying its result, AFTER the await: the value
  // it returns is the authoritative post-commit thread (it includes the
  // just-sent message), so it must always win. Claiming the send's token at start
  // let a parent-conversation-refresh that began during the send window take a
  // higher token and apply a PRE-commit snapshot, dropping the send's result and
  // making the sent message vanish until the next event. mountedRef additionally
  // blocks setState after unmount.
  const applySeqRef = useRef(0);
  const mountedRef = useRef(true);
  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);
  const applyThread = useCallback((seq: number, view: ThreadView): boolean => {
    if (!mountedRef.current || seq !== applySeqRef.current) return false;
    setThread(view);
    return true;
  }, []);

  // Reading the conversation marks it read server-side, so no separate
  // mark-read call is needed.
  const refresh = useCallback(async () => {
    const seq = ++applySeqRef.current;
    void listChildThreads(studentId)
      .then((rows) => {
        if (mountedRef.current && seq === applySeqRef.current) {
          setSummaries(rows);
          setSummariesFailed(false);
        }
      })
      .catch((err: unknown) => {
        // The list says whether a leadership conversation exists. Without it
        // the switch stays on screen (see showAudienceChoice), so an existing
        // leadership conversation remains reachable; only its unread count is
        // missing until the next refresh (focus or new message) loads it.
        logger.warn("ogs_child_threads_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (mountedRef.current && seq === applySeqRef.current) {
          setSummariesFailed(true);
        }
      });
    try {
      const view = await getChildConversation(studentId, audience);
      if (applyThread(seq, view)) clearLoadError();
    } catch (err) {
      logger.warn("ogs_conversation_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      if (mountedRef.current && seq === applySeqRef.current) {
        void showLoadError(err, {
          object: tm("errorObjectConversation"),
          retry: () => void refreshRef.current(),
        });
      }
    }
  }, [studentId, audience, applyThread, clearLoadError, showLoadError, tm]);
  // The retry runs the latest refresh, not the one of the failed attempt.
  const refreshRef = useRef(refresh);
  useLayoutEffect(() => {
    refreshRef.current = refresh;
  });

  useEffect(() => {
    let active = true;
    void (async () => {
      setLoading(true);
      await refresh();
      if (active) {
        setLoading(false);
        // Opening the conversation marked it read server-side; refresh the badge.
        nudgeUnreadBadge();
      }
    })();
    return () => {
      active = false;
    };
  }, [studentId, refresh]);

  // Real-time: the portal-wide ParentRealtimeBridge owns the SINGLE SSE
  // connection to /api/parent/sse/events and dispatches `parent-conversation-
  // refresh` (carrying the affected studentId) per parent_message. Subscribe via
  // the shared useMessagesActivity hook (eventName override) instead of opening a
  // SECOND EventSource — that duplicate doubled SSE connections + backend
  // goroutines and fired every event twice. The hook skips the refetch when the
  // event names a DIFFERENT child (a multi-child guardian gets one event per child).
  const refreshConversation = useCallback(
    () => void refresh().then(nudgeUnreadBadge),
    [refresh],
  );
  useMessagesActivity({
    eventName: "parent-conversation-refresh",
    studentId,
    // refresh() marks the thread read server-side; nudge the badge AFTER it
    // resolves so the sidebar count drops the message this chat just read. The
    // bridge's own pre-read badge dispatch races ahead of this and would
    // otherwise leave the badge stale (see nudgeUnreadBadge).
    onMatch: refreshConversation,
    // SSE is this chat's only refresh path after mount; if the bridge dropped
    // while the tab slept a message could have been missed, so refetch on return.
    refetchOnFocus: true,
  });

  const messages = thread?.messages ?? [];
  const counterpart = thread?.counterpart_name ?? "OGS";
  const leadershipThread = summaries.find(
    (summary) => summary.audience === "leadership",
  );
  const teamThread = summaries.find((summary) => summary.audience === "team");
  // The choice exists while the school offers it, and stays for a parent who
  // already writes with the leadership after the school switched it off.
  const showAudienceChoice =
    thread?.leadership_option?.available === true ||
    leadershipThread !== undefined ||
    summariesFailed ||
    audience === "leadership";
  const leadershipView = audience === "leadership";
  // The leadership cannot be written to (#5200): the school does not offer
  // it and either no leadership conversation exists yet, or nobody of the
  // leadership reads the existing one any more. Its history stays readable;
  // writing goes to the team.
  const leadershipClosed =
    leadershipView &&
    thread !== null &&
    thread.leadership_option?.available !== true &&
    (!thread.thread_id ||
      (thread.readers !== undefined &&
        !hasLeadershipReaders(thread.readers.school)));
  const leadershipClosedText = thread?.thread_id
    ? tm("leadershipUnstaffed")
    : tm("leadershipUnavailable");
  const readersLine = leadershipClosed
    ? leadershipClosedText
    : thread?.readers
      ? parentReadersLine(thread.readers, tm)
      : null;

  // Switching shows the other conversation. Not while a message is on its
  // way: it belongs to the conversation it was written in. The draft stays,
  // so a text can still go to the other side.
  const changeAudience = (next: MessageAudience) => {
    if (next === audience || sending) return;
    setLoading(true);
    clearSendError();
    if (onAudienceChange) onAudienceChange(next);
    else setOwnAudience(next);
  };

  // Keep the conversation scrolled to the newest message.
  useEffect(() => {
    const el = scrollRef.current;
    if (el) el.scrollTop = el.scrollHeight;
    // A send error shrinks the list; keep the bubble marked as not sent in
    // view (#5011).
  }, [thread, care.loading, sendError]);

  const attempt = useMessageSendAttempt(conversationKey);
  const { run: runSend, dismiss: dismissFailedSend } = send;
  const handleSend = useCallback(async () => {
    const body = draft.trim();
    if (!body || sending) return;
    const clientMessageId = attempt.begin(body);
    clearSendError();
    // A new attempt replaces the bubble marked as not sent.
    dismissFailedSend();
    setDraft("");
    const outcome = await runSend({
      confirmed: (current) =>
        current.messages.some(
          (message) =>
            message.client_message_id === clientMessageId &&
            message.sender_kind === "guardian",
        ),
      optimistic: (current, { id }) => ({
        ...current,
        messages: [
          ...current.messages,
          {
            id,
            kind: "message",
            sender_kind: "guardian",
            sender_name: "",
            body,
            created_at: "",
          },
        ],
      }),
      request: () =>
        postChildMessage(studentId, body, undefined, audience, clientMessageId),
      // Claim the token AFTER the POST resolves so this authoritative result
      // wins. A successful send means the thread is loadable, so clear any
      // stale load error left by an earlier failed background refresh. A
      // result for a conversation no longer on screen (the route changed the
      // audience meanwhile) claims nothing: the load of the new one must win.
      commit: (view) => {
        if (
          audienceRef.current === audience &&
          applyThread(++applySeqRef.current, view)
        ) {
          clearLoadError();
        }
      },
      keepFailed: true,
      refusal: {
        codes: PARENT_PORTAL_SEND_REFUSALS,
        matches: (key) => key === conversationKey,
      },
    });
    if (outcome.status === "done") {
      attempt.complete(clientMessageId);
      // Sending advances the reader's own cursor (any prior staff messages
      // are now read), so refresh the sidebar badge.
      nudgeUnreadBadge();
      return;
    }
    if (outcome.status !== "failed") return;
    const err = outcome.error;
    // The text goes back into the field unless a new one is there.
    setDraft((current) => (current === "" ? body : current));
    logger.warn("ogs_message_send_failed", {
      error: err instanceof Error ? err.message : String(err),
    });
    void showSendError(err, {
      object: tm("errorObjectMessage"),
      retry: () => void sendRef.current(),
    });
    if (outcome.refused) {
      // The school refused this conversation (relationship gone, no
      // writing right): drop the history, the composer and the overview of
      // both conversations, then ask again, so only what the server still
      // grants shows.
      ++applySeqRef.current;
      setThread(null);
      setSummaries([]);
      void refreshRef.current();
    } else if (
      err instanceof ApiError &&
      err.code === "communication.leadership_messages_unavailable"
    ) {
      // The leadership is gone since the chat loaded: reload it, so the
      // composer gives way to the hint about the team conversation.
      void refreshRef.current();
    }
  }, [
    draft,
    sending,
    studentId,
    audience,
    audienceRef,
    applyThread,
    clearLoadError,
    clearSendError,
    showSendError,
    tm,
    runSend,
    dismissFailedSend,
    conversationKey,
    attempt,
  ]);
  // The retry sends the current draft, not the one of the failed attempt.
  const sendRef = useRef(handleSend);
  useLayoutEffect(() => {
    sendRef.current = handleSend;
  });

  return (
    <div
      ref={containerRef}
      className="flex min-h-[20rem] w-full flex-col gap-3 overflow-hidden"
    >
      <section className="moto-content-surface flex min-h-0 flex-1 flex-col rounded-2xl border shadow-sm">
        <div className="border-b border-gray-100 px-4 py-3 sm:px-5">
          {showBack ? <BackBar /> : null}
          {loading ? (
            <div
              className="flex min-h-7 items-center justify-between gap-3"
              aria-hidden="true"
            >
              <Skeleton className="h-5 w-48 max-w-2/3" />
              {showChild ? <Skeleton className="h-7 w-28 rounded-lg" /> : null}
            </div>
          ) : (
            <div className="flex items-center justify-between gap-3">
              <div className="min-w-0">
                <h1 className="text-base leading-tight font-semibold tracking-tight break-words text-gray-900 sm:text-lg">
                  {thread?.school_name
                    ? tm(leadershipView ? "ogsLeadership" : "ogsTeam", {
                        school: thread.school_name,
                      })
                    : counterpart}
                </h1>
              </div>
              {showChild && thread?.student_name ? (
                <div
                  className="bg-moto-green-soft inline-flex w-fit shrink-0 items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-xs font-semibold text-gray-900"
                  title={tm("aboutChild", { name: thread.student_name })}
                >
                  <MotoConceptIcon
                    concept={childrenConcept}
                    size={16}
                    aria-hidden="true"
                  />
                  {thread.student_name}
                </div>
              ) : null}
            </div>
          )}
          {showAudienceChoice ? (
            <SegmentedControl<MessageAudience>
              className="mt-3"
              ariaLabel={tm("audienceLabel")}
              value={audience}
              onChange={changeAudience}
              items={[
                {
                  value: "team",
                  label: audienceLabel(tm, "team", teamThread, audience),
                  disabled: sending,
                },
                {
                  value: "leadership",
                  label: audienceLabel(
                    tm,
                    "leadership",
                    leadershipThread,
                    audience,
                  ),
                  disabled: sending,
                },
              ]}
            />
          ) : null}
          {readersLine && !loading ? (
            <p
              className="mt-2 text-sm text-gray-600"
              data-testid="conversation-readers"
            >
              {readersLine}
            </p>
          ) : null}
        </div>

        <div
          ref={scrollRef}
          className="flex-1 space-y-2.5 overflow-y-auto bg-gray-50/60 px-3 py-3 sm:px-5 sm:py-4"
        >
          {/* Keep already-loaded messages on screen even if a later background
              refresh (SSE / focus) fails transiently — only fall back to the
              error state when there is nothing to show. Otherwise a flaky
              refetch would wipe a visible conversation. */}
          {loading ? (
            <ThreadSkeleton />
          ) : messages.length > 0 ? (
            messages.map((message, index) => {
              const eventI18n =
                message.kind === "event"
                  ? parentEventI18nDescriptor(message, locale)
                  : null;
              const previous = messages[index - 1];
              const sameSenderAsPrevious =
                previous?.kind === "message" &&
                message.kind === "message" &&
                previous.sender_kind === message.sender_kind &&
                previous.sender_name === message.sender_name;
              const genericStaffSender =
                message.kind === "message" &&
                message.sender_kind === "staff" &&
                (message.sender_name === counterpart ||
                  (thread?.school_name
                    ? message.sender_name ===
                      tm("ogsTeam", { school: thread.school_name })
                    : false));
              return message.kind === "event" ? (
                <ChatEventCard
                  key={message.id}
                  // Backend system-event bodies are German; render the localized
                  // text from the event's structured fields, falling back to the
                  // raw body only for events we can't localize.
                  body={
                    eventI18n
                      ? t(eventI18n.key, eventI18n.values)
                      : message.body
                  }
                  createdAt={message.created_at}
                  locale={locale}
                />
              ) : message.kind === "request" ? (
                <RequestItem
                  key={message.id}
                  message={message}
                  locale={locale}
                />
              ) : (
                <div key={message.id}>
                  {message.group_thread_id && (
                    <GroupThreadReference
                      groupThreadId={message.group_thread_id}
                      title={message.group_thread_title}
                    />
                  )}
                  <ChatBubble
                    body={message.body}
                    own={message.sender_kind === "guardian"}
                    senderName={
                      message.sender_kind === "staff" &&
                      message.sender_name === counterpart &&
                      thread?.school_name
                        ? tm("ogsTeam", { school: thread.school_name })
                        : message.sender_name
                    }
                    createdAt={message.created_at}
                    locale={locale}
                    // The parent's own bubbles are always the logged-in guardian
                    // (one guardian account per thread), so drop the redundant name
                    // and keep just the time. Staff bubbles still show "Vorname N.".
                    showOwnSenderName={false}
                    showSenderName={
                      !genericStaffSender && !sameSenderAsPrevious
                    }
                    tone="parent"
                    pending={send.stateOf(message.id)}
                    pendingLabel={
                      send.stateOf(message.id) === "failed"
                        ? tm("notSent")
                        : send.stateOf(message.id) === "pending"
                          ? tm("sending")
                          : undefined
                    }
                    deliveryStatus={
                      message.sender_kind === "guardian"
                        ? message.read_by_staff
                          ? "read"
                          : "sent"
                        : undefined
                    }
                    deliveryStatusLabel={
                      message.sender_kind === "guardian"
                        ? message.read_by_staff
                          ? tm("readByOgs")
                          : tm("sent")
                        : undefined
                    }
                  />
                </div>
              );
            })
          ) : loadError ? (
            <LoadErrorAlert error={loadError} />
          ) : (
            <EmptyThread
              // A closed leadership conversation invites no first message.
              description={
                leadershipClosed
                  ? null
                  : tm(
                      leadershipView
                        ? "emptyLeadershipDescription"
                        : "emptyThreadDescription",
                    )
              }
            />
          )}
        </div>

        {/* Angeheftet am unteren Rand, mit Sicherheitsbereich des Geraets:
            auf dem Handy darf nichts unter dem Home-Indikator kleben. */}
        <div className="border-t border-gray-100 px-3 py-3 pb-[calc(0.75rem+env(safe-area-inset-bottom))] sm:px-5 sm:pb-3">
          <FormErrorAlert message={sendError} className="mb-3" />
          <QuickActions
            features={care.features}
            loading={care.loading}
            childName={thread?.student_name}
            onPick={(key) => setActiveModal(key)}
          />
          {/* Only show the free-text composer when the school has parent notes
              enabled AND this relationship grants notes.write (both folded into
              care.features.notes_enabled). A pickup-only/emergency guardian, or
              a school with messaging off, gets a read-only history instead of a
              composer that always 403s on send.

              Gate on care.loading FIRST: features start at DEFAULT_FEATURES
              (notes_enabled = false) until getChildFeatures resolves, so without
              this guard a messaging-enabled school would flash the read-only
              fallback on first paint before flipping to the composer.

              Without the conversation nobody knows who would read a message
              (#5200): no composer then; a failed load above offers the retry. */}
          {care.loading || (loading && !thread) ? (
            <ConversationComposerSkeleton />
          ) : !care.features.notes_enabled ? (
            <p className="rounded-xl bg-gray-50 px-4 py-3 text-[15px] text-gray-600">
              {tm("writingDisabled")}
            </p>
          ) : !thread ? null : leadershipClosed ? (
            <p className="rounded-xl bg-gray-50 px-4 py-3 text-[15px] text-gray-600">
              {leadershipClosedText}
            </p>
          ) : (
            <>
              {/* Wer mitliest, steht auch direkt am Schreibfeld: dort fällt
                  die Entscheidung, was man schreibt (#5200). Die volle Liste
                  steht im Kopf; hier bleibt es eine Zeile. */}
              {readersLine ? (
                <p
                  className="mb-2 truncate text-xs text-gray-500"
                  title={readersLine}
                >
                  {readersLine}
                </p>
              ) : null}
              <MessageComposer
                value={draft}
                onChange={setDraft}
                onSend={() => void handleSend()}
                sending={sending}
                placeholder={tm(
                  leadershipView
                    ? "composerPlaceholderLeadership"
                    : "composerPlaceholder",
                )}
                tone="parent"
                sendLabel={tm("send")}
                sendingLabel={tm("sending")}
                fieldLabel={tm("composerLabel")}
              />
            </>
          )}
        </div>
      </section>

      {/* Structured actions use their own status and request flows, so they do
          not create cards in the conversation. The child-care hook updates its
          own local state, so no conversation refetch is needed. */}
      {activeModal === "sick" &&
        (care.features.sick_note_enabled ||
          (care.features.excused_note_enabled ??
            care.features.sick_note_enabled)) && (
          <SickNoteModal
            studentId={studentId}
            onClose={() => setActiveModal(null)}
            onSubmit={async (dates, reason, status, recipientIds) => {
              return care.reportSick(dates, reason, status, recipientIds);
            }}
            sickEnabled={care.features.sick_note_enabled}
            excusedEnabled={
              care.features.excused_note_enabled ??
              care.features.sick_note_enabled
            }
            sickRequiresApproval={care.features.sick_requires_approval}
            excusedRequiresApproval={care.features.excused_requires_approval}
          />
        )}
      {activeModal === "pickup" && (
        <PickupTimeModal
          studentId={studentId}
          careExceptions={care.careExceptions}
          pickupChangeRequests={care.pickupChangeRequests}
          careExceptionsLoaded={care.careExceptionsLoaded}
          pickupChangeRequestsLoaded={care.pickupChangeRequestsLoaded}
          pickupChangeEnabled={care.features.pickup_change_enabled}
          departureModeEnabled={
            care.features.pickup_change_mode_enabled === true
          }
          childFirstName={thread?.student_name?.split(/\s+/)[0]}
          today={today}
          onClose={() => setActiveModal(null)}
          onSubmit={async (params) => {
            await care.saveCareException(params);
          }}
          onRemove={async (date) => {
            await care.removeCareException(date);
          }}
          cutoffTime={care.features.pickup_change_cutoff_time}
          todayClosed={care.features.pickup_change_today_closed}
          onCutoffPassed={care.refresh}
        />
      )}
    </div>
  );
}

// Historical change-request card. Since #1803 the chat no longer CREATES
// requests (the Stammdaten page owns that) — but past kind="request" rows still
// arrive in the timeline and must render read-only: title + status + timestamp,
// no diff (messages no longer carry one) and no withdraw button (withdrawal now
// happens on the Stammdaten page). New request activity shows up as
// non-interactive "event" pills instead.
function RequestItem({
  message,
  locale,
}: Readonly<{ message: ParentMessage; locale: string }>) {
  const t = useTranslations("parentOgsMessaging");
  return (
    <div className="moto-content-surface rounded-2xl border p-4 shadow-sm">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-sm font-semibold text-gray-900">
            {t(parentRequestTypeI18nKey(message.request_type))}
          </p>
          <p className="mt-1 text-xs text-gray-500">
            {formatChatTime(message.created_at, locale)}
            {message.read_by_staff ? `, ${t("readByStaff")}` : ""}
          </p>
        </div>
        <RequestStatusBadge
          label={t(parentRequestStatusI18nKey(message.request_status))}
        />
      </div>
    </div>
  );
}

// Quick actions above the composer. The structured actions previously hid behind
// an unlabelled "+", so parents rarely discovered them and typed the same thing
// as free text instead, which silently loses the auto-apply.
//
// These structured actions are available next to the conversation, while
// permanent care schedule and master data requests remain on the Stammdaten
// page. Feature flags still gate the pills.
//
// Pills render only once the per-child feature flags have loaded: the pickup
// flag arrives async (defaults off), so rendering early made "Abholung" pop in a
// beat after the others. While loading we reserve one pill-row of height so the
// whole set appears at once without shifting the composer.
//
// While the phone keyboard is open the row steps aside: the chat is then only a
// few hundred pixels tall and the line being typed has priority (#3664).
function QuickActions({
  features,
  loading,
  childName,
  onPick,
}: Readonly<{
  features: ChildFeatures;
  loading: boolean;
  childName?: string;
  onPick: (key: OgsActionKey) => void;
}>) {
  if (loading)
    return (
      <div
        className="mb-3 h-9 in-data-chat-keyboard:hidden"
        aria-hidden="true"
      />
    );
  const actions = getOgsActions(features).filter((action) => action.enabled);
  if (actions.length === 0) return null;
  return (
    <div className="mb-3 flex flex-wrap gap-2 in-data-chat-keyboard:hidden">
      {actions.map((action) => (
        <QuickActionPill
          key={action.key}
          action={action}
          childName={childName}
          withMode={features.pickup_change_mode_enabled === true}
          onPick={onPick}
        />
      ))}
    </div>
  );
}

function QuickActionPill({
  action,
  childName,
  withMode,
  onPick,
}: Readonly<{
  action: ReturnType<typeof getOgsActions>[number];
  childName?: string;
  /** Die Abholung darf auch die Abholart ändern (#3945). */
  withMode: boolean;
  onPick: (key: OgsActionKey) => void;
}>) {
  const careT = useTranslations("parentChildCare");
  const todayT = useTranslations("parentToday");
  const firstName = childName?.trim().split(/\s+/)[0];
  const label = firstName
    ? action.key === "sick"
      ? todayT("actions.sick", { name: firstName })
      : todayT(withMode ? "actions.pickupWithMode" : "actions.pickup", {
          name: firstName,
        })
    : careT(`actions.${action.key}.shortLabel`);
  return (
    <button
      type="button"
      onClick={() => onPick(action.key)}
      title={careT(`actions.${action.key}.hint`)}
      className="inline-flex min-h-9 shrink-0 items-center gap-1.5 rounded-full border border-gray-200 bg-white px-3 py-2 text-sm font-medium text-gray-700 transition-colors hover:border-gray-300 hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none active:bg-gray-100 sm:px-3.5"
    >
      {action.key === "sick" ? (
        <MotoDuotoneIcon icon={XIcon} tone="red" size={20} weight="bold" />
      ) : (
        <MotoConceptIcon
          concept={action.concept}
          size={20}
          aria-hidden="true"
        />
      )}
      <span className="sm:hidden">
        {todayT(
          action.key === "sick"
            ? "actions.sickCompact"
            : withMode
              ? "actions.pickupCompactWithMode"
              : "actions.pickupCompact",
        )}
      </span>
      <span className="hidden sm:inline">{label}</span>
    </button>
  );
}

// Eine Antwort auf eine Gruppennachricht (#4784) traegt den Verweis auf die
// Gruppennachricht, damit klar ist, worauf sie sich bezieht.
function GroupThreadReference({
  groupThreadId,
  title,
}: Readonly<{ groupThreadId: string; title?: string }>) {
  const t = useTranslations("parentGroupThreads");
  return (
    <div className="mb-1 flex justify-end">
      <NavigationLink
        href={parentPath(`/parents/messages/group/${groupThreadId}`)}
        className="text-moto-blue max-w-[85%] truncate text-xs font-medium underline-offset-2 hover:underline"
      >
        {t("answerTo", { title: title ?? "" })}
      </NavigationLink>
    </div>
  );
}

type ParentMessagesT = ReturnType<typeof useTranslations<"parentMessages">>;

// The label of one side of the switch, with the unread count of the
// conversation not on screen.
function audienceLabel(
  tm: ParentMessagesT,
  value: MessageAudience,
  summary: ThreadSummary | undefined,
  current: MessageAudience,
): string {
  const unread = value === current ? 0 : (summary?.unread ?? 0);
  const key = value === "team" ? "audienceTeam" : "audienceLeadership";
  return unread > 0 ? tm(`${key}Unread`, { count: unread }) : tm(key);
}

function EmptyThread({
  description,
}: Readonly<{ description: string | null }>) {
  const tm = useTranslations("parentMessages");
  return (
    <div className="flex h-full flex-col items-center justify-center py-10 text-center">
      <h2 className="text-sm font-semibold text-gray-900">
        {tm("emptyThreadTitle")}
      </h2>
      {description ? (
        <p className="mt-1 text-sm leading-6 text-gray-600">{description}</p>
      ) : null}
    </div>
  );
}

function ThreadSkeleton() {
  return (
    <div className="space-y-3">
      <Skeleton className="h-12 w-2/3 rounded-2xl bg-gray-100" />
      <Skeleton className="ml-auto h-12 w-1/2 rounded-2xl bg-gray-100" />
      <Skeleton className="h-12 w-3/5 rounded-2xl bg-gray-100" />
    </div>
  );
}

function ConversationComposerSkeleton() {
  return (
    <div
      data-testid="parent-conversation-composer-skeleton"
      className="moto-content-surface rounded-xl border p-3 shadow-sm"
      aria-hidden="true"
    >
      <Skeleton className="h-16 w-full rounded-lg" />
      <div className="mt-3 flex justify-end">
        <Skeleton className="h-9 w-24 rounded-lg" />
      </div>
    </div>
  );
}

// Rückweg zum Posteingang der Elternnachrichten, auf jeder Breite. Er sitzt
// in der Kopfkarte über dem Titel, wie auf jeder anderen Unterseite.
function BackBar() {
  const tm = useTranslations("parentMessages");
  return (
    <BackLink
      href={parentPath("/parents/messages")}
      label={tm("back")}
      className="mb-2"
    />
  );
}
