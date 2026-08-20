"use client";

/**
 * Eine Anmeldungsänderung als Zeile der Eltern-Liste (#2435): der Wunsch einer
 * Familie, ihre Anmeldung nach dem Absenden noch zu ändern. Entschieden wird
 * sie weiterhin in der Detailansicht mit Rückfrage-Dialog — die Karte führt
 * dorthin, statt die Freigabe ein zweites Mal nachzubauen.
 *
 * Sie nutzt dieselbe Lese-Karte wie die übrigen Zeilen der Liste, damit die
 * Arten nebeneinander gleich aussehen.
 */

import { ArrowRight } from "lucide-react";

import { RequestReviewCard } from "~/components/students/request-review-card";
import { ButtonLink } from "~/components/ui/button";
import type { EnrollmentChangeRequest } from "~/lib/change-request-list-api";
import { enrollmentChangeRequestFieldLabel } from "~/lib/enrollment-change-request-diff";
import { enrollmentChangeRequestStatusMeta } from "~/components/enrollment/enrollment-change-request-status";
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

  return (
    <RequestReviewCard
      childName={childNames}
      summary={
        row.origin === "admin" ? "Anmeldung · Korrektur der OGS" : "Anmeldung"
      }
      submittedAt={row.created_at}
      submittedByName={row.guardian_name}
      history={{
        kind: "readonly",
        label: meta.label,
        tone: meta.tone,
        decidedAt: view === "history" ? row.decided_at : undefined,
        decidedByName: row.decided_by_name,
        // Die Begründung der Karte ist die der Entscheidung; die der Familie
        // steht im Rumpf, damit eine die andere nicht verdrängt.
        reason: row.decision_note?.trim(),
      }}
      action={
        <ButtonLink
          href={tenantPath(
            `/admin/enrollments/change-requests/${encodeURIComponent(row.id)}`,
          )}
          variant="ghost"
          size="compact"
        >
          {view === "open" ? "Prüfen" : "Ansehen"}
          <ArrowRight className="h-4 w-4" aria-hidden="true" />
        </ButtonLink>
      }
    >
      <p className="text-sm text-gray-700">
        Geändert: {changedSummary(row.changed_fields)}
      </p>
      {parentNote && (
        <p className="mt-1 text-sm text-gray-600 italic">„{parentNote}“</p>
      )}
    </RequestReviewCard>
  );
}
