"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { useApiLoadError } from "~/contexts/ToastContext";
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
  const pathname = usePathname();
  const router = useRouter();
  const searchParams = useSearchParams();
  const requestedBrief = searchParams.get("brief");
  const [items, setItems] = useState<ParentAnnouncement[]>([]);
  const [loaded, setLoaded] = useState(false);
  // A failed load replaces the list with retry (#2518), so it never reads
  // as "nothing to do".
  const {
    error: loadError,
    show: showLoadError,
    clear: clearLoadError,
  } = useApiLoadError();
  const [openId, setOpenId] = useState<string | null>(null);
  // Only the latest load may update the page; unmount invalidates all.
  const loadSeqRef = useRef(0);

  const load = useCallback(() => {
    const seq = ++loadSeqRef.current;
    setLoaded(false);
    clearLoadError();
    listAnnouncements()
      .then((list) => {
        if (seq === loadSeqRef.current) setItems(list);
      })
      .catch((err: unknown) => {
        logger.error("parent_news_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (seq !== loadSeqRef.current) return;
        void showLoadError(err, {
          object: t("errorObjectList"),
          retry: () => loadRef.current(),
        });
      })
      .finally(() => {
        if (seq === loadSeqRef.current) setLoaded(true);
      });
  }, [clearLoadError, showLoadError, t]);
  // The retry runs the latest load, not the one of the failed attempt.
  const loadRef = useRef(load);
  useLayoutEffect(() => {
    loadRef.current = load;
  });

  useEffect(() => {
    load();
    return () => {
      loadSeqRef.current += 1;
    };
  }, [load]);

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
        // Deliberately silent: the person did not start this refresh, and the
        // open letter already shows that it is out of date.
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
      <div data-parent-tour="news">
        <ParentPageHeader
          kicker={t("kicker")}
          title={t("title")}
          description={t("description")}
        />
      </div>

      {!loaded ? (
        <NewsListSkeleton />
      ) : loadError ? (
        <LoadErrorAlert error={loadError} />
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
