"use client";

import { useRef, useState } from "react";
import { Button } from "~/components/ui/button";
import { CheckboxCard } from "~/components/ui/checkbox-card";
import { ISODatePicker } from "~/components/ui/date-picker";
import { FormModal } from "~/components/ui/form-modal";
import { useFormModalPending } from "~/components/ui/use-form-modal-pending";
import { Textarea } from "~/components/ui/textarea";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import {
  berlinTodayISO,
  formatDate,
  parseISODate,
  toISODate,
} from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  CLARIFICATION_REASON_LABELS,
  belegRetentionService,
  belegTypeLabel,
  type BelegClarification,
} from "~/lib/operator/beleg-retention-api";

const logger = createLogger({ component: "BelegClarificationReviewModal" });

/** Längste Spanne bis zur nächsten Prüfung, wie das Backend sie prüft. */
const MAX_REVIEW_DAYS = 365;

function shiftDays(day: string, days: number): string {
  const date = parseISODate(day);
  date.setDate(date.getDate() + days);
  return toISODate(date);
}

/**
 * Eskalation ist Pflicht, sobald das Prüfdatum des Eintrags erreicht ist
 * oder die Prüfung es nach hinten schiebt (Löschkonzept Abschnitt 3); das
 * Backend lehnt die Prüfung sonst mit 400 ab.
 */
function escalationRequired(
  reviewOn: string,
  nextReviewOn: string,
  today: string,
): boolean {
  return reviewOn <= today || (nextReviewOn !== "" && nextReviewOn > reviewOn);
}

/**
 * Prüfung eines Eintrags der Klärungsliste (Löschkonzept Abschnitt 3): was
 * mit dem Träger geklärt ist, gegebenenfalls die Eskalation an Florian mit
 * Grund und das nächste Prüfdatum. Ist der Eintrag an seinem Prüfdatum noch
 * offen, geht er an Florian. Löscht nichts.
 */
export function ClarificationReviewModal({
  entry,
  organizationId,
  onClose,
  onSaved,
}: {
  readonly entry: BelegClarification | null;
  readonly organizationId: string;
  readonly onClose: () => void;
  readonly onSaved: () => Promise<unknown>;
}) {
  const [note, setNote] = useState("");
  const [escalated, setEscalated] = useState(false);
  const [escalationReason, setEscalationReason] = useState("");
  const [nextReviewOn, setNextReviewOn] = useState("");
  const save = useFormModalPending(
    entry !== null,
    entry ? `${entry.belegType}/${entry.reason}` : undefined,
  );
  const saving = save.pending;
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { success } = useToast();
  const today = berlinTodayISO();
  const required =
    entry !== null && escalationRequired(entry.reviewOn, nextReviewOn, today);
  const escalating = escalated || required;
  const complete =
    note.trim() !== "" &&
    nextReviewOn !== "" &&
    (!escalating || escalationReason.trim() !== "");

  const close = () => {
    setNote("");
    setEscalated(false);
    setEscalationReason("");
    setNextReviewOn("");
    formErrors.clear();
    onClose();
  };

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!entry || !complete) return;
    await save.run(async (op) => {
      formErrors.clear();
      try {
        await belegRetentionService.recordClarificationReview(organizationId, {
          beleg_type: entry.belegType,
          reason: entry.reason,
          note: note.trim(),
          escalated: escalating,
          escalation_reason: escalating ? escalationReason.trim() : "",
          next_review_on: nextReviewOn,
        });
        success("Die Prüfung ist erfasst.");
        if (op.isCurrent()) close();
        await onSaved();
      } catch (saveError) {
        logger.warn("beleg_clarification_review_failed", {
          error:
            saveError instanceof Error ? saveError.message : String(saveError),
        });
        if (!op.isCurrent()) return;
        void formErrors.show(saveError, { object: "die Prüfung" });
      }
    });
  };

  return (
    <FormModal
      isOpen={entry !== null}
      onClose={close}
      pending={save}
      title="Prüfung erfassen"
      error={formErrors.error}
      isBackdropDismissDisabled
      footer={
        <Button
          type="submit"
          form="beleg-clarification-review-form"
          variant="primary"
          size="md"
          disabled={!complete || saving}
          isLoading={saving}
          loadingText="Wird erfasst..."
        >
          Prüfung erfassen
        </Button>
      }
    >
      <form
        id="beleg-clarification-review-form"
        ref={formRef}
        onSubmit={(event) => void submit(event)}
        className="space-y-4"
      >
        {entry && (
          <p className="text-sm text-gray-700">
            {belegTypeLabel(entry.belegType)}:{" "}
            {CLARIFICATION_REASON_LABELS[entry.reason] ?? entry.reason}. moto
            löscht diese Belege weiter nicht.
          </p>
        )}
        <Textarea
          id="beleg-clarification-review-note"
          label="Ergebnis der Prüfung"
          value={note}
          onChange={(event) => setNote(event.target.value)}
          hint="Was ist mit dem Träger geklärt, was fehlt noch?"
          maxLength={1000}
          required
        />
        <CheckboxCard
          checked={escalating}
          onChange={setEscalated}
          disabled={required}
          label="An Florian eskalieren"
          hint={escalationHint(entry, today, required)}
        />
        {escalating && (
          <Textarea
            id="beleg-clarification-review-escalation"
            label="Grund der Eskalation"
            value={escalationReason}
            onChange={(event) => setEscalationReason(event.target.value)}
            maxLength={1000}
            required
          />
        )}
        <ISODatePicker
          id="beleg-clarification-review-next"
          label="Nächste Prüfung"
          value={nextReviewOn}
          onChange={setNextReviewOn}
          min={shiftDays(today, 1)}
          max={shiftDays(today, MAX_REVIEW_DAYS)}
          required
        />
      </form>
    </FormModal>
  );
}

function escalationHint(
  entry: BelegClarification | null,
  today: string,
  required: boolean,
): string {
  if (!entry || !required) {
    return "Wenn die Zuordnung nach der Prüfung noch offen ist.";
  }
  if (entry.reviewOn <= today) {
    return "Das Prüfdatum ist erreicht. Die offene Zuordnung geht an Florian.";
  }
  return `Ein Prüfdatum nach dem ${formatDate(entry.reviewOn)} geht nur mit Eskalation.`;
}
