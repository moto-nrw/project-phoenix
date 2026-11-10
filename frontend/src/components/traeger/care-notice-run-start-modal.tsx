"use client";

// Einen Jahreslauf starten (#6219): erster Monat, Schulen und Bescheiddatum.
// Der Lauf gilt vom ersten Monat bis Dezember desselben Jahres. Alle
// Schulen sind vorgewählt. Ohne Bescheiddatum gilt der Tag der Ausstellung.

import { useEffect, useRef, useState } from "react";
import { Button } from "~/components/ui/button";
import { CheckboxCard } from "~/components/ui/checkbox-card";
import { CustomSelect } from "~/components/ui/custom-select";
import { ISODatePicker } from "~/components/ui/date-picker";
import { EditActions } from "~/components/ui/edit-actions";
import { FormFieldset } from "~/components/ui/form-fieldset";
import { FormModal } from "~/components/ui/form-modal";
import { useFormModalPending } from "~/components/ui/use-form-modal-pending";
import { useApiFormError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import type { CareSchool } from "~/lib/traeger-care-billing-api";
import type {
  CareNoticeRun,
  CareNoticeRunStartInput,
} from "~/lib/traeger-care-notices-api";
import { formatMonth, monthRange } from "./care-billing-format";

const logger = createLogger({ component: "CareNoticeRunStartModal" });

/** The first months a run may start with: every month of this and next year. */
function noticeStartMonths(suggestedYear: number) {
  return monthRange(`${suggestedYear}-01-01`, 24).map((value) => ({
    value,
    label: formatMonth(value),
  }));
}

export function CareNoticeRunStartModal({
  isOpen,
  schools,
  initialMonth,
  suggestedYear,
  onClose,
  onStart,
  onStarted,
}: Readonly<{
  isOpen: boolean;
  schools: readonly CareSchool[];
  initialMonth: string;
  suggestedYear: number;
  onClose: () => void;
  /** Starts the run; may refresh data, but must not close or navigate. */
  onStart: (input: CareNoticeRunStartInput) => Promise<CareNoticeRun>;
  /** Closes and opens the run; called only while this dialog is current. */
  onStarted: (run: CareNoticeRun) => void;
}>) {
  const [fromMonth, setFromMonth] = useState(initialMonth);
  const [selected, setSelected] = useState<string[]>([]);
  const [noticeDate, setNoticeDate] = useState("");
  const save = useFormModalPending(isOpen);
  const saving = save.pending;
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;

  useEffect(() => {
    if (!isOpen) return;
    setFromMonth(initialMonth);
    setSelected(schools.map((school) => school.id));
    setNoticeDate("");
    clearErrors();
  }, [isOpen, initialMonth, schools, clearErrors]);

  const year = Number(fromMonth.slice(0, 4));
  const bounds = { min: `${year - 1}-12-01`, max: `${year}-12-31` };
  const allSelected = selected.length === schools.length;

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
    if (noticeDate && (noticeDate < bounds.min || noticeDate > bounds.max)) {
      invalid.notice_date = `Das Datum muss im Jahr ${year} liegen.`;
    }
    if (Object.keys(invalid).length > 0) {
      formErrors.invalid("Bitte prüfen Sie Ihre Angaben.", invalid);
      return;
    }
    await save.run(async (op) => {
      let run: CareNoticeRun;
      try {
        run = await onStart({
          year,
          fromMonth,
          schoolIds: selected,
          noticeDate: noticeDate || null,
        });
      } catch (err) {
        logger.error("care_notice_run_start_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        // Protokolliert; eine späte Ablehnung zeigt kein neuerer Vorgang.
        if (!op.isCurrent()) return;
        void formErrors.show(err, { object: "der Jahreslauf" });
        return;
      }
      // A late answer must not close or navigate away from a newer dialog.
      if (op.isCurrent()) onStarted(run);
    });
  };

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      pending={save}
      isBackdropDismissDisabled
      title="Jahreslauf starten"
      size="md"
      error={formErrors.error}
      footer={
        <EditActions
          onCancel={onClose}
          onSave={() => formRef.current?.requestSubmit()}
          saving={saving}
          disabled={schools.length === 0}
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
          id="care-notice-run-from"
          name="from_month"
          label="Ab Monat"
          hint={`Der Bescheid gilt bis Dezember ${year}.`}
          error={formErrors.fieldError("from_month")}
          value={fromMonth}
          options={noticeStartMonths(suggestedYear)}
          onChange={setFromMonth}
        />
        <FormFieldset
          legend="Schulen"
          required
          error={formErrors.fieldError("school_ids")}
        >
          <div className="space-y-2">
            {schools.length > 1 ? (
              <Button
                type="button"
                variant="link"
                size="md"
                onClick={() =>
                  setSelected(
                    allSelected ? [] : schools.map((school) => school.id),
                  )
                }
              >
                {allSelected ? "Keine wählen" : "Alle wählen"}
              </Button>
            ) : null}
            <div className="grid gap-2 sm:grid-cols-2">
              {schools.map((school) => (
                <CheckboxCard
                  key={school.id}
                  checked={selected.includes(school.id)}
                  onChange={(checked) => toggle(school.id, checked)}
                  label={school.name}
                />
              ))}
            </div>
          </div>
        </FormFieldset>
        <ISODatePicker
          id="care-notice-run-date"
          name="notice_date"
          label="Bescheiddatum"
          value={noticeDate}
          min={bounds.min}
          max={bounds.max}
          defaultMonth={fromMonth}
          placeholder="Tag der Ausstellung"
          error={formErrors.fieldError("notice_date")}
          onChange={setNoticeDate}
        />
        <p className="text-sm text-gray-600">
          Jedes Kind mit gültigem Vertrag bekommt einen Bescheid mit
          Monatsraten. Sie prüfen zuerst die Vorschau. Die Nummer kommt erst
          beim Ausstellen.
        </p>
      </form>
    </FormModal>
  );
}
