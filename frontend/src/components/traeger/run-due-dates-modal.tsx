"use client";

// Fälligkeiten eines berechneten Laufs (#4689): je Zahlungsweg das Datum,
// an dem seine Rechnungen fällig sind. Ohne Datum gilt das Zahlungsziel der
// Leistung ab dem Rechnungsdatum; eine Familie mit eigenem Zahltag behält
// ihren Tag. Nach der Freigabe ändern sich die Daten nicht mehr.

import { useEffect, useRef, useState } from "react";
import { ISODatePicker } from "~/components/ui/date-picker";
import { EditActions } from "~/components/ui/edit-actions";
import { FormModal } from "~/components/ui/form-modal";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  PAYMENT_METHODS,
  type PaymentMethod,
  type RunDueDate,
  saveRunDueDates,
} from "~/lib/traeger-billing-terms-api";
import { PAYMENT_METHOD_LABELS } from "./billing-payment-format";

const logger = createLogger({ component: "RunDueDatesModal" });

type Draft = Record<PaymentMethod, string>;

function draftOf(dates: readonly RunDueDate[]): Draft {
  const draft: Draft = { transfer: "", direct_debit: "" };
  for (const due of dates) draft[due.paymentMethod] = due.dueOn;
  return draft;
}

export function RunDueDatesModal({
  isOpen,
  kind,
  runId,
  dueDates,
  onClose,
  onSaved,
}: Readonly<{
  isOpen: boolean;
  kind: "care" | "holiday";
  runId: string;
  dueDates: readonly RunDueDate[];
  onClose: () => void;
  onSaved: (dates: RunDueDate[]) => void;
}>) {
  const [draft, setDraft] = useState<Draft>(() => draftOf(dueDates));
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;
  const { success } = useToast();

  useEffect(() => {
    if (!isOpen) return;
    setDraft(draftOf(dueDates));
    clearErrors();
  }, [isOpen, dueDates, clearErrors]);

  const submit = async () => {
    formErrors.clear();
    setSaving(true);
    try {
      const saved = await saveRunDueDates(
        kind,
        runId,
        PAYMENT_METHODS.filter((method) => draft[method] !== "").map(
          (method) => ({ paymentMethod: method, dueOn: draft[method] }),
        ),
      );
      success("Die Fälligkeiten sind gespeichert.");
      onSaved(saved);
    } catch (err) {
      logger.error("run_due_dates_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "die Fälligkeiten" });
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
      title="Fälligkeiten ändern"
      size="md"
      error={formErrors.error}
      footer={
        <EditActions
          onCancel={onClose}
          onSave={() => formRef.current?.requestSubmit()}
          saving={saving}
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
        <p className="text-sm text-gray-600">
          Bis zu diesem Tag zahlen die Familien die Rechnungen dieses Laufs.
          Ohne Datum gilt das Zahlungsziel der Leistung.
        </p>
        {PAYMENT_METHODS.map((method) => (
          <ISODatePicker
            key={method}
            id={`run-due-${kind}-${method}`}
            name={method}
            label={`${PAYMENT_METHOD_LABELS[method]} fällig am`}
            value={draft[method]}
            error={formErrors.fieldError(method) ?? undefined}
            onChange={(value) =>
              setDraft((current) => ({ ...current, [method]: value }))
            }
          />
        ))}
        <p className="text-sm text-gray-600">
          Familien mit eigenem Zahltag behalten ihren Tag.
        </p>
      </form>
    </FormModal>
  );
}
