"use client";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";
import type { CalendarPeriod } from "~/lib/calendar-period-helpers";
import { getGermanWeekdayShort } from "~/lib/timetable-helpers";
import { timetableRequiredMark } from "../timetable-style";
import { Field } from "./field";
import { isoWeekday } from "./form-model";
import type { EventFormState, RepeatMode } from "./form-model";
import { WEEKDAYS } from "./use-event-form";

const REPEAT_OPTIONS: Array<{ value: RepeatMode; label: string }> = [
  { value: "none", label: "Nie" },
  { value: "weekly", label: "Jede Woche" },
  { value: "biweekly", label: "Alle 2 Wochen" },
];

function weekdayLabel(iso: number): string {
  const ref = new Date(2024, 0, 1);
  ref.setDate(ref.getDate() + (iso - 1));
  return getGermanWeekdayShort(ref);
}

export interface StepWiederholungProps {
  form: EventFormState;
  update: <K extends keyof EventFormState>(
    key: K,
    value: EventFormState[K],
  ) => void;
  updateRepeat: (repeat: RepeatMode) => void;
  /**
   * Sets the Planungszeitraum and, for new series, moves an out-of-period
   * Datum to the first occurrence inside the period (#2135).
   */
  selectCalendarPeriod: (nextId: string) => void;
  toggleWeekday: (iso: number) => void;
  fieldErrors: Record<string, string>;
  calendarPeriods: CalendarPeriod[];
  showPeriodField: boolean;
  expanded: boolean;
  isSeriesFlow: boolean;
  isEditingSeries: boolean;
  biweeklyUnavailable: boolean;
  abWeekHint: string;
  quickPreset: string;
  handleQuickPresetChange: (value: string) => void;
  dateWeekday: number;
  dateWeekdayName: string;
  manualWeekPattern: React.RefObject<1 | 2 | null>;
}

/**
 * Wizard step 2 "Wiederholung": repeat mode (one-off / weekly / A-B), the
 * weekday picker, the Planungszeitraum and the series-related notices. JSX moved
 * verbatim from the previous single-page form — all visibility rules unchanged.
 */
export function StepWiederholung({
  form,
  update,
  updateRepeat,
  selectCalendarPeriod,
  toggleWeekday,
  fieldErrors,
  calendarPeriods,
  showPeriodField,
  expanded,
  isSeriesFlow,
  isEditingSeries,
  biweeklyUnavailable,
  abWeekHint,
  quickPreset,
  handleQuickPresetChange,
  dateWeekday,
  dateWeekdayName,
  manualWeekPattern,
}: Readonly<StepWiederholungProps>) {
  const legacyWeekendWeekdays = form.weekdays.filter((iso) => iso > 5);
  const normalizeLegacyWeekend = (legacyWeekday: number) => {
    const weekdays = form.weekdays.filter((iso) => iso !== legacyWeekday);
    // This control explicitly migrates the selected legacy day to Monday. A
    // different workday (for example Friday) must not turn that migration into
    // a silent deletion of future weekend appointments.
    if (!weekdays.includes(1)) {
      weekdays.push(1);
    }
    update(
      "weekdays",
      weekdays.sort((a, b) => a - b),
    );
  };

  return (
    <>
      {expanded ? (
        <div className="flex flex-col gap-1">
          <span className="text-xs font-semibold text-gray-700">
            Wiederholt sich
          </span>
          <Tabs
            value={form.repeat}
            onValueChange={(value) => {
              const nextRepeat = value as RepeatMode;
              updateRepeat(nextRepeat);
              if (nextRepeat !== "none" && form.weekdays.length === 0) {
                const weekday = isoWeekday(form.date);
                update(
                  "weekdays",
                  weekday >= 1 && weekday <= 5 ? [weekday] : [1],
                );
              }
            }}
          >
            <TabsList aria-label="Wiederholung" className="w-fit">
              {REPEAT_OPTIONS.map((option) => (
                <TabsTrigger
                  key={option.value}
                  value={option.value}
                  disabled={
                    (isEditingSeries && option.value === "none") ||
                    (!isEditingSeries &&
                      option.value === "biweekly" &&
                      biweeklyUnavailable)
                  }
                >
                  {option.label}
                </TabsTrigger>
              ))}
            </TabsList>
          </Tabs>
        </div>
      ) : (
        <Field label="Wiederholt sich" htmlFor="event_quick_repeat">
          <CustomSelect
            id="event_quick_repeat"
            ariaLabel="Wiederholt sich"
            value={quickPreset}
            options={[
              { value: "einmalig", label: "Einmalig" },
              // On Sa/So a weekly preset would silently save a Monday
              // series (weekdays are Mo–Fr) — omit it instead.
              ...(dateWeekday >= 1 && dateWeekday <= 5
                ? [
                    {
                      value: "woechentlich-am",
                      label: `Wöchentlich am ${dateWeekdayName}`,
                    },
                  ]
                : []),
              { value: "jeden-wochentag", label: "Jeden Wochentag (Mo–Fr)" },
              { value: "benutzerdefiniert", label: "Benutzerdefiniert …" },
            ]}
            onChange={(next) => handleQuickPresetChange(next)}
          />
        </Field>
      )}

      {expanded && form.repeat === "biweekly" && (
        <div className="flex flex-col gap-1">
          <span className="text-xs font-semibold text-gray-700">
            Wochenrhythmus
          </span>
          <Tabs
            value={form.weekPattern === 1 ? "A" : "B"}
            onValueChange={(value) => {
              manualWeekPattern.current = value === "A" ? 1 : 2;
              update("weekPattern", value === "A" ? 1 : 2);
            }}
          >
            <TabsList aria-label="A/B-Woche" className="w-fit">
              <TabsTrigger value="A">Woche A</TabsTrigger>
              <TabsTrigger value="B">Woche B</TabsTrigger>
            </TabsList>
          </Tabs>
          <p className="text-xs text-gray-500">{abWeekHint}</p>
          {fieldErrors.weekPattern && (
            <p role="alert" className="text-moto-red mt-1 text-xs">
              {fieldErrors.weekPattern}
            </p>
          )}
        </div>
      )}

      {expanded && isSeriesFlow && (
        <>
          <div className="flex flex-col gap-1">
            <span className="text-xs font-semibold text-gray-700">
              Wochentage <span className={timetableRequiredMark}>*</span>
            </span>
            <div className="flex flex-wrap gap-1.5">
              {WEEKDAYS.map((iso) => {
                const isActive = form.weekdays.includes(iso);
                return (
                  <Button
                    key={iso}
                    type="button"
                    variant={isActive ? "primary" : "outline"}
                    size="compact"
                    className="min-w-[44px]"
                    onClick={() => toggleWeekday(iso)}
                    aria-pressed={isActive}
                  >
                    {weekdayLabel(iso)}
                  </Button>
                );
              })}
              {legacyWeekendWeekdays.map((iso) => (
                <Button
                  key={iso}
                  type="button"
                  variant="outline_danger"
                  size="compact"
                  className="min-w-[44px]"
                  onClick={() => normalizeLegacyWeekend(iso)}
                  aria-label={`Alten Wochenendtag ${weekdayLabel(iso)} zu Montag ändern`}
                >
                  {weekdayLabel(iso)} zu Mo
                </Button>
              ))}
            </div>
            {legacyWeekendWeekdays.length > 0 && (
              <p className="text-xs text-gray-500">
                Wochenendtermine aus bestehenden Serien werden zu Montag
                verschoben oder entfernt, wenn ein Werktag verbleibt.
              </p>
            )}
            {fieldErrors.weekdays && (
              <p role="alert" className="text-moto-red mt-1 text-xs">
                {fieldErrors.weekdays}
              </p>
            )}
          </div>

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            {showPeriodField ? (
              <Field
                label="Planungszeitraum"
                htmlFor="event_period"
                required
                error={fieldErrors.calendarPeriodId}
              >
                <CustomSelect
                  id="event_period"
                  ariaLabel="Planungszeitraum"
                  ariaDescribedBy={
                    fieldErrors.calendarPeriodId
                      ? "event_period_error"
                      : undefined
                  }
                  value={form.calendarPeriodId}
                  options={[
                    { value: "", label: "Zeitraum auswählen …" },
                    ...calendarPeriods.map((period) => ({
                      value: period.id,
                      label: period.name,
                    })),
                  ]}
                  onChange={selectCalendarPeriod}
                  required
                  invalid={Boolean(fieldErrors.calendarPeriodId)}
                  placeholder="Zeitraum auswählen …"
                />
              </Field>
            ) : (
              <div className="flex flex-col justify-end gap-1">
                <span className="text-xs font-semibold text-gray-700">
                  Planungszeitraum
                </span>
                <div className="flex h-10 items-center rounded-lg border border-gray-200 bg-gray-50 px-3 text-sm text-gray-600">
                  <span className="truncate">
                    Gilt in{" "}
                    <span className="font-semibold text-gray-800">
                      {calendarPeriods.find(
                        (p) => p.id === form.calendarPeriodId,
                      )?.name ?? "dem aktuellen Planungszeitraum"}
                    </span>
                  </span>
                </div>
                {fieldErrors.calendarPeriodId && (
                  <p role="alert" className="text-moto-red mt-1 text-xs">
                    {fieldErrors.calendarPeriodId}
                  </p>
                )}
              </div>
            )}
          </div>
        </>
      )}

      {isSeriesFlow && calendarPeriods.length === 0 && (
        <Alert
          type="error"
          message="Für diese Woche gibt es keinen aktiven Planungszeitraum. Lege zuerst oben im Plan einen Zeitraum an."
        />
      )}
    </>
  );
}
