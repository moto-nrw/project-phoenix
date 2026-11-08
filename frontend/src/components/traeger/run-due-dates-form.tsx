"use client";

// Fälligkeiten eines berechneten Laufs (#4689): je Zahlungsweg das Datum,
// an dem seine Rechnungen fällig sind. Ohne Datum gilt das Zahlungsziel der
// Leistung ab dem Rechnungsdatum; eine Familie mit eigenem Zahltag behält
// ihren Tag. Nach der Freigabe ändern sich die Daten nicht mehr.

import { useRef, useState } from "react";
import { ISODatePicker } from "~/components/ui/date-picker";
import { EditActions } from "~/components/ui/edit-actions";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  PAYMENT_METHODS,
  type PaymentMethod,
  type RunDueDate,
  saveRunDueDates,
} from "~/lib/traeger-billing-terms-api";
import { PAYMENT_METHOD_LABELS } from "./billing-payment-format";

const logger = createLogger({ component: "RunDueDatesForm" });

type Draft = Record<PaymentMethod, string>;

function draftOf(dates: readonly RunDueDate[]): Draft {
  const draft: Draft = { transfer: "", direct_debit: "" };
  for (const due of dates) draft[due.paymentMethod] = due.dueOn;
  return draft;
}

/** The form of the due dates of a calculated run, on its own page. */
export function RunDueDatesForm({
  kind,
  runId,
  dueDates,
  onCancel,
  onSaved,
}: Readonly<{
  kind: "care" | "holiday";
  runId: string;
  dueDates: readonly RunDueDate[];
  onCancel: () => void;
  onSaved: (dates: RunDueDate[]) => void;
}>) {
  const [draft, setDraft] = useState<Draft>(() => draftOf(dueDates));
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { success } = useToast();

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
    <SectionCard title="Fälligkeiten">
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
        <FormErrorAlert message={formErrors.error} />
        <EditActions onCancel={onCancel} saving={saving} />
      </form>
    </SectionCard>
  );
}
