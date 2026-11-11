"use client";

// Debitorennummer einer Familie in der Liste „Familien“ ändern (#6692), auch
// einer Familie, die nur zur Ferienbetreuung angemeldet ist. moto schlägt die
// Nummer bei jeder neuen Ferienanmeldung der Familie vor.

import { useEffect, useRef, useState } from "react";
import { EditActions } from "~/components/ui/edit-actions";
import { FormModal } from "~/components/ui/form-modal";
import { useFormModalPending } from "~/components/ui/use-form-modal-pending";
import { Input } from "~/components/ui/input";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  type BillingFamily,
  setFamilyDebtorNumber,
} from "~/lib/traeger-billing-families-api";
import { familyName } from "./billing-family-format";

const logger = createLogger({ component: "FamilyDebtorNumberModal" });

/** The longest debtor number the backend keeps. */
const DEBTOR_NUMBER_MAX_LENGTH = 35;

export function FamilyDebtorNumberModal({
  family,
  onClose,
  onSaved,
}: Readonly<{
  /** The family to change; null keeps the dialog closed. */
  family: BillingFamily | null;
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
        <div>
          <Input
            ref={fieldRef}
            id="family-debtor-number"
            name="debtor_number"
            label="Debitorennummer"
            autoComplete="off"
            controlSize="compact"
            maxLength={DEBTOR_NUMBER_MAX_LENGTH}
            value={value}
            aria-describedby="family-debtor-number-hint"
            error={formErrors.fieldError("debtor_number") ?? undefined}
            onChange={(event) => setValue(event.target.value)}
          />
          <p
            id="family-debtor-number-hint"
            className="mt-1 text-xs text-gray-600"
          >
            Die Nummer der Familie bei der Kasse der Gemeinde. moto schlägt sie
            bei jeder neuen Ferienanmeldung der Familie vor. Ausgestellte
            Rechnungen behalten ihre Nummer.
          </p>
        </div>
      </form>
    </FormModal>
  );
}
