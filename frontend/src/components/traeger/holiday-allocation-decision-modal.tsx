"use client";

// Eine Entscheidung über einen offenen Platz (#4700). Das Büro bestätigt sie
// hier, weil die Eltern danach eine E-Mail bekommen. Weicht die Entscheidung
// vom Vorschlag ab, ist eine kurze Begründung Pflicht: sie steht im Verlauf.

import { useRef, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { FormModal } from "~/components/ui/form-modal";
import { Textarea } from "~/components/ui/textarea";
import { useApiFormError } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import {
  type HolidayAllocationEntry,
  type HolidayAllocationPlacement,
  type HolidayAllocationWeek,
  MAX_ALLOCATION_REASON,
} from "~/lib/traeger-holiday-allocation-api";
import { deviatesFromProposal } from "./holiday-allocation-model";
import { formatHolidayWeek } from "./holiday-registration-format";

/** Refusals after which the list is read again instead of shown here. */
const STALE_ALLOCATION_CODES: ReadonlySet<string> = new Set([
  "enrollment.holiday_allocation_changed",
  "enrollment.holiday_allocation_full",
]);

export function isStaleAllocationError(err: unknown): boolean {
  return (
    err instanceof ApiError &&
    err.code !== undefined &&
    STALE_ALLOCATION_CODES.has(err.code)
  );
}

const PLACEMENT_LABELS: Readonly<Record<HolidayAllocationPlacement, string>> = {
  placed: "Platz",
  waitlisted: "Warteliste",
};

export interface AllocationDecisionTarget {
  readonly week: HolidayAllocationWeek;
  readonly entry: HolidayAllocationEntry;
  readonly placement: HolidayAllocationPlacement;
}

const FORM_ID = "holiday-allocation-decision";

export function HolidayAllocationDecisionModal({
  target,
  onClose,
  onConfirm,
  onStale,
}: Readonly<{
  target: AllocationDecisionTarget | null;
  onClose: () => void;
  /** Sends the decision; throws the refusal. */
  onConfirm: (reason: string) => Promise<void>;
  /** The list changed meanwhile: the page reads it again and says so. */
  onStale: (err: unknown) => void;
}>) {
  // A new target starts with an empty reason; the key resets the form.
  return target ? (
    <DecisionForm
      key={`${target.entry.bookingId}-${target.placement}`}
      target={target}
      onClose={onClose}
      onConfirm={onConfirm}
      onStale={onStale}
    />
  ) : null;
}

function DecisionForm({
  target,
  onClose,
  onConfirm,
  onStale,
}: Readonly<{
  target: AllocationDecisionTarget;
  onClose: () => void;
  onConfirm: (reason: string) => Promise<void>;
  onStale: (err: unknown) => void;
}>) {
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const [reason, setReason] = useState("");
  const [saving, setSaving] = useState(false);
  const { entry, week, placement } = target;
  const deviates = deviatesFromProposal(entry, placement);
  const noPlace =
    placement === "placed" && week.free !== null && week.free <= 0;
  const name = `${entry.childFirstName} ${entry.childLastName}`;
  const actionLabel =
    placement === "placed" ? "Platz geben" : "Auf die Warteliste";

  const submit = async () => {
    if (deviates && reason.trim() === "") {
      formErrors.invalid(
        "Sie weichen vom Vorschlag ab. Bitte schreiben Sie kurz, warum.",
        {
          reason: "Bitte geben Sie eine Begründung ein.",
        },
      );
      return;
    }
    formErrors.clear();
    setSaving(true);
    try {
      await onConfirm(deviates ? reason.trim() : "");
    } catch (err) {
      setSaving(false);
      if (isStaleAllocationError(err)) {
        onStale(err);
        return;
      }
      // The backend names the field of the first decision; here it is the one field.
      if (err instanceof ApiError && err.errors) {
        err.errors = err.errors.map((error) => ({
          ...error,
          field: error.field.endsWith(".reason") ? "reason" : error.field,
        }));
      }
      void formErrors.show(err, { object: "die Entscheidung" });
      return;
    }
    setSaving(false);
  };

  return (
    <FormModal
      isOpen
      onClose={onClose}
      closeDisabled={saving}
      isBackdropDismissDisabled={reason !== ""}
      title={
        placement === "placed"
          ? `${name} einen Platz geben?`
          : `${name} auf die Warteliste setzen?`
      }
      size="md"
      error={formErrors.error}
      footer={
        <>
          <Button
            type="button"
            variant="outline"
            size="md"
            className="flex-1"
            disabled={saving}
            onClick={onClose}
          >
            Abbrechen
          </Button>
          <Button
            type="submit"
            form={FORM_ID}
            variant={placement === "placed" ? "success" : "primary"}
            size="md"
            className="flex-1"
            disabled={noPlace}
            isLoading={saving}
            loadingText="Speichert …"
          >
            {actionLabel}
          </Button>
        </>
      }
    >
      <form
        ref={formRef}
        id={FORM_ID}
        noValidate
        className="space-y-4"
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
      >
        <DataGrid>
          <DataField label="Woche">
            {formatHolidayWeek(week.startsOn, week.endsOn)}
          </DataField>
          <DataField label="Vorschlag von moto">
            {PLACEMENT_LABELS[entry.proposal]}, Rang {entry.rank}
          </DataField>
        </DataGrid>
        {noPlace ? (
          <Alert
            type="warning"
            message="In dieser Woche ist kein Platz frei. Erhöhen Sie zuerst im Reiter „Plätze“ die Zahl der Plätze."
          />
        ) : null}
        {deviates && !noPlace ? (
          <Textarea
            name="reason"
            label="Begründung"
            required
            rows={3}
            autoGrow
            maxLength={MAX_ALLOCATION_REASON}
            hint={`Sie weichen vom Vorschlag ab. Die Begründung steht im Verlauf. Noch ${MAX_ALLOCATION_REASON - reason.length} Zeichen.`}
            placeholder="Zum Beispiel: Härtefall nach Gespräch mit der Familie."
            value={reason}
            error={formErrors.fieldError("reason") ?? undefined}
            onChange={(event) => setReason(event.target.value)}
          />
        ) : null}
        <p className="text-sm text-gray-600">
          Die Eltern bekommen danach eine E-Mail.
        </p>
      </form>
    </FormModal>
  );
}
