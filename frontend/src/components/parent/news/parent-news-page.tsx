"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { Alert } from "~/components/ui/alert";
import { Skeleton } from "~/components/ui/skeleton";
import {
  NewsCard,
  NewsDetailModal,
} from "~/components/parent/news/news-components";
import { createLogger } from "~/lib/logger";
import { type ParentAnnouncement, listAnnouncements } from "~/lib/parent-api";

const logger = createLogger({ component: "ParentNewsPage" });

/**
 * "Aus der OGS": alles, was die Schule an alle schickt.
 *
 * Heisst absichtlich nicht mehr "Neuigkeiten": Eltern erwarten unter dem Wort
 * eine Nachrichtenseite, gemeint sind Aushaenge, Umfragen und Elterninfos.
 * Ungelesenes traegt eine blaue Kante, offene Handlungen eine orange.
 */
export function ParentNewsPage() {
  const t = useTranslations("parentNews");
  const tDash = useTranslations("parentDashboard");
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

  return (
    <div className="space-y-4">
      {/* Siehe Kalender: die Kopfzeile traegt den sichtbaren Titel. */}
      <h1 className="sr-only">{t("title")}</h1>

      {!loaded ? (
        <div className="space-y-3">
          <Skeleton className="h-32 w-full rounded-2xl" />
          <Skeleton className="h-32 w-full rounded-2xl" />
        </div>
      ) : loadError ? (
        <Alert type="error" message={tDash("newsActionError")} />
      ) : items.length === 0 ? (
        <p className="rounded-2xl border border-gray-200 bg-white p-5 text-[17px] text-gray-600 shadow-sm">
          {t("empty")}
        </p>
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
