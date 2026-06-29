"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";

import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { Loading } from "~/components/ui/loading";
import { createLogger } from "~/lib/logger";
import {
  type StaffMasterDataChange,
  decideMasterDataChangeRequest,
  listMasterDataChangeRequests,
} from "~/lib/master-data-review-api";

const logger = createLogger({ component: "MasterDataReviewList" });

type ReviewTranslator = ReturnType<
  typeof useTranslations<"staffMasterDataReview">
>;

function fieldLabel(field: string, t: ReviewTranslator): string {
  // The known field keys all have labels; fall back to the raw key.
  const known = new Set([
    "first_name",
    "last_name",
    "birthday",
    "health_info",
    "email",
    "primary",
    "address_street",
    "address_city",
    "address_postal_code",
    "preferred_contact_method",
    "language_preference",
    "allowed_departure_modes",
  ]);
  return known.has(field) ? t(`fields.${field}`) : field;
}

function formatValue(value: unknown, empty: string): string {
  if (value === null || value === undefined || value === "") return empty;
  if (typeof value === "string") return value;
  if (typeof value === "object") {
    // Departure modes: { mon: ["pickup"], ... } — render a compact summary.
    return Object.entries(value as Record<string, unknown>)
      .map(([day, modes]) =>
        Array.isArray(modes) ? `${day}: ${modes.join("/")}` : `${day}`,
      )
      .join(", ");
  }
  return String(value);
}

export function MasterDataReviewList() {
  const t = useTranslations("staffMasterDataReview");
  const [rows, setRows] = useState<StaffMasterDataChange[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [reasons, setReasons] = useState<Record<string, string>>({});
  const [notice, setNotice] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setRows(await listMasterDataChangeRequests());
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      logger.warn("master_data_review_load_failed", { error: message });
      setError(message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const decide = useCallback(
    async (row: StaffMasterDataChange, approve: boolean) => {
      setBusyId(row.id);
      setError(null);
      setNotice(null);
      try {
        await decideMasterDataChangeRequest(
          row.id,
          approve,
          reasons[row.id]?.trim() || undefined,
        );
        setRows((prev) => prev.filter((r) => r.id !== row.id));
        setNotice(approve ? t("approved") : t("rejected"));
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        logger.warn("master_data_review_decide_failed", {
          error: message,
          request_id: row.id,
        });
        setError(t("decideError"));
      } finally {
        setBusyId(null);
      }
    },
    [reasons, t],
  );

  const columns = useMemo<DataTableColumn<StaffMasterDataChange>[]>(
    () => [
      {
        key: "child",
        header: t("columns.child"),
        render: (row) => (
          <span className="font-medium text-gray-900">
            {row.first_name} {row.last_name}
          </span>
        ),
        sortValue: (row) => `${row.last_name} ${row.first_name}`,
      },
      {
        key: "field",
        header: t("columns.field"),
        render: (row) => fieldLabel(row.field_key, t),
      },
      {
        key: "change",
        header: t("columns.change"),
        render: (row) => (
          <span className="text-sm">
            <span className="text-gray-500 line-through">
              {formatValue(row.old_value, t("empty_value"))}
            </span>
            <span className="mx-2 text-gray-400">→</span>
            <span className="font-medium text-gray-900">
              {formatValue(row.new_value, t("empty_value"))}
            </span>
          </span>
        ),
      },
      {
        key: "actions",
        header: t("columns.actions"),
        align: "right",
        render: (row) => (
          <div className="flex flex-col items-end gap-2">
            <input
              type="text"
              value={reasons[row.id] ?? ""}
              onChange={(e) =>
                setReasons((prev) => ({ ...prev, [row.id]: e.target.value }))
              }
              placeholder={t("reason")}
              className="h-8 w-44 rounded-md border-0 bg-white px-2 text-sm text-gray-900 ring-1 ring-gray-200 ring-inset focus:ring-2 focus:ring-gray-400 focus:outline-none"
            />
            <div className="flex gap-2">
              <Button
                type="button"
                variant="success"
                size="compact"
                disabled={busyId === row.id}
                onClick={() => void decide(row, true)}
              >
                {t("approve")}
              </Button>
              <Button
                type="button"
                variant="outline_danger"
                size="compact"
                disabled={busyId === row.id}
                onClick={() => void decide(row, false)}
              >
                {t("reject")}
              </Button>
            </div>
          </div>
        ),
      },
    ],
    [t, reasons, busyId, decide],
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
        emptyState={<span className="text-sm text-gray-500">{t("empty")}</span>}
      />
    </div>
  );
}
