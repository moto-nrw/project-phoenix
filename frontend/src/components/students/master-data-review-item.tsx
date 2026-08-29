"use client";

import { useState } from "react";

import {
  RequestReviewCard,
  ReviewDiffPanel,
} from "~/components/students/request-review-card";
import { formatDate } from "~/lib/date-helpers";
import { CONTACT_METHODS, LANGUAGE_PREFERENCES } from "~/lib/guardian-helpers";
import { createLogger } from "~/lib/logger";
import { useToast } from "~/contexts/ToastContext";
import {
  type StaffMasterDataChange,
  decideMasterDataChangeRequest,
} from "~/lib/master-data-review-api";
import {
  DEPARTURE_MODE_LABELS,
  DEPARTURE_WEEKDAYS,
  type DepartureMode,
} from "~/lib/student-helpers";

const logger = createLogger({ component: "MasterDataReviewItem" });

// German-only staff UI: the staff shell ships a minimal client message catalog
// (shell namespaces only — see shell-intl-provider.tsx), so this page hardcodes its
// German strings like the rest of the staff/admin surface instead of using
// useTranslations, which would resolve to raw keys here.
const EMPTY_VALUE = "—";

const FIELD_LABELS: Record<string, string> = {
  first_name: "Vorname",
  last_name: "Nachname",
  birthday: "Geburtsdatum",
  school_class: "Klasse",
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

// Exported for the Historie view, which renders the same field labels.
export function fieldLabel(field: string): string {
  // Known field keys have German labels; fall back to the raw key.
  return FIELD_LABELS[field] ?? field;
}

// German labels for the wire values (#2362). Unknown keys fall back to the raw
// value so future wire additions stay visible instead of crashing or vanishing.
const WEEKDAY_LABELS: Record<string, string> = Object.fromEntries(
  DEPARTURE_WEEKDAYS.map((day) => [day.key, day.label]),
);

const CONTACT_METHOD_LABELS: Record<string, string> = Object.fromEntries(
  CONTACT_METHODS.map((method) => [method.value, method.label]),
);

const LANGUAGE_LABELS: Record<string, string> = Object.fromEntries(
  LANGUAGE_PREFERENCES.map((lang) => [lang.value, lang.label]),
);

function departureModeLabel(mode: unknown): string {
  if (typeof mode === "string")
    return DEPARTURE_MODE_LABELS[mode as DepartureMode] ?? mode;
  return JSON.stringify(mode) ?? EMPTY_VALUE;
}

// Exported as formatMasterDataValue for the Historie view (same rendering
// rules for old/new values as the pending queue).
export function formatValue(
  field: string,
  value: unknown,
  empty: string,
): string {
  if (value === null || value === undefined || value === "") return empty;
  if (typeof value === "string") {
    if (field === "preferred_contact_method")
      return CONTACT_METHOD_LABELS[value] ?? value;
    if (field === "language_preference") return LANGUAGE_LABELS[value] ?? value;
    // Only format well-formed ISO dates — formatDate on anything else yields
    // "Invalid Date", which would break the raw-value fallback contract.
    if (field === "birthday" && /^\d{4}-\d{2}-\d{2}$/.test(value))
      return formatDate(value);
    return value;
  }
  if (typeof value === "object") {
    // Departure modes: { mon: ["pickup"], ... } — render a compact summary
    // with German weekday and mode labels ("Montag: Wird abgeholt").
    return Object.entries(value as Record<string, unknown>)
      .map(([day, modes]) => {
        const dayLabel = WEEKDAY_LABELS[day] ?? day;
        return Array.isArray(modes)
          ? `${dayLabel}: ${modes.map(departureModeLabel).join(" / ")}`
          : dayLabel;
      })
      .join(", ");
  }
  return String(value);
}

/**
 * Eine offene Stammdaten-Anfrage als entscheidbare Karte. Begründung ist
 * optional; nach der Entscheidung meldet onDecided den Hinweistext und der
 * Aufrufer entfernt die Zeile aus der Liste (#2432).
 */
export function MasterDataReviewItem({
  row,
  onDecided,
  grouped = false,
}: Readonly<{
  row: StaffMasterDataChange;
  onDecided: (notice: string) => void;
  grouped?: boolean;
}>) {
  const toast = useToast();
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);

  const decide = async (approve: boolean) => {
    setBusy(true);
    try {
      await decideMasterDataChangeRequest(
        row.id,
        approve,
        reason.trim() || undefined,
      );
      onDecided(approve ? "Änderung übernommen" : "Änderung abgelehnt");
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      logger.warn("master_data_review_decide_failed", {
        error: message,
        request_id: row.id,
      });
      toast.error("Die Entscheidung konnte nicht gespeichert werden.", {
        duration: 8000,
      });
      setBusy(false);
    }
  };

  return (
    <RequestReviewCard
      type="master_data"
      childName={`${row.first_name} ${row.last_name}`}
      grouped={grouped}
      summary={fieldLabel(row.field_key)}
      submittedAt={row.created_at}
      reason={reason}
      onReasonChange={setReason}
      reasonPlaceholder="Begründung (optional)"
      busy={busy}
      onApprove={() => void decide(true)}
      onReject={() => void decide(false)}
    >
      <ReviewDiffPanel title="Änderungen">
        {/* Field name lives in the collapsed summary; the expanded panel
            shows only the value change. */}
        <div className="flex flex-wrap items-baseline gap-2 text-sm">
          <span className="text-gray-400 line-through">
            {formatValue(row.field_key, row.old_value, EMPTY_VALUE)}
          </span>
          <span className="text-gray-400" aria-hidden="true">
            →
          </span>
          <span className="font-medium text-gray-900">
            {formatValue(row.field_key, row.new_value, EMPTY_VALUE)}
          </span>
        </div>
      </ReviewDiffPanel>
    </RequestReviewCard>
  );
}
