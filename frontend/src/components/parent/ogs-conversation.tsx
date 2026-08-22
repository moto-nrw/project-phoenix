"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { XIcon } from "@phosphor-icons/react/ssr";
import { useLocale, useTranslations } from "next-intl";
import { ArrowLeft } from "lucide-react";
import { parentPath } from "~/lib/parent-url";
import { Alert } from "~/components/ui/alert";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { Skeleton } from "~/components/ui/skeleton";
import { MessageComposer } from "~/components/messaging/message-composer";
import { ChatBubble, ChatEventCard } from "~/components/messaging/chat-bubble";
import { RequestStatusBadge } from "~/components/messaging/request-status-badge";
import { useChatViewportLock } from "~/lib/hooks/use-chat-viewport-lock";
import { parentMessageError } from "~/lib/parent-message-error";
import {
  parentEventI18nDescriptor,
  parentRequestStatusI18nKey,
  parentRequestTypeI18nKey,
} from "~/lib/messaging-status";
import {
  type ChildFeatures,
  type ChildToday,
  type ParentMessage,
  type ThreadView,
  UNKNOWN_CHILD_TODAY,
  getChildConversation,
  getChildToday,
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
import { formatChatTime } from "~/lib/date-helpers";

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
}: {
  readonly studentId: string;
  readonly showBack?: boolean;
  readonly showChild?: boolean;
}) {
  const t = useTranslations("parentOgsMessaging");
  const tm = useTranslations("parentMessages");
  const locale = useLocale();
  const [thread, setThread] = useState<ThreadView | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [draft, setDraft] = useState("");
  const care = useChildCare(studentId);
  const [today, setToday] = useState<ChildToday>(UNKNOWN_CHILD_TODAY);
  const [activeModal, setActiveModal] = useState<OgsActionKey | null>(null);
  const [sending, setSending] = useState(false);
  const [sendError, setSendError] = useState<string | null>(null);
  const scrollRef = useRef<HTMLDivElement | null>(null);
  // Pin the chat to the viewport and lock page scroll (only the list scrolls).
  const containerRef = useChatViewportLock<HTMLDivElement>(!loading);

  useEffect(() => {
    if (activeModal !== "pickup") return;
    void getChildToday(studentId)
      .then(setToday)
      .catch(() => {
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
    try {
      const view = await getChildConversation(studentId);
      if (applyThread(seq, view)) setLoadError(null);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unknown error";
      logger.warn("ogs_conversation_load_failed", { error: message });
      if (mountedRef.current && seq === applySeqRef.current) {
        setLoadError(message);
      }
    }
  }, [studentId, applyThread]);

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

  // Keep the conversation scrolled to the newest message.
  useEffect(() => {
    const el = scrollRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [thread, care.loading]);

  const handleSend = useCallback(async () => {
    const body = draft.trim();
    if (!body || sending) return;
    setSending(true);
    setSendError(null);
    try {
      const view = await postChildMessage(studentId, body);
      // Claim the token AFTER the POST resolves so this authoritative result wins.
      // A successful send means the thread is loadable, so clear any stale
      // load error left by an earlier failed background refresh.
      if (applyThread(++applySeqRef.current, view)) setLoadError(null);
      setDraft("");
      // Sending advances the reader's own cursor (any prior staff messages are now
      // read), so refresh the sidebar badge.
      nudgeUnreadBadge();
    } catch (err) {
      logger.warn("ogs_message_send_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setSendError(parentMessageError(err, tm));
    } finally {
      setSending(false);
    }
  }, [draft, sending, studentId, applyThread, tm]);

  return (
    <div
      ref={containerRef}
      className="flex min-h-[20rem] w-full flex-col gap-3 overflow-hidden"
    >
      {showBack ? <BackBar /> : null}

      <section className="moto-content-surface flex min-h-0 flex-1 flex-col rounded-2xl border shadow-sm">
        <div className="border-b border-gray-100 px-4 py-3 sm:px-5">
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
                    ? tm("ogsTeam", { school: thread.school_name })
                    : counterpart}
                </h1>
              </div>
              {showChild && thread?.student_name ? (
                <div
                  className="bg-moto-green-soft inline-flex w-fit shrink-0 items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-xs font-semibold text-gray-900"
                  title={tm("aboutChild", { name: thread.student_name })}
                >
                  <MotoConceptIcon
                    concept="children"
                    size={16}
                    aria-hidden="true"
                  />
                  {thread.student_name}
                </div>
              ) : null}
            </div>
          )}
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
                <ChatBubble
                  key={message.id}
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
                  showSenderName={!genericStaffSender && !sameSenderAsPrevious}
                  tone="parent"
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
              );
            })
          ) : loadError ? (
            <Alert type="error" message={tm("loadError")} />
          ) : (
            <EmptyThread />
          )}
        </div>

        {/* Angeheftet am unteren Rand, mit Sicherheitsbereich des Geraets:
            auf dem Handy darf nichts unter dem Home-Indikator kleben. */}
        <div className="border-t border-gray-100 px-3 py-3 pb-[calc(0.75rem+env(safe-area-inset-bottom))] sm:px-5 sm:pb-3">
          {sendError ? (
            <div className="mb-3">
              <Alert type="error" message={sendError} />
            </div>
          ) : null}
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
              fallback on first paint before flipping to the composer. */}
          {care.loading ? (
            <ConversationComposerSkeleton />
          ) : care.features.notes_enabled ? (
            <MessageComposer
              value={draft}
              onChange={setDraft}
              onSend={() => void handleSend()}
              sending={sending}
              placeholder={tm("composerPlaceholder")}
              tone="parent"
              sendLabel={tm("send")}
              sendingLabel={tm("sending")}
              fieldLabel={tm("composerLabel")}
            />
          ) : (
            <p className="rounded-xl bg-gray-50 px-4 py-3 text-[15px] text-gray-600">
              {tm("writingDisabled")}
            </p>
          )}
        </div>
      </section>

      {/* Structured actions use their own status and request flows, so they do
          not create cards in the conversation. The child-care hook updates its
          own local state, so no conversation refetch is needed. */}
      {activeModal === "sick" && (
        <SickNoteModal
          onClose={() => setActiveModal(null)}
          onSubmit={async (dates, reason, status) => {
            return care.reportSick(dates, reason, status);
          }}
          sickRequiresApproval={care.features.sick_requires_approval}
          excusedRequiresApproval={care.features.excused_requires_approval}
        />
      )}
      {activeModal === "pickup" && (
        <PickupTimeModal
          careExceptions={care.careExceptions}
          pickupChangeRequests={care.pickupChangeRequests}
          careExceptionsLoaded={care.careExceptionsLoaded}
          pickupChangeRequestsLoaded={care.pickupChangeRequestsLoaded}
          pickupChangeEnabled={care.features.pickup_change_enabled}
          childFirstName={thread?.student_name?.split(/\s+/)[0]}
          today={today}
          onClose={() => setActiveModal(null)}
          onSubmit={async (params) => {
            await care.saveCareException(params);
          }}
          onRemove={async (date) => {
            await care.removeCareException(date);
          }}
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
    <div className="rounded-2xl border border-gray-200 bg-white p-4 shadow-sm">
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
  if (loading) return <div className="mb-3 h-9" aria-hidden="true" />;
  const actions = getOgsActions(features).filter((action) => action.enabled);
  if (actions.length === 0) return null;
  return (
    <div className="mb-3 flex flex-wrap gap-2">
      {actions.map((action) => (
        <QuickActionPill
          key={action.key}
          action={action}
          childName={childName}
          onPick={onPick}
        />
      ))}
    </div>
  );
}

function QuickActionPill({
  action,
  childName,
  onPick,
}: Readonly<{
  action: ReturnType<typeof getOgsActions>[number];
  childName?: string;
  onPick: (key: OgsActionKey) => void;
}>) {
  const careT = useTranslations("parentChildCare");
  const todayT = useTranslations("parentToday");
  const firstName = childName?.trim().split(/\s+/)[0];
  const label = firstName
    ? action.key === "sick"
      ? todayT("actions.sick", { name: firstName })
      : todayT("actions.pickup", { name: firstName })
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
            : "actions.pickupCompact",
        )}
      </span>
      <span className="hidden sm:inline">{label}</span>
    </button>
  );
}

function EmptyThread() {
  const tm = useTranslations("parentMessages");
  return (
    <div className="flex h-full flex-col items-center justify-center py-10 text-center">
      <h2 className="text-sm font-semibold text-gray-900">
        {tm("emptyThreadTitle")}
      </h2>
      <p className="mt-1 text-sm leading-6 text-gray-600">
        {tm("emptyThreadDescription")}
      </p>
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
      className="rounded-xl border border-gray-200 bg-white p-3"
      aria-hidden="true"
    >
      <Skeleton className="h-16 w-full rounded-lg" />
      <div className="mt-3 flex justify-end">
        <Skeleton className="h-9 w-24 rounded-lg" />
      </div>
    </div>
  );
}

// Always-visible parents-portal back link. The kit BackButton/MobileBackButton
// don't fit here: BackButton routes via useTenantRouter (the tenant portal) and
// both are mobile-only by design, whereas this affordance must work on desktop in
// the parents portal too. A plain Link to the portal-absolute path is the correct
// primitive; the kit has no desktop, portal-agnostic back component to reuse.
function BackBar() {
  const tm = useTranslations("parentMessages");
  return (
    <Link
      href={parentPath("/parents/messages")}
      className="inline-flex h-8 w-fit items-center gap-2 rounded-lg px-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-50 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
    >
      <ArrowLeft className="h-4 w-4" aria-hidden="true" />
      {tm("back")}
    </Link>
  );
}
