"use client";

import { useCallback, useEffect, useState } from "react";

import { Loading } from "~/components/ui/loading";
import {
  RequestReviewCard,
  ReviewDiffPanel,
} from "~/components/students/request-review-card";
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

  if (loading) return <Loading fullPage={false} />;

  return (
    <div className="space-y-3">
      {error && (
        <div className="rounded-xl border border-[#FF3130]/20 bg-[#FF3130]/10 p-3 text-sm text-[#CC2626]">
          {error}
        </div>
      )}
      {notice && (
        <div className="rounded-xl border border-gray-200 bg-gray-50 p-3 text-sm text-gray-700">
          {notice}
        </div>
      )}
      {rows.length === 0 ? (
        <p className="rounded-2xl border border-gray-200 bg-white p-4 text-sm text-gray-500 shadow-sm">
          Keine offenen Änderungsanfragen.
        </p>
      ) : (
        rows.map((row) => (
          <RequestReviewCard
            key={row.id}
            childName={`${row.first_name} ${row.last_name}`}
            summary={fieldLabel(row.field_key)}
            reason={reasons[row.id] ?? ""}
            onReasonChange={(value) =>
              setReasons((prev) => ({ ...prev, [row.id]: value }))
            }
            reasonPlaceholder="Begründung (optional)"
            busy={busyId === row.id}
            onApprove={() => void decide(row, true)}
            onReject={() => void decide(row, false)}
          >
            <ReviewDiffPanel>
              {/* Field name lives in the collapsed summary; the expanded panel
                  shows only the value change. */}
              <div className="flex flex-wrap items-baseline gap-2 text-sm">
                <span className="text-gray-400 line-through">
                  {formatValue(row.old_value, EMPTY_VALUE)}
                </span>
                <span className="text-gray-400" aria-hidden="true">
                  →
                </span>
                <span className="font-medium text-gray-900">
                  {formatValue(row.new_value, EMPTY_VALUE)}
                </span>
              </div>
            </ReviewDiffPanel>
          </RequestReviewCard>
        ))
      )}
    </div>
  );
}
