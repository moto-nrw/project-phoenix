"use client";

// Debitorennummer einer Familie in der Liste „Familien“ ändern (#6692), auch
// einer Familie, die nur zur Ferienbetreuung angemeldet ist. moto schlägt die
// Nummer bei jeder neuen Ferienanmeldung der Familie vor.

import { useEffect, useRef, useState } from "react";
import { EditActions } from "~/components/ui/edit-actions";
import { FormModal } from "~/components/ui/form-modal";
import { useFormModalPending } from "~/components/ui/use-form-modal-pending";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
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
  setFamilyDebtorNumber,
} from "~/lib/traeger-billing-families-api";
import { familyName } from "./billing-family-format";
import { Input } from "~/components/ui/input";

const logger = createLogger({ component: "FamilyDebtorNumberModal" });

export function FamilyDebtorNumberModal({
  family,
  format = NO_DEBTOR_NUMBER_FORMAT,
  onClose,
  onSaved,
}: Readonly<{
  /** The family to change; null keeps the dialog closed. */
  family: BillingFamily | null;
  /** The format of the Träger's numbers (#6675). */
  format?: DebtorNumberFormat;
  onClose: () => void;
  /** After a save; the page reads the families again. */
  onSaved: () => Promise<unknown>;
}>) {
  const isOpen = family !== null;
  const [value, setValue] = useState("");
  const formRef = useRef<HTMLFormElement>(null);
  const fieldRef = useRef<HTMLInputElement>(null);
  const pending = useFormModalPending(isOpen);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;
  const { success } = useToast();

  useEffect(() => {
    if (!family) return;
    setValue(family.familyDebtorNumber);
    clearErrors();
  }, [family, clearErrors]);

  const submit = async () => {
    if (!family) return;
    formErrors.clear();
    const formatError = debtorNumberFormatError(value, format);
    if (formatError) {
      formErrors.invalid("Bitte prüfen Sie Ihre Angaben.", {
        debtor_number: formatError,
      });
      return;
    }
    const result = await pending.run(async (op) => {
      try {
        await setFamilyDebtorNumber(family.familyEmail, value.trim());
        success("Die Debitorennummer ist gespeichert.");
        return op.isCurrent() ? "saved" : "late";
      } catch (err) {
        logger.error("family_debtor_number_save_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (op.isCurrent()) {
          void formErrors.show(err, { object: "die Debitorennummer" });
        }
        return "failed";
      }
    });
    if (result === undefined || result === "failed") return;
    if (result === "saved") onClose();
    await onSaved();
  };

  const name = family ? familyName(family) : "";
  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      pending={pending}
      isBackdropDismissDisabled
      title={
        family?.familyDebtorNumber
          ? "Debitorennummer ändern"
          : "Debitorennummer eintragen"
      }
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
            <span className="break-all">{family.familyEmail}</span>
          </p>
        ) : null}
        <Input
          ref={fieldRef}
          id="family-debtor-number"
          name="debtor_number"
          label="Debitorennummer"
          autoComplete="off"
          controlSize="compact"
          inputMode={format.digitsOnly ? "numeric" : undefined}
          maxLength={DEBTOR_NUMBER_MAX_LENGTH}
          value={value}
          hint={`${debtorNumberFormatHint(format)} Die Nummer der Familie bei der Kasse der Gemeinde. moto schlägt sie bei jeder neuen Ferienanmeldung der Familie vor. Ausgestellte Rechnungen behalten ihre Nummer.`.trim()}
          error={formErrors.fieldError("debtor_number") ?? undefined}
          onChange={(event) => setValue(event.target.value)}
        />
      </form>
    </FormModal>
  );
}
