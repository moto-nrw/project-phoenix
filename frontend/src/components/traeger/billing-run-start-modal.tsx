"use client";

// Einen Abrechnungslauf starten: für einen Ferienzeitraum (#4668) oder für
// einen Monat der Monatsbeiträge (#4684). Beide wählen eine Sache aus einer
// Liste, starten den Lauf und öffnen danach seine Vorschau.

import { useEffect, useRef, useState } from "react";
import { CustomSelect } from "~/components/ui/custom-select";
import { EditActions } from "~/components/ui/edit-actions";
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

interface RunStartOption {
  readonly value: string;
  readonly label: string;
}

/**
 * The dialog that starts a run: one choice from a list, a line that says
 * what the run bills, and the start. `onStart` starts the run; a refusal
 * shows at the top of the form.
 */
export function RunStartModal({
  isOpen,
  title,
  field,
  options,
  initialValue,
  hint,
  object,
  onClose,
  onStart,
}: Readonly<{
  isOpen: boolean;
  title: string;
  /** The field: its id, the name the backend reports it by, its label. */
  field: {
    readonly id: string;
    readonly name: string;
    readonly label: string;
    readonly placeholder?: string;
    readonly missing: string;
  };
  options: readonly RunStartOption[];
  /** Preselected when it is one of the options, else the first option. */
  initialValue?: string;
  hint: string;
  /** The run in the error text, e.g. „der Lauf“. */
  object: string;
  onClose: () => void;
  onStart: (value: string) => Promise<void>;
}>) {
  const [value, setValue] = useState("");
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;

  // The options are built on every render; their values decide the choice.
  const optionValues = options.map((option) => option.value).join("\n");
  useEffect(() => {
    if (!isOpen) return;
    const values = optionValues ? optionValues.split("\n") : [];
    setValue(
      initialValue && values.includes(initialValue)
        ? initialValue
        : (values[0] ?? ""),
    );
    clearErrors();
  }, [isOpen, optionValues, initialValue, clearErrors]);

  const submit = async () => {
    formErrors.clear();
    if (!value) {
      formErrors.invalid("Bitte prüfen Sie Ihre Angaben.", {
        [field.name]: field.missing,
      });
      return;
    }
    setSaving(true);
    try {
      await onStart(value);
    } catch (err) {
      logger.error("billing_run_start_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object });
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
      title={title}
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
          id={field.id}
          name={field.name}
          label={field.label}
          error={formErrors.fieldError(field.name)}
          placeholder={field.placeholder}
          value={value}
          options={options}
          onChange={setValue}
        />
        <p className="text-sm text-gray-600">{hint}</p>
      </form>
    </FormModal>
  );
}

export function BillingRunStartModal({
  isOpen,
  periods,
  initialPeriodId,
  onClose,
  onStarted,
}: Readonly<{
  isOpen: boolean;
  periods: readonly BillingPeriod[];
  /** Vorgewählt, wenn die Liste auf diesen Zeitraum gefiltert ist (#5688). */
  initialPeriodId?: string;
  onClose: () => void;
  onStarted: (run: BillingRun) => void;
}>) {
  return (
    <RunStartModal
      isOpen={isOpen}
      title="Lauf starten"
      field={{
        id: "billing-run-period",
        name: "holiday_period_id",
        label: "Ferienzeitraum",
        placeholder: "Ferienzeitraum wählen",
        missing: "Bitte wählen Sie einen Ferienzeitraum.",
      }}
      options={periods.map((period) => ({
        value: period.id,
        label: `${period.name} (${formatDateRange(period.startsOn, period.endsOn)})`,
      }))}
      initialValue={initialPeriodId}
      hint="Der Lauf rechnet alle bestätigten Anmeldungen ab, die noch keine Rechnung haben. Sie prüfen zuerst die Vorschau."
      object="der Lauf"
      onClose={onClose}
      onStart={async (periodId) => onStarted(await startBillingRun(periodId))}
    />
  );
}
