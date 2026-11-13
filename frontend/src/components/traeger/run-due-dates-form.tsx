"use client";

// Fälligkeiten eines berechneten Laufs (#4689): je Zahlungsweg das Datum,
// an dem seine Rechnungen fällig sind. Ohne Datum gilt das Zahlungsziel der
// Leistung ab dem Rechnungsdatum; eine Familie mit eigenem Zahltag behält
// ihren Tag. Nach der Freigabe ändern sich die Daten nicht mehr.

import { Fragment, useRef, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { ISODatePicker } from "~/components/ui/date-picker";
import { EditActions } from "~/components/ui/edit-actions";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  PAYMENT_METHODS,
  type PaymentMethod,
  type RunDueDate,
  saveRunDueDates,
} from "~/lib/traeger-billing-terms-api";
import { PAYMENT_METHOD_LABELS } from "./billing-payment-format";
import {
  type DebitItem,
  debitHints,
  draftTooEarlyHint,
  termDueDates,
  tooEarlyHint,
} from "./care-debit-hints";

const logger = createLogger({ component: "RunDueDatesForm" });

type Draft = Record<PaymentMethod, string>;

const NO_DEBIT_ITEMS: readonly DebitItem[] = [];

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
  debitItems = NO_DEBIT_ITEMS,
  prenotificationDays = null,
  issuedOn = null,
  onCancel,
  onSaved,
}: Readonly<{
  kind: "care" | "holiday";
  runId: string;
  dueDates: readonly RunDueDate[];
  /** The run's invoices: the date that applies without one, and debits
   * that come too early (#4695). */
  debitItems?: readonly DebitItem[];
  /** The creditor's pre-notification days; null while unknown. */
  prenotificationDays?: number | null;
  /** The run's invoice date, when set: the hint names the first day. */
  issuedOn?: string | null;
  onCancel: () => void;
  onSaved: (dates: RunDueDate[]) => void;
}>) {
  const [draft, setDraft] = useState<Draft>(() => draftOf(dueDates));
  const saved = draftOf(dueDates);
  const termDates = termDueDates(debitItems);
  // The direct debits that come too early: while the saved day stands, the
  // families the preview found; for a new day, checked as it is typed
  // (needs the invoice date; without one the check follows on saving).
  const earlyHint =
    draft.direct_debit === saved.direct_debit
      ? tooEarlyHint(debitHints(debitItems), prenotificationDays, issuedOn)
      : draftTooEarlyHint(
          debitItems,
          draft.direct_debit,
          prenotificationDays,
          issuedOn,
        );
  const earlyHintId = `run-due-${kind}-direct_debit-early`;
  // Under each field: the day that applies while it is empty.
  const hintFor = (method: PaymentMethod): string | undefined => {
    const term = termDates[method];
    return draft[method] === "" && term
      ? `Ohne Datum gilt der ${formatDate(term)}.`
      : undefined;
  };
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
      void formErrors.show(err, { object: "die Änderung der Fälligkeiten" });
    } finally {
      setSaving(false);
    }
  };

  return (
    <SectionCard
      title="Fällig je Zahlungsweg"
      description="Bis zu diesem Tag zahlen die Familien die Rechnungen dieses Laufs. Ohne Datum gilt das Zahlungsziel der Leistung."
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
        {PAYMENT_METHODS.map((method) => {
          const early = method === "direct_debit" ? earlyHint : undefined;
          return (
            <Fragment key={method}>
              <ISODatePicker
                id={`run-due-${kind}-${method}`}
                name={method}
                label={`${PAYMENT_METHOD_LABELS[method]} fällig am`}
                value={draft[method]}
                hint={hintFor(method)}
                ariaDescribedBy={early ? earlyHintId : undefined}
                error={formErrors.fieldError(method) ?? undefined}
                onChange={(value) =>
                  setDraft((current) => ({ ...current, [method]: value }))
                }
              />
              {early ? (
                <Alert id={earlyHintId} type="warning" message={early} />
              ) : null}
            </Fragment>
          );
        })}
        <p className="text-sm text-gray-600">
          Familien mit eigenem Zahltag behalten ihren Tag.
        </p>
        <FormErrorAlert message={formErrors.error} />
        <EditActions onCancel={onCancel} saving={saving} />
      </form>
    </SectionCard>
  );
}
