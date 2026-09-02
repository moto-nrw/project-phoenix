"use client";

import { useCallback, useEffect, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { Alert } from "~/components/ui/alert";
import { Skeleton } from "~/components/ui/skeleton";
import {
  isOutstandingAnnouncement,
  NewsCard,
  NewsDetailModal,
} from "~/components/parent/news/news-components";
import { createLogger } from "~/lib/logger";
import { type ParentAnnouncement, listAnnouncements } from "~/lib/parent-api";
import { ParentPage, ParentPageHeader } from "~/components/parent/parent-page";

const logger = createLogger({ component: "ParentNewsPage" });

/** Elternbriefe buendeln Mitteilungen, Umfragen und Elterninformationen. */
export function ParentNewsPage() {
  const t = useTranslations("parentNews");
  const tDash = useTranslations("parentDashboard");
  const pathname = usePathname();
  const router = useRouter();
  const searchParams = useSearchParams();
  const requestedBrief = searchParams.get("brief");
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

  useEffect(() => {
    if (requestedBrief && items.some((item) => item.id === requestedBrief)) {
      setOpenId(requestedBrief);
    }
  }, [items, requestedBrief]);

  const applyState = useCallback(
    (id: string, patch: Partial<ParentAnnouncement>) => {
      setItems((prev) =>
        prev.map((item) => (item.id === id ? { ...item, ...patch } : item)),
      );
    },
    [],
  );

  // Ein Lesen oder Bestaetigen wurde abgewiesen, weil die Meldung nicht mehr
  // aktuell ist: neu laden, damit eine zurueckgezogene verschwindet.
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

  const closeItem = useCallback(() => {
    setOpenId(null);
    if (!requestedBrief) return;
    const params = new URLSearchParams(searchParams.toString());
    params.delete("brief");
    const query = params.toString();
    router.replace(query ? `${pathname}?${query}` : pathname, {
      scroll: false,
    });
  }, [pathname, requestedBrief, router, searchParams]);
  const outstandingItems = items.filter(isOutstandingAnnouncement);
  const completedItems = items.filter(
    (item) => !isOutstandingAnnouncement(item),
  );

  const renderItems = (group: readonly ParentAnnouncement[]) => (
    <ul className="mt-3 space-y-3">
      {group.map((item) => (
        <li key={item.id}>
          <NewsCard item={item} onOpen={(opened) => setOpenId(opened.id)} />
        </li>
      ))}
    </ul>
  );

  return (
    <ParentPage>
      <ParentPageHeader
        kicker={t("kicker")}
        title={t("title")}
        description={t("description")}
      />

      {!loaded ? (
        <NewsListSkeleton />
      ) : loadError ? (
        <Alert type="error" message={tDash("newsActionError")} />
      ) : items.length === 0 ? (
        <p className="moto-content-surface rounded-2xl border p-5 text-sm leading-6 text-gray-600 shadow-sm backdrop-blur-md">
          {t("empty")}
        </p>
      ) : (
        <div className="space-y-8">
          {outstandingItems.length > 0 && (
            <section aria-labelledby="parent-news-outstanding">
              <h2
                id="parent-news-outstanding"
                aria-label={t("openLabel", {
                  count: outstandingItems.length,
                })}
                className="flex items-baseline gap-2 px-1 text-lg font-semibold text-gray-950"
              >
                {t("open")}
                <span
                  className="text-moto-blue-strong tabular-nums"
                  aria-hidden="true"
                >
                  {outstandingItems.length}
                </span>
              </h2>
              {renderItems(outstandingItems)}
            </section>
          )}

          {completedItems.length > 0 && (
            <section aria-labelledby="parent-news-completed">
              <h2
                id="parent-news-completed"
                className="px-1 text-lg font-semibold text-gray-950"
              >
                {t("completed")}
              </h2>
              {renderItems(completedItems)}
            </section>
          )}
        </div>
      )}

      {openItem && (
        <NewsDetailModal
          item={openItem}
          onClose={closeItem}
          onUpdated={applyState}
          onStale={refetchOnStale}
        />
      )}
    </ParentPage>
  );
}

function NewsListSkeleton() {
  return (
    <div
      data-testid="parent-news-skeleton"
      className="space-y-3"
      aria-hidden="true"
    >
      <Skeleton className="ml-1 h-6 w-28" />
      {[0, 1].map((item) => (
        <div
          key={item}
          className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-5"
        >
          <div className="flex gap-3">
            <Skeleton className="size-10 shrink-0 rounded-xl" />
            <div className="min-w-0 flex-1">
              <div className="flex items-start justify-between gap-3">
                <Skeleton className="h-5 w-2/3" />
                <Skeleton className="h-5 w-16 rounded-full" />
              </div>
              <Skeleton className="mt-3 h-4 w-full" />
              <Skeleton className="mt-2 h-4 w-4/5" />
              <div className="mt-4 flex items-center justify-between gap-3">
                <Skeleton className="h-3 w-28" />
                <Skeleton className="h-8 w-24 rounded-lg" />
              </div>
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}
