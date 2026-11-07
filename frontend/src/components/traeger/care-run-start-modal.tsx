"use client";

// Einen Monatslauf starten (#4684): Die Abrechnung wählt den Monat, den der
// Lauf nach den gültigen Verträgen abrechnet.

import { useEffect, useRef, useState } from "react";
import { CustomSelect } from "~/components/ui/custom-select";
import { EditActions } from "~/components/ui/edit-actions";
import { FormModal } from "~/components/ui/form-modal";
import { useApiFormError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { type CareRun, startCareRun } from "~/lib/traeger-care-billing-api";
import { addMonths, formatMonth, monthRange } from "./care-billing-format";

const logger = createLogger({ component: "CareRunStartModal" });

/** Twelve months back and three ahead of the suggested month. */
const MONTHS_BACK = 12;
const MONTHS_AHEAD = 3;

export function CareRunStartModal({
  isOpen,
  suggestedMonth,
  onClose,
  onStarted,
}: Readonly<{
  isOpen: boolean;
  /** The first month without a released run, "YYYY-MM-01". */
  suggestedMonth: string;
  onClose: () => void;
  onStarted: (run: CareRun) => void;
}>) {
  const [month, setMonth] = useState(suggestedMonth);
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;

  useEffect(() => {
    if (!isOpen) return;
    setMonth(suggestedMonth);
    clearErrors();
  }, [isOpen, suggestedMonth, clearErrors]);

  const options = monthRange(
    addMonths(suggestedMonth, -MONTHS_BACK),
    MONTHS_BACK + MONTHS_AHEAD + 1,
  )
    .reverse()
    .map((value) => ({ value, label: formatMonth(value) }));

  const submit = async () => {
    formErrors.clear();
    setSaving(true);
    try {
      onStarted(await startCareRun(month));
    } catch (err) {
      logger.error("care_run_start_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "der Monatslauf" });
    } finally {
      setSaving(false);
    }
  };

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      closeDisabled={saving}
      isBackdropDismissDisabled
      title="Monatslauf starten"
      size="md"
      error={formErrors.error}
      footer={
        <EditActions
          onCancel={onClose}
          onSave={() => formRef.current?.requestSubmit()}
          saving={saving}
          saveLabel="Lauf starten"
        />
      }
    >
      <form
        ref={formRef}
        noValidate
        className="space-y-3"
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
      >
        <CustomSelect
          id="care-run-month"
          name="billing_month"
          label="Monat"
          error={formErrors.fieldError("billing_month")}
          value={month}
          options={options}
          onChange={setMonth}
        />
        <p className="text-sm text-gray-600">
          Der Lauf rechnet jedes Kind mit gültigem Vertrag in diesem Monat ab.
          Es zählen die gebuchten Tage, nicht die Anwesenheit. Sie prüfen zuerst
          die Vorschau.
        </p>
      </form>
    </FormModal>
  );
}
