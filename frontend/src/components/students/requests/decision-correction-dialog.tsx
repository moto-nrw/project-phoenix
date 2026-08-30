"use client";

/**
 * „Entscheidung korrigieren" (#2267). Eine gefallene Entscheidung wird
 * ersetzt, die alte bleibt im Verlauf stehen. Bewusst ein eigener Dialog und
 * keine Schaltfläche in der Zeile: eine Korrektur ändert echte Daten des
 * Kindes und darf nicht aus Versehen passieren.
 */

import { useState } from "react";

import { Alert } from "~/components/ui/alert";
import { ConfirmationModal } from "~/components/ui/modal";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { Textarea } from "~/components/ui/textarea";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  ChangeRequestStaleError,
  correctRequestDecision,
  type ParentRequestKind,
} from "~/lib/change-request-list-api";
import { STALE_REQUEST_NOTICE } from "./request-copy";

const logger = createLogger({ component: "DecisionCorrectionDialog" });

export interface CorrectionTarget {
  readonly kind: ParentRequestKind;
  readonly requestID: string;
  readonly expectedVersion: string;
  readonly childName: string;
  readonly priorStatus: string;
  readonly priorDecidedAt?: string;
  readonly priorDecidedBy?: string;
  readonly priorReason?: string;
}

function priorLine(target: CorrectionTarget): string {
  const verb = target.priorStatus === "approved" ? "Freigegeben" : "Abgelehnt";
  const date = target.priorDecidedAt
    ? ` am ${formatDate(target.priorDecidedAt)}`
    : "";
  const person = target.priorDecidedBy ? ` von ${target.priorDecidedBy}` : "";
  return `Bisher: ${verb}${date}${person}`;
}

export function DecisionCorrectionDialog({
  target,
  reasonRequired,
  onClose,
  onCorrected,
}: Readonly<{
  target: CorrectionTarget;
  /** Verlangt die Schule beim Entscheiden eine Begründung? */
  reasonRequired: boolean;
  onClose: () => void;
  onCorrected: (notice: string) => void;
}>) {
  // Vorbelegt auf das Gegenteil der bisherigen Entscheidung: wer korrigiert,
  // will fast immer das andere Ergebnis.
  const [approve, setApprove] = useState(target.priorStatus !== "approved");
  const [reason, setReason] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const save = async () => {
    const trimmed = reason.trim();
    // Eine Korrektur braucht immer einen Grund: sie widerspricht einer
    // Entscheidung, die jemand anderes getroffen hat.
    if (trimmed === "") {
      setError("Bitte tragen Sie ein, warum Sie korrigieren.");
      return;
    }
    setSaving(true);
    setError(null);
    try {
      await correctRequestDecision(target.kind, target.requestID, {
        approve,
        reason: trimmed,
        expectedVersion: target.expectedVersion,
      });
      onCorrected(
        "Die Entscheidung wurde korrigiert. Die alte Entscheidung bleibt gespeichert.",
      );
    } catch (err) {
      logger.warn("parent_request_correction_failed", {
        error: err instanceof Error ? err.message : String(err),
        kind: target.kind,
      });
      setError(
        err instanceof ChangeRequestStaleError
          ? STALE_REQUEST_NOTICE
          : err instanceof Error
            ? err.message
            : "Die Korrektur konnte nicht gespeichert werden.",
      );
      setSaving(false);
    }
  };

  return (
    <ConfirmationModal
      isOpen
      onClose={onClose}
      onConfirm={() => void save()}
      title="Entscheidung korrigieren?"
      confirmText="Korrektur speichern"
      cancelText="Zurück"
      isConfirmLoading={saving}
      isDismissDisabled={saving}
      mobileSheet
    >
      <div className="space-y-3">
        <p className="text-sm text-gray-700">
          {target.childName ? `${target.childName}. ` : ""}
          {priorLine(target)}
        </p>
        {target.priorReason && (
          <p className="text-sm text-gray-600 italic">„{target.priorReason}“</p>
        )}
        <div className="space-y-1">
          <p className="text-sm font-medium text-gray-800">
            Was soll jetzt gelten?
          </p>
          <SegmentedControl
            ariaLabel="Neues Ergebnis"
            value={approve ? "approve" : "reject"}
            onChange={(value) => setApprove(value === "approve")}
            items={[
              { value: "approve", label: "Freigeben" },
              { value: "reject", label: "Ablehnen" },
            ]}
          />
        </div>
        <label
          htmlFor="correction-reason"
          className="block space-y-1 text-sm font-medium text-gray-800"
        >
          <span>
            Warum korrigieren Sie?
            {reasonRequired ? "" : " Pflicht bei einer Korrektur."}
          </span>
          <Textarea
            id="correction-reason"
            value={reason}
            rows={2}
            disabled={saving}
            onChange={(event) => {
              setReason(event.target.value);
              setError(null);
            }}
          />
        </label>
        {error && <Alert type="warning" message={error} />}
        <p className="text-sm text-gray-600">
          Die alte Entscheidung bleibt im Verlauf stehen.
        </p>
      </div>
    </ConfirmationModal>
  );
}
