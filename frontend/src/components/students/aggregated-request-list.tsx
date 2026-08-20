"use client";

/**
 * Aggregierte Eltern-Anfragenliste (#2432): EINE Liste über alle vier
 * Anfragearten (Stammdaten, Betreuungszeiten, Angebote, Entschuldigungen)
 * statt vier gestapelter Abschnitte. Suche und Filter wirken serverseitig;
 * nachgeladen wird über den Keyset-Cursor des Aggregations-Endpunkts.
 *
 * Die Komponente rendert je nach `view` entweder entscheidbare Karten (die
 * bestehenden Entscheiden-Abläufe leben in den per-Art-Item-Komponenten)
 * oder die read-only Historie-Karten. Der Aufrufer remountet sie beim
 * Umschalten Offen ↔ Historie (key={view}), wie zuvor die Einzelsektionen.
 */

import { useCallback, useEffect, useRef, useState } from "react";

import { TrayIcon } from "@phosphor-icons/react/ssr";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { ListSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import { CareRequestReviewItem } from "~/components/students/care-request-review-item";
import { ExcusedRequestReviewItem } from "~/components/students/excused-request-review-item";
import { MasterDataReviewItem } from "~/components/students/master-data-review-item";
import { OfferingRequestReviewItem } from "~/components/students/offering-request-review-item";
import { RequestHistoryItem } from "~/components/students/request-history-item";
import { createLogger } from "~/lib/logger";
import {
  type AggregatedHistoryRequest,
  type AggregatedOpenRequest,
  type AggregatedRequestParams,
  type AggregatedRequestStatus,
  type AggregatedRequestType,
  listAggregatedOpenRequests,
  listAggregatedRequestHistory,
} from "~/lib/change-request-list-api";

const logger = createLogger({ component: "AggregatedRequestList" });

export interface AggregatedRequestFilters {
  readonly search: string;
  /** Leer = alle Arten. */
  readonly types: readonly AggregatedRequestType[];
  /** Nur Historie; leer = alle Status. */
  readonly statuses: readonly AggregatedRequestStatus[];
  /** Nur Historie, YYYY-MM-DD. */
  readonly from?: string;
  /** Nur Historie, YYYY-MM-DD. */
  readonly to?: string;
}

type AnyItem = AggregatedOpenRequest | AggregatedHistoryRequest;

function itemKey(item: AnyItem): string {
  return `${item.request_type}:${item.data.id}`;
}

export function AggregatedRequestList({
  view,
  filters,
}: Readonly<{
  view: "open" | "history";
  filters: AggregatedRequestFilters;
}>) {
  const [items, setItems] = useState<AnyItem[]>([]);
  const [nextCursor, setNextCursor] = useState<string | undefined>(undefined);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  // Set while THIS list dispatches change-requests-refresh so its own listener
  // (below) doesn't refetch — it already removed the decided row optimistically.
  // dispatchEvent is synchronous, so the flag only has to cover that one call.
  const suppressSelfReloadRef = useRef(false);

  const fetchPage = useCallback(
    (cursor?: string) => {
      const params: AggregatedRequestParams = {
        search: filters.search,
        types: filters.types,
        cursor,
        ...(view === "history"
          ? { statuses: filters.statuses, from: filters.from, to: filters.to }
          : {}),
      };
      return view === "history"
        ? listAggregatedRequestHistory(params)
        : listAggregatedOpenRequests(params);
    },
    [view, filters],
  );

  // Bei sehr selektiven Filtern kann eine Server-Seite leer sein, aber einen
  // Cursor tragen (Scan-Budget des Aggregators). Bis zu drei Folgeseiten
  // automatisch nachziehen, damit niemand blind auf „Weitere Einträge laden"
  // drücken muss; danach übernimmt der sichtbare Nachlade-Knopf.
  const fetchFilledPage = useCallback(
    async (cursor?: string) => {
      let page = await fetchPage(cursor);
      for (
        let follows = 0;
        page.items.length === 0 && page.next_cursor && follows < 3;
        follows++
      ) {
        page = await fetchPage(page.next_cursor);
      }
      return page;
    },
    [fetchPage],
  );

  // Erste Seite laden — auch bei jeder Such-/Filteränderung (fetchPage wechselt
  // die Identität), dann ersetzt die Antwort die Liste komplett.
  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    fetchFilledPage()
      .then((page) => {
        if (cancelled) return;
        setItems([...page.items]);
        setNextCursor(page.next_cursor);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        const message = err instanceof Error ? err.message : String(err);
        logger.warn("aggregated_request_list_load_failed", { error: message });
        setError("Anfragen konnten nicht geladen werden.");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [fetchFilledPage]);

  // Refetch ohne Spinner, wenn eine Entscheidung anderswo fällt: Entscheidungen
  // in diesem Fenster senden change-requests-refresh, Entscheidungen anderswo
  // kommen als SSE-abgeleitetes messages-unread-refresh bzw. beim Fokuswechsel
  // an. Nur die offene Arbeitsliste braucht das — die Historie mountet beim
  // Umschalten frisch.
  const reloadInPlace = useCallback(async () => {
    try {
      const page = await fetchFilledPage();
      setItems([...page.items]);
      setNextCursor(page.next_cursor);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      logger.warn("aggregated_request_list_reload_failed", { error: message });
    }
  }, [fetchFilledPage]);

  useEffect(() => {
    if (view !== "open") return;
    const handler = () => {
      if (suppressSelfReloadRef.current) return;
      void reloadInPlace();
    };
    const onFocus = () => void reloadInPlace();
    const onVisibility = () => {
      if (typeof document !== "undefined" && !document.hidden) {
        void reloadInPlace();
      }
    };
    window.addEventListener("change-requests-refresh", handler);
    window.addEventListener("messages-unread-refresh", handler);
    window.addEventListener("focus", onFocus);
    document.addEventListener("visibilitychange", onVisibility);
    return () => {
      window.removeEventListener("change-requests-refresh", handler);
      window.removeEventListener("messages-unread-refresh", handler);
      window.removeEventListener("focus", onFocus);
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, [view, reloadInPlace]);

  const loadMore = useCallback(async () => {
    if (!nextCursor) return;
    setLoadingMore(true);
    setError(null);
    try {
      const page = await fetchFilledPage(nextCursor);
      setItems((prev) => [...prev, ...page.items]);
      setNextCursor(page.next_cursor);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      logger.warn("aggregated_request_list_load_more_failed", {
        error: message,
      });
      setError("Weitere Anfragen konnten nicht geladen werden.");
    } finally {
      setLoadingMore(false);
    }
  }, [fetchFilledPage, nextCursor]);

  // Nach einer Entscheidung: Zeile entfernen, Hinweis zeigen und das
  // Badge/die Geschwister-Ansichten über change-requests-refresh anstoßen.
  // Der eigene Listener ist währenddessen unterdrückt — die Zeile ist schon weg.
  const handleDecided = useCallback((key: string, decidedNotice: string) => {
    setItems((prev) => prev.filter((item) => itemKey(item) !== key));
    suppressSelfReloadRef.current = true;
    window.dispatchEvent(new Event("change-requests-refresh"));
    suppressSelfReloadRef.current = false;
    setNotice(decidedNotice);
  }, []);

  if (loading) {
    return (
      <SkeletonRegion label="Anfragen werden geladen">
        <ListSkeleton rows={3} avatar={false} />
      </SkeletonRegion>
    );
  }

  const hasActiveFilters =
    filters.search.trim() !== "" ||
    filters.types.length > 0 ||
    filters.statuses.length > 0 ||
    Boolean(filters.from) ||
    Boolean(filters.to);

  return (
    <div className="space-y-3">
      {error && <Alert type="error" message={error} />}
      {notice && <Alert type="success" message={notice} />}
      {items.length === 0 && !error ? (
        <EmptyState
          icon={<TrayIcon size={32} aria-hidden="true" />}
          title={
            view === "open"
              ? "Keine offenen Anfragen."
              : "Noch keine entschiedenen Anfragen."
          }
          description={
            hasActiveFilters
              ? "Für die aktuelle Suche und Filter gibt es keine Treffer."
              : undefined
          }
          variant="compact"
        />
      ) : (
        items.map((item) => {
          const key = itemKey(item);
          if (view === "history") {
            return (
              <RequestHistoryItem
                key={key}
                item={item as AggregatedHistoryRequest}
              />
            );
          }
          const open = item as AggregatedOpenRequest;
          const onDecided = (decidedNotice: string) =>
            handleDecided(key, decidedNotice);
          switch (open.request_type) {
            case "master_data":
              return (
                <MasterDataReviewItem
                  key={key}
                  row={open.data}
                  onDecided={onDecided}
                />
              );
            case "care_schedule":
              return (
                <CareRequestReviewItem
                  key={key}
                  row={open.data}
                  onDecided={onDecided}
                />
              );
            case "offering":
              return (
                <OfferingRequestReviewItem
                  key={key}
                  row={open.data}
                  onDecided={onDecided}
                />
              );
            case "excused":
              return (
                <ExcusedRequestReviewItem
                  key={key}
                  row={open.data}
                  onDecided={onDecided}
                />
              );
          }
        })
      )}
      {nextCursor && (
        <div className="flex justify-center pt-1">
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={() => void loadMore()}
            disabled={loadingMore}
          >
            {loadingMore ? "Wird geladen…" : "Weitere Einträge laden"}
          </Button>
        </div>
      )}
    </div>
  );
}
