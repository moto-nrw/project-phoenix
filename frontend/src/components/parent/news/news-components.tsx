"use client";

/**
 * Shared parent-news building blocks (#1669): the compact feed card and the
 * detail modal. Used by the dashboard Elternbriefe panel (latest few) and the
 * dedicated /parents/news page (full feed). Opening the detail marks the
 * announcement read (the read stat parents feed the OGS). Acknowledging stays
 * an explicit button.
 */

import { useCallback, useEffect, useRef, useState } from "react";
import { Check, ChevronRight, ExternalLink } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";

import { Modal } from "~/components/ui/modal";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { Button } from "~/components/ui/button";
import { LinkifiedText } from "~/components/ui/linkified-text";
import { Alert } from "~/components/ui/alert";
import { Checkbox } from "~/components/ui/checkbox";
import { ConceptIconTile } from "~/components/ui/concept-icon-tile";
import { Radio } from "~/components/ui/radio";
import { StatusBadge } from "~/components/ui/status-badge";
import { formatBerlinDate, formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  ParentApiError,
  type ParentAnnouncement,
  type ParentAnnouncementPollChild,
  acknowledgeAnnouncement,
  markAnnouncementRead,
  respondToAnnouncement,
} from "~/lib/parent-api";

const logger = createLogger({ component: "ParentNews" });

/** Tell the sidebar badge to refetch after a read/ack. */
function refreshUnreadBadge() {
  window.dispatchEvent(new Event("parent-news-unread-refresh"));
}

/**
 * The backend rejects a read/ack that targets an announcement it no longer
 * serves as current: 404 (retracted / expired / audience change) or 409 (the
 * loaded published_at is stale after an unpublish -> edit -> republish
 * correction). Both mean the wording on screen may be outdated.
 */
function isStaleAnnouncementError(err: unknown): boolean {
  return (
    err instanceof ParentApiError && (err.status === 404 || err.status === 409)
  );
}

/** True when the announcement asks a question the parent can still answer. */
function isPoll(item: ParentAnnouncement): boolean {
  return item.response_type !== "none";
}

function isPollClosed(item: ParentAnnouncement): boolean {
  return (
    item.response_deadline !== undefined &&
    new Date(item.response_deadline) <= new Date()
  );
}

/**
 * True when this is a survey that is still collecting answers AND at least one
 * of the guardian's children has none yet. The "you still owe the school an
 * answer" state the dashboard sorts on.
 */
export function isOpenPoll(item: ParentAnnouncement): boolean {
  if (!isPoll(item) || isPollClosed(item)) return false;
  return (item.children ?? []).some(
    (child) => child.selected_options.length === 0,
  );
}

/** One shared definition for the list hierarchy and the sidebar count. */
export function isOutstandingAnnouncement(item: ParentAnnouncement): boolean {
  return (
    !item.read ||
    (item.requires_acknowledgement && !item.acknowledged) ||
    isOpenPoll(item)
  );
}

function pollAnswerSummary(
  item: ParentAnnouncement,
  child: ParentAnnouncementPollChild,
  format: (name: string, answer: string) => string,
): string | null {
  const optionLabels = new Map(
    (item.options ?? []).map((option) => [option.id, option.label]),
  );
  const answer = child.selected_options
    .map((optionId) => optionLabels.get(optionId))
    .filter((label): label is string => label !== undefined)
    .join(", ");
  if (!answer) return null;
  return format(child.first_name, answer);
}

function NewsCardMeta({
  item,
}: Readonly<{ item: ParentAnnouncement }>): React.ReactNode {
  const t = useTranslations("parentDashboard");
  const type = isPoll(item) ? t("newsPoll") : t("newsLetter");
  const outstanding = isOutstandingAnnouncement(item);
  return (
    <span className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs font-semibold tracking-wide uppercase">
      <span className={outstanding ? "text-moto-blue-strong" : "text-gray-500"}>
        {type}
      </span>
      {isBindingLetter(item) && !item.acknowledged && (
        <>
          <span className="text-gray-300" aria-hidden="true">
            ·
          </span>
          <span className="text-[#9A4F00]">{t("newsLetterBadge")}</span>
        </>
      )}
      {item.priority === "important" && (
        <>
          <span className="text-gray-300" aria-hidden="true">
            ·
          </span>
          <span className="text-[#9A4F00]">{t("newsImportant")}</span>
        </>
      )}
    </span>
  );
}

/**
 * A binding Elternbrief (#2384): the same text also arrived by e-mail, and the
 * confirmation in the portal is the one that counts. Older announcements have no
 * delivery_mode at all, which correctly reads as "not a letter".
 */
function isBindingLetter(item: ParentAnnouncement): boolean {
  return item.delivery_mode === "letter";
}

function NewsCardState({
  item,
}: Readonly<{ item: ParentAnnouncement }>): React.ReactNode {
  const t = useTranslations("parentDashboard");
  const locale = useLocale();

  if (!isPoll(item)) {
    const complete = item.requires_acknowledgement
      ? item.acknowledged
      : item.read;
    const label = item.requires_acknowledgement
      ? item.acknowledged
        ? t("newsAcknowledged")
        : t("newsReadAndConfirm")
      : item.read
        ? t("newsRead")
        : t("newsReadAnnouncement");
    return (
      <span
        className={`flex items-center gap-1.5 text-sm font-semibold ${complete ? "text-moto-green-strong" : "text-gray-900"}`}
      >
        {complete && (
          <Check
            className="text-moto-green-strong h-4 w-4 shrink-0"
            aria-hidden="true"
          />
        )}
        {label}
      </span>
    );
  }

  const children = item.children ?? [];
  const answered = children.filter(
    (child) => child.selected_options.length > 0,
  );
  const answerSummaries = answered
    .map((child) =>
      pollAnswerSummary(item, child, (name, answer) =>
        children.length === 1
          ? t("newsPollAnswerSummary", { answer })
          : t("newsPollChildAnswer", { name, answer }),
      ),
    )
    .filter((summary): summary is string => summary !== null);
  const closed = isPollClosed(item);
  const complete = children.length > 0 && answered.length === children.length;
  const action = closed
    ? answered.length > 0
      ? t("newsPollClosed")
      : t("newsPollNoAnswer")
    : complete
      ? t("newsPollDone")
      : answered.length > 0
        ? t("newsPollContinue")
        : t("newsAnswer");

  return (
    <span className="flex flex-wrap items-center gap-x-3 gap-y-1 text-sm">
      <span
        className={`flex items-center gap-1.5 font-semibold ${complete || (closed && answered.length > 0) ? "text-moto-green-strong" : "text-gray-900"}`}
      >
        {(complete || closed) && answered.length > 0 && (
          <Check
            className="text-moto-green-strong h-4 w-4 shrink-0"
            aria-hidden="true"
          />
        )}
        {action}
      </span>
      {!complete && answered.length > 0 && (
        <span className="text-gray-500">
          {t("newsPollAnsweredCount", {
            answered: answered.length,
            total: children.length,
          })}
        </span>
      )}
      {answerSummaries.length > 0 && (
        <span className="text-gray-600">{answerSummaries.join(" · ")}</span>
      )}
      {item.response_deadline && !closed && (
        <span className="text-gray-500">
          {t("newsPollDeadline", {
            date: formatBerlinDate(item.response_deadline, locale),
          })}
        </span>
      )}
    </span>
  );
}

/**
 * The answer state of an Umfrage, lifted out of the rendering so the modal can
 * put "Antwort speichern" in its footer bar, where every other modal in the
 * parent portal puts its primary action (#1371).
 *
 * Selecting is local; nothing is sent until the guardian saves. A tap that
 * writes straight through reads as a slip waiting to happen. The answer is a
 * commitment the school plans with, so it gets a deliberate confirmation.
 */
function usePollAnswers(
  item: ParentAnnouncement,
  onUpdated: (id: string, patch: Partial<ParentAnnouncement>) => void,
  onStale?: (id: string) => void,
) {
  const t = useTranslations("parentDashboard");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const children = item.children ?? [];
  const closed = isPollClosed(item);
  const multi = item.response_type === "multi_choice";

  // What is selected on screen, per child. A corrected poll is a new immutable
  // version, so reset drafts for a changed version/options. Ordinary response
  // updates do not reset them: while a multi-child save is in progress, one
  // child may have persisted while another still needs a retry.
  const pollVersionSignature = [
    item.id,
    item.published_at ?? "",
    item.response_type,
    ...(item.options ?? []).map((option) => `${option.id}:${option.label}`),
  ].join("|");
  const [draft, setDraft] = useState<Record<string, string[]>>(() =>
    Object.fromEntries(
      children.map((c) => [c.student_id, [...c.selected_options]]),
    ),
  );
  useEffect(() => {
    setDraft(
      Object.fromEntries(
        children.map((c) => [c.student_id, [...c.selected_options]]),
      ),
    );
    setError(null);
    // The version includes every option id and label, rather than the children:
    // response updates must preserve drafts for unresolved children.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pollVersionSignature]);

  const selectionFor = (child: ParentAnnouncementPollChild): string[] =>
    draft[child.student_id] ?? [];

  const toggle = (child: ParentAnnouncementPollChild, optionId: string) => {
    if (closed) return;
    const selected = selectionFor(child);
    const next = multi
      ? selected.includes(optionId)
        ? selected.filter((id) => id !== optionId)
        : [...selected, optionId]
      : selected.includes(optionId)
        ? [] // tapping the chosen option again clears it
        : [optionId];
    setDraft((prev) => ({ ...prev, [child.student_id]: next }));
  };

  const isDirty = (child: ParentAnnouncementPollChild): boolean => {
    const saved = [...child.selected_options]
      .sort((a, b) => a.localeCompare(b))
      .join(",");
    const current = [...selectionFor(child)]
      .sort((a, b) => a.localeCompare(b))
      .join(",");
    return saved !== current;
  };
  const dirtyChildren = children.filter(isDirty);

  const save = async (): Promise<boolean> => {
    if (!item.published_at || dirtyChildren.length === 0) return false;
    setSaving(true);
    setError(null);
    const savedSelections = new Map<string, string[]>();
    try {
      for (const child of dirtyChildren) {
        await respondToAnnouncement(
          item.id,
          child.student_id,
          selectionFor(child),
          item.published_at,
        );
        savedSelections.set(child.student_id, selectionFor(child));
        // Responses are persisted one child at a time. Reconcile each success
        // immediately, so a later failure leaves the parent with the saved
        // responses shown as saved and only the remaining child to retry.
        onUpdated(item.id, {
          children: children.map((candidate) => ({
            ...candidate,
            selected_options:
              savedSelections.get(candidate.student_id) ??
              candidate.selected_options,
          })),
        });
      }
      refreshUnreadBadge();
      return true;
    } catch (err: unknown) {
      logger.error("parent_news_poll_answer_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      if (isStaleAnnouncementError(err)) {
        onStale?.(item.id);
      }
      setError(t("newsActionError"));
      return false;
    } finally {
      setSaving(false);
    }
  };

  return {
    children,
    options: item.options ?? [],
    closed,
    multi,
    saving,
    error,
    selectionFor,
    toggle,
    isDirty,
    canSave: dirtyChildren.length > 0,
    save,
  };
}

/** One card per child: name, saved state, and the answer options. */
function PollAnswerRows({
  poll,
}: Readonly<{ poll: ReturnType<typeof usePollAnswers> }>) {
  const t = useTranslations("parentDashboard");
  const { children, options, closed, multi, saving } = poll;
  if (children.length === 0 || options.length === 0) return null;

  return (
    <div className="space-y-4">
      {children.map((child) => {
        const selected = poll.selectionFor(child);
        const answered = child.selected_options.length > 0;
        const dirty = poll.isDirty(child);
        const status = dirty
          ? { label: t("newsPollUnsaved"), tone: "orange" as const }
          : answered
            ? { label: t("newsPollAnswered"), tone: "green" as const }
            : { label: t("newsPollOpen"), tone: "gray" as const };
        return (
          <fieldset
            key={child.student_id}
            disabled={closed || saving}
            className="rounded-2xl border border-gray-200 bg-white p-4 shadow-sm disabled:opacity-60"
          >
            <legend className="mb-3 w-full">
              <span className="flex items-center justify-between gap-3">
                <span className="min-w-0 truncate text-base font-semibold text-gray-900">
                  {child.first_name} {child.last_name}
                </span>
                <StatusBadge label={status.label} tone={status.tone} />
              </span>
            </legend>
            <div className="space-y-2">
              {options.map((option) => {
                const active = selected.includes(option.id);
                return (
                  <label
                    key={option.id}
                    className={`flex min-h-12 cursor-pointer items-center gap-3 rounded-xl border px-4 py-3 text-base font-medium transition-colors has-[:disabled]:cursor-not-allowed ${
                      active
                        ? "border-gray-400 bg-gray-50 text-gray-950"
                        : "border-gray-200 bg-white text-gray-700 hover:border-gray-300 hover:bg-gray-50"
                    }`}
                  >
                    {multi ? (
                      <Checkbox
                        checked={active}
                        onChange={() => poll.toggle(child, option.id)}
                      />
                    ) : (
                      <Radio
                        name={`poll-${itemSafeId(child.student_id)}`}
                        checked={active}
                        onChange={() => poll.toggle(child, option.id)}
                      />
                    )}
                    <span>{option.label}</span>
                  </label>
                );
              })}
            </div>
            {!multi && selected.length > 0 && !closed && (
              <button
                type="button"
                disabled={saving}
                onClick={() => poll.toggle(child, selected[0]!)}
                className="mt-2 min-h-11 rounded-lg px-2 text-sm font-semibold text-gray-600 underline decoration-gray-300 underline-offset-4 transition-colors hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
              >
                {t("newsPollClear")}
              </button>
            )}
          </fieldset>
        );
      })}
    </div>
  );
}

function itemSafeId(value: string): string {
  return value.replace(/[^a-zA-Z0-9_-]/g, "-");
}

/**
 * Compact feed card: type, title, preview and completion state.
 *
 * A poll is NOT answered here. The card only flags that an answer is due and
 * opens the detail view. Answering in a list, next to four other items, is how
 * you tap the wrong card; the detail view shows the full question first.
 */
export function NewsCard({
  item,
  onOpen,
}: Readonly<{
  item: ParentAnnouncement;
  onOpen: (item: ParentAnnouncement) => void;
}>) {
  const t = useTranslations("parentDashboard");
  const locale = useLocale();
  const outstanding = isOutstandingAnnouncement(item);
  const concept = isPoll(item)
    ? "polls"
    : item.requires_acknowledgement
      ? "confirmations"
      : "news";

  return (
    <button
      type="button"
      onClick={() => onOpen(item)}
      className={`focus-visible:ring-moto-blue flex min-h-12 w-full items-center gap-3 rounded-xl border p-4 text-left shadow-sm transition-colors focus-visible:ring-2 focus-visible:outline-none active:bg-gray-100 ${
        outstanding
          ? "border-moto-blue/50 bg-moto-blue-soft hover:border-moto-blue/70 hover:bg-moto-blue/10"
          : "border-gray-200 bg-white hover:border-gray-300 hover:bg-gray-50"
      }`}
    >
      <span className="relative flex size-10 shrink-0 items-center justify-center rounded-xl bg-gray-100">
        <MotoConceptIcon concept={concept} tone="blue" size={22} />
        {outstanding && (
          <>
            <span
              className="bg-moto-blue absolute top-0.5 right-0.5 size-2 rounded-full ring-2 ring-white"
              aria-hidden="true"
            />
            <span className="sr-only">{t("newsOutstanding")}</span>
          </>
        )}
      </span>
      <span className="min-w-0 flex-1">
        <NewsCardMeta item={item} />
        <span
          className={`mt-1 block truncate text-sm text-gray-900 ${outstanding ? "font-semibold" : "font-medium"}`}
        >
          {item.title}
        </span>
        <span className="mt-0.5 block text-xs text-gray-500">
          {item.school_name}
          {item.published_at
            ? ` · ${formatDate(item.published_at, false, locale)}`
            : ""}
        </span>
        <span className="mt-1.5 line-clamp-2 block text-sm leading-5 text-gray-600">
          {item.body}
        </span>
        <span className="mt-2 block">
          <NewsCardState item={item} />
        </span>
      </span>
      <ChevronRight
        className="h-5 w-5 shrink-0 text-gray-400"
        aria-hidden="true"
      />
    </button>
  );
}

function NewsMessageSection({
  item,
}: Readonly<{ item: ParentAnnouncement }>): React.ReactNode {
  const t = useTranslations("parentDashboard");
  const locale = useLocale();
  const headingId = `news-message-${item.id}`;

  return (
    <section
      aria-labelledby={headingId}
      className="rounded-2xl border border-gray-200 bg-white p-4 shadow-sm"
    >
      <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
        <h4 id={headingId} className="text-base font-semibold text-gray-950">
          {t("newsMessageFrom", { school: item.school_name })}
        </h4>
        {item.published_at && (
          <time className="text-sm text-gray-500">
            {formatDate(item.published_at, false, locale)}
          </time>
        )}
      </div>
      <p className="mt-4 text-base leading-7 whitespace-pre-line text-gray-800">
        <LinkifiedText text={item.body} />
      </p>

      {item.link_url && (
        <a
          href={item.link_url}
          target="_blank"
          rel="noopener noreferrer"
          className="focus-visible:ring-moto-blue mt-4 flex min-h-11 w-full items-center gap-3 border-t border-gray-100 pt-4 text-left text-gray-900 transition-colors hover:text-gray-600 focus-visible:ring-2 focus-visible:outline-none"
        >
          <ExternalLink
            className="text-moto-blue h-5 w-5 shrink-0"
            aria-hidden="true"
          />
          <span className="min-w-0 flex-1">
            <span className="block text-sm font-semibold">
              {t("newsExternalLink")}
            </span>
            <span className="mt-0.5 block truncate text-sm font-normal text-gray-500">
              {item.link_url}
            </span>
          </span>
        </a>
      )}
    </section>
  );
}

function NewsActionContext({
  item,
}: Readonly<{ item: ParentAnnouncement }>): React.ReactNode {
  const t = useTranslations("parentDashboard");
  const locale = useLocale();
  const poll = isPoll(item);

  if (poll) {
    const showDeadline = item.response_deadline !== undefined;
    const showMulti = item.response_type === "multi_choice";
    if (!showDeadline && !showMulti) return null;

    return (
      <div className="flex flex-wrap gap-x-4 gap-y-1 px-1 text-sm text-gray-600">
        {showMulti && <span>{t("newsPollMulti")}</span>}
        {showDeadline && (
          <span>
            {isPollClosed(item)
              ? t("newsPollClosed")
              : t("newsPollDeadline", {
                  date: formatBerlinDate(item.response_deadline!, locale),
                })}
          </span>
        )}
      </div>
    );
  }

  if (!item.requires_acknowledgement) return null;

  return (
    <div className="flex items-start gap-3 px-1">
      <ConceptIconTile concept="confirmations" variant="section" tone="blue" />
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <p className="text-sm font-semibold text-gray-900">
            {t("newsAcknowledgement")}
          </p>
          {item.acknowledged && (
            <StatusBadge label={t("newsAcknowledged")} tone="green" />
          )}
        </div>
        <p className="mt-0.5 text-sm leading-6 text-gray-600">
          {isBindingLetter(item)
            ? t("newsLetterBindingHint")
            : t("newsAcknowledgementHint")}
        </p>
      </div>
    </div>
  );
}

/**
 * Detail view: full text (URLs clickable), optional link, explicit acknowledge.
 * Opening it marks the announcement read and nudges the sidebar badge.
 */
export function NewsDetailModal({
  item,
  onClose,
  onUpdated,
  onStale,
}: Readonly<{
  item: ParentAnnouncement;
  onClose: () => void;
  onUpdated: (id: string, patch: Partial<ParentAnnouncement>) => void;
  /**
   * Invoked when the backend rejects a read/ack because the announcement is no
   * longer current (retracted/expired or corrected since it loaded). The parent
   * should refetch the feed so the list + unread badge stop showing stale items.
   */
  onStale?: (id: string) => void;
}>) {
  const t = useTranslations("parentDashboard");
  const [busy, setBusy] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [stale, setStale] = useState(false);
  const markedRef = useRef(false);
  const poll = usePollAnswers(item, onUpdated, onStale);

  // Reset the per-version local flags when the id or published_at (the version
  // token) changes. On the stale-correction path the parent refetches the feed
  // and passes the refreshed announcement with its new published_at back into
  // this same modal instance; without this the stale warning would linger and the
  // acknowledge button stay hidden (and the read mark suppressed via markedRef)
  // for the now-current announcement. Declared before the mark-read effect below
  // so markedRef is cleared before that effect re-runs. A no-op on first mount.
  useEffect(() => {
    setStale(false);
    setActionError(null);
    markedRef.current = false;
  }, [item.id, item.published_at]);

  useEffect(() => {
    // published_at is the version the backend verifies; feed items always carry
    // it, so a missing value just means there is nothing to mark yet.
    if (item.read || markedRef.current || !item.published_at) return;
    markedRef.current = true;
    markAnnouncementRead(item.id, item.published_at)
      .then(() => {
        onUpdated(item.id, { read: true });
        refreshUnreadBadge();
      })
      .catch((err: unknown) => {
        logger.error("parent_news_mark_read_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (isStaleAnnouncementError(err)) {
          // The announcement was retracted/republished after it loaded: the
          // shown wording may be outdated. Flag it and let the parent refetch.
          setStale(true);
          refreshUnreadBadge();
          onStale?.(item.id);
        } else {
          // Transient failure: let a later rerender retry the mark.
          markedRef.current = false;
        }
      });
  }, [item.id, item.read, item.published_at, onUpdated, onStale]);

  const handleAcknowledge = useCallback(async () => {
    if (!item.published_at) return;
    setBusy(true);
    setActionError(null);
    try {
      await acknowledgeAnnouncement(item.id, item.published_at);
      onUpdated(item.id, { read: true, acknowledged: true });
      refreshUnreadBadge();
      // Done. Like every other parent-portal modal, a successful write closes
      // the dialog and hands the confirmation back to the list behind it.
      onClose();
    } catch (err: unknown) {
      logger.error("parent_news_acknowledge_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      if (isStaleAnnouncementError(err)) {
        setStale(true);
        refreshUnreadBadge();
        onStale?.(item.id);
      } else {
        setActionError(t("newsActionError"));
      }
    } finally {
      setBusy(false);
    }
  }, [item.id, item.published_at, onUpdated, onStale, onClose, t]);

  // A stale announcement can't be acknowledged (the backend rejects the write),
  // so hide the button and surface the stale banner instead.
  const needsAck =
    item.requires_acknowledgement && !item.acknowledged && !stale;
  const actionInProgress = busy || poll.saving;

  return (
    <Modal
      isOpen
      onClose={onClose}
      title={item.title}
      closeLabel={t("newsClose")}
      backdropLabel={t("newsClose")}
      isDismissDisabled={actionInProgress}
      mobileSheet
      footer={
        <>
          {isPoll(item) && !poll.closed && !stale && (
            <Button
              type="button"
              size="md"
              className="order-1 sm:order-2"
              onClick={() => {
                void poll.save().then((saved) => {
                  if (saved) onClose();
                });
              }}
              disabled={!poll.canSave || poll.saving}
              isLoading={poll.saving}
              loadingText={t("newsPollSaving")}
            >
              {t("newsPollSave")}
            </Button>
          )}
          {needsAck && (
            <Button
              type="button"
              size="md"
              className="order-1 gap-1.5 sm:order-2"
              onClick={() => void handleAcknowledge()}
              disabled={busy}
              isLoading={busy}
              loadingText={t("newsAcknowledgeSaving")}
            >
              <Check className="h-4 w-4" aria-hidden="true" />
              {t("newsReadAndConfirm")}
            </Button>
          )}
          <Button
            type="button"
            variant="outline"
            size="md"
            className="order-2 sm:order-1"
            onClick={onClose}
            disabled={actionInProgress}
          >
            {t("newsClose")}
          </Button>
        </>
      }
    >
      <div className="space-y-5">
        <NewsMessageSection item={item} />
        <NewsActionContext item={item} />

        {stale && <Alert type="warning" message={t("newsStaleError")} />}
        {actionError && <Alert type="error" message={actionError} />}
        {poll.error && <Alert type="error" message={poll.error} />}

        {isPoll(item) && <PollAnswerRows poll={poll} />}
      </div>
    </Modal>
  );
}
