"use client";

// Debitorennummern der Kinder einer Familie in der Liste „Familien“ ändern
// (#7007), wenn der Träger die Nummer je Kind führt. Jedes Kind eines
// Betreuungsvertrags der Familie hat ein Feld; Geschwister haben eigene
// Nummern. Die nächste Monatsrechnung eines Kindes nimmt seine Nummer.

import { useEffect, useRef, useState } from "react";
import { familyKeyLabel } from "~/lib/family-key";
import { EditActions } from "~/components/ui/edit-actions";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { useFormModalPending } from "~/components/ui/use-form-modal-pending";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { formatDate } from "~/lib/date-helpers";
import {
  DEBTOR_NUMBER_MAX_LENGTH,
  type DebtorNumberFormat,
  NO_DEBTOR_NUMBER_FORMAT,
  debtorNumberFormatError,
  debtorNumberFormatHint,
} from "~/lib/debtor-number-format";
import { createLogger } from "~/lib/logger";
import {
  type BillingFamily,
  type FamilyChild,
  setChildDebtorNumber,
} from "~/lib/traeger-billing-families-api";
import { familyName } from "./billing-family-format";

const logger = createLogger({ component: "ChildDebtorNumbersModal" });

/** The field name of a child, unique within the family. */
function fieldName(index: number): string {
  return `child_debtor_number_${index}`;
}

/** Name of the child as the field labels it, with its date of birth. */
export function childLabel(child: FamilyChild): string {
  const name = `${child.firstName} ${child.lastName}`.trim();
  return child.birthDate
    ? `${name}, geb. ${formatDate(child.birthDate)}`
    : name;
}

export function ChildDebtorNumbersModal({
  family,
  format = NO_DEBTOR_NUMBER_FORMAT,
  onClose,
  onSaved,
}: Readonly<{
  /** The family whose children to change; null keeps the dialog closed. */
  family: BillingFamily | null;
  /** The format of the Träger's numbers (#6675). */
  format?: DebtorNumberFormat;
  onClose: () => void;
  /** After a save; the page reads the families again. */
  onSaved: () => Promise<unknown>;
}>) {
  const isOpen = family !== null;
  const children = family?.children ?? [];
  const [values, setValues] = useState<string[]>([]);
  // The numbers as stored: a save that stops after some children keeps
  // what it already wrote, so a retry compares with that.
  const [stored, setStored] = useState<string[]>([]);
  const formRef = useRef<HTMLFormElement>(null);
  const pending = useFormModalPending(isOpen);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;
  const { success } = useToast();

  useEffect(() => {
    if (!family) return;
    const numbers = (family.children ?? []).map((child) => child.debtorNumber);
    setValues(numbers);
    setStored(numbers);
    clearErrors();
  }, [family, clearErrors]);

  const submit = async () => {
    if (!family) return;
    formErrors.clear();
    const invalid: Record<string, string> = {};
    values.forEach((value, index) => {
      const error = debtorNumberFormatError(value, format);
      if (error) invalid[fieldName(index)] = error;
    });
    if (Object.keys(invalid).length > 0) {
      formErrors.invalid("Bitte prüfen Sie Ihre Angaben.", invalid);
      return;
    }
    const changed = children.flatMap((child, index) => {
      const value = (values[index] ?? "").trim();
      return value === (stored[index] ?? "") ? [] : [{ child, index, value }];
    });
    const result = await pending.run(async (op) => {
      try {
        for (const { child, index, value } of changed) {
          const saved = await setChildDebtorNumber(
            family.familyEmail,
            child,
            value,
          );
          setStored((current) =>
            current.map((number, at) =>
              at === index ? saved.debtorNumber : number,
            ),
          );
        }
        if (changed.length > 0) {
          success(
            changed.length === 1
              ? "Die Debitorennummer ist gespeichert."
              : "Die Debitorennummern sind gespeichert.",
          );
        }
        return op.isCurrent() ? "saved" : "late";
      } catch (err) {
        logger.error("child_debtor_number_save_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (op.isCurrent()) {
          void formErrors.show(err, { object: "die Debitorennummer" });
        }
        return "failed";
      }
    });
    if (result === undefined) return;
    if (result === "saved") onClose();
    // A failed save may have kept the numbers before it: read them again.
    await onSaved();
  };

  const name = family ? familyName(family) : "";
  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      pending={pending}
      isBackdropDismissDisabled
      title="Debitorennummern der Kinder"
      size="md"
      error={formErrors.error}
      footer={
        <EditActions
          onCancel={onClose}
          onSave={() => formRef.current?.requestSubmit()}
          saving={pending.pending}
        />
      }
    >
      <form
        ref={formRef}
        noValidate
        className="space-y-4"
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
      >
        {family ? (
          <p className="text-sm text-gray-700">
            {name ? (
              <span className="font-medium text-gray-900">{name}</span>
            ) : null}
            {name ? " · " : null}
            <span className="break-all">
              {familyKeyLabel(family.familyEmail)}
            </span>
          </p>
        ) : null}
        <p className="text-sm text-gray-600">
          {`Jedes Kind hat eine eigene Nummer, auch Geschwister. ${debtorNumberFormatHint(format)} Die nächste Rechnung eines Kindes nimmt seine Nummer. Ausgestellte Rechnungen behalten ihre Nummer.`.replace(
            /\s+/g,
            " ",
          )}
        </p>
        {children.map((child, index) => (
          <Input
            key={`${child.firstName}-${child.lastName}-${child.birthDate ?? ""}`}
            id={`child-debtor-number-${index}`}
            name={fieldName(index)}
            label={childLabel(child)}
            autoComplete="off"
            controlSize="compact"
            inputMode={format.digitsOnly ? "numeric" : undefined}
            maxLength={DEBTOR_NUMBER_MAX_LENGTH}
            value={values[index] ?? ""}
            error={formErrors.fieldError(fieldName(index)) ?? undefined}
            onChange={(event) => {
              // A refusal belongs to the value it refused.
              formErrors.clear();
              setValues((current) =>
                current.map((value, at) =>
                  at === index ? event.target.value : value,
                ),
              );
            }}
          />
        ))}
      </form>
    </FormModal>
  );
}
