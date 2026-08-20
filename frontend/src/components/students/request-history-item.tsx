"use client";

/**
 * Eine entschiedene Eltern-Anfrage beliebiger Art als read-only Karte (wer
 * hat wann was mit welcher Begründung entschieden). Gerendert von der
 * aggregierten Anfragenliste (#2432); die Kartenvarianten stammen aus der
 * früheren per-Art-Historie (#2417).
 */

import { formatDate } from "~/lib/date-helpers";
import type {
  AggregatedHistoryRequest,
  DirectCorrection,
} from "~/lib/change-request-list-api";
import type { StaffCareRequestHistoryEntry } from "~/lib/care-request-review-api";
import type { StaffExcusedRequestHistoryEntry } from "~/lib/excused-request-review-api";
import type { StaffMasterDataHistoryEntry } from "~/lib/master-data-review-api";
import type { StaffOfferingRequestHistoryEntry } from "~/lib/offering-request-review-api";
import {
  fieldLabel,
  formatValue as formatMasterDataValue,
} from "~/components/students/master-data-review-item";
import { RequestReviewCard } from "~/components/students/request-review-card";

/**
 * Die Änderungs-/Beantragt-Liste einer Historien-Karte: eine Zeile je Angebot
 * bzw. Feld, in der ruhigen grauen Fläche der Review-Karten.
 */
function DiffList({
  title,
  lines,
}: Readonly<{
  title: string;
  lines: readonly { key: string; text: string }[];
}>) {
  if (lines.length === 0) return null;
  return (
    <div className="space-y-1 rounded-lg bg-gray-50 p-3">
      <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
        {title}
      </p>
      {lines.map((line) => (
        <p key={line.key} className="text-sm text-gray-700">
          {line.text}
        </p>
      ))}
    </div>
  );
}

/** Angebots-Zeilen („Mittagessen: Mo → abgemeldet") aus einem Diff. */
function offeringDiffLines(
  diff: readonly {
    offering_id: string;
    label: string;
    old: string;
    new: string;
  }[],
) {
  return diff.map((line) => ({
    key: line.offering_id,
    text: `${line.label}: ${line.old} → ${line.new}`,
  }));
}

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
      <DiffList
        title={showDiff ? "Änderungen" : "Beantragt"}
        lines={entries.map((entry) => ({
          key: `${entry.label}-${entry.new}`,
          text: showDiff
            ? `${entry.label}: ${entry.old || "—"} → ${entry.new}`
            : `${entry.label}: ${entry.new}`,
        }))}
      />
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
      <DiffList title="Änderungen" lines={offeringDiffLines(row.diff)} />
      {row.diff.length === 0 && (
        <DiffList
          title="Beantragt"
          lines={(row.requested ?? []).map((line) => ({
            key: line.offering_id,
            text: `${line.label}: ${line.new}`,
          }))}
        />
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
      summary="Entschuldigte Abmeldung"
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

/**
 * Eine Direkt-Korrektur der Verwaltung (#2436). Sie ist keine Anfrage: es gibt
 * niemanden, der sie eingereicht hat, und nichts, was entschieden wurde — nur
 * wer wann was geändert hat und warum.
 */
function DirectCorrectionCard({ row }: Readonly<{ row: DirectCorrection }>) {
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
      <DiffList title="Änderungen" lines={offeringDiffLines(row.diff)} />
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
    case "direct_correction":
      return <DirectCorrectionCard row={item.data} />;
  }
}
