"use client";

// Einen Monatslauf starten (#4684, #5916): Monat, Schulen und
// Rechnungsdatum. Alle Schulen sind vorgewählt. Eine Schule, die schon in
// einem offenen Lauf des Monats ist, lässt sich nicht wählen; die Zeile
// nennt den Lauf. Ohne Rechnungsdatum gilt der Tag der Freigabe.

import { useCallback, useEffect, useRef, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button, ButtonLink } from "~/components/ui/button";
import { CheckboxCard } from "~/components/ui/checkbox-card";
import { CustomSelect } from "~/components/ui/custom-select";
import { ISODatePicker } from "~/components/ui/date-picker";
import { EditActions } from "~/components/ui/edit-actions";
import { FormFieldset } from "~/components/ui/form-fieldset";
import { FormModal } from "~/components/ui/form-modal";
import { Loading } from "~/components/ui/loading";
import { useApiFormError } from "~/contexts/ToastContext";
import { traegerPath } from "~/lib/traeger-url";
import { createLogger } from "~/lib/logger";
import type { CareRunStartInput } from "~/lib/traeger-care-billing-api";
import {
  type CareRunSchool,
  fetchCareRunSchools,
} from "~/lib/traeger-care-review-api";
import { formatMonth } from "./care-billing-format";
import { invoiceDateBounds } from "./care-review-format";

const logger = createLogger({ component: "CareRunStartModal" });

interface MonthOption {
  readonly value: string;
  readonly label: string;
}

/** The schools of a month and the ones a new run may bill. */
function freeSchools(schools: readonly CareRunSchool[]): string[] {
  return schools
    .filter((school) => school.openRunId === null)
    .map((school) => school.id);
}

export function CareRunStartModal({
  isOpen,
  months,
  initialMonth,
  onClose,
  onStart,
}: Readonly<{
  isOpen: boolean;
  months: readonly MonthOption[];
  initialMonth: string;
  onClose: () => void;
  onStart: (input: CareRunStartInput) => Promise<void>;
}>) {
  const [month, setMonth] = useState(initialMonth);
  const [schools, setSchools] = useState<CareRunSchool[] | null>(null);
  const [selected, setSelected] = useState<string[]>([]);
  const [invoiceDate, setInvoiceDate] = useState("");
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors, show: showFormError } = formErrors;

  useEffect(() => {
    if (!isOpen) return;
    setMonth(initialMonth);
    setInvoiceDate("");
    clearErrors();
  }, [isOpen, initialMonth, clearErrors]);

  const loadSchools = useCallback(
    async (forMonth: string, signal: { cancelled: boolean }) => {
      setSchools(null);
      try {
        const list = await fetchCareRunSchools(forMonth);
        if (signal.cancelled) return;
        setSchools(list);
        setSelected(freeSchools(list));
      } catch (err) {
        if (signal.cancelled) return;
        logger.error("care_run_schools_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setSchools([]);
        void showFormError(err, { object: "die Liste der Schulen" });
      }
    },
    [showFormError],
  );

  useEffect(() => {
    if (!isOpen || !month) return;
    const signal = { cancelled: false };
    void loadSchools(month, signal);
    return () => {
      signal.cancelled = true;
    };
  }, [isOpen, month, loadSchools]);

  const free = schools ? freeSchools(schools) : [];
  // The open runs that already bill this month's schools, for the way there.
  const openRunIds = [
    ...new Set(
      (schools ?? [])
        .map((school) => school.openRunId)
        .filter((id): id is string => id !== null),
    ),
  ];
  const allSelected = free.length > 0 && selected.length === free.length;
  const bounds = invoiceDateBounds(month);

  const toggle = (schoolId: string, checked: boolean) =>
    setSelected((current) =>
      checked
        ? [...current, schoolId]
        : current.filter((id) => id !== schoolId),
    );

  const submit = async () => {
    formErrors.clear();
    const invalid: Record<string, string> = {};
    if (selected.length === 0) {
      invalid.school_ids = "Bitte wählen Sie mindestens eine Schule.";
    }
    if (invoiceDate && (invoiceDate < bounds.min || invoiceDate > bounds.max)) {
      invalid.invoice_date = "Das Datum liegt zu weit vom Monat entfernt.";
    }
    if (Object.keys(invalid).length > 0) {
      formErrors.invalid("Bitte prüfen Sie Ihre Angaben.", invalid);
      return;
    }
    setSaving(true);
    try {
      await onStart({
        billingMonth: month,
        schoolIds: selected,
        invoiceDate: invoiceDate || null,
      });
    } catch (err) {
      logger.error("care_run_start_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "der Monatslauf" });
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
      title="Monatslauf starten"
      titleLevel={2}
      size="md"
      error={formErrors.error}
      footer={
        <EditActions
          onCancel={onClose}
          onSave={() => formRef.current?.requestSubmit()}
          saving={saving}
          disabled={schools === null || free.length === 0}
          saveLabel="Vorschau öffnen"
        />
      }
    >
      <form
        ref={formRef}
        noValidate
        className="space-y-5"
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
      >
        <CustomSelect
          id="care-run-month"
          name="billing_month"
          label="Monat"
          error={formErrors.fieldError("billing_month")}
          value={month}
          options={months}
          onChange={setMonth}
        />
        <FormFieldset
          legend="Schulen"
          required
          error={formErrors.fieldError("school_ids")}
          hint="Jede Schule ist in einem Monat nur in einem offenen Lauf."
        >
          {schools === null ? (
            <Loading />
          ) : free.length === 0 && schools.length > 0 ? (
            <Alert
              type="info"
              message={`Für ${formatMonth(month)} ist jede Schule schon in einem offenen Lauf. Öffnen Sie diesen Lauf oder wählen Sie einen anderen Monat.`}
              action={
                openRunIds.length === 1 ? (
                  <ButtonLink
                    href={traegerPath(
                      `/traeger/abrechnung/monatsbeitraege/${openRunIds[0]}`,
                    )}
                    variant="surface"
                    size="compact"
                  >
                    Offenen Lauf öffnen
                  </ButtonLink>
                ) : undefined
              }
            />
          ) : (
            <div className="space-y-2">
              {free.length > 1 ? (
                <Button
                  type="button"
                  variant="link"
                  size="md"
                  onClick={() => setSelected(allSelected ? [] : free)}
                >
                  {allSelected ? "Keine wählen" : "Alle wählen"}
                </Button>
              ) : null}
              <div className="grid gap-2 sm:grid-cols-2">
                {schools.map((school) => (
                  <CheckboxCard
                    key={school.id}
                    checked={selected.includes(school.id)}
                    disabled={school.openRunId !== null}
                    onChange={(checked) => toggle(school.id, checked)}
                    label={school.name}
                    hint={
                      school.openRunId !== null
                        ? `Schon in einem offenen Lauf für ${formatMonth(month)}`
                        : undefined
                    }
                  />
                ))}
              </div>
            </div>
          )}
        </FormFieldset>
        <ISODatePicker
          id="care-run-invoice-date"
          name="invoice_date"
          label="Rechnungsdatum"
          value={invoiceDate}
          min={bounds.min}
          max={bounds.max}
          defaultMonth={month}
          placeholder="Tag der Freigabe"
          error={formErrors.fieldError("invoice_date")}
          onChange={setInvoiceDate}
        />
        <p className="text-sm text-gray-600">
          Der Lauf rechnet jedes Kind mit gültigem Vertrag nach den gebuchten
          Tagen ab. Sie prüfen zuerst die Vorschau. Die Rechnungen bekommen ihre
          Nummer erst bei der Freigabe.
        </p>
      </form>
    </FormModal>
  );
}
