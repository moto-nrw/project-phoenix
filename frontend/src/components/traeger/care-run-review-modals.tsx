"use client";

// Die Dialoge der Prüfphase eines Monatslaufs (#5916): eine Rechnung mit
// Grund ausschließen und das Rechnungsdatum des Laufs ändern. Ein Ausschluss
// betrifft genau eine Rechnung (#6346); der Dialog nennt, was mit der anderen
// Rechnung desselben Vertrags geschieht. Die Laufseite lädt diese Datei erst,
// wenn ein Dialog aufgeht.

import { useEffect, useRef, useState } from "react";
import { ISODatePicker } from "~/components/ui/date-picker";
import { EditActions } from "~/components/ui/edit-actions";
import { FormModal } from "~/components/ui/form-modal";
import { InsetPanel } from "~/components/ui/inset-panel";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { Textarea } from "~/components/ui/textarea";
import { useApiFormError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import type {
  CareInvoice,
  CareRunDetail,
} from "~/lib/traeger-care-billing-api";
import {
  excludeCareInvoice,
  setCareRunInvoiceDate,
} from "~/lib/traeger-care-review-api";
import { formatMonth } from "./care-billing-format";
import {
  exclusionScopeText,
  exclusionTitle,
  invoiceDateBounds,
  invoiceKindLabel,
} from "./care-review-format";
import { formatEuro } from "./contribution-format";

const logger = createLogger({ component: "CareRunReviewModals" });

/** The longest reason the backend keeps. */
const MAX_REASON = 500;

/**
 * Leaves one invoice of the preview out of the release, with a reason. Only
 * this invoice: the dialog names the other invoice of the contract that
 * stays in the run, or the Mensa that goes with it (#6346).
 */
export function CareExcludeModal({
  runId,
  invoice,
  invoices,
  onClose,
  onExcluded,
}: Readonly<{
  runId: string;
  invoice: CareInvoice | null;
  /** Every invoice of the run, so the dialog can name the other one. */
  invoices: readonly CareInvoice[];
  onClose: () => void;
  onExcluded: (run: CareRunDetail, invoice: CareInvoice) => void;
}>) {
  const [reason, setReason] = useState("");
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear } = formErrors;

  useEffect(() => {
    if (!invoice) return;
    setReason("");
    clear();
  }, [invoice, clear]);

  const submit = async () => {
    if (!invoice) return;
    formErrors.clear();
    if (!reason.trim()) {
      formErrors.invalid("Bitte prüfen Sie Ihre Angaben.", {
        reason: "Bitte nennen Sie einen Grund.",
      });
      return;
    }
    setSaving(true);
    try {
      onExcluded(
        await excludeCareInvoice(runId, invoice, reason.trim()),
        invoice,
      );
    } catch (err) {
      logger.error("traeger_care_invoice_exclude_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "die Rechnung" });
    } finally {
      setSaving(false);
    }
  };

  const child = invoice
    ? `${invoice.childFirstName} ${invoice.childLastName}`.trim()
    : "";
  const scope = invoice
    ? exclusionScopeText(invoice, invoices, formatEuro)
    : null;
  return (
    <FormModal
      isOpen={invoice !== null}
      onClose={onClose}
      closeDisabled={saving}
      isBackdropDismissDisabled
      title={
        invoice ? exclusionTitle(invoice, invoices) : "Rechnung ausschließen"
      }
      size="md"
      error={formErrors.error}
      footer={
        <EditActions
          onCancel={onClose}
          onSave={() => formRef.current?.requestSubmit()}
          saving={saving}
          saveLabel="Ausschließen"
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
        {invoice ? (
          <InsetPanel padding="md" data-testid="care-exclusion-invoice">
            <div className="flex items-start justify-between gap-4">
              <div className="min-w-0">
                <p className="text-sm font-semibold text-gray-900">{child}</p>
                <p className="mt-0.5 text-sm text-gray-600">
                  {invoiceKindLabel(invoice)} · {invoice.schoolName}
                </p>
              </div>
              <p className="shrink-0 text-base font-semibold text-gray-900 tabular-nums">
                {formatEuro(invoice.totalCents)}
              </p>
            </div>
            {scope ? (
              <p className="mt-3 border-t border-gray-200 pt-3 text-sm text-gray-700">
                {scope}
              </p>
            ) : null}
          </InsetPanel>
        ) : null}
        <Textarea
          id="care-exclusion-reason"
          name="reason"
          label="Grund"
          required
          autoGrow
          rows={3}
          maxLength={MAX_REASON}
          placeholder="Zum Beispiel: Kündigung wird noch geprüft"
          hint="Der Grund steht im Protokoll des Laufs und in der Prüfliste."
          value={reason}
          error={formErrors.fieldError("reason")}
          onChange={(event) => setReason(event.target.value)}
        />
        <p className="text-sm text-gray-600">
          Die Rechnung bekommt bei der Freigabe keine Nummer. Sie kommt im
          nächsten Lauf dieses Monats wieder.
        </p>
      </form>
    </FormModal>
  );
}

type DateChoice = "release" | "date";

/** Sets the invoice date of the run, or the day of the release. */
export function CareInvoiceDateModal({
  run,
  isOpen,
  onClose,
  onSaved,
}: Readonly<{
  run: CareRunDetail;
  isOpen: boolean;
  onClose: () => void;
  onSaved: (run: CareRunDetail) => void;
}>) {
  const [choice, setChoice] = useState<DateChoice>("release");
  const [date, setDate] = useState("");
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear } = formErrors;
  const bounds = invoiceDateBounds(run.billingMonth);

  useEffect(() => {
    if (!isOpen) return;
    setChoice(run.invoiceDate ? "date" : "release");
    setDate(run.invoiceDate ?? "");
    clear();
  }, [isOpen, run.invoiceDate, clear]);

  const submit = async () => {
    formErrors.clear();
    if (choice === "date" && !date) {
      formErrors.invalid("Bitte prüfen Sie Ihre Angaben.", {
        invoice_date: "Bitte wählen Sie ein Datum.",
      });
      return;
    }
    setSaving(true);
    try {
      onSaved(
        await setCareRunInvoiceDate(run.id, choice === "date" ? date : null),
      );
    } catch (err) {
      logger.error("traeger_care_invoice_date_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "das Rechnungsdatum" });
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
      title="Rechnungsdatum"
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
        <SegmentedControl<DateChoice>
          ariaLabel="Rechnungsdatum"
          variant="joined"
          fullWidth
          value={choice}
          onChange={setChoice}
          items={[
            { value: "release", label: "Tag der Freigabe" },
            { value: "date", label: "Festes Datum" },
          ]}
        />
        {choice === "date" ? (
          <ISODatePicker
            id="care-run-invoice-date-edit"
            name="invoice_date"
            label="Datum"
            required
            value={date}
            min={bounds.min}
            max={bounds.max}
            defaultMonth={run.billingMonth}
            error={formErrors.fieldError("invoice_date")}
            onChange={setDate}
          />
        ) : null}
        <p className="text-sm text-gray-600">
          Jede Rechnung für {formatMonth(run.billingMonth)} trägt dieses Datum,
          auch in der Prüfliste.
        </p>
      </form>
    </FormModal>
  );
}
