"use client";

// Debitorennummer einer bestätigten Ferienanmeldung (#5621): Sie steht im
// Schreiben als Verwendungszweck. Das Träger-Büro trägt sie nach oder ändert
// sie, bis die Rechnung ausgestellt ist; danach hält die Rechnung sie fest.

import { useEffect, useRef, useState } from "react";
import { Button } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { EditActions } from "~/components/ui/edit-actions";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { SectionCard } from "~/components/ui/section-card";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { changeHolidayDebtorNumber } from "~/lib/traeger-contributions-api";
import {
  DEBTOR_NUMBER_HINT,
  DEBTOR_NUMBER_MAX_LENGTH,
} from "./holiday-registration-status-card";

const logger = createLogger({
  component: "HolidayRegistrationDebtorNumberCard",
});

/** What the card says, before and after the invoice is issued. */
function cardDescription(set: boolean, invoiced: boolean): string {
  if (!invoiced) {
    return "Sie steht im Schreiben als Verwendungszweck. Ändern geht, bis die Rechnung ausgestellt ist.";
  }
  return set
    ? "Die Rechnung ist ausgestellt. Sie behält diese Nummer. Ändern geht nicht mehr."
    : "Die Rechnung ist ohne Debitorennummer ausgestellt. Eintragen geht nicht mehr.";
}

export function HolidayRegistrationDebtorNumberCard({
  periodId,
  registrationId,
  debtorNumber,
  required,
  invoiced,
  onChanged,
}: Readonly<{
  periodId: string;
  registrationId: string;
  /** The current number; empty without one. */
  debtorNumber: string;
  /** The Träger asks for the debtor number (#5621). */
  required: boolean;
  /** An issued invoice bills the registration and keeps its number. */
  invoiced: boolean;
  /** After a change; the page reads the registration again. */
  onChanged: () => Promise<unknown>;
}>) {
  const [open, setOpen] = useState(false);
  const [value, setValue] = useState("");
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;
  const { success } = useToast();

  useEffect(() => {
    if (!open) return;
    setValue(debtorNumber);
    clearErrors();
  }, [open, debtorNumber, clearErrors]);

  const submit = async () => {
    formErrors.clear();
    if (required && !value.trim()) {
      formErrors.invalid("Bitte prüfen Sie Ihre Angaben.", {
        debtor_number: "Bitte geben Sie die Debitorennummer ein.",
      });
      return;
    }
    setSaving(true);
    try {
      await changeHolidayDebtorNumber(periodId, registrationId, value.trim());
      success("Die Debitorennummer ist gespeichert.");
      setOpen(false);
      await onChanged();
    } catch (err) {
      logger.error("holiday_debtor_number_change_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "die Debitorennummer" });
    } finally {
      setSaving(false);
    }
  };

  const set = Boolean(debtorNumber);
  return (
    <SectionCard
      title="Debitorennummer"
      description={cardDescription(Boolean(debtorNumber), invoiced)}
      actions={
        invoiced ? undefined : (
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={() => setOpen(true)}
          >
            {set ? "Ändern" : "Eintragen"}
          </Button>
        )
      }
    >
      {set ? (
        <DataGrid columns={2}>
          <DataField label="Debitorennummer">
            <span className="font-mono">{debtorNumber}</span>
          </DataField>
        </DataGrid>
      ) : (
        <p className="text-sm text-gray-700">
          Noch keine Nummer. Ohne Nummer steht im Schreiben die Rechnungsnummer
          als Verwendungszweck.
        </p>
      )}
      <FormModal
        isOpen={open}
        onClose={() => setOpen(false)}
        closeDisabled={saving}
        isBackdropDismissDisabled
        title={set ? "Debitorennummer ändern" : "Debitorennummer eintragen"}
        size="md"
        error={formErrors.error}
        footer={
          <EditActions
            onCancel={() => setOpen(false)}
            onSave={() => formRef.current?.requestSubmit()}
            saving={saving}
          />
        }
      >
        <form
          ref={formRef}
          noValidate
          className="space-y-2"
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
        >
          <Input
            id="holiday-debtor-number"
            name="debtor_number"
            label="Debitorennummer"
            required={required}
            autoComplete="off"
            maxLength={DEBTOR_NUMBER_MAX_LENGTH}
            value={value}
            aria-describedby="holiday-debtor-number-hint"
            error={formErrors.fieldError("debtor_number") ?? undefined}
            onChange={(event) => setValue(event.target.value)}
          />
          <p id="holiday-debtor-number-hint" className="text-sm text-gray-600">
            {DEBTOR_NUMBER_HINT}
          </p>
        </form>
      </FormModal>
    </SectionCard>
  );
}
