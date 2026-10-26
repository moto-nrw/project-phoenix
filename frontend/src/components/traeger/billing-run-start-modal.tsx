"use client";

// Einen Abrechnungslauf für einen Ferienzeitraum starten (#4668).

import { useEffect, useRef, useState } from "react";
import { CustomSelect } from "~/components/ui/custom-select";
import { EditActions } from "~/components/ui/edit-actions";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { FormModal } from "~/components/ui/form-modal";
import { useApiFormError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  type BillingPeriod,
  type BillingRun,
  startBillingRun,
} from "~/lib/traeger-billing-api";
import { formatDateRange } from "./holiday-period-format";

const logger = createLogger({ component: "BillingRunStartModal" });

export function BillingRunStartModal({
  isOpen,
  periods,
  onClose,
  onStarted,
}: Readonly<{
  isOpen: boolean;
  periods: readonly BillingPeriod[];
  onClose: () => void;
  onStarted: (run: BillingRun) => void;
}>) {
  const [periodId, setPeriodId] = useState("");
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;

  useEffect(() => {
    if (!isOpen) return;
    setPeriodId(periods[0]?.id ?? "");
    clearErrors();
  }, [isOpen, periods, clearErrors]);

  const submit = async () => {
    formErrors.clear();
    if (!periodId) {
      formErrors.invalid("Bitte prüfen Sie Ihre Angaben.", {
        holiday_period_id: "Bitte wählen Sie einen Ferienzeitraum.",
      });
      return;
    }
    setSaving(true);
    try {
      onStarted(await startBillingRun(periodId));
    } catch (err) {
      logger.error("billing_run_start_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "der Lauf" });
    } finally {
      setSaving(false);
    }
  };

  const periodError = formErrors.fieldError("holiday_period_id");
  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      closeDisabled={saving}
      isBackdropDismissDisabled
      title="Lauf starten"
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
        <span
          id="billing-run-period-label"
          className="mb-2 block text-sm font-medium text-gray-700"
        >
          Ferienzeitraum
        </span>
        <CustomSelect
          name="holiday_period_id"
          ariaLabelledBy="billing-run-period-label"
          ariaDescribedBy={periodError ? "billing-run-period-error" : undefined}
          invalid={Boolean(periodError)}
          placeholder="Ferienzeitraum wählen"
          value={periodId}
          options={periods.map((period) => ({
            value: period.id,
            label: `${period.name} (${formatDateRange(period.startsOn, period.endsOn)})`,
          }))}
          onChange={setPeriodId}
        />
        <FieldErrorHint id="billing-run-period-error" message={periodError} />
        <p className="text-sm text-gray-600">
          Der Lauf rechnet alle bestätigten Anmeldungen ab, die noch keine
          Rechnung haben. Sie prüfen zuerst die Vorschau.
        </p>
      </form>
    </FormModal>
  );
}
