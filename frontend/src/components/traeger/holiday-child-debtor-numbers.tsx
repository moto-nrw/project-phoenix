"use client";

// Debitorennummern der Kinder einer Ferienanmeldung (#7007), wenn der Träger
// die Nummer je Kind führt: Jedes Kind bekommt eine eigene Rechnung mit
// seiner Nummer. Das Bestätigen schlägt die Nummer vor, die das Kind schon
// hat, und merkt sich, was das Büro einträgt. Danach ändert das Büro die
// Nummern hier; ausgestellte Rechnungen behalten ihre Nummer.

import { useEffect, useRef, useState } from "react";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { EditActions } from "~/components/ui/edit-actions";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { FormFieldset } from "~/components/ui/form-fieldset";
import { Input } from "~/components/ui/input";
import type { RefreshAfterWrite } from "~/lib/hooks/use-refresh-after-write";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import {
  DEBTOR_NUMBER_MAX_LENGTH,
  type DebtorNumberFormat,
  debtorNumberFormatError,
  debtorNumberFormatHint,
} from "~/lib/debtor-number-format";
import { createLogger } from "~/lib/logger";
import {
  type HolidayChildDebtorNumber,
  type HolidayChildDebtorNumberInput,
  setHolidayChildDebtorNumbers,
} from "~/lib/traeger-contributions-api";
import { DEBTOR_NUMBER_TEXT_CLASS } from "./family-debtor-number";

const logger = createLogger({ component: "HolidayChildDebtorNumbers" });

/** The entered numbers by child ID. */
export type ChildNumberValues = Readonly<Record<string, string>>;

/** The field of a child's number, as the backend names its refusal. */
function childNumberField(childId: string): string {
  return `child_debtor_number_${childId}`;
}

/** The numbers each child has now, as the fields start. */
export function storedChildNumbers(
  children: readonly HolidayChildDebtorNumber[],
): ChildNumberValues {
  return Object.fromEntries(
    children.map((child) => [child.childId, child.debtorNumber]),
  );
}

/** The entered numbers as the backend takes them. */
export function childNumberInputs(
  children: readonly HolidayChildDebtorNumber[],
  values: ChildNumberValues,
): HolidayChildDebtorNumberInput[] {
  return children.map((child) => ({
    childId: child.childId,
    debtorNumber: (values[child.childId] ?? "").trim(),
  }));
}

/**
 * The field hints the form finds before it asks the backend: a child
 * without a number where the Träger wants one, another format (#6675).
 */
export function childNumberHints(
  children: readonly HolidayChildDebtorNumber[],
  values: ChildNumberValues,
  required: boolean,
  format: DebtorNumberFormat,
): Record<string, string> | null {
  const hints: Record<string, string> = {};
  for (const child of children) {
    const value = (values[child.childId] ?? "").trim();
    const error =
      required && !value.trim()
        ? "Bitte geben Sie die Debitorennummer ein."
        : value === child.debtorNumber
          ? null
          : debtorNumberFormatError(value, format);
    if (error) hints[childNumberField(child.childId)] = error;
  }
  return Object.keys(hints).length > 0 ? hints : null;
}

function childName(child: HolidayChildDebtorNumber): string {
  return `${child.firstName} ${child.lastName}`.trim();
}

/** One field per child, in the order of the form. */
export function ChildDebtorNumberInputs({
  idPrefix,
  childNumbers,
  values,
  required,
  format,
  fieldError,
  onChange,
}: Readonly<{
  idPrefix: string;
  childNumbers: readonly HolidayChildDebtorNumber[];
  values: ChildNumberValues;
  required: boolean;
  format: DebtorNumberFormat;
  fieldError: (field: string) => string | null | undefined;
  onChange: (childId: string, value: string) => void;
}>) {
  return (
    <FormFieldset
      legend="Debitorennummern der Kinder"
      required={required}
      hint={`Jedes Kind bekommt eine eigene Rechnung mit seiner Nummer. ${debtorNumberFormatHint(format)}`.trim()}
      className="sm:max-w-xl"
    >
      <div className="space-y-3">
        {childNumbers.map((child) => (
          <Input
            key={child.childId}
            id={`${idPrefix}-${child.childId}`}
            name={childNumberField(child.childId)}
            label={childName(child)}
            required={required}
            autoComplete="off"
            controlSize="touch"
            inputMode={format.digitsOnly ? "numeric" : undefined}
            maxLength={DEBTOR_NUMBER_MAX_LENGTH}
            value={values[child.childId] ?? ""}
            error={fieldError(childNumberField(child.childId)) ?? undefined}
            onChange={(event) => onChange(child.childId, event.target.value)}
          />
        ))}
      </div>
    </FormFieldset>
  );
}

/**
 * The numbers of the children of a confirmed registration: shown, and
 * edited in place. `editing` belongs to the card, which hides its own button
 * while the fields are open.
 */
export function ChildDebtorNumbersField({
  periodId,
  registrationId,
  childNumbers: children,
  required,
  format,
  invoiced,
  editing,
  onEditingChange,
  onChanged,
}: Readonly<{
  periodId: string;
  registrationId: string;
  childNumbers: readonly HolidayChildDebtorNumber[];
  required: boolean;
  format: DebtorNumberFormat;
  invoiced: boolean;
  editing: boolean;
  onEditingChange: (editing: boolean) => void;
  /** Reloads the registration; never rejects (#6652). */
  onChanged: RefreshAfterWrite;
}>) {
  const [values, setValues] = useState<ChildNumberValues>(() =>
    storedChildNumbers(children),
  );
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;
  const { success } = useToast();

  useEffect(() => {
    if (!editing) return;
    setValues(storedChildNumbers(children));
    clearErrors();
  }, [editing, children, clearErrors]);

  const submit = async () => {
    if (saving) return;
    formErrors.clear();
    const hints = childNumberHints(children, values, required, format);
    if (hints) {
      formErrors.invalid("Bitte prüfen Sie Ihre Angaben.", hints);
      return;
    }
    setSaving(true);
    try {
      await setHolidayChildDebtorNumbers(
        periodId,
        registrationId,
        childNumberInputs(children, values),
      );
    } catch (err) {
      logger.error("holiday_child_debtor_numbers_change_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "die Debitorennummern" });
      setSaving(false);
      return;
    }
    setSaving(false);
    success("Die Debitorennummern sind gespeichert.");
    onEditingChange(false);
    await onChanged();
  };

  if (editing) {
    return (
      <form
        ref={formRef}
        noValidate
        className="space-y-4"
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
      >
        <FormErrorAlert message={formErrors.error} />
        <ChildDebtorNumberInputs
          idPrefix="holiday-child-debtor-number"
          childNumbers={children}
          values={values}
          required={required}
          format={format}
          fieldError={formErrors.fieldError}
          onChange={(childId, value) => {
            // A refusal belongs to the value it refused.
            formErrors.clear();
            setValues((current) => ({ ...current, [childId]: value }));
          }}
        />
        <EditActions
          onCancel={() => onEditingChange(false)}
          saving={saving}
          saveType="submit"
        />
      </form>
    );
  }

  return (
    <div className="space-y-2">
      <DataGrid columns={2}>
        {children.map((child) => (
          <DataField
            key={child.childId}
            label={`Debitorennummer ${child.firstName}`.trim()}
          >
            {child.debtorNumber ? (
              <span className={`wrap-anywhere ${DEBTOR_NUMBER_TEXT_CLASS}`}>
                {child.debtorNumber}
              </span>
            ) : (
              "Noch keine Nummer."
            )}
          </DataField>
        ))}
      </DataGrid>
      <p>
        {invoiced
          ? "Ausgestellte Rechnungen behalten ihre Nummer. Eine geänderte Nummer gilt für neue Rechnungen je Kind."
          : "Jedes Kind bekommt eine eigene Rechnung. Sie nennt seine Nummer als Verwendungszweck."}
      </p>
    </div>
  );
}
