"use client";

/**
 * Die Historie als eine Liste read-only Zeilen (#2432). Seit #2267 trägt eine
 * entschiedene Zeile zusätzlich „Entscheidung korrigieren", sobald das
 * Backend sie dafür freigibt (`can_correct`).
 */

import { useState } from "react";

import { EnrollmentRequestItem } from "~/components/students/enrollment-request-item";
import { RequestHistoryItem } from "~/components/students/request-history-item";
import { RequestRowHeader } from "~/components/students/request-review-card";
import type { CareWithdrawalCompletion } from "~/lib/care-exit-api";
import type {
  AggregatedHistoryRequest,
  ParentRequestKind,
} from "~/lib/change-request-list-api";
import { itemKey, type AnyItem } from "./case-model";
import {
  DecisionCorrectionDialog,
  type CorrectionTarget,
} from "./decision-correction-dialog";
import { HistoryWithdrawalCard } from "./withdrawal-cards";

const CORRECTABLE_KINDS: readonly string[] = [
  "master_data",
  "care_schedule",
  "offering",
  "excused",
];

/**
 * Arten, bei denen die Schätzung unten KEINE Korrektur anbietet. Achtung: das
 * ist gröber als die Wahrheit. Eine Abholzeit-Änderung liegt in derselben
 * Warteschlange wie ein Wochenplan (`care_schedule`), lässt sich aber sehr
 * wohl zurücknehmen — nur ein Wochenplan nicht, weil sein Schnappschuss
 * Anzeige ist und kein Ausgangszustand. Aus der Zeile allein sind die beiden
 * nicht zu unterscheiden; deshalb entscheidet `can_correct` und die Schätzung
 * greift nur, solange das Feld fehlt.
 */
const UNCORRECTABLE_KINDS: readonly string[] = ["care_schedule", "offering"];

const DECIDED_STATUSES: readonly string[] = ["approved", "rejected"];

/**
 * Darf diese Zeile korrigiert werden? Maßgeblich ist `can_correct` des
 * Backends — es kennt den Unterschied zwischen einer Abholzeit und einem
 * Wochenplan, den diese Zeile nicht sieht. Nur wenn das Feld fehlt, wird
 * vorsichtig geschätzt: eine entschiedene Zeile einer Art, die eine Rücknahme
 * sicher zulässt. Die Schätzung verschweigt dabei die Abholzeit-Korrektur;
 * das ist der Preis dafür, keine Korrektur anzubieten, die sicher scheitert.
 */
function mayCorrect(
  canCorrect: boolean | undefined,
  requestType: string,
  status: string | undefined,
): boolean {
  if (canCorrect === true) return true;
  if (canCorrect === false) return false;
  return (
    DECIDED_STATUSES.includes(status ?? "") &&
    !UNCORRECTABLE_KINDS.includes(requestType)
  );
}

function correctionTarget(item: AnyItem): CorrectionTarget | null {
  const row = item as AggregatedHistoryRequest & {
    can_correct?: boolean;
    expected_version?: string;
  };
  if (!CORRECTABLE_KINDS.includes(item.request_type)) return null;
  const data = item.data as {
    id: string;
    status?: string;
    decided_at?: string;
    decided_by_name?: string;
    reason?: string;
    decision_reason?: string;
    review_reason?: string;
    first_name?: string;
    last_name?: string;
    student_name?: string;
  };
  if (!mayCorrect(row.can_correct, item.request_type, data.status)) return null;
  return {
    kind: item.request_type as ParentRequestKind,
    requestID: data.id,
    expectedVersion: row.expected_version ?? "",
    childName:
      data.student_name ??
      `${data.first_name ?? ""} ${data.last_name ?? ""}`.trim(),
    priorStatus: data.status ?? "",
    priorDecidedAt: data.decided_at,
    priorDecidedBy: data.decided_by_name,
    priorReason: data.reason ?? data.decision_reason ?? data.review_reason,
  };
}

export function HistoryRequestList({
  items,
  withdrawals,
  reasonRequired,
  onCorrected,
}: Readonly<{
  items: readonly AnyItem[];
  withdrawals: readonly CareWithdrawalCompletion[];
  reasonRequired: boolean;
  onCorrected: (notice: string) => void;
}>) {
  const [target, setTarget] = useState<CorrectionTarget | null>(null);
  return (
    <div className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
      <RequestRowHeader view="history" />
      {withdrawals.map((row) => (
        <HistoryWithdrawalCard key={`care_withdrawal:${row.id}`} row={row} />
      ))}
      {items.map((item) => {
        if (item.request_type === "enrollment") {
          return (
            <EnrollmentRequestItem
              key={itemKey(item)}
              row={item.data}
              view="history"
            />
          );
        }
        const correctable = correctionTarget(item);
        return (
          <RequestHistoryItem
            key={itemKey(item)}
            item={item as AggregatedHistoryRequest}
            onCorrect={correctable ? () => setTarget(correctable) : undefined}
          />
        );
      })}
      {target && (
        <DecisionCorrectionDialog
          target={target}
          reasonRequired={reasonRequired}
          onClose={() => setTarget(null)}
          onCorrected={(notice: string) => {
            setTarget(null);
            onCorrected(notice);
          }}
        />
      )}
    </div>
  );
}
