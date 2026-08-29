"use client";

/**
 * Aggregierte Eltern-Anfragenliste (#2432, umgebaut in #2267): EINE Liste
 * über alle Anfragearten statt vier gestapelter Abschnitte.
 *
 * Seit #2267 ist die offene Arbeitsliste zweigeteilt: links die Kinder mit
 * offenen Anfragen, rechts alle Anfragen des gewählten Kindes. Auf schmalen
 * Geräten ersetzt der Detailbereich die Liste; „Zur Liste" bringt Blick und
 * Tastatur genau dorthin zurück, wo sie waren.
 *
 * Bewusst NICHT über components/database/master-detail-layout.tsx gebaut: das
 * legt den Detailbereich auf schmalen Geräten in eine Schublade (die Vorgabe
 * verlangt ein echtes Ersetzen) und friert Höhen auf 100dvh ein, was bei
 * 200 % Zoom zu abgeschnittenen Inhalten führt.
 */

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ListSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import { useViewportAtLeast } from "~/lib/hooks/use-viewport-at-least";
import { useReturnFocus } from "~/lib/hooks/use-return-focus";
import { createLogger } from "~/lib/logger";
import { useTenantSafe } from "~/lib/tenant-context";
import { staffReasonRequired } from "~/lib/tenant-api";
import type { CareWithdrawalCompletion } from "~/lib/care-exit-api";
import {
  bulkApproveParentRequests,
  ChangeRequestStaleError,
  type AggregatedOpenRequest,
  type BulkApproveRequestRef,
  type ParentRequestKind,
  type RequestReviewAccess,
} from "~/lib/change-request-list-api";
import {
  bucketCases,
  groupOpenCases,
  hasReviewMetadata,
  itemKey,
  type OpenCase,
} from "~/components/students/requests/case-model";
import {
  BulkApprovalPanel,
  BulkConfirmationDialog,
} from "~/components/students/requests/bulk-approval-panel";
import { HistoryRequestList } from "~/components/students/requests/history-request-list";
import { RequestCaseDetail } from "~/components/students/requests/request-case-detail";
import {
  RequestCaseList,
  caseRowID,
} from "~/components/students/requests/request-case-list";
import {
  NoCaseSelectedState,
  RequestEmptyState,
} from "~/components/students/requests/request-empty-states";
import { STALE_REQUEST_NOTICE } from "~/components/students/requests/request-copy";
import {
  useMergedRequestFeed,
  useRequestSources,
  useWithdrawalFeed,
} from "~/components/students/requests/use-request-feed";
import { useWithdrawalDialogs } from "~/components/students/requests/withdrawal-cards";

export type { AggregatedRequestFilters } from "~/components/students/requests/filters";
import type { AggregatedRequestFilters } from "~/components/students/requests/filters";

const logger = createLogger({ component: "AggregatedRequestList" });

/** Ab dieser Breite stehen Liste und Detailbereich nebeneinander. */
const SPLIT_VIEW_MIN_WIDTH = 1024;

function bulkRef(item: AggregatedOpenRequest): BulkApproveRequestRef[] {
  if (!hasReviewMetadata(item)) return [];
  return [
    {
      kind: item.request_type as ParentRequestKind,
      id: String(item.data.id),
      expected_version: item.expected_version,
    },
  ];
}

export function AggregatedRequestList({
  view,
  filters,
}: Readonly<{
  view: "open" | "history";
  filters: AggregatedRequestFilters;
}>) {
  // useTenantSafe statt useTenant: die Liste wird auch außerhalb des
  // Tenant-Providers gerendert (Tests, Einbettungen). Ohne Provider gilt die
  // strengste Fassung — lieber einmal zu viel begründen als zu wenig.
  const tenant = useTenantSafe();
  const reasonRequired = staffReasonRequired(
    tenant?.tenant?.parentRequestReasonPolicy,
  );
  const [reviewAccess, setReviewAccess] = useState<
    RequestReviewAccess | undefined
  >(undefined);
  const sources = useRequestSources(view, filters, setReviewAccess);
  const feed = useMergedRequestFeed(sources, view);
  const { items, setItems, hasMore, loading, loadingMore, error, setError } =
    feed;
  const withdrawalsFeed = useWithdrawalFeed(view, filters, setError);
  const {
    items: withdrawals,
    setItems: setWithdrawals,
    loading: withdrawalsLoading,
    loadingMore: withdrawalsLoadingMore,
    hasMore: withdrawalsHaveMore,
    load: loadWithdrawals,
    loadMore: loadMoreWithdrawals,
  } = withdrawalsFeed;
  const [notice, setNotice] = useState<string | null>(null);
  const [staleNotice, setStaleNotice] = useState<string | null>(null);
  const [selectedCaseKey, setSelectedCaseKey] = useState<string | null>(null);
  const [selectedForBulk, setSelectedForBulk] = useState<Set<string>>(
    () => new Set(),
  );
  const [bulkReason, setBulkReason] = useState("");
  const [bulkConfirmOpen, setBulkConfirmOpen] = useState(false);
  const [bulkSaving, setBulkSaving] = useState(false);
  const wide = useViewportAtLeast(SPLIT_VIEW_MIN_WIDTH);
  const returnFocus = useReturnFocus();
  // Set while THIS list dispatches change-requests-refresh so its own listener
  // doesn't refetch — it already removed the decided row optimistically.
  const suppressSelfReloadRef = useRef(false);

  useEffect(() => {
    if (view !== "open") return;
    const handler = () => {
      if (suppressSelfReloadRef.current) return;
      void feed.reload();
      void loadWithdrawals();
    };
    const onFocus = () => {
      void feed.reload();
      void loadWithdrawals();
    };
    const onVisibility = () => {
      if (typeof document !== "undefined" && !document.hidden) {
        void feed.reload();
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
  }, [view, feed, loadWithdrawals]);

  const handleDecided = useCallback(
    (key: string, decidedNotice: string) => {
      setItems((prev) => prev.filter((item) => itemKey(item) !== key));
      setSelectedForBulk((current) => {
        if (!current.has(key)) return current;
        const next = new Set(current);
        next.delete(key);
        return next;
      });
      suppressSelfReloadRef.current = true;
      window.dispatchEvent(new Event("change-requests-refresh"));
      suppressSelfReloadRef.current = false;
      setNotice(decidedNotice);
    },
    [setItems],
  );

  // Eine Anfrage wurde zwischenzeitlich geändert: die Zeile bleibt stehen und
  // wird neu geladen, statt eine Entscheidung vorzutäuschen, die nicht galt.
  const handleStale = useCallback(() => {
    setStaleNotice(STALE_REQUEST_NOTICE);
    void feed.reload();
  }, [feed]);

  const openCases = useMemo(
    () => (view === "open" ? groupOpenCases(items, withdrawals) : []),
    [items, view, withdrawals],
  );
  const buckets = useMemo(() => bucketCases(openCases), [openCases]);

  // Auswahl aufheben, sobald das Kind keine offenen Anfragen mehr hat.
  useEffect(() => {
    if (
      selectedCaseKey !== null &&
      !openCases.some((childCase) => childCase.key === selectedCaseKey)
    ) {
      setSelectedCaseKey(null);
    }
  }, [openCases, selectedCaseKey]);

  const selectedCase =
    openCases.find((childCase) => childCase.key === selectedCaseKey) ??
    // In der breiten Ansicht steht rechts das erste Kind, solange nichts
    // gewählt ist: eine leere halbe Seite hilft niemandem.
    (wide && selectedCaseKey === null ? openCases[0] : undefined);

  const selectCase = useCallback(
    (childCase: OpenCase) => {
      returnFocus.remember(caseRowID(childCase.key));
      setSelectedCaseKey(childCase.key);
      setNotice(null);
      setStaleNotice(null);
    },
    [returnFocus],
  );

  const backToList = useCallback(() => {
    setSelectedCaseKey(null);
    returnFocus.restore();
  }, [returnFocus]);

  const selectedBulkItems = useMemo(
    () =>
      items.filter(
        (item): item is AggregatedOpenRequest =>
          view === "open" && selectedForBulk.has(itemKey(item)),
      ),
    [items, selectedForBulk, view],
  );

  const confirmBulkApproval = useCallback(async () => {
    const refs = selectedBulkItems.flatMap(bulkRef);
    setBulkSaving(true);
    setError(null);
    try {
      const count = await bulkApproveParentRequests(refs, bulkReason.trim());
      const selectedKeys = new Set(selectedForBulk);
      setItems((current) =>
        current.filter((item) => !selectedKeys.has(itemKey(item))),
      );
      setSelectedForBulk(new Set());
      setBulkReason("");
      setBulkConfirmOpen(false);
      setNotice(`${count} Anfragen wurden freigegeben.`);
      suppressSelfReloadRef.current = true;
      window.dispatchEvent(new Event("change-requests-refresh"));
      suppressSelfReloadRef.current = false;
    } catch (err: unknown) {
      logger.warn("parent_request_bulk_approval_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setBulkConfirmOpen(false);
      if (err instanceof ChangeRequestStaleError) {
        setStaleNotice(err.message);
      } else {
        setError(
          err instanceof Error
            ? err.message
            : "Die Anfragen konnten nicht gemeinsam freigegeben werden.",
        );
      }
      await feed.reload();
    } finally {
      setBulkSaving(false);
    }
  }, [
    bulkReason,
    feed,
    selectedBulkItems,
    selectedForBulk,
    setError,
    setItems,
  ]);

  const handleWithdrawalFinished = useCallback(
    (row: CareWithdrawalCompletion, deleted = false) => {
      setWithdrawals((current) => current.filter((item) => item.id !== row.id));
      setNotice(
        deleted ? "Das Kind wurde gelöscht." : "Die Betreuung wurde beendet.",
      );
      window.dispatchEvent(new Event("change-requests-refresh"));
    },
    [setWithdrawals],
  );
  const withdrawalDialogs = useWithdrawalDialogs(handleWithdrawalFinished);

  const handleBulkSelection = useCallback((key: string, checked: boolean) => {
    setSelectedForBulk((current) => {
      const next = new Set(current);
      if (checked) next.add(key);
      else next.delete(key);
      return next;
    });
  }, []);

  const handleProtectionChanged = useCallback(
    (studentID: string, enabled: boolean) => {
      setItems((current) =>
        current.map((item) =>
          "student_id" in item && item.student_id === studentID
            ? { ...item, family_protected: enabled }
            : item,
        ),
      );
      setNotice(
        enabled
          ? "Der Familienschutz ist jetzt aktiv."
          : "Der Familienschutz ist jetzt aus.",
      );
    },
    [setItems],
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
  const hasActiveFilters =
    filters.search.trim() !== "" ||
    filters.types.length > 0 ||
    filters.statuses.length > 0 ||
    Boolean(filters.from) ||
    Boolean(filters.to);
  const visibleWithdrawals =
    view === "history"
      ? withdrawals.filter((row) => {
          const resolvedDate = row.resolvedAt?.slice(0, 10);
          return (
            resolvedDate !== undefined &&
            (!filters.from || resolvedDate >= filters.from) &&
            (!filters.to || resolvedDate <= filters.to)
          );
        })
      : withdrawals;

  const showList = view === "open" && (wide || selectedCase === undefined);
  const showDetail = view === "open" && (wide || selectedCase !== undefined);

  return (
    <div className="space-y-3">
      {error && <Alert type="error" message={error} />}
      {staleNotice && <Alert type="warning" message={staleNotice} />}
      {notice && <Alert type="success" message={notice} />}
      {items.length === 0 && visibleWithdrawals.length === 0 && !error ? (
        <RequestEmptyState
          view={view}
          hasMore={hasMore}
          hasActiveFilters={hasActiveFilters}
          reviewAccess={reviewAccess}
        />
      ) : view === "open" ? (
        <>
          <BulkApprovalPanel
            count={selectedBulkItems.length}
            reason={bulkReason}
            setReason={setBulkReason}
            reasonRequired={reasonRequired}
            open={() => setBulkConfirmOpen(true)}
          />
          <div className="grid gap-3 lg:grid-cols-[minmax(20rem,26rem)_minmax(0,1fr)]">
            {showList && (
              <RequestCaseList
                buckets={buckets}
                selectedKey={selectedCase?.key ?? null}
                onSelect={selectCase}
              />
            )}
            {showDetail &&
              (selectedCase ? (
                <RequestCaseDetail
                  key={selectedCase.key}
                  childCase={selectedCase}
                  canManageFamilyProtection={Boolean(
                    filters.canManageFamilyProtection,
                  )}
                  reasonRequired={reasonRequired}
                  selected={selectedForBulk}
                  narrow={!wide}
                  onBackToList={backToList}
                  onSelectionChange={handleBulkSelection}
                  onDecided={handleDecided}
                  onProtectionChanged={handleProtectionChanged}
                  onReload={handleStale}
                  onNotice={setNotice}
                  finishWithdrawal={withdrawalDialogs.finishWithdrawal}
                  removeWithdrawal={withdrawalDialogs.removeWithdrawal}
                />
              ) : (
                <NoCaseSelectedState />
              ))}
          </div>
        </>
      ) : (
        <HistoryRequestList
          items={items}
          withdrawals={visibleWithdrawals}
          reasonRequired={reasonRequired}
          onCorrected={(corrected) => {
            setNotice(corrected);
            void feed.reload();
          }}
        />
      )}
      {(hasMore || withdrawalsHaveMore) && (
        <div className="flex justify-center pt-1">
          <Button
            type="button"
            variant="outline"
            size="md"
            className="max-sm:min-h-11"
            onClick={() => {
              if (hasMore) void feed.loadMore();
              if (withdrawalsHaveMore) void loadMoreWithdrawals();
            }}
            disabled={loadingMore || withdrawalsLoadingMore}
          >
            {loadingMore || withdrawalsLoadingMore
              ? "Wird geladen…"
              : "Weitere Einträge laden"}
          </Button>
        </div>
      )}
      {withdrawalDialogs.dialogs}
      <BulkConfirmationDialog
        open={bulkConfirmOpen}
        count={selectedBulkItems.length}
        reason={bulkReason}
        saving={bulkSaving}
        close={() => setBulkConfirmOpen(false)}
        confirm={() => void confirmBulkApproval()}
      />
    </div>
  );
}
