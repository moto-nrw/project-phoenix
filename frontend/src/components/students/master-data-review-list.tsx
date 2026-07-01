"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

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

// German-only staff UI: the staff shell ships a minimal client message catalog
// (parentNav only — see shell-nav-intl-provider.tsx), so this page hardcodes its
// German strings like the rest of the staff/admin surface instead of using
// useTranslations, which would resolve to raw keys here.
const EMPTY_VALUE = "—";

const FIELD_LABELS: Record<string, string> = {
  first_name: "Vorname",
  last_name: "Nachname",
  birthday: "Geburtsdatum",
  health_info: "Gesundheitshinweise",
  email: "E-Mail",
  primary: "Telefonnummer",
  address_street: "Straße",
  address_city: "Ort",
  address_postal_code: "PLZ",
  preferred_contact_method: "Kontaktweg",
  language_preference: "Sprache",
  allowed_departure_modes: "Dauerhafte Gehzeiten",
};

function fieldLabel(field: string): string {
  // Known field keys have German labels; fall back to the raw key.
  return FIELD_LABELS[field] ?? field;
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
        setNotice(approve ? "Änderung übernommen" : "Änderung abgelehnt");
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        logger.warn("master_data_review_decide_failed", {
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

  const columns = useMemo<DataTableColumn<StaffMasterDataChange>[]>(
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
        key: "field",
        header: "Feld",
        render: (row) => fieldLabel(row.field_key),
      },
      {
        key: "change",
        header: "Änderung",
        render: (row) => (
          <span className="text-sm">
            <span className="text-gray-500 line-through">
              {formatValue(row.old_value, EMPTY_VALUE)}
            </span>
            <span className="mx-2 text-gray-400">→</span>
            <span className="font-medium text-gray-900">
              {formatValue(row.new_value, EMPTY_VALUE)}
            </span>
          </span>
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
              onChange={(e) =>
                setReasons((prev) => ({ ...prev, [row.id]: e.target.value }))
              }
              placeholder="Begründung (optional)"
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
    [reasons, busyId, decide],
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
            Keine offenen Änderungsanfragen.
          </span>
        }
      />
    </div>
  );
}
