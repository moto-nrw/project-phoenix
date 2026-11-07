"use client";

// Neue Monatspreise einer Schule ab einem Monat (#4684). Je Modul und
// Wochentag ein Preis für den ganzen Monat. Ein leeres Feld behält den
// bisherigen Preis. Ein Monat, den schon eine Rechnung abrechnet, steht
// nicht zur Wahl.

import { useEffect, useMemo, useRef, useState } from "react";
import { CustomSelect } from "~/components/ui/custom-select";
import { EditActions } from "~/components/ui/edit-actions";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  type CarePriceInput,
  type CareTariffs,
  type CareWeekday,
  saveCarePriceList,
} from "~/lib/traeger-care-billing-api";
import {
  WEEKDAY_LONG,
  WEEKDAY_SHORT,
  endsAtLabel,
  firstOpenMonth,
  formatMonth,
  monthRange,
  priceRowsAt,
  priceStands,
  shownWeekdays,
  standAt,
} from "./care-billing-format";
import { centsToEuroInput, euroInputToCents } from "./holiday-period-model";

const logger = createLogger({ component: "CarePriceListModal" });

/** How many months ahead a new price list may start. */
const MONTHS_AHEAD = 18;

type Draft = Record<string, string>;

function draftKey(offeringId: string, weekday: CareWeekday): string {
  return `${offeringId}:${weekday}`;
}

export function CarePriceListModal({
  data,
  school,
  currentMonth,
  initialMonth,
  onClose,
  onSaved,
}: Readonly<{
  data: CareTariffs;
  /** The school whose prices change; null keeps the modal closed. */
  school: { id: string; name: string } | null;
  /** The current month, "YYYY-MM-01". */
  currentMonth: string;
  /** A month to start with, e.g. the price list the card shows. */
  initialMonth?: string;
  onClose: () => void;
  onSaved: (data: CareTariffs) => void;
}>) {
  const [month, setMonth] = useState("");
  const [draft, setDraft] = useState<Draft>({});
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;
  const { success } = useToast();

  const firstMonth = school
    ? firstOpenMonth(data, school.id, currentMonth)
    : currentMonth;
  const months = useMemo(
    () => monthRange(firstMonth, MONTHS_AHEAD + 1),
    [firstMonth],
  );
  const rows = useMemo(
    () => (school && month ? priceRowsAt(data, school.id, month) : []),
    [data, school, month],
  );
  const weekdays = shownWeekdays(rows);

  // Open with the asked month when it may still change, else the first
  // month that may.
  useEffect(() => {
    if (!school) return;
    const start =
      initialMonth && initialMonth >= firstMonth ? initialMonth : firstMonth;
    setMonth(start);
    clearErrors();
  }, [school, initialMonth, firstMonth, clearErrors]);

  // Prefill every field with the price that holds in the chosen month.
  useEffect(() => {
    if (!school || !month) return;
    const stand = standAt(priceStands(data, school.id), month);
    const holding = priceRowsAt(data, school.id, stand || month);
    const next: Draft = {};
    for (const row of holding) {
      for (const [weekday, cents] of Object.entries(row.prices)) {
        next[draftKey(row.careOfferingId, weekday as CareWeekday)] =
          centsToEuroInput(cents);
      }
    }
    setDraft(next);
  }, [data, school, month]);

  const submit = async () => {
    if (!school) return;
    formErrors.clear();
    const prices: CarePriceInput[] = [];
    const fieldErrors: Record<string, string> = {};
    for (const row of rows) {
      for (const weekday of weekdays) {
        const value = draft[draftKey(row.careOfferingId, weekday)]?.trim();
        if (!value) continue;
        const cents = euroInputToCents(value);
        if (cents === null) {
          fieldErrors[draftKey(row.careOfferingId, weekday)] =
            "Bitte einen Betrag wie 32,50 eingeben.";
          continue;
        }
        prices.push({
          careOfferingId: row.careOfferingId,
          moduleName: row.moduleName,
          weekday,
          monthlyCents: cents,
        });
      }
    }
    if (Object.keys(fieldErrors).length > 0) {
      formErrors.invalid(
        "Bitte prüfen Sie die markierten Preise.",
        fieldErrors,
      );
      return;
    }
    if (prices.length === 0) {
      formErrors.invalid("Bitte tragen Sie mindestens einen Preis ein.", {});
      return;
    }
    setSaving(true);
    try {
      onSaved(await saveCarePriceList(school.id, month, prices));
      success(`Die Preise gelten ab ${formatMonth(month)}.`);
    } catch (err) {
      logger.error("care_price_list_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "die Preise" });
    } finally {
      setSaving(false);
    }
  };

  return (
    <FormModal
      isOpen={school !== null}
      onClose={onClose}
      closeDisabled={saving}
      isBackdropDismissDisabled
      title={school ? `Preise für ${school.name}` : "Preise"}
      size="lg"
      error={formErrors.error}
      footer={
        <EditActions
          onCancel={onClose}
          onSave={() => formRef.current?.requestSubmit()}
          saving={saving}
          saveLabel="Preise speichern"
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
        <div className="grid gap-3 sm:grid-cols-[minmax(0,16rem)_1fr] sm:items-end">
          <CustomSelect
            id="care-price-month"
            name="valid_from"
            label="Gültig ab"
            value={month}
            options={months.map((value) => ({
              value,
              label: `1. ${formatMonth(value)}`,
            }))}
            onChange={setMonth}
            error={formErrors.fieldError("valid_from")}
          />
          <p className="text-sm text-gray-600 sm:pb-2.5">
            Monatspreis je gebuchtem Wochentag. Leer lassen: Der bisherige Preis
            gilt weiter.
          </p>
        </div>
        {rows.length === 0 ? (
          <p className="text-sm text-gray-600">
            Diese Schule hat noch keine Module. Module kommen aus den gültigen
            Verträgen der Schule.
          </p>
        ) : (
          rows.map((row) => (
            <fieldset
              key={row.careOfferingId}
              className="rounded-xl border border-gray-200 p-4"
            >
              <legend className="px-1 text-sm font-semibold text-gray-900">
                {row.moduleName}
                {row.endsAt ? (
                  <span className="ml-2 font-normal text-gray-600">
                    {endsAtLabel(row.endsAt)}
                  </span>
                ) : null}
              </legend>
              <div className="grid grid-cols-3 gap-3 sm:grid-cols-5">
                {weekdays.map((weekday) => {
                  const key = draftKey(row.careOfferingId, weekday);
                  const booked = row.bookedWeekdays.includes(weekday);
                  return (
                    <Input
                      key={key}
                      id={`care-price-${key}`}
                      name={key}
                      controlSize="compact"
                      label={`${WEEKDAY_SHORT[weekday]} in €`}
                      aria-label={`${row.moduleName}, ${WEEKDAY_LONG[weekday]}, Monatspreis in Euro`}
                      inputMode="decimal"
                      placeholder={booked ? "fehlt" : "–"}
                      value={draft[key] ?? ""}
                      error={formErrors.fieldError(key)}
                      onChange={(event) =>
                        setDraft((current) => ({
                          ...current,
                          [key]: event.target.value,
                        }))
                      }
                    />
                  );
                })}
              </div>
            </fieldset>
          ))
        )}
      </form>
    </FormModal>
  );
}
