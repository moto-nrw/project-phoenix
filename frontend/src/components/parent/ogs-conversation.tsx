"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { ArrowLeft, ChevronRight, Pencil } from "lucide-react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Skeleton } from "~/components/ui/skeleton";
import { MessageComposer } from "~/components/messaging/message-composer";
import { ChatBubble, ChatEventCard } from "~/components/messaging/chat-bubble";
import { RequestDiffPanel } from "~/components/messaging/request-diff-panel";
import { RequestStatusBadge } from "~/components/messaging/request-status-badge";
import { useChatViewportLock } from "~/lib/hooks/use-chat-viewport-lock";
import { getApiErrorMessage } from "~/components/ui/modal-utils";
import {
  PARENT_DEPARTURE_MODE_I18N_KEYS,
  PARENT_DIFF_CARE_KIND_I18N_KEYS,
  parentEventI18nDescriptor,
  parentRequestStatusI18nKey,
  parentRequestTypeI18nKey,
  type RequestDiffEntry,
} from "~/lib/messaging-status";
import {
  type ChildFeatures,
  type ParentMessage,
  type ThreadView,
  createChildRequest,
  getChildConversation,
  postChildMessage,
  withdrawChildRequest,
} from "~/lib/parent-api";
import {
  CareScheduleRequestModal,
  PickupTimeModal,
  RequestChooserModal,
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
  const [thread, setThread] = useState<ThreadView | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [draft, setDraft] = useState("");
  const care = useChildCare(studentId);
  const [activeModal, setActiveModal] = useState<OgsActionKey | null>(null);
  const [requestChooserOpen, setRequestChooserOpen] = useState(false);
  const [sending, setSending] = useState(false);
  const [sendError, setSendError] = useState<string | null>(null);
  const scrollRef = useRef<HTMLDivElement | null>(null);
  // Pin the chat to the viewport and lock page scroll (only the list scrolls).
  const containerRef = useChatViewportLock<HTMLDivElement>(!loading);

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
  useMessagesActivity({
    eventName: "parent-conversation-refresh",
    studentId,
    // refresh() marks the thread read server-side; nudge the badge AFTER it
    // resolves so the sidebar count drops the message this chat just read. The
    // bridge's own pre-read badge dispatch races ahead of this and would
    // otherwise leave the badge stale (see nudgeUnreadBadge).
    onMatch: () => void refresh().then(nudgeUnreadBadge),
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
  }, [thread]);

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
      setSendError(
        getApiErrorMessage(
          err,
          "senden",
          "Nachrichten",
          "Die Nachricht konnte nicht gesendet werden.",
        ),
      );
    } finally {
      setSending(false);
    }
  }, [draft, sending, studentId, applyThread]);

  // The request modals own their form state and error display; this just
  // performs the API call and updates the thread. Throwing propagates back to
  // the modal so it can surface the message and stay open.
  const submitRequest = useCallback(
    async (type: "care_schedule", payload: Record<string, unknown>) => {
      const view = await createChildRequest(studentId, type, payload);
      // Claim the token AFTER the request resolves so this authoritative result wins.
      applyThread(++applySeqRef.current, view);
      // Submitting a request advances the guardian read cursor server-side
      // (MarkReadToNewest), so any prior staff messages are now read — nudge the
      // sidebar badge so it drops them instead of going stale until an unrelated
      // refresh, matching handleSend.
      nudgeUnreadBadge();
    },
    [studentId, applyThread],
  );

  // Self-service actions (sick note, pickup change) emit a system event into
  // this thread server-side; refetch so the new event card appears at once
  // instead of waiting on the SSE round-trip.
  const refreshAfterSelfService = useCallback(() => {
    void refresh().then(nudgeUnreadBadge);
  }, [refresh]);

  const withdrawRequest = useCallback(
    async (requestId: string) => {
      setSendError(null);
      try {
        const view = await withdrawChildRequest(studentId, requestId);
        // Claim the token AFTER the request resolves so this authoritative result wins.
        applyThread(++applySeqRef.current, view);
        // Withdrawing also advances the read cursor server-side, so nudge the
        // badge like submitRequest/handleSend rather than leaving it stale.
        nudgeUnreadBadge();
      } catch (err) {
        if (!mountedRef.current) return;
        setSendError(
          getApiErrorMessage(
            err,
            "zurückziehen",
            "Anfrage",
            "Die Anfrage konnte nicht zurückgezogen werden.",
          ),
        );
      }
    },
    [studentId, applyThread],
  );

  return (
    <div
      ref={containerRef}
      className="mx-auto flex min-h-[20rem] w-full max-w-5xl flex-col gap-3 overflow-hidden"
    >
      {showBack ? <BackBar /> : null}

      <section className="flex min-h-0 flex-1 flex-col rounded-2xl border border-gray-200 bg-white shadow-sm">
        <div className="border-b border-gray-100 px-5 py-4 sm:px-6">
          <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
            Austausch mit der OGS
          </p>
          <h1 className="mt-0.5 text-lg font-semibold break-words text-gray-900">
            {counterpart}
          </h1>
          {showChild && thread?.student_name ? (
            <p className="text-sm text-gray-500">zu {thread.student_name}</p>
          ) : null}
        </div>

        <div
          ref={scrollRef}
          className="flex-1 space-y-3 overflow-y-auto px-5 py-4 sm:px-6"
        >
          {/* Keep already-loaded messages on screen even if a later background
              refresh (SSE / focus) fails transiently — only fall back to the
              error state when there is nothing to show. Otherwise a flaky
              refetch would wipe a visible conversation. */}
          {loading ? (
            <ThreadSkeleton />
          ) : messages.length > 0 ? (
            messages.map((message) => {
              const eventI18n =
                message.kind === "event"
                  ? parentEventI18nDescriptor(message)
                  : null;
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
                />
              ) : message.kind === "request" ? (
                <RequestItem
                  key={message.id}
                  message={message}
                  onWithdraw={() => void withdrawRequest(message.id)}
                />
              ) : (
                <ChatBubble
                  key={message.id}
                  body={message.body}
                  own={message.sender_kind === "guardian"}
                  senderName={message.sender_name}
                  createdAt={message.created_at}
                  readReceiptLabel={
                    message.sender_kind === "guardian" && message.read_by_staff
                      ? t("readByStaff")
                      : undefined
                  }
                />
              );
            })
          ) : loadError ? (
            <Alert
              type="error"
              message="Die Nachrichten konnten nicht geladen werden."
            />
          ) : (
            <EmptyThread />
          )}
        </div>

        <div className="border-t border-gray-100 px-5 py-4 sm:px-6">
          {sendError ? (
            <div className="mb-3">
              <Alert type="error" message={sendError} />
            </div>
          ) : null}
          <QuickActions
            features={care.features}
            loading={care.loading}
            onPick={(key) => setActiveModal(key)}
            onOpenRequests={() => setRequestChooserOpen(true)}
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
          {care.loading ? null : care.features.notes_enabled ? (
            <MessageComposer
              value={draft}
              onChange={setDraft}
              onSend={() => void handleSend()}
              sending={sending}
              placeholder="Nachricht an die OGS schreiben..."
            />
          ) : (
            <p className="rounded-lg bg-gray-50 px-4 py-3 text-sm text-gray-500">
              Das Schreiben von Nachrichten ist für dieses Kind nicht aktiviert.
              Sie können den Verlauf weiterhin lesen.
            </p>
          )}
        </div>
      </section>

      {requestChooserOpen && (
        <RequestChooserModal
          features={care.features}
          onClose={() => setRequestChooserOpen(false)}
          onPick={(key) => {
            setRequestChooserOpen(false);
            setActiveModal(key);
          }}
        />
      )}
      {activeModal === "sick" && (
        <SickNoteModal
          onClose={() => setActiveModal(null)}
          onSubmit={async (dates, reason, status) => {
            await care.reportSick(dates, reason, status);
            refreshAfterSelfService();
          }}
        />
      )}
      {activeModal === "pickup" && (
        <PickupTimeModal
          careExceptions={care.careExceptions}
          careExceptionsLoaded={care.careExceptionsLoaded}
          pickupChangeEnabled={care.features.pickup_change_enabled}
          onClose={() => setActiveModal(null)}
          onSubmit={async (params) => {
            await care.saveCareException(params);
            refreshAfterSelfService();
          }}
          onRemove={async (date) => {
            await care.removeCareException(date);
            refreshAfterSelfService();
          }}
        />
      )}
      {activeModal === "care_schedule" && (
        <CareScheduleRequestModal
          onClose={() => setActiveModal(null)}
          onSubmit={(payload) => submitRequest("care_schedule", payload)}
        />
      )}
    </div>
  );
}

function RequestItem({
  message,
  onWithdraw,
}: Readonly<{ message: ParentMessage; onWithdraw: () => void }>) {
  const t = useTranslations("parentOgsMessaging");
  // The backend builds each diff label as a German string ("Montag ·
  // Bringzeit"). Re-derive it from the entry's structured fields so it renders
  // in the guardian's language; fall back to the German label only when the
  // backend sent no structured discriminators (legacy payloads).
  const localizeDiffLabel = (entry: RequestDiffEntry): string => {
    const careKey = entry.care_kind
      ? PARENT_DIFF_CARE_KIND_I18N_KEYS[entry.care_kind]
      : undefined;
    if (entry.weekday && careKey) {
      return `${t(`diffWeekday${entry.weekday}`)} · ${t(careKey)}`;
    }
    return entry.label;
  };
  // The backend's old/new for a departure_mode row are German labels ("Geht
  // alleine", "Wird abgeholt"). Re-derive them from the raw mode keys so they
  // render in the guardian's language; fall back to the raw key only for a mode
  // we don't have a translation for. old_modes carries the allowed set ("alone"
  // standing in for an empty set), joined the same way the backend joins them.
  const localizeMode = (key: string): string => {
    const modeKey = PARENT_DEPARTURE_MODE_I18N_KEYS[key];
    return modeKey ? t(modeKey) : key;
  };
  const localizedDiff = message.diff?.map((entry) => {
    const localized = { ...entry, label: localizeDiffLabel(entry) };
    if (entry.care_kind !== "departure_mode") {
      return localized;
    }
    return {
      ...localized,
      old: entry.old_modes?.length
        ? entry.old_modes.map(localizeMode).join(" / ")
        : entry.old,
      new: entry.new_mode ? localizeMode(entry.new_mode) : entry.new,
    };
  });
  return (
    <div className="rounded-2xl border border-gray-200 bg-white p-4 shadow-sm">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-sm font-semibold text-gray-900">
            {t(parentRequestTypeI18nKey(message.request_type))}
          </p>
          <p className="mt-1 text-xs text-gray-500">
            {formatChatTime(message.created_at)}
            {message.read_by_staff ? ` · ${t("readByStaff")}` : ""}
          </p>
        </div>
        <RequestStatusBadge
          label={t(parentRequestStatusI18nKey(message.request_status))}
        />
      </div>
      <RequestDiffPanel diff={localizedDiff} heading={t("diffHeading")} />
      {message.request_status === "offen" ? (
        <Button
          type="button"
          variant="outline"
          size="md"
          className="mt-3"
          onClick={onWithdraw}
        >
          {t("requestWithdraw")}
        </Button>
      ) : null}
    </div>
  );
}

// Quick actions above the composer. The structured actions previously hid behind
// an unlabelled "+", so parents rarely discovered them and typed the same thing
// as free text instead, which silently loses the auto-apply.
//
// The two groups are deliberately NOT shown alike, because they are not alike:
//   - Immediate self-service (sick note, pickup-for-a-day) → white action pills.
//     Frequent, low-stakes, take effect at once. One tap, one thing.
//   - Change requests (care schedule, master data) → a single, equally large
//     "Dauerhafte Änderung" pill that opens a chooser. Rare, consequential,
//     confirmed by the OGS.
// The request pill is set apart by its wording alone plus a trailing chevron
// that reads as "opens options" rather than "does it now". The chooser then
// spells out "the OGS confirms before it takes effect" in full, where there is
// room, so it can never be misread. Feature flags still gate the pills.
//
// Pills render only once the per-child feature flags have loaded: the pickup
// flag arrives async (defaults off), so rendering early made "Abholung" pop in a
// beat after the others. While loading we reserve one pill-row of height so the
// whole set appears at once without shifting the composer.
function QuickActions({
  features,
  loading,
  onPick,
  onOpenRequests,
}: Readonly<{
  features: ChildFeatures;
  loading: boolean;
  onPick: (key: OgsActionKey) => void;
  onOpenRequests: () => void;
}>) {
  const t = useTranslations("parentChildCare");
  if (loading) return <div className="mb-3 h-9" aria-hidden="true" />;
  const actions = getOgsActions(features).filter((action) => action.enabled);
  const direct = actions.filter((action) => action.group === "direct");
  const hasRequests = actions.some((action) => action.group === "request");
  if (direct.length === 0 && !hasRequests) return null;
  return (
    <div className="mb-3 flex flex-wrap gap-2">
      {direct.map((action) => (
        <QuickActionPill key={action.key} action={action} onPick={onPick} />
      ))}
      {hasRequests ? (
        <button
          type="button"
          onClick={onOpenRequests}
          title={t("request.entryHint")}
          className="inline-flex min-h-9 shrink-0 items-center gap-1.5 rounded-full border border-gray-200 bg-white px-3.5 py-2 text-sm font-medium text-gray-700 transition-colors hover:border-gray-300 hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        >
          <Pencil className="h-4 w-4 text-gray-400" aria-hidden="true" />
          {t("request.entryLabel")}
          <ChevronRight
            className="-mr-1 h-4 w-4 text-gray-400"
            aria-hidden="true"
          />
        </button>
      ) : null}
    </div>
  );
}

function QuickActionPill({
  action,
  onPick,
}: Readonly<{
  action: ReturnType<typeof getOgsActions>[number];
  onPick: (key: OgsActionKey) => void;
}>) {
  const t = useTranslations("parentChildCare");
  return (
    <button
      type="button"
      onClick={() => onPick(action.key)}
      title={t(`actions.${action.key}.hint`)}
      className="inline-flex min-h-9 shrink-0 items-center gap-1.5 rounded-full border border-gray-200 bg-white px-3.5 py-2 text-sm font-medium text-gray-700 transition-colors hover:border-gray-300 hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
    >
      <action.Icon className="h-4 w-4 text-gray-400" aria-hidden="true" />
      {t(`actions.${action.key}.shortLabel`)}
    </button>
  );
}

function EmptyThread() {
  return (
    <div className="flex h-full flex-col items-center justify-center py-10 text-center">
      <h2 className="text-sm font-semibold text-gray-900">
        Noch keine Nachrichten
      </h2>
      <p className="mt-1 text-sm leading-6 text-gray-600">
        Schreiben Sie die erste Nachricht an die OGS.
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

// Always-visible parents-portal back link. The kit BackButton/MobileBackButton
// don't fit here: BackButton routes via useTenantRouter (the tenant portal) and
// both are mobile-only by design, whereas this affordance must work on desktop in
// the parents portal too. A plain Link to the portal-absolute path is the correct
// primitive; the kit has no desktop, portal-agnostic back component to reuse.
function BackBar() {
  return (
    <Link
      href="/parents/messages"
      className="inline-flex h-8 w-fit items-center gap-2 rounded-lg px-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-50 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
    >
      <ArrowLeft className="h-4 w-4" aria-hidden="true" />
      Zurück
    </Link>
  );
}
