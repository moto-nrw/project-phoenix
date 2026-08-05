"use client";

/**
 * Parents-portal Neuigkeiten page (#1669): the full cross-school announcement
 * feed. The dashboard panel shows only the newest few; this page is the
 * canonical channel — sidebar item with unread badge, list of compact cards,
 * tap opens the detail (which marks the announcement read).
 */

import { useCallback, useEffect, useState } from "react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { useTranslations } from "next-intl";

import {
  NewsCard,
  NewsDetailModal,
} from "~/components/parent/news/news-components";
import { Skeleton } from "~/components/ui/skeleton";
import { Alert } from "~/components/ui/alert";
import { ConceptPageHeader } from "~/components/ui/concept-section-header";
import { createLogger } from "~/lib/logger";
import { type ParentAnnouncement, listAnnouncements } from "~/lib/parent-api";

const logger = createLogger({ component: "ParentNewsPage" });

export default function ParentNewsPage() {
  const t = useTranslations("parentDashboard");
  const [items, setItems] = useState<ParentAnnouncement[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [loadError, setLoadError] = useState(false);
  const [openId, setOpenId] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    listAnnouncements()
      .then((list) => {
        if (active) setItems(list);
      })
      .catch((err: unknown) => {
        logger.error("parent_news_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (active) setLoadError(true);
      })
      .finally(() => {
        if (active) setLoaded(true);
      });
    return () => {
      active = false;
    };
  }, []);

  const applyState = useCallback(
    (id: string, patch: Partial<ParentAnnouncement>) => {
      setItems((prev) =>
        prev.map((item) => (item.id === id ? { ...item, ...patch } : item)),
      );
    },
    [],
  );

  // A read/ack was rejected because the announcement is no longer current;
  // refetch so a retracted item drops out and a corrected one refreshes.
  const refetchOnStale = useCallback(() => {
    listAnnouncements()
      .then(setItems)
      .catch((err: unknown) => {
        logger.error("parent_news_refetch_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
  }, []);

  const openItem = items.find((item) => item.id === openId) ?? null;

  return (
    <div className="mx-auto w-full max-w-7xl space-y-6">
      <header className="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm sm:p-6">
        <ConceptPageHeader
          title={t("newsTitle")}
          eyebrow={t("newsEyebrow")}
          concept="news"
          subtitle={t("newsDescription")}
        />
      </header>

      {!loaded ? (
        <div className="space-y-3">
          <Skeleton className="h-24 w-full rounded-xl" />
          <Skeleton className="h-24 w-full rounded-xl" />
          <Skeleton className="h-24 w-full rounded-xl" />
        </div>
      ) : loadError ? (
        <Alert type="error" message={t("newsActionError")} />
      ) : items.length === 0 ? (
        <div className="rounded-2xl border border-dashed border-gray-300 bg-gray-50 p-6">
          <div className="flex items-start gap-3">
            <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-white text-gray-500 shadow-sm ring-1 ring-gray-200">
              <MotoConceptIcon concept="news" size={22} />
            </span>
            <div className="min-w-0">
              <h2 className="text-sm font-semibold text-gray-900">
                {t("noNewsTitle")}
              </h2>
              <p className="mt-1 text-sm leading-6 text-gray-600">
                {t("noNewsPageDescription")}
              </p>
            </div>
          </div>
        </div>
      ) : (
        <ul className="space-y-3">
          {items.map((item) => (
            <li key={item.id}>
              <NewsCard item={item} onOpen={(opened) => setOpenId(opened.id)} />
            </li>
          ))}
        </ul>
      )}

      {openItem && (
        <NewsDetailModal
          item={openItem}
          onClose={() => setOpenId(null)}
          onUpdated={applyState}
          onStale={refetchOnStale}
        />
      )}
    </div>
  );
}
