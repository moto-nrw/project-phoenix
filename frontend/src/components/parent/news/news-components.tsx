"use client";

/**
 * Shared parent-news building blocks (#1669): the compact feed card and the
 * detail modal. Used by the dashboard "Neuigkeiten" panel (latest few) and the
 * dedicated /parents/news page (full feed). Opening the detail marks the
 * announcement read (the read stat parents feed the OGS) — acknowledging stays
 * an explicit button.
 */

import { useCallback, useEffect, useRef, useState } from "react";
import { Check, ChevronRight, ExternalLink } from "lucide-react";
import { useTranslations } from "next-intl";

import { Modal } from "~/components/ui/modal";
import { LinkifiedText } from "~/components/ui/linkified-text";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  type ParentAnnouncement,
  acknowledgeAnnouncement,
  markAnnouncementRead,
} from "~/lib/parent-api";

const logger = createLogger({ component: "ParentNews" });

/** Tell the sidebar badge to refetch after a read/ack. */
function refreshUnreadBadge() {
  window.dispatchEvent(new Event("parent-news-unread-refresh"));
}

export function NewsBadges({
  item,
}: Readonly<{ item: ParentAnnouncement }>): React.ReactNode {
  const t = useTranslations("parentDashboard");
  return (
    <>
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

/** Compact feed card: badges, title, school + date, two preview lines. */
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
}: Readonly<{
  item: ParentAnnouncement;
  onClose: () => void;
  onUpdated: (id: string, patch: Partial<ParentAnnouncement>) => void;
}>) {
  const t = useTranslations("parentDashboard");
  const [busy, setBusy] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const markedRef = useRef(false);

  useEffect(() => {
    if (item.read || markedRef.current) return;
    markedRef.current = true;
    markAnnouncementRead(item.id)
      .then(() => {
        onUpdated(item.id, { read: true });
        refreshUnreadBadge();
      })
      .catch((err: unknown) => {
        logger.error("parent_news_mark_read_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
  }, [item.id, item.read, onUpdated]);

  const handleAcknowledge = useCallback(async () => {
    setBusy(true);
    setActionError(null);
    try {
      await acknowledgeAnnouncement(item.id);
      onUpdated(item.id, { read: true, acknowledged: true });
      refreshUnreadBadge();
    } catch (err: unknown) {
      logger.error("parent_news_acknowledge_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setActionError(t("newsActionError"));
    } finally {
      setBusy(false);
    }
  }, [item.id, onUpdated, t]);

  const needsAck = item.requires_acknowledgement && !item.acknowledged;

  return (
    <Modal isOpen onClose={onClose} title={item.title}>
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

        {actionError && (
          <p
            role="alert"
            className="rounded-lg bg-[#FF31301A] px-3 py-2 text-sm text-[#CC2626]"
          >
            {actionError}
          </p>
        )}

        {needsAck && (
          <div className="border-t border-gray-100 pt-3">
            <button
              type="button"
              disabled={busy}
              onClick={() => void handleAcknowledge()}
              className="inline-flex items-center gap-1.5 rounded-lg bg-gray-900 px-3 py-1.5 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-gray-800 disabled:opacity-60"
            >
              <Check className="h-4 w-4" aria-hidden="true" />
              {t("newsAcknowledge")}
            </button>
          </div>
        )}
      </div>
    </Modal>
  );
}
