"use client";

/**
 * Eine Anmeldungsänderung als Zeile der Eltern-Liste (#2435): der Wunsch einer
 * Familie, ihre Anmeldung nach dem Absenden noch zu ändern. Entschieden wird
 * sie weiterhin in der Detailansicht mit Rückfrage-Dialog — die Karte führt
 * dorthin, statt die Freigabe ein zweites Mal nachzubauen.
 */

import Link from "next/link";
import { ArrowRight } from "lucide-react";

import { StatusBadge } from "~/components/ui/status-badge";
import type { EnrollmentChangeRequest } from "~/lib/change-request-list-api";
import { formatDate } from "~/lib/date-helpers";
import { enrollmentChangeRequestFieldLabel } from "~/lib/enrollment-change-request-diff";
import { enrollmentChangeRequestStatusMeta } from "~/lib/enrollment-change-request-status";
import { useTenantAwarePath } from "~/lib/tenant-path";

/** „Vorname, Telefon" — welche Teile der Anmeldung die Anfrage betrifft. */
function changedSummary(fields: readonly string[]): string {
  if (fields.length === 0) return "Keine Angabe";
  return fields.map(enrollmentChangeRequestFieldLabel).join(", ");
}

export function EnrollmentRequestItem({
  row,
  view,
}: Readonly<{
  row: EnrollmentChangeRequest;
  view: "open" | "history";
}>) {
  const tenantPath = useTenantAwarePath();
  const meta = enrollmentChangeRequestStatusMeta(row.status);
  const childNames =
    row.child_names.length > 0
      ? row.child_names.join(", ")
      : (row.guardian_name ?? "Anmeldung");
  const parentNote = row.parent_note?.trim();
  const decisionNote = row.decision_note?.trim();

  return (
    <div className="moto-content-surface rounded-2xl border p-4 shadow-sm">
      <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
        <span className="font-medium text-gray-900">{childNames}</span>
        <span className="text-sm text-gray-600">
          ·{" "}
          {row.origin === "admin"
            ? "Anmeldung · Korrektur der OGS"
            : "Anmeldung"}
        </span>
        <StatusBadge label={meta.label} tone={meta.tone} />
      </div>
      <p className="mt-1 text-xs text-gray-500">
        Eingereicht am {formatDate(row.created_at)}
        {row.guardian_name ? ` von ${row.guardian_name}` : ""}
        {view === "history" && row.decided_at
          ? ` · Entschieden am ${formatDate(row.decided_at)}${
              row.decided_by_name ? ` von ${row.decided_by_name}` : ""
            }`
          : ""}
      </p>
      <p className="mt-2 text-sm text-gray-700">
        Geändert: {changedSummary(row.changed_fields)}
      </p>
      {/* Zwei getrennte Begründungen: die der Familie beim Einreichen und die
          der Entscheidung. Eine davon zu verschlucken nimmt der Historie
          genau die Frage, für die es sie gibt. */}
      {parentNote && (
        <p className="mt-1 text-sm text-gray-600 italic">
          Begründung: „{parentNote}“
        </p>
      )}
      {decisionNote && (
        <p className="mt-1 text-sm text-gray-600 italic">
          Entscheidung: „{decisionNote}“
        </p>
      )}
      <div className="mt-3">
        <Link
          href={tenantPath(
            `/admin/enrollments/change-requests/${encodeURIComponent(row.id)}`,
          )}
          className="inline-flex h-8 items-center gap-2 rounded-md px-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        >
          {view === "open" ? "Prüfen" : "Ansehen"}
          <ArrowRight className="h-4 w-4" aria-hidden="true" />
        </Link>
      </div>
    </div>
  );
}
