"use client";

/**
 * Historie der Eltern-Änderungsanfragen: read-only Karten der bereits
 * entschiedenen Anfragen (wer hat wann was mit welcher Begründung
 * entschieden), pro Warteschlange cursor-paginiert über "Weitere Einträge
 * laden". Die Sektionen mounten frisch, wenn die Seite von "Offen" auf
 * "Historie" umschaltet — deshalb brauchen sie keine refresh-Event-Listener
 * wie die Pending-Listen.
 */

import type { ReactNode } from "react";
import { useCallback, useEffect, useState } from "react";

import {
  StatusBadge,
  type StatusBadgeTone,
} from "~/components/ui/status-badge";
import { Button } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { SkeletonRegion, ListSkeleton } from "~/components/ui/page-skeletons";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  listMasterDataChangeRequestHistory,
  type StaffMasterDataHistoryEntry,
} from "~/lib/master-data-review-api";
import {
  listCareScheduleChangeRequestHistory,
  type StaffCareRequestHistoryEntry,
} from "~/lib/care-request-review-api";
import {
  listOfferingChangeRequestHistory,
  type StaffOfferingRequestHistoryEntry,
} from "~/lib/offering-request-review-api";
import {
  listExcusedAbsenceRequestHistory,
  type StaffExcusedRequestHistoryEntry,
} from "~/lib/excused-request-review-api";
import {
  fieldLabel,
  formatValue as formatMasterDataValue,
} from "~/components/students/master-data-review-list";

const logger = createLogger({ component: "RequestHistoryList" });

const STATUS_META: Record<string, { label: string; tone: StatusBadgeTone }> = {
  approved: { label: "Freigegeben", tone: "green" },
  rejected: { label: "Abgelehnt", tone: "red" },
  withdrawn: { label: "Zurückgezogen", tone: "gray" },
  auto_applied: { label: "Automatisch übernommen", tone: "gray" },
};

/** Flat read-only card for one decided request. */
export function RequestHistoryCard({
  childName,
  summary,
  status,
  decidedAt,
  decidedByName,
  reason,
  children,
}: Readonly<{
  childName: string;
  summary?: string;
  status: string;
  decidedAt: string;
  decidedByName?: string;
  reason?: string;
  children?: ReactNode;
}>) {
  const meta = STATUS_META[status] ?? { label: status, tone: "gray" as const };
  return (
    <div className="moto-content-surface rounded-2xl border p-4 shadow-sm">
      <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
        <span className="font-medium text-gray-900">{childName}</span>
        {summary && <span className="text-sm text-gray-600">· {summary}</span>}
        <StatusBadge label={meta.label} tone={meta.tone} />
      </div>
      <p className="mt-1 text-xs text-gray-500">
        Entschieden am {formatDate(decidedAt)}
        {decidedByName ? ` von ${decidedByName}` : ""}
      </p>
      {reason && (
        <p className="mt-1 text-sm text-gray-600 italic">„{reason}“</p>
      )}
      {children && <div className="mt-2">{children}</div>}
    </div>
  );
}

interface HistoryPageResult<T> {
  readonly items: readonly T[];
  readonly next_cursor?: string;
}

/**
 * Generic fetch/cursor/empty/error shell shared by the four queues. The
 * queue-specific card comes from renderItem.
 */
function RequestHistorySection<T>({
  fetchPage,
  renderItem,
  getKey,
  skeletonLabel,
}: Readonly<{
  fetchPage: (cursor?: string) => Promise<HistoryPageResult<T>>;
  renderItem: (item: T) => ReactNode;
  getKey: (item: T) => string;
  skeletonLabel: string;
}>) {
  const [items, setItems] = useState<readonly T[]>([]);
  const [nextCursor, setNextCursor] = useState<string | undefined>(undefined);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const page = await fetchPage();
        if (cancelled) return;
        setItems(page.items);
        setNextCursor(page.next_cursor);
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        logger.warn("request_history_load_failed", { error: message });
        if (!cancelled) setError(message);
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [fetchPage]);

  const loadMore = useCallback(async () => {
    if (!nextCursor) return;
    setLoadingMore(true);
    setError(null);
    try {
      const page = await fetchPage(nextCursor);
      setItems((prev) => [...prev, ...page.items]);
      setNextCursor(page.next_cursor);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      logger.warn("request_history_load_more_failed", { error: message });
      setError(message);
    } finally {
      setLoadingMore(false);
    }
  }, [fetchPage, nextCursor]);

  if (loading) {
    return (
      <SkeletonRegion label={skeletonLabel}>
        <ListSkeleton rows={3} avatar={false} />
      </SkeletonRegion>
    );
  }

  return (
    <div className="space-y-3">
      {error && (
        <div className="border-moto-red/20 bg-moto-red/10 text-moto-red-strong rounded-xl border p-3 text-sm">
          {error}
        </div>
      )}
      {items.length === 0 && !error ? (
        <EmptyState
          title="Noch keine entschiedenen Anfragen."
          variant="compact"
        />
      ) : (
        items.map((item) => <div key={getKey(item)}>{renderItem(item)}</div>)
      )}
      {nextCursor && (
        <div className="flex justify-center">
          <Button
            type="button"
            size="md"
            variant="outline"
            onClick={() => void loadMore()}
            disabled={loadingMore}
          >
            Weitere Einträge laden
          </Button>
        </div>
      )}
    </div>
  );
}

export function MasterDataHistoryList() {
  return (
    <RequestHistorySection<StaffMasterDataHistoryEntry>
      fetchPage={listMasterDataChangeRequestHistory}
      getKey={(row) => row.id}
      skeletonLabel="Historie wird geladen"
      renderItem={(row) => (
        <RequestHistoryCard
          childName={`${row.first_name} ${row.last_name}`}
          summary={fieldLabel(row.field_key)}
          status={row.status}
          decidedAt={row.decided_at}
          decidedByName={row.decided_by_name}
          reason={row.review_reason}
        >
          <p className="text-sm text-gray-700">
            {formatMasterDataValue(row.field_key, row.old_value, "leer")}
            {" → "}
            {formatMasterDataValue(row.field_key, row.new_value, "leer")}
          </p>
        </RequestHistoryCard>
      )}
    />
  );
}

export function CareRequestHistoryList() {
  return (
    <RequestHistorySection<StaffCareRequestHistoryEntry>
      fetchPage={listCareScheduleChangeRequestHistory}
      getKey={(row) => row.id}
      skeletonLabel="Historie wird geladen"
      renderItem={(row) => (
        <RequestHistoryCard
          childName={`${row.first_name} ${row.last_name}`}
          summary="Betreuungszeiten"
          status={row.status}
          decidedAt={row.decided_at}
          decidedByName={row.decided_by_name}
          reason={row.decision_reason}
        >
          {row.requested.length > 0 && (
            <div className="space-y-1 rounded-lg bg-gray-50 p-3">
              <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                Beantragt
              </p>
              {row.requested.map((entry) => (
                <p
                  key={`${entry.label}-${entry.new}`}
                  className="text-sm text-gray-700"
                >
                  {entry.label}: {entry.new}
                </p>
              ))}
            </div>
          )}
        </RequestHistoryCard>
      )}
    />
  );
}

export function OfferingRequestHistoryList() {
  return (
    <RequestHistorySection<StaffOfferingRequestHistoryEntry>
      fetchPage={listOfferingChangeRequestHistory}
      getKey={(row) => row.id}
      skeletonLabel="Historie wird geladen"
      renderItem={(row) => (
        <RequestHistoryCard
          childName={row.student_name}
          summary="Betreuungsangebote und AGs"
          status={row.status}
          decidedAt={row.decided_at}
          decidedByName={row.decided_by_name}
          reason={row.reason}
        >
          {row.diff.length > 0 && (
            <div className="space-y-1 rounded-lg bg-gray-50 p-3">
              <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                Änderungen
              </p>
              {row.diff.map((line) => (
                <p key={line.offering_id} className="text-sm text-gray-700">
                  {line.label}: {line.old} → {line.new}
                </p>
              ))}
            </div>
          )}
        </RequestHistoryCard>
      )}
    />
  );
}

export function ExcusedRequestHistoryList() {
  return (
    <RequestHistorySection<StaffExcusedRequestHistoryEntry>
      fetchPage={listExcusedAbsenceRequestHistory}
      getKey={(row) => row.id}
      skeletonLabel="Historie wird geladen"
      renderItem={(row) => (
        <RequestHistoryCard
          childName={`${row.first_name} ${row.last_name}`}
          summary="Entschuldigte Abmeldung"
          status={row.status}
          decidedAt={row.decided_at}
          decidedByName={row.decided_by_name}
          reason={row.reason}
        >
          <p className="text-sm text-gray-700">
            {row.dates.map((d) => formatDate(d)).join(", ")}
            {row.note ? ` · ${row.note}` : ""}
          </p>
        </RequestHistoryCard>
      )}
    />
  );
}
