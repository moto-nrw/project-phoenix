"use client";

// Einen Abrechnungslauf starten: für einen Ferienzeitraum (#4668) oder für
// einen Monat der Monatsbeiträge (#4684). Beide wählen eine Sache aus einer
// Liste, starten den Lauf und öffnen danach seine Vorschau.

import { useEffect, useRef, useState } from "react";
import { Alert } from "~/components/ui/alert";
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
 * Choices that cannot start a run now because one of their runs is still a
 * preview: the dialog says so and opens that run instead.
 */
interface BlockedChoices {
  readonly values: readonly string[];
  readonly hint: string;
  readonly onOpen: (value: string) => void;
}

/**
 * The dialog that starts a run: one choice from a list, a line that says
 * what the run bills, and the start. `onStart` starts the run; a refusal
 * shows at the top of the form.
 */
function RunStartModal({
  isOpen,
  title,
  field,
  options,
  initialValue,
  hint,
  object,
  blocked,
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
  /**
   * Preselected when it is one of the options, else the first option that
   * can start, else the first option.
   */
  initialValue?: string;
  hint: string;
  /** The run in the error text, e.g. „der Lauf“. */
  object: string;
  blocked?: BlockedChoices;
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
  const blockedValues = (blocked?.values ?? []).join("\n");
  useEffect(() => {
    if (!isOpen) return;
    const values = optionValues ? optionValues.split("\n") : [];
    const taken = new Set(blockedValues ? blockedValues.split("\n") : []);
    setValue(
      initialValue && values.includes(initialValue)
        ? initialValue
        : (values.find((option) => !taken.has(option)) ?? values[0] ?? ""),
    );
    clearErrors();
  }, [isOpen, optionValues, blockedValues, initialValue, clearErrors]);
  const isBlocked = !!value && !!blocked?.values.includes(value);

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
      // Opened from the page head: the title follows the page's h1.
      titleLevel={2}
      size="md"
      error={formErrors.error}
      footer={
        <EditActions
          onCancel={onClose}
          onSave={() =>
            isBlocked
              ? blocked?.onOpen(value)
              : formRef.current?.requestSubmit()
          }
          saving={saving}
          saveLabel={isBlocked ? "Offenen Lauf öffnen" : title}
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
          onChange={(next) => {
            setValue(next);
            formErrors.clear();
          }}
          wrapLabels
        />
        {/* A choice with a run in preview is a state to see, not a hint
            to read past: a notice, as in the monthly start dialog. */}
        {isBlocked && blocked ? (
          <Alert type="info" message={blocked.hint} />
        ) : (
          <p className="text-sm text-gray-600">{hint}</p>
        )}
      </form>
    </FormModal>
  );
}

export function BillingRunStartModal({
  isOpen,
  periods,
  initialPeriodId,
  openRunIds,
  onOpenRun,
  onClose,
  onStarted,
}: Readonly<{
  isOpen: boolean;
  periods: readonly BillingPeriod[];
  /** Vorgewählt, wenn die Liste auf diesen Zeitraum gefiltert ist (#5688). */
  initialPeriodId?: string;
  /** The run still in preview per holiday period: no second one starts. */
  openRunIds?: ReadonlyMap<string, string>;
  onOpenRun?: (runId: string) => void;
  onClose: () => void;
  onStarted: (run: BillingRun) => void;
}>) {
  const open = openRunIds ?? new Map<string, string>();
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
        label: `${period.name} (${formatDateRange(period.startsOn, period.endsOn)})${open.has(period.id) ? " · Vorschau offen" : ""}`,
      }))}
      initialValue={initialPeriodId}
      blocked={
        onOpenRun
          ? {
              values: [...open.keys()],
              hint: "Für diesen Ferienzeitraum ist schon ein Lauf in der Vorschau. Öffnen Sie ihn, statt einen neuen zu starten.",
              onOpen: (periodId) => {
                const runId = open.get(periodId);
                if (runId) onOpenRun(runId);
              },
            }
          : undefined
      }
      hint="Der Lauf rechnet alle bestätigten Anmeldungen ab, die noch keine Rechnung haben. Sie prüfen zuerst die Vorschau."
      object="der Lauf"
      onClose={onClose}
      onStart={async (periodId) => onStarted(await startBillingRun(periodId))}
    />
  );
}
