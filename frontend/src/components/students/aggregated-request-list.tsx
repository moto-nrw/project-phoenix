"use client";

/**
 * Aggregierte Eltern-Anfragenliste (#2432): EINE Liste über alle vier
 * Anfragearten (Stammdaten, Betreuungszeiten, Angebote, Abwesenheiten)
 * statt vier gestapelter Abschnitte. Suche und Filter wirken serverseitig;
 * nachgeladen wird über den Keyset-Cursor des Aggregations-Endpunkts.
 *
 * Die Komponente rendert je nach `view` entweder entscheidbare Karten (die
 * bestehenden Entscheiden-Abläufe leben in den per-Art-Item-Komponenten)
 * oder die read-only Historie-Karten. Der Aufrufer remountet sie beim
 * Umschalten Offen ↔ Historie (key={view}), wie zuvor die Einzelsektionen.
 */

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { TrayIcon } from "@phosphor-icons/react/ssr";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { ConfirmationModal } from "~/components/ui/modal";
import { ListSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import { CareExitModal } from "~/components/students/care-exit-modal";
import { CareRequestReviewItem } from "~/components/students/care-request-review-item";
import { StudentDeletionModal } from "~/components/students/student-deletion-modal";
import { ExcusedRequestReviewItem } from "~/components/students/excused-request-review-item";
import { MasterDataReviewItem } from "~/components/students/master-data-review-item";
import { OfferingRequestReviewItem } from "~/components/students/offering-request-review-item";
import { EnrollmentRequestItem } from "~/components/students/enrollment-request-item";
import { RequestHistoryItem } from "~/components/students/request-history-item";
import {
  RequestReviewCard,
  RequestRowHeader,
} from "~/components/students/request-review-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  fetchCareWithdrawals,
  type CareWithdrawalCompletion,
} from "~/lib/care-exit-api";
import {
  type AggregatedHistoryRequest,
  type AggregatedOpenRequest,
  type AggregatedRequestParams,
  type AggregatedRequestStatus,
  type AggregatedRequestType,
  listAggregatedOpenRequests,
  listAggregatedRequestHistory,
  listEnrollmentChangeRequests,
} from "~/lib/change-request-list-api";
import {
  createFeedState,
  takeMergedPage,
  type FeedSource,
} from "~/lib/request-feed";

const logger = createLogger({ component: "AggregatedRequestList" });
const WITHDRAWAL_PAGE_SIZE = 100;

export interface AggregatedRequestFilters {
  readonly search: string;
  /**
   * Nur die Einträge dieses Kindes — das Änderungsprotokoll der Kinderkartei
   * (#2437). Ohne Angabe: alle Kinder, die die Person sehen darf.
   */
  readonly studentId?: string;
  /**
   * Darf der Aggregator über die vier Kinderdaten-Arten abgefragt werden? Er
   * verlangt users:update oder users:absence; wer nur Anmeldungsänderungen
   * entscheidet, bekäme sonst für die ganze Liste einen 403. Ohne Angabe ja —
   * er ist die Hauptquelle der Liste.
   */
  readonly includeAggregated?: boolean;
  /**
   * Dürfen Anmeldungsänderungen mitgeladen werden? Sie hängen an config:manage
   * und kommen aus einem eigenen Endpunkt; ohne das Recht bleibt die Quelle
   * weg, statt der Seite einen 403 einzuhandeln.
   */
  readonly includeEnrollment?: boolean;
  /** Offene Komplett-Abmeldungen; verlangt users:delete. */
  readonly includeCareWithdrawals?: boolean;
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

function CareWithdrawalHistoryRows({
  rows,
}: Readonly<{ rows: CareWithdrawalCompletion[] }>) {
  return rows.map((row) => {
    const deleted = row.outcome === "deleted";
    const childName =
      deleted || row.studentId === ""
        ? "Gelöschtes Kind"
        : `${row.firstName} ${row.lastName}`.trim();
    return (
      <RequestReviewCard
        key={`care_withdrawal:${row.id}`}
        type="care_withdrawal"
        typeLabel="Abmeldung"
        childName={childName}
        summary={deleted ? "Kind sofort gelöscht" : "Betreuung beendet"}
        badge={
          <StatusBadge
            tone="gray"
            label={deleted ? "Gelöscht" : "Abgeschlossen"}
          />
        }
        history={{
          kind: "readonly",
          label: deleted ? "Gelöscht" : "Abgeschlossen",
          tone: "gray",
        }}
      >
        {row.resolvedAt ? (
          <p className="text-sm text-gray-600">
            Erledigt am {formatDate(row.resolvedAt)}
          </p>
        ) : null}
      </RequestReviewCard>
    );
  });
}

/** Wie viele Zeilen eine Seite der zusammengeführten Liste zeigt. */
const PAGE_SIZE = 25;

export function AggregatedRequestList({
  view,
  filters,
}: Readonly<{
  view: "open" | "history";
  filters: AggregatedRequestFilters;
}>) {
  const [items, setItems] = useState<AnyItem[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [withdrawals, setWithdrawals] = useState<CareWithdrawalCompletion[]>(
    [],
  );
  const [withdrawalsLoading, setWithdrawalsLoading] = useState(false);
  const [careExitWithdrawal, setCareExitWithdrawal] =
    useState<CareWithdrawalCompletion | null>(null);
  const [deletionWithdrawal, setDeletionWithdrawal] =
    useState<CareWithdrawalCompletion | null>(null);
  const [deletionWarningWithdrawal, setDeletionWarningWithdrawal] =
    useState<CareWithdrawalCompletion | null>(null);
  // Set while THIS list dispatches change-requests-refresh so its own listener
  // (below) doesn't refetch — it already removed the decided row optimistically.
  // dispatchEvent is synchronous, so the flag only has to cover that one call.
  const suppressSelfReloadRef = useRef(false);
  const withdrawalGenerationRef = useRef(0);

  // Die Quellen des Reiters: der Aggregator über die vier users:update-Arten
  // und — mit config:manage — die Anmeldungsänderungen aus ihrem eigenen
  // Endpunkt. Beide liefern nach Zeitpunkt sortierte Seiten; zusammengeführt
  // wird in request-feed.
  const sources = useMemo<FeedSource<AnyItem>[]>(() => {
    const params: AggregatedRequestParams = {
      search: filters.search,
      studentId: filters.studentId,
      ...(view === "history"
        ? { statuses: filters.statuses, from: filters.from, to: filters.to }
        : {}),
    };
    const wantsType = (type: AggregatedRequestType) =>
      filters.types.length === 0 || filters.types.includes(type);
    // Der Aggregator kennt die Anmeldungen nicht: seine Art-Liste kommt ohne
    // sie, und ist NUR sie gewählt, wird er gar nicht erst gefragt (er würde
    // die unbekannte Art mit 400 abweisen).
    const aggregatedTypes = filters.types.filter(
      (type) => type !== "enrollment" && type !== "care_withdrawal",
    );
    const built: FeedSource<AnyItem>[] = [];
    if (
      filters.includeAggregated !== false &&
      (filters.types.length === 0 || aggregatedTypes.length > 0)
    ) {
      const aggregatedParams: AggregatedRequestParams = {
        ...params,
        types: aggregatedTypes,
      };
      built.push({
        key: "aggregated",
        fetchPage: (cursor) =>
          view === "history"
            ? listAggregatedRequestHistory({ ...aggregatedParams, cursor })
            : listAggregatedOpenRequests({ ...aggregatedParams, cursor }),
      });
    }
    if (filters.includeEnrollment && wantsType("enrollment")) {
      built.push({
        key: "enrollment",
        fetchPage: (cursor) =>
          listEnrollmentChangeRequests(view, { ...params, cursor }),
      });
    }
    return built;
  }, [view, filters]);

  // Der Lesezustand je Quelle (Puffer + Cursor) lebt außerhalb des Renders:
  // „Weitere Einträge laden" macht genau dort weiter, wo jede Quelle stand.
  const feedRef = useRef(createFeedState<AnyItem>(sources));
  // Jede Neu-Ladung erhält einen eigenen Zustand. Antworten einer älteren
  // Ladung dürfen weder die sichtbare Liste noch den Cursor des neuen Feeds
  // verändern.
  const feedGenerationRef = useRef(0);
  const feedLoadingRef = useRef(false);
  const loadMoreInFlightRef = useRef(false);

  const loadFirstPage = useCallback(() => {
    const feed = createFeedState<AnyItem>(sources);
    const generation = ++feedGenerationRef.current;
    feedRef.current = feed;
    feedLoadingRef.current = true;
    return {
      generation,
      page: takeMergedPage(sources, feed, PAGE_SIZE).finally(() => {
        if (generation === feedGenerationRef.current) {
          feedLoadingRef.current = false;
        }
      }),
    };
  }, [sources]);

  // Erste Seite laden — auch bei jeder Such-/Filteränderung (fetchPage wechselt
  // die Identität), dann ersetzt die Antwort die Liste komplett.
  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    const { generation, page } = loadFirstPage();
    page
      .then((page) => {
        if (cancelled || generation !== feedGenerationRef.current) return;
        setItems(page.items);
        setHasMore(page.hasMore);
        setLoading(false);
      })
      .catch((err: unknown) => {
        if (cancelled || generation !== feedGenerationRef.current) return;
        const message = err instanceof Error ? err.message : String(err);
        logger.warn("aggregated_request_list_load_failed", { error: message });
        setError("Anfragen konnten nicht geladen werden.");
        setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [loadFirstPage]);

  const loadWithdrawals = useCallback(async () => {
    const generation = ++withdrawalGenerationRef.current;
    const selected = filters.types;
    const typeMatches =
      selected.length === 0 || selected.includes("care_withdrawal");
    if (!filters.includeCareWithdrawals || !typeMatches) {
      setWithdrawals([]);
      return;
    }
    try {
      const items: CareWithdrawalCompletion[] = [];
      let pageNumber = 1;
      let total = 0;
      do {
        const page = await fetchCareWithdrawals({
          search: filters.search,
          page: pageNumber,
          pageSize: WITHDRAWAL_PAGE_SIZE,
          ...(view === "history" ? { state: "resolved" as const } : {}),
        });
        items.push(...page.items);
        total = page.total;
        if (page.items.length === 0) break;
        pageNumber += 1;
      } while (items.length < total);
      if (generation === withdrawalGenerationRef.current) {
        setWithdrawals(items);
      }
    } catch (err: unknown) {
      if (generation !== withdrawalGenerationRef.current) return;
      logger.warn("care_withdrawal_list_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError("Abmeldungen konnten nicht geladen werden.");
    }
  }, [filters.includeCareWithdrawals, filters.search, filters.types, view]);

  useEffect(() => {
    let cancelled = false;
    setWithdrawalsLoading(filters.includeCareWithdrawals === true);
    void loadWithdrawals().finally(() => {
      if (!cancelled) setWithdrawalsLoading(false);
    });
    return () => {
      cancelled = true;
    };
  }, [filters.includeCareWithdrawals, loadWithdrawals, view]);

  // Refetch ohne Spinner, wenn eine Entscheidung anderswo fällt: Entscheidungen
  // in diesem Fenster senden change-requests-refresh, Entscheidungen anderswo
  // kommen als SSE-abgeleitetes messages-unread-refresh bzw. beim Fokuswechsel
  // an. Nur die offene Arbeitsliste braucht das — die Historie mountet beim
  // Umschalten frisch.
  const reloadInPlace = useCallback(async () => {
    const { generation, page } = loadFirstPage();
    try {
      const result = await page;
      if (generation !== feedGenerationRef.current) return;
      setItems(result.items);
      setHasMore(result.hasMore);
      setLoading(false);
    } catch (err) {
      if (generation !== feedGenerationRef.current) return;
      const message = err instanceof Error ? err.message : String(err);
      logger.warn("aggregated_request_list_reload_failed", { error: message });
      setLoading(false);
    }
  }, [loadFirstPage]);

  useEffect(() => {
    if (view !== "open") return;
    const handler = () => {
      if (suppressSelfReloadRef.current) return;
      void reloadInPlace();
      void loadWithdrawals();
    };
    const onFocus = () => {
      void reloadInPlace();
      void loadWithdrawals();
    };
    const onVisibility = () => {
      if (typeof document !== "undefined" && !document.hidden) {
        void reloadInPlace();
        void loadWithdrawals();
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
  }, [view, reloadInPlace, loadWithdrawals]);

  const loadMore = useCallback(async () => {
    if (!hasMore || feedLoadingRef.current || loadMoreInFlightRef.current)
      return;
    const generation = feedGenerationRef.current;
    const feed = feedRef.current;
    loadMoreInFlightRef.current = true;
    setLoadingMore(true);
    setError(null);
    try {
      const page = await takeMergedPage(sources, feed, PAGE_SIZE);
      if (generation !== feedGenerationRef.current) return;
      setItems((prev) => [...prev, ...page.items]);
      setHasMore(page.hasMore);
    } catch (err) {
      if (generation !== feedGenerationRef.current) return;
      const message = err instanceof Error ? err.message : String(err);
      logger.warn("aggregated_request_list_load_more_failed", {
        error: message,
      });
      setError("Weitere Anfragen konnten nicht geladen werden.");
    } finally {
      loadMoreInFlightRef.current = false;
      setLoadingMore(false);
    }
  }, [sources, hasMore]);

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

  const handleWithdrawalFinished = useCallback(
    (row: CareWithdrawalCompletion) => {
      setWithdrawals((current) => current.filter((item) => item.id !== row.id));
      setNotice("Die Betreuung wurde beendet.");
      window.dispatchEvent(new Event("change-requests-refresh"));
    },
    [],
  );

  if (loading || withdrawalsLoading) {
    return (
      <SkeletonRegion label="Anfragen werden geladen">
        <ListSkeleton rows={3} avatar={false} />
      </SkeletonRegion>
    );
  }

  // filters.studentId zählt bewusst NICHT als aktiver Filter: im
  // Änderungsprotokoll eines Kindes ist es der Kontext, kein Suchkriterium.
  // "Keine Treffer für Suche und Filter" wäre dort nur verwirrend.
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
      {items.length === 0 && withdrawals.length === 0 && !error ? (
        <EmptyState
          icon={<TrayIcon size={32} aria-hidden="true" />}
          // Die Quellen durchsuchen je Abruf nur ein Stück der Historie. Sind
          // noch ältere Seiten da, wäre „noch nichts entschieden“ schlicht
          // falsch — dann sagt der Text, dass weitergesucht werden kann.
          title={
            hasMore
              ? "Hier ist noch nichts gefunden."
              : view === "open"
                ? "Keine offenen Anfragen."
                : "Noch keine entschiedenen Anfragen."
          }
          description={
            hasMore
              ? "Ältere Einträge sind noch nicht geladen. Mit „Weitere Einträge laden“ weitersuchen."
              : hasActiveFilters
                ? "Für die aktuelle Suche und Filter gibt es keine Treffer."
                : undefined
          }
          variant="compact"
        />
      ) : (
        // Eine gemeinsame Fläche mit Spaltenkopf statt gestapelter Karten: so
        // richten sich die Zeilen aneinander aus und lesen sich als Tabelle.
        <div className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
          <RequestRowHeader view={view} />
          {view === "open"
            ? withdrawals.map((row) => {
                const name = `${row.firstName} ${row.lastName}`.trim();
                const overdue = row.urgency === "overdue";
                return (
                  <RequestReviewCard
                    key={`care_withdrawal:${row.id}`}
                    type="care_withdrawal"
                    typeLabel="Abmeldung"
                    childName={name}
                    summary={`Keine Betreuungstage ab ${formatDate(row.firstBookinglessDay)}`}
                    badge={
                      <StatusBadge
                        tone={overdue ? "red" : "orange"}
                        label={overdue ? "Überfällig" : "Geplant"}
                      />
                    }
                    history={{
                      kind: "readonly",
                      label: overdue ? "Überfällig" : "Geplant",
                      tone: overdue ? "red" : "orange",
                    }}
                    action={
                      <div className="flex flex-wrap gap-1">
                        <Button
                          type="button"
                          variant="ghost"
                          size="compact"
                          onClick={() => setCareExitWithdrawal(row)}
                        >
                          Betreuung beenden
                        </Button>
                        <Button
                          type="button"
                          variant="ghost"
                          size="compact"
                          onClick={() => setDeletionWarningWithdrawal(row)}
                        >
                          Kind sofort löschen
                        </Button>
                      </div>
                    }
                  >
                    <p className="text-sm text-gray-600">
                      Für dieses Kind ist kein Betreuungstag mehr gebucht.
                      Beenden Sie jetzt die Betreuung.
                    </p>
                  </RequestReviewCard>
                );
              })
            : null}
          {view === "history" ? (
            <CareWithdrawalHistoryRows rows={withdrawals} />
          ) : null}
          {items.map((item) => {
            const key = itemKey(item);
            // Anmeldungsänderungen tragen in beiden Ansichten dieselbe Karte:
            // Entschieden wird in der Detailansicht, hierhin verlinkt sie nur.
            if (item.request_type === "enrollment") {
              return (
                <EnrollmentRequestItem key={key} row={item.data} view={view} />
              );
            }
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
          })}
        </div>
      )}
      {hasMore && (
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
      {careExitWithdrawal && (
        <CareExitModal
          isOpen
          studentIds={[careExitWithdrawal.studentId]}
          completionId={careExitWithdrawal.id}
          firstBookinglessDay={careExitWithdrawal.firstBookinglessDay}
          onClose={() => setCareExitWithdrawal(null)}
          onFinished={() => {
            setCareExitWithdrawal(null);
            handleWithdrawalFinished(careExitWithdrawal);
          }}
        />
      )}
      {deletionWithdrawal && (
        <StudentDeletionModal
          isOpen
          studentId={deletionWithdrawal.studentId}
          completionId={deletionWithdrawal.id}
          displayName={`${deletionWithdrawal.firstName} ${deletionWithdrawal.lastName}`.trim()}
          onClose={() => setDeletionWithdrawal(null)}
          onDeleted={() => {
            setDeletionWithdrawal(null);
            handleWithdrawalFinished(deletionWithdrawal);
          }}
        />
      )}
      {deletionWarningWithdrawal && (
        <ConfirmationModal
          isOpen
          onClose={() => setDeletionWarningWithdrawal(null)}
          onConfirm={() => {
            setDeletionWithdrawal(deletionWarningWithdrawal);
            setDeletionWarningWithdrawal(null);
          }}
          title="Kind sofort löschen"
          confirmText="Löschen prüfen"
          cancelText="Zurück"
          mobileSheet
        >
          <p className="text-sm text-gray-600">
            Das Kind wird sofort gelöscht. Auch ein späterer letzter
            Betreuungstag wird nicht abgewartet.
          </p>
        </ConfirmationModal>
      )}
    </div>
  );
}
