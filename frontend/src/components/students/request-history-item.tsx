"use client";

/**
 * Eine entschiedene Eltern-Anfrage beliebiger Art als read-only Karte (wer
 * hat wann was mit welcher Begründung entschieden). Gerendert von der
 * aggregierten Anfragenliste (#2432); die Kartenvarianten stammen aus der
 * früheren per-Art-Historie (#2417).
 */

import { formatDate } from "~/lib/date-helpers";
import type { AggregatedHistoryRequest } from "~/lib/change-request-list-api";
import type { StaffCareRequestHistoryEntry } from "~/lib/care-request-review-api";
import type { StaffExcusedRequestHistoryEntry } from "~/lib/excused-request-review-api";
import type { StaffMasterDataHistoryEntry } from "~/lib/master-data-review-api";
import type { StaffOfferingRequestHistoryEntry } from "~/lib/offering-request-review-api";
import {
  fieldLabel,
  formatValue as formatMasterDataValue,
} from "~/components/students/master-data-review-item";
import {
  RequestReviewCard,
  ReviewDiffPanel,
} from "~/components/students/request-review-card";

function MasterDataHistoryCard({
  row,
}: Readonly<{ row: StaffMasterDataHistoryEntry }>) {
  return (
    <RequestReviewCard
      childName={`${row.first_name} ${row.last_name}`}
      summary={`Stammdaten · ${fieldLabel(row.field_key)}`}
      submittedAt={row.created_at}
      history={{
        status: row.status,
        decidedAt: row.decided_at,
        decidedByName: row.decided_by_name,
        reason: row.review_reason,
      }}
    >
      <p className="text-sm text-gray-700">
        {formatMasterDataValue(row.field_key, row.old_value, "leer")}
        {" → "}
        {formatMasterDataValue(row.field_key, row.new_value, "leer")}
      </p>
    </RequestReviewCard>
  );
}

function CareHistoryCard({
  row,
}: Readonly<{ row: StaffCareRequestHistoryEntry }>) {
  // Frozen decision-time diff (#2430) when present, payload summary otherwise.
  const showDiff = (row.diff?.length ?? 0) > 0;
  const entries = showDiff ? (row.diff ?? []) : row.requested;
  return (
    <RequestReviewCard
      childName={`${row.first_name} ${row.last_name}`}
      summary={
        row.request_kind === "pickup_change" ? "Abholzeit" : "Betreuungszeiten"
      }
      submittedAt={row.created_at}
      history={{
        status: row.status,
        decidedAt: row.decided_at,
        decidedByName: row.decided_by_name,
        reason: row.decision_reason,
      }}
    >
      {entries.length > 0 && (
        <ReviewDiffPanel title={showDiff ? "Änderungen" : "Beantragt"}>
          {entries.map((entry) => (
            <p
              key={`${entry.label}-${entry.new}`}
              className="text-sm text-gray-700"
            >
              {showDiff
                ? `${entry.label}: ${entry.old || "—"} → ${entry.new}`
                : `${entry.label}: ${entry.new}`}
            </p>
          ))}
        </ReviewDiffPanel>
      )}
    </RequestReviewCard>
  );
}

function OfferingHistoryCard({
  row,
}: Readonly<{ row: StaffOfferingRequestHistoryEntry }>) {
  return (
    <RequestReviewCard
      childName={row.student_name}
      summary="Betreuungsangebote und AGs"
      submittedAt={row.created_at}
      history={{
        status: row.status,
        decidedAt: row.decided_at,
        decidedByName: row.decided_by_name,
        reason: row.reason,
      }}
    >
      {row.diff.length > 0 && (
        <ReviewDiffPanel title="Änderungen">
          {row.diff.map((line) => (
            <p key={line.offering_id} className="text-sm text-gray-700">
              {line.label}: {line.old} → {line.new}
            </p>
          ))}
        </ReviewDiffPanel>
      )}
      {row.diff.length === 0 && (row.requested?.length ?? 0) > 0 && (
        <ReviewDiffPanel title="Beantragt">
          {(row.requested ?? []).map((line) => (
            <p key={line.offering_id} className="text-sm text-gray-700">
              {line.label}: {line.new}
            </p>
          ))}
        </ReviewDiffPanel>
      )}
    </RequestReviewCard>
  );
}

function ExcusedHistoryCard({
  row,
}: Readonly<{ row: StaffExcusedRequestHistoryEntry }>) {
  return (
    <RequestReviewCard
      childName={`${row.first_name} ${row.last_name}`}
      summary={
        row.absence_status === "sick"
          ? "Krankmeldung"
          : "Entschuldigte Abmeldung"
      }
      submittedAt={row.created_at}
      history={{
        status: row.status,
        decidedAt: row.decided_at,
        decidedByName: row.decided_by_name,
        reason: row.reason,
      }}
    >
      <p className="text-sm text-gray-700">
        {row.dates.map((d) => formatDate(d)).join(", ")}
        {row.note ? ` · ${row.note}` : ""}
      </p>
    </RequestReviewCard>
  );
}

export function RequestHistoryItem({
  item,
}: Readonly<{ item: AggregatedHistoryRequest }>) {
  switch (item.request_type) {
    case "master_data":
      return <MasterDataHistoryCard row={item.data} />;
    case "care_schedule":
      return <CareHistoryCard row={item.data} />;
    case "offering":
      return <OfferingHistoryCard row={item.data} />;
    case "excused":
      return <ExcusedHistoryCard row={item.data} />;
    case "direct_correction": {
      const row = item.data;
      return (
        <RequestReviewCard
          childName={row.student_name}
          summary="Betreuungsangebote und AGs"
          history={{
            kind: "correction",
            decidedAt: row.changed_at,
            decidedByName: row.changed_by_name,
            reason: row.reason,
          }}
        >
          <ReviewDiffPanel title="Änderungen">
            {row.diff.map((line) => (
              <p key={line.offering_id} className="text-sm text-gray-700">
                {line.label}: {line.old} → {line.new}
              </p>
            ))}
            {/* Auch eine Korrektur ohne Tagesänderung bleibt in der Historie
                stehen (etwa nur der Wechsel zwischen selbst gesetzten und
                automatisch übernommenen Tagen). Sie sagt das hin, statt eine
                leere Fläche zu zeigen. */}
            {row.diff.length === 0 && (
              <p className="text-sm text-gray-600">
                Keine Änderung an den gebuchten Tagen
              </p>
            )}
          </ReviewDiffPanel>
        </RequestReviewCard>
      );
    }
  }
}
