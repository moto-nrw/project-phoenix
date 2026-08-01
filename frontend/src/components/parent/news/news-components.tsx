"use client";

/**
 * Shared parent-news building blocks (#1669): the compact feed card and the
 * detail modal. Used by the dashboard "Neuigkeiten" panel (latest few) and the
 * dedicated /parents/news page (full feed). Opening the detail marks the
 * announcement read (the read stat parents feed the OGS) — acknowledging stays
 * an explicit button.
 */

import { useCallback, useEffect, useRef, useState } from "react";
import { Check, ChevronRight, ExternalLink, ListChecks } from "lucide-react";
import { useTranslations } from "next-intl";

import { Modal } from "~/components/ui/modal";
import { Button } from "~/components/ui/button";
import { LinkifiedText } from "~/components/ui/linkified-text";
import { formatDate } from "~/lib/date-helpers";
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
 * of the guardian's children has none yet — the "you still owe the school an
 * answer" state the dashboard sorts on.
 */
export function isOpenPoll(item: ParentAnnouncement): boolean {
  if (!isPoll(item) || isPollClosed(item)) return false;
  return (item.children ?? []).some(
    (child) => child.selected_options.length === 0,
  );
}

function NewsBadges({
  item,
}: Readonly<{ item: ParentAnnouncement }>): React.ReactNode {
  const t = useTranslations("parentDashboard");
  const poll = isPoll(item);
  return (
    <>
      {poll && (
        <span className="inline-flex items-center gap-1 rounded-full bg-[#5080D8]/10 px-2 py-0.5 text-xs font-semibold text-[#5080D8]">
          <ListChecks className="h-3 w-3" aria-hidden="true" />
          {t("newsPoll")}
        </span>
      )}
      {isOpenPoll(item) && (
        <span className="inline-flex items-center rounded-full bg-[#FEF4E7] px-2 py-0.5 text-xs font-semibold text-[#B45309]">
          {t("newsPollNeedsAnswer")}
        </span>
      )}
      {poll && item.response_deadline && !isPollClosed(item) && (
        <span className="inline-flex items-center rounded-full bg-gray-100 px-2 py-0.5 text-xs font-semibold text-gray-700">
          {t("newsPollDeadline", { date: formatDate(item.response_deadline) })}
        </span>
      )}
      {poll && isPollClosed(item) && (
        <span className="inline-flex items-center rounded-full bg-gray-100 px-2 py-0.5 text-xs font-semibold text-gray-500">
          {t("newsPollClosed")}
        </span>
      )}
      {!item.read && (
        <span className="inline-flex items-center rounded-full bg-gray-900 px-2 py-0.5 text-xs font-semibold text-white">
          {t("newsNew")}
        </span>
      )}
      {item.priority === "important" && (
        <span className="inline-flex items-center rounded-full bg-gray-100 px-2 py-0.5 text-xs font-semibold text-gray-700">
          {t("newsImportant")}
        </span>
      )}
      {item.acknowledged && (
        <span className="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2 py-0.5 text-xs font-semibold text-gray-600">
          <Check className="h-3 w-3" aria-hidden="true" />
          {t("newsAcknowledged")}
        </span>
      )}
    </>
  );
}

/**
 * The answer state of an Umfrage, lifted out of the rendering so the modal can
 * put "Antwort speichern" in its footer bar — where every other modal in the
 * parent portal puts its primary action (#1371).
 *
 * Selecting is local; nothing is sent until the guardian saves. A tap that
 * writes straight through reads as a slip waiting to happen — the answer is a
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
    const saved = [...child.selected_options].sort().join(",");
    const current = [...selectionFor(child)].sort().join(",");
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
    <div className="space-y-3">
      {multi && <p className="text-sm text-gray-600">{t("newsPollMulti")}</p>}
      {children.map((child) => {
        const selected = poll.selectionFor(child);
        const answered = child.selected_options.length > 0;
        const dirty = poll.isDirty(child);
        return (
          <div
            key={child.student_id}
            className="rounded-xl border border-gray-200 p-3"
          >
            <div className="mb-2 flex items-center justify-between gap-2">
              <p className="truncate text-sm font-semibold text-gray-900">
                {child.first_name}
              </p>
              <span
                className={`shrink-0 text-xs ${
                  dirty
                    ? "text-[#B45309]"
                    : answered
                      ? "text-[#4d7719]"
                      : "text-gray-500"
                }`}
              >
                {dirty
                  ? t("newsPollUnsaved")
                  : answered
                    ? t("newsPollAnswered")
                    : t("newsPollOpen")}
              </span>
            </div>
            <div className="flex flex-wrap gap-2">
              {options.map((option) => {
                const active = selected.includes(option.id);
                return (
                  <button
                    key={option.id}
                    type="button"
                    disabled={closed || saving}
                    aria-pressed={active}
                    onClick={() => poll.toggle(child, option.id)}
                    className={`rounded-full px-3 py-1.5 text-sm font-medium transition-colors disabled:opacity-60 ${
                      active
                        ? "bg-[#83CD2D] text-white"
                        : "bg-gray-100 text-gray-700 hover:bg-gray-200"
                    }`}
                  >
                    {option.label}
                  </button>
                );
              })}
            </div>
          </div>
        );
      })}
    </div>
  );
}

/**
 * Compact feed card: badges, title, school + date, two preview lines.
 *
 * A poll is NOT answered here — the card only flags that an answer is due and
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
  return (
    <button
      type="button"
      onClick={() => onOpen(item)}
      className={`flex w-full items-center gap-3 rounded-xl border bg-white p-4 text-left shadow-sm transition-colors hover:border-gray-300 hover:bg-gray-50 ${
        item.read ? "border-gray-200" : "border-gray-300"
      }`}
    >
      <span className="min-w-0 flex-1">
        <span className="flex flex-wrap items-center gap-2">
          <NewsBadges item={item} />
        </span>
        <span className="mt-1.5 block truncate text-sm font-semibold text-gray-900">
          {item.title}
        </span>
        <span className="mt-0.5 block text-xs text-gray-500">
          {item.school_name}
          {item.published_at ? ` · ${formatDate(item.published_at)}` : ""}
        </span>
        <span className="mt-1.5 line-clamp-2 block text-sm leading-5 text-gray-600">
          {item.body}
        </span>
      </span>
      <ChevronRight
        className="h-5 w-5 shrink-0 text-gray-400"
        aria-hidden="true"
      />
    </button>
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
  // and passes the refreshed announcement — same id, new published_at — back into
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
      // Done — like every other parent-portal modal, a successful write closes
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

  return (
    <Modal
      isOpen
      onClose={onClose}
      title={item.title}
      closeLabel={t("newsClose")}
      footer={
        <>
          <Button type="button" variant="outline" size="md" onClick={onClose}>
            {t("newsClose")}
          </Button>
          {isPoll(item) && !poll.closed && !stale && (
            <Button
              type="button"
              size="md"
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
              className="gap-1.5"
              onClick={() => void handleAcknowledge()}
              disabled={busy}
            >
              <Check className="h-4 w-4" aria-hidden="true" />
              {t("newsAcknowledge")}
            </Button>
          )}
        </>
      }
    >
      <div className="space-y-4">
        <div className="flex flex-wrap items-center gap-2">
          <NewsBadges item={item} />
          <span className="text-xs text-gray-500">
            {item.school_name}
            {item.published_at ? ` · ${formatDate(item.published_at)}` : ""}
          </span>
        </div>

        <p className="text-sm leading-6 whitespace-pre-line text-gray-800">
          <LinkifiedText text={item.body} />
        </p>

        {item.link_url && (
          <a
            href={item.link_url}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex max-w-full items-center gap-1.5 text-sm font-medium text-[#5080D8] underline underline-offset-2 hover:text-[#3f68b5]"
          >
            <ExternalLink className="h-4 w-4 shrink-0" aria-hidden="true" />
            <span className="truncate">{item.link_url}</span>
          </a>
        )}

        {stale && (
          <p
            role="alert"
            className="rounded-lg bg-[#F78C101A] px-3 py-2 text-sm text-[#B45309]"
          >
            {t("newsStaleError")}
          </p>
        )}

        {actionError && (
          <p
            role="alert"
            className="rounded-lg bg-[#FF31301A] px-3 py-2 text-sm text-[#CC2626]"
          >
            {actionError}
          </p>
        )}

        {poll.error && (
          <p
            role="alert"
            className="rounded-lg bg-[#FF31301A] px-3 py-2 text-sm text-[#CC2626]"
          >
            {poll.error}
          </p>
        )}

        {isPoll(item) && <PollAnswerRows poll={poll} />}
      </div>
    </Modal>
  );
}
