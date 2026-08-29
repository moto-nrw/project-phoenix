"use client";

/**
 * Eine Anmeldungsänderung als Zeile der Eltern-Liste (#2435): der Wunsch einer
 * Familie, ihre Anmeldung nach dem Absenden noch zu ändern. Entschieden wird
 * sie weiterhin in der Detailansicht mit Rückfrage-Dialog — die Karte führt
 * dorthin, statt die Freigabe ein zweites Mal nachzubauen.
 *
 * Sie nutzt dieselbe Lese-Karte wie die übrigen Zeilen der Liste, damit die
 * Arten nebeneinander gleich aussehen, und zeigt wie diese das echte
 * vorher → nachher statt nur der Namen der geänderten Bereiche.
 */

import { useMemo } from "react";
import { ArrowRight } from "lucide-react";

import { enrollmentChangeRequestStatusMeta } from "~/components/enrollment/enrollment-change-request-status";
import {
  RequestReviewCard,
  ReviewDiffPanel,
} from "~/components/students/request-review-card";
import { ButtonLink } from "~/components/ui/button";
import type { EnrollmentChangeRequest } from "~/lib/change-request-list-api";
import {
  buildEnrollmentChangeRequestDiffGroups,
  formatEnrollmentChangeRequestValue,
} from "~/lib/enrollment-change-request-diff";
import { useTenantAwarePath } from "~/lib/tenant-path";

/**
 * Wie viele geänderte Felder eine Zeile zeigt. Eine Anmeldungsänderung kann
 * das ganze Formular betreffen; ungekürzt füllte eine einzige Zeile die
 * Liste. Der Rest steht in der Detailansicht.
 */
const MAX_DIFF_ROWS = 6;

export function EnrollmentRequestItem({
  row,
  view,
  grouped = false,
}: Readonly<{
  row: EnrollmentChangeRequest;
  view: "open" | "history";
  grouped?: boolean;
}>) {
  const tenantPath = useTenantAwarePath();
  const meta = enrollmentChangeRequestStatusMeta(row.status);
  const childNames =
    row.child_names.length > 0
      ? row.child_names.join(", ")
      : (row.guardian_name ?? "Anmeldung");
  const parentNote = row.parent_note?.trim();

  // Dieselbe Aufbereitung wie in der Detailansicht — ein zweiter Vergleich
  // wäre ein zweites Verständnis davon, was sich geändert hat.
  const rows = useMemo(
    () =>
      buildEnrollmentChangeRequestDiffGroups({
        baseSnapshot: row.base_snapshot,
        proposedSnapshot: row.proposed_snapshot,
        diff: row.diff,
      }).flatMap((group) => group.rows),
    [row.base_snapshot, row.proposed_snapshot, row.diff],
  );
  const shown = rows.slice(0, MAX_DIFF_ROWS);
  const hidden = rows.length - shown.length;

  return (
    <RequestReviewCard
      type="enrollment"
      childName={childNames}
      grouped={grouped}
      summary={row.origin === "admin" ? "Korrektur der OGS" : undefined}
      submittedAt={row.created_at}
      submittedByName={row.origin === "admin" ? undefined : row.guardian_name}
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
      {shown.length > 0 ? (
        <ReviewDiffPanel title="Änderungen">
          {shown.map((entry) => (
            <p key={entry.id} className="text-sm text-gray-700">
              {entry.label}:{" "}
              {formatEnrollmentChangeRequestValue(entry.before, "leer")} →{" "}
              {formatEnrollmentChangeRequestValue(entry.after, "leer")}
            </p>
          ))}
          {hidden > 0 && (
            <p className="text-sm text-gray-500">
              und {hidden} weitere {hidden === 1 ? "Änderung" : "Änderungen"}
            </p>
          )}
        </ReviewDiffPanel>
      ) : (
        <p className="text-sm text-gray-600">Keine erkennbare Feldänderung</p>
      )}
      {parentNote && (
        <p className="mt-1 text-sm text-gray-600 italic">„{parentNote}“</p>
      )}
    </RequestReviewCard>
  );
}
