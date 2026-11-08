"use client";

// Die Preise eines Moduls im Preisdialog (#6196): die Preisform, „Essen
// extra“ und eine Tabelle mit zwei Zeilen, oben das 1. Kind, unten jedes
// weitere Kind. Spalten sind die Wochentage oder die Tage je Woche (1 bis 5).
// Ein leeres Feld für weitere Kinder heißt: derselbe Preis wie für das
// 1. Kind. Zwei Abkürzungen sparen Tipparbeit: „Für alle Tage übernehmen“
// und „Werte übernehmen von“ einem anderen Modul.

import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { CustomSelect } from "~/components/ui/custom-select";
import { FormFieldset } from "~/components/ui/form-fieldset";
import { Input } from "~/components/ui/input";
import { SegmentedControl } from "~/components/ui/segmented-control";
import type {
  CarePriceForm,
  CareWeekday,
} from "~/lib/traeger-care-billing-api";
import {
  SCALE_STEPS,
  WEEKDAY_LONG,
  WEEKDAY_SHORT,
  daysLabel,
  endsAtLabel,
} from "./care-billing-format";

/** The draft of one module: its form, meal note and the typed amounts. */
export interface ModuleDraft {
  readonly form: CarePriceForm;
  readonly mealExtra: boolean;
  /** Euro text per key: a weekday ("mon") or a number of days ("3"). */
  readonly first: Readonly<Record<string, string>>;
  readonly further: Readonly<Record<string, string>>;
}

export const EMPTY_MODULE_DRAFT: ModuleDraft = {
  form: "weekday",
  mealExtra: false,
  first: {},
  further: {},
};

/** The field key of one amount, also the key of its error. */
export function priceFieldKey(
  offeringId: string,
  key: string,
  sibling: boolean,
): string {
  return `${offeringId}:${key}${sibling ? ":weitere" : ""}`;
}

interface Column {
  readonly key: string;
  readonly short: string;
  readonly long: string;
}

export function draftColumns(
  form: CarePriceForm,
  weekdays: readonly CareWeekday[],
): Column[] {
  if (form === "days_per_week") {
    return SCALE_STEPS.map((days) => ({
      key: String(days),
      short: daysLabel(days),
      long: `${daysLabel(days)} je Woche`,
    }));
  }
  return weekdays.map((weekday) => ({
    key: weekday,
    short: WEEKDAY_SHORT[weekday],
    long: WEEKDAY_LONG[weekday],
  }));
}

const FORM_ITEMS = [
  { value: "weekday" as const, label: "Je Wochentag" },
  { value: "days_per_week" as const, label: "Nach Tagen je Woche" },
];

export function CarePriceModuleEditor({
  offeringId,
  moduleName,
  endsAt,
  bookedWeekdays,
  weekdays,
  draft,
  copySources,
  fieldError,
  onChange,
  onCopyFrom,
}: Readonly<{
  offeringId: string;
  moduleName: string;
  endsAt: string;
  bookedWeekdays: readonly CareWeekday[];
  /** The weekday columns of the school. */
  weekdays: readonly CareWeekday[];
  draft: ModuleDraft;
  /** Other modules of the school whose amounts this one may take. */
  copySources: readonly { value: string; label: string }[];
  fieldError: (key: string) => string | undefined;
  onChange: (next: ModuleDraft) => void;
  onCopyFrom: (offeringId: string) => void;
}>) {
  const columns = draftColumns(draft.form, weekdays);
  const firstColumn = columns[0];
  const firstValue = firstColumn ? draft.first[firstColumn.key]?.trim() : "";
  const firstFurther = firstColumn
    ? draft.further[firstColumn.key]?.trim()
    : "";

  const setAmount = (key: string, sibling: boolean, value: string) =>
    onChange(
      sibling
        ? { ...draft, further: { ...draft.further, [key]: value } }
        : { ...draft, first: { ...draft.first, [key]: value } },
    );

  // Monday's amounts for every weekday: Ispringen prices most modules the
  // same on every day.
  const fillAllDays = () => {
    const first = { ...draft.first };
    const further = { ...draft.further };
    for (const column of columns) {
      if (firstValue) first[column.key] = firstValue;
      if (firstFurther) further[column.key] = firstFurther;
    }
    onChange({ ...draft, first, further });
  };

  const rows = [
    { sibling: false, label: "1. Kind" },
    { sibling: true, label: "Jedes weitere Kind" },
  ];

  return (
    <FormFieldset
      legend={
        <>
          {moduleName}
          {endsAt ? (
            <span className="ml-2 font-normal text-gray-600">
              {endsAtLabel(endsAt)}
            </span>
          ) : null}
        </>
      }
    >
      <div className="space-y-4">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <SegmentedControl
            items={FORM_ITEMS}
            value={draft.form}
            onChange={(form) => onChange({ ...draft, form })}
            ariaLabel={`Preisform für ${moduleName}`}
          />
          <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
            <label
              htmlFor={`care-price-meal-${offeringId}`}
              className="flex items-center gap-2 text-sm text-gray-900"
            >
              <Checkbox
                id={`care-price-meal-${offeringId}`}
                checked={draft.mealExtra}
                onChange={(event) =>
                  onChange({ ...draft, mealExtra: event.target.checked })
                }
              />
              Essen extra
            </label>
            {copySources.length > 0 ? (
              <CustomSelect
                id={`care-price-copy-${offeringId}`}
                value=""
                placeholder="Werte übernehmen von …"
                ariaLabel={`Werte für ${moduleName} von einem anderen Modul übernehmen`}
                options={copySources}
                onChange={onCopyFrom}
                controlSize="compact"
                className="w-56"
              />
            ) : null}
          </div>
        </div>
        <p className="text-sm text-gray-600">
          {draft.form === "days_per_week"
            ? "Der Monatsbeitrag hängt von der Zahl der gebuchten Tage ab, nicht vom Wochentag."
            : "Der Monatsbeitrag ist die Summe der gebuchten Wochentage."}{" "}
          Leer bei weiteren Kindern heißt: gleicher Preis wie beim 1. Kind.
          {draft.mealExtra ? " Das Essen rechnet die Mensa ab." : ""}
        </p>
        <div
          className="grid grid-cols-3 gap-x-3 gap-y-2 sm:grid-cols-[8.5rem_repeat(var(--cols),minmax(0,1fr))] sm:items-center"
          style={{ ["--cols" as string]: columns.length }}
        >
          <span className="hidden sm:block" aria-hidden="true" />
          {columns.map((column) => (
            <span
              key={column.key}
              className="hidden text-xs font-medium text-gray-600 sm:block"
              aria-hidden="true"
            >
              {column.short}
            </span>
          ))}
          {rows.map((row) => (
            <div key={row.label} className="contents">
              <span className="col-span-3 pt-1 text-sm font-medium text-gray-900 sm:col-span-1 sm:pt-0">
                {row.label}
              </span>
              {columns.map((column) => {
                const field = priceFieldKey(
                  offeringId,
                  column.key,
                  row.sibling,
                );
                const booked =
                  draft.form === "weekday"
                    ? bookedWeekdays.includes(column.key as CareWeekday)
                    : Number(column.key) <= bookedWeekdays.length;
                const value = row.sibling
                  ? draft.further[column.key]
                  : draft.first[column.key];
                return (
                  <div key={field} className="min-w-0">
                    <span
                      className="mb-1 block text-xs text-gray-600 sm:hidden"
                      aria-hidden="true"
                    >
                      {column.short}
                    </span>
                    <Input
                      id={`care-price-${field}`}
                      name={field}
                      controlSize="compact"
                      aria-label={`${moduleName}, ${column.long}, ${row.label}, Monatspreis in Euro`}
                      inputMode="decimal"
                      placeholder={
                        row.sibling ? "wie 1." : booked ? "fehlt" : "–"
                      }
                      value={value ?? ""}
                      error={fieldError(field)}
                      onChange={(event) =>
                        setAmount(column.key, row.sibling, event.target.value)
                      }
                    />
                  </div>
                );
              })}
            </div>
          ))}
        </div>
        {draft.form === "weekday" && columns.length > 1 && firstValue ? (
          <Button
            type="button"
            variant="link"
            size="compact"
            onClick={fillAllDays}
            aria-label={`${firstColumn?.long}-Preise von ${moduleName} für alle Tage übernehmen`}
          >
            {firstColumn?.short}-Preise für alle Tage übernehmen
          </Button>
        ) : null}
      </div>
    </FormFieldset>
  );
}
