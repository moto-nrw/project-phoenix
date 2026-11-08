"use client";

// Eine ausgestellte Monatsrechnung stornieren (#4684), wie der Storno der
// Ferienbetreuung (#4690): Pflicht-Begründung, eigene Nummer aus dem
// Nummernkreis der Monatsbeiträge, die Rechnung selbst ändert sich nie. Der
// Satz über der Begründung sagt, ob die Familie ein Storno-Schreiben
// bekommt: nur wenn sie die Rechnung schon bekommen hat. Die neue Rechnung
// stellt ein neuer Lauf für denselben Monat aus.

import { useEffect, useRef, useState } from "react";
import { EditActions } from "~/components/ui/edit-actions";
import { FormModal } from "~/components/ui/form-modal";
import { Textarea } from "~/components/ui/textarea";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  type CareLetterMailing,
  type CareLetters,
  cancelCareInvoice,
} from "~/lib/traeger-care-billing-api";
import { formatEuro } from "./contribution-format";

const logger = createLogger({ component: "CareCancellationModal" });

const MAX_REASON = 500;

/** The invoice reached the family: it has a mail, sent or not. */
export function invoiceWentOut(invoice: CareLetterMailing): boolean {
  return invoice.sends > 0;
}

/** What the family gets from the Storno, in one line. */
export function cancellationEffect(invoice: CareLetterMailing): string {
  return invoiceWentOut(invoice)
    ? "Die Familie bekommt ein Storno-Schreiben. Senden Sie es danach."
    : "Die Familie hat die Rechnung noch nicht bekommen. Sie bekommt auch kein Storno.";
}

export function CareCancellationModal({
  runId,
  invoice,
  onClose,
  onCancelled,
}: Readonly<{
  runId: string;
  /** The invoice to cancel; null keeps the modal closed. */
  invoice: CareLetterMailing | null;
  onClose: () => void;
  onCancelled: (letters: CareLetters) => void;
}>) {
  const [reason, setReason] = useState("");
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;
  const { success } = useToast();

  useEffect(() => {
    if (!invoice) return;
    setReason("");
    clearErrors();
  }, [invoice, clearErrors]);

  const submit = async () => {
    if (!invoice) return;
    formErrors.clear();
    if (!reason.trim()) {
      formErrors.invalid("Bitte prüfen Sie Ihre Angaben.", {
        reason: "Bitte geben Sie eine Begründung an.",
      });
      return;
    }
    setSaving(true);
    try {
      const letters = await cancelCareInvoice(
        runId,
        invoice.invoiceId,
        reason.trim(),
      );
      const storno = letters.documents.find(
        (document) =>
          document.kind === "cancellation" &&
          document.invoiceId === invoice.invoiceId,
      );
      const issued = storno
        ? `Storno ${storno.number} ist ausgestellt.`
        : "Der Storno ist ausgestellt.";
      success(
        invoiceWentOut(invoice)
          ? `${issued} Senden Sie das Storno-Schreiben an die Familie.`
          : `${issued} Die Familie bekommt kein Schreiben.`,
      );
      onCancelled(letters);
    } catch (err) {
      logger.error("care_invoice_cancel_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "der Storno" });
    } finally {
      setSaving(false);
    }
  };

  return (
    <FormModal
      isOpen={invoice !== null}
      onClose={onClose}
      closeDisabled={saving}
      isBackdropDismissDisabled
      title={invoice ? `Rechnung ${invoice.number} stornieren?` : ""}
      size="md"
      error={formErrors.error}
      footer={
        <EditActions
          onCancel={onClose}
          onSave={() => formRef.current?.requestSubmit()}
          saving={saving}
          saveLabel="Stornieren"
          savingLabel="Wird storniert …"
        />
      }
    >
      {invoice ? (
        <form
          ref={formRef}
          noValidate
          className="space-y-4"
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
        >
          <div className="space-y-2 text-sm text-gray-700">
            <p>
              Der Storno nimmt {formatEuro(invoice.totalCents)} für{" "}
              {invoice.childFirstName} {invoice.childLastName} zurück. Er
              bekommt eine eigene Nummer.
            </p>
            <p className="font-medium text-gray-900">
              {cancellationEffect(invoice)}
            </p>
            <p>
              Die neue Rechnung stellt ein neuer Monatslauf für denselben Monat
              aus.
            </p>
          </div>
          <Textarea
            id="care-cancellation-reason"
            name="reason"
            label="Begründung"
            required
            placeholder="Zum Beispiel: Falscher Empfänger. Neue Rechnung folgt."
            value={reason}
            maxLength={MAX_REASON}
            error={formErrors.fieldError("reason") ?? undefined}
            onChange={(event) => setReason(event.target.value)}
          />
        </form>
      ) : null}
    </FormModal>
  );
}
