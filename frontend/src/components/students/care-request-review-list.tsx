"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { Loading } from "~/components/ui/loading";
import { createLogger } from "~/lib/logger";
import {
  type StaffCareRequest,
  decideCareScheduleChangeRequest,
  listCareScheduleChangeRequests,
} from "~/lib/care-request-review-api";

const logger = createLogger({ component: "CareRequestReviewList" });

// German-only staff UI — hardcoded strings like MasterDataReviewList (the
// staff shell ships no full message catalog).
export function CareRequestReviewList() {
  const [rows, setRows] = useState<StaffCareRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [reasons, setReasons] = useState<Record<string, string>>({});
  const [reasonErrors, setReasonErrors] = useState<Record<string, boolean>>({});
  const [notice, setNotice] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setRows(await listCareScheduleChangeRequests());
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      logger.warn("care_request_review_load_failed", { error: message });
      setError(message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const decide = useCallback(
    async (row: StaffCareRequest, approve: boolean) => {
      const reason = reasons[row.id]?.trim() ?? "";
      if (!approve && reason === "") {
        // The backend requires a reason for rejections; surface that before
        // the request instead of a generic error after it.
        setReasonErrors((prev) => ({ ...prev, [row.id]: true }));
        return;
      }
      setBusyId(row.id);
      setError(null);
      setNotice(null);
      try {
        await decideCareScheduleChangeRequest(
          row.id,
          approve,
          reason || undefined,
        );
        setRows((prev) => prev.filter((r) => r.id !== row.id));
        setNotice(
          approve
            ? "Betreuungszeiten übernommen"
            : "Betreuungszeit-Anfrage abgelehnt",
        );
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        logger.warn("care_request_review_decide_failed", {
          error: message,
          request_id: row.id,
        });
        setError("Die Entscheidung konnte nicht gespeichert werden.");
      } finally {
        setBusyId(null);
      }
    },
    [reasons],
  );

  const columns = useMemo<DataTableColumn<StaffCareRequest>[]>(
    () => [
      {
        key: "child",
        header: "Kind",
        render: (row) => (
          <span className="font-medium text-gray-900">
            {row.first_name} {row.last_name}
          </span>
        ),
        sortValue: (row) => `${row.last_name} ${row.first_name}`,
      },
      {
        key: "changes",
        header: "Änderung",
        render: (row) => (
          <div className="space-y-1">
            {row.diff.length === 0 && (
              <span className="text-sm text-gray-500">—</span>
            )}
            {row.diff.map((entry) => (
              <div key={entry.label} className="text-sm">
                <span className="text-xs text-gray-500">{entry.label}: </span>
                <span className="text-gray-500 line-through">{entry.old}</span>
                <span className="mx-2 text-gray-400">→</span>
                <span className="font-medium text-gray-900">{entry.new}</span>
              </div>
            ))}
          </div>
        ),
      },
      {
        key: "actions",
        header: "Aktion",
        align: "right",
        render: (row) => (
          <div className="flex flex-col items-end gap-2">
            <input
              type="text"
              value={reasons[row.id] ?? ""}
              onChange={(e) => {
                setReasons((prev) => ({ ...prev, [row.id]: e.target.value }));
                setReasonErrors((prev) => ({ ...prev, [row.id]: false }));
              }}
              placeholder="Begründung (Pflicht bei Ablehnung)"
              className="h-8 w-52 rounded-md border-0 bg-white px-2 text-sm text-gray-900 ring-1 ring-gray-200 ring-inset focus:ring-2 focus:ring-gray-400 focus:outline-none"
            />
            {reasonErrors[row.id] && (
              <span className="text-xs text-[#CC2626]">
                Für eine Ablehnung ist eine Begründung erforderlich.
              </span>
            )}
            <div className="flex gap-2">
              <Button
                type="button"
                variant="success"
                size="compact"
                disabled={busyId === row.id}
                onClick={() => void decide(row, true)}
              >
                Freigeben
              </Button>
              <Button
                type="button"
                variant="outline_danger"
                size="compact"
                disabled={busyId === row.id}
                onClick={() => void decide(row, false)}
              >
                Ablehnen
              </Button>
            </div>
          </div>
        ),
      },
    ],
    [reasons, reasonErrors, busyId, decide],
  );

  if (loading) return <Loading fullPage={false} />;

  return (
    <div className="space-y-4">
      {error && (
        <div className="rounded-xl border border-[#FF3130]/20 bg-[#FF3130]/10 p-3 text-sm text-[#CC2626]">
          {error}
        </div>
      )}
      {notice && (
        <div className="rounded-xl border border-[#83CD2D]/30 bg-[#83CD2D]/10 p-3 text-sm text-[#4A7A15]">
          {notice}
        </div>
      )}
      <DataTable
        columns={columns}
        rows={rows}
        getRowKey={(row) => row.id}
        emptyState={
          <span className="text-sm text-gray-500">
            Keine offenen Betreuungszeit-Anfragen.
          </span>
        }
      />
    </div>
  );
}
