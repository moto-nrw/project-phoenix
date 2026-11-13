"use client";

// Eine buchbare Woche eines Ferienzeitraums (#4659): Tage, ausrichtende OGS
// und die Zeitmodelle mit ihrem Preis. Ein gebuchtes Zeitmodell lässt sich
// nicht entfernen, nur archivieren (#5994). Eine Woche wird ganz oder
// tageweise gebucht; tageweise ist der Preis ein Tagespreis (#4701).

import { Archive, Plus, X } from "lucide-react";
import { useCallback } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { ISODateInput } from "~/components/ui/date-picker";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { Input } from "~/components/ui/input";
import { SectionLabel } from "~/components/ui/section-label";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { StatusBadge } from "~/components/ui/status-badge";
import { TimeField } from "~/components/ui/time-field";
import { formatDate } from "~/lib/date-helpers";
import { bookableHolidayDays, holidayCareDays } from "~/lib/holiday-days";
import type { HolidayBookingUnit, TraegerSchool } from "~/lib/traeger-api";
import { formatDayWithWeekday } from "./closing-day-format";
import type { WeekNoCareDays } from "./use-week-no-care-days";
import { HolidayDayPlacesGrid } from "./holiday-week-places";
import {
  emptyTimeModel,
  MAX_TIME_MODELS_PER_WEEK,
  type TimeModelDraft,
  weekBooked,
  type WeekDraft,
} from "./holiday-period-model";
import { InsetPanel } from "~/components/ui/inset-panel";

const BOOKING_UNITS = [
  { value: "week", label: "Ganze Woche" },
  { value: "day", label: "Einzelne Tage" },
] as const satisfies readonly { value: HolidayBookingUnit; label: string }[];

/** One line under the choice: what parents book and what the price means. */
function bookingUnitHint(unit: HolidayBookingUnit, booked: boolean): string {
  if (booked) {
    return unit === "day"
      ? "Eltern haben schon Tage gebucht. Buchungsart und Tage der Woche bleiben deshalb so."
      : "Eltern haben die Woche schon gebucht. Die Buchungsart bleibt deshalb so.";
  }
  return unit === "day"
    ? "Eltern wählen einzelne Tage von Montag bis Freitag. Der Preis gilt je Tag. Die Plätze gelten für jeden Tag."
    : "Eltern buchen alle Tage der Woche. Der Preis gilt für die ganze Woche.";
}

type FieldError = (name: string) => string | null | undefined;

/**
 * Nennt die Tage ohne Betreuung und was daraus folgt (#4702). Bei einzelnen
 * Tagen fällt nur dieser Tag weg (#6447); eine ganze Woche ist so nicht
 * buchbar.
 */
function noCareDaysNotice(
  noCare: WeekNoCareDays,
  week: WeekDraft,
  booked: boolean,
): { readonly type: "info" | "warning"; readonly message: string } {
  const days = noCare.days
    .map(
      (day) =>
        `${formatDayWithWeekday(day.date)} ${day.kind === "holiday" ? "Feiertag" : "Schließtag"} (${day.name})`,
    )
    .join(", ");
  const head = `In dieser Woche ist keine Betreuung: ${days}.`;
  if (week.bookingUnit === "day") {
    if (
      bookableHolidayDays(week.startsOn, week.endsOn, noCare.days).length === 0
    ) {
      return {
        type: "warning",
        message: `${head} Es bleibt kein Tag zum Buchen. Bitte wählen Sie andere Tage.`,
      };
    }
    return {
      type: "info",
      message: `${head} Eltern können ${noCare.days.length === 1 ? "diesen Tag" : "diese Tage"} nicht buchen. Die anderen Tage bleiben buchbar.`,
    };
  }
  if (noCare.keptAsStored) {
    return {
      type: "warning",
      message: `${head} Bisherige Buchungen bleiben. Neue Buchungen sind nicht möglich.`,
    };
  }
  return {
    type: "warning",
    message: booked
      ? `${head} Bitte wählen Sie andere Tage.`
      : `${head} So ist die Woche nicht buchbar. Wählen Sie bei „Buchung“ „Einzelne Tage“. Dann fällt nur dieser Tag weg.`,
  };
}

function weekTitle(index: number, week: WeekDraft): string {
  if (!week.startsOn || !week.endsOn) return `Woche ${index + 1}`;
  return `Woche ${index + 1}: ${formatDate(week.startsOn)} bis ${formatDate(week.endsOn)}`;
}

/**
 * An archived time model: shown, but no longer changed or offered. The days
 * booked on it keep its times.
 */
function ArchivedTimeModelRow({
  model,
  unit,
}: Readonly<{ model: TimeModelDraft; unit: HolidayBookingUnit }>) {
  return (
    <div className="flex flex-col gap-1 border-t border-gray-100 pt-3 first:border-t-0 first:pt-0">
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-sm font-medium text-gray-900">{model.name}</span>
        <StatusBadge tone="gray" size="xs" label="Archiviert" />
      </div>
      <p className="text-sm text-gray-600">
        {model.startsAt} bis {model.endsAt} Uhr, {model.price} Euro
        {unit === "day" ? " je Tag" : ""}. Neu buchen kann es niemand mehr.
        Gebuchte Tage behalten es.
      </p>
    </div>
  );
}

function TimeModelRow({
  model,
  unit,
  field,
  fieldError,
  onChange,
  onRemove,
}: Readonly<{
  model: TimeModelDraft;
  unit: HolidayBookingUnit;
  field: string;
  fieldError: FieldError;
  onChange: (model: TimeModelDraft) => void;
  onRemove: () => void;
}>) {
  if (model.archived) return <ArchivedTimeModelRow model={model} unit={unit} />;
  const startsError = fieldError(`${field}.starts_at`);
  const endsError = fieldError(`${field}.ends_at`);
  return (
    <div className="grid grid-cols-1 gap-3 border-t border-gray-100 pt-3 first:border-t-0 first:pt-0 sm:grid-cols-[minmax(0,2fr)_minmax(0,1fr)_minmax(0,1fr)_minmax(0,1fr)_7rem] sm:items-start">
      <Input
        name={`${field}.name`}
        label="Name"
        required
        controlSize="touch"
        placeholder="z. B. bis 13 Uhr"
        value={model.name}
        maxLength={80}
        error={fieldError(`${field}.name`) ?? undefined}
        onChange={(event) => onChange({ ...model, name: event.target.value })}
      />
      <div>
        <TimeField
          name={`${field}.starts_at`}
          label="Beginn"
          required
          // The example shows once, under an empty field; a filled field
          // needs none.
          hint={model.startsAt ? "" : "z. B. 08:00"}
          placeholder=""
          value={model.startsAt}
          invalid={Boolean(startsError)}
          describedBy={startsError ? `${field}.starts_at-error` : undefined}
          onChange={(startsAt) => onChange({ ...model, startsAt })}
        />
        <FieldErrorHint id={`${field}.starts_at-error`} message={startsError} />
      </div>
      <div>
        <TimeField
          name={`${field}.ends_at`}
          label="Ende"
          required
          hint={model.endsAt ? "" : "z. B. 13:00"}
          placeholder=""
          value={model.endsAt}
          invalid={Boolean(endsError)}
          describedBy={endsError ? `${field}.ends_at-error` : undefined}
          onChange={(endsAt) => onChange({ ...model, endsAt })}
        />
        <FieldErrorHint id={`${field}.ends_at-error`} message={endsError} />
      </div>
      <Input
        name={`${field}.price_cents`}
        label={unit === "day" ? "Preis je Tag in Euro" : "Preis in Euro"}
        required
        controlSize="touch"
        inputMode="decimal"
        placeholder="z. B. 45,00"
        value={model.price}
        // What a new price of a booked time model changes stands once
        // above the weeks, not under every price.
        error={fieldError(`${field}.price_cents`) ?? undefined}
        onChange={(event) => onChange({ ...model, price: event.target.value })}
      />
      {/* A fixed column, so rows with "Archivieren" and rows with the X
          keep their fields in line. */}
      <div className="flex sm:justify-end sm:pt-8">
        {model.protected ? (
          // Bookings name it: it stays for their days and leaves the offer.
          <Button
            type="button"
            variant="outline"
            size="compact"
            className="gap-1.5"
            aria-label={`Zeitmodell ${model.name || "ohne Namen"} archivieren`}
            onClick={() => onChange({ ...model, archived: true })}
          >
            <Archive className="h-4 w-4" aria-hidden="true" />
            Archivieren
          </Button>
        ) : (
          // Icon only next to the row on wide screens; on a phone the row
          // stacks, so the button names what it removes. The name starts
          // with that visible text, so voice control finds it (WCAG 2.5.3).
          <Button
            type="button"
            variant="ghost"
            size="compact"
            aria-label={`Zeitmodell entfernen: ${model.name || "ohne Namen"}`}
            onClick={onRemove}
          >
            <X className="h-4 w-4" aria-hidden="true" />
            <span className="sm:hidden">Zeitmodell entfernen</span>
          </Button>
        )}
      </div>
    </div>
  );
}

export function HolidayWeekEditor({
  index,
  week,
  schools,
  noCareDays,
  fieldError,
  onChange,
  onRemove,
  onDateValidity,
}: Readonly<{
  index: number;
  week: WeekDraft;
  schools: readonly TraegerSchool[];
  /** Days without care at the host school (#4702). */
  noCareDays?: WeekNoCareDays;
  fieldError: FieldError;
  onChange: (week: WeekDraft) => void;
  onRemove: () => void;
  /** Reports whether the typed text of a date field is a valid date. */
  onDateValidity: (
    weekKey: string,
    field: "starts_on" | "ends_on",
    valid: boolean,
  ) => void;
}>) {
  const prefix = `weeks.${index}`;
  const startsValidity = useCallback(
    (valid: boolean) => onDateValidity(week.key, "starts_on", valid),
    [onDateValidity, week.key],
  );
  const endsValidity = useCallback(
    (valid: boolean) => onDateValidity(week.key, "ends_on", valid),
    [onDateValidity, week.key],
  );
  const hostError = fieldError(`${prefix}.host_school_id`);
  const setTimeModel = (modelIndex: number, model: TimeModelDraft) =>
    onChange({
      ...week,
      timeModels: week.timeModels.map((current, i) =>
        i === modelIndex ? model : current,
      ),
    });

  const title = weekTitle(index, week);
  // A week with bookings stays: moto refuses to remove it when saving, so the
  // form does not offer it (like a booked time model, which can only be
  // archived).
  const booked = weekBooked(week);
  const unitHintId = `${prefix}.booking_unit-hint`;
  const notice =
    noCareDays && noCareDays.days.length > 0
      ? noCareDaysNotice(noCareDays, week, booked)
      : null;
  return (
    // Eine Zeile der Formularkarte, keine eigene Karte (#5146): die Wochen
    // trennt eine Linie wie die Zeitmodelle darunter.
    <section className="space-y-4 border-t border-gray-200 pt-6 first:border-t-0 first:pt-0">
      {/* The week's head on a muted strip: the weeks stand apart at a
          glance, and the fields below keep the card's full width. */}
      <InsetPanel
        tone="muted"
        padding="compact"
        className="flex flex-wrap items-center justify-between gap-3"
      >
        <h4 className="text-sm font-semibold text-gray-900">{title}</h4>
        {booked ? (
          <p className="text-sm text-gray-600">
            Für diese Woche gibt es Anmeldungen. Sie bleibt im Zeitraum.
          </p>
        ) : (
          <Button
            type="button"
            variant="outline_danger"
            size="md"
            onClick={onRemove}
          >
            Woche entfernen
          </Button>
        )}
      </InsetPanel>
      <div role="group" aria-label={title} className="space-y-4">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <div>
            <ISODateInput
              id={`${prefix}.starts_on`}
              label="Erster Tag"
              required
              value={week.startsOn}
              error={fieldError(`${prefix}.starts_on`)}
              onValidityChange={startsValidity}
              onChange={(startsOn) => onChange({ ...week, startsOn })}
            />
          </div>
          <div>
            <ISODateInput
              id={`${prefix}.ends_on`}
              label="Letzter Tag"
              required
              value={week.endsOn}
              min={week.startsOn || undefined}
              minDateError="Der letzte Tag liegt vor dem ersten Tag."
              error={fieldError(`${prefix}.ends_on`)}
              onValidityChange={endsValidity}
              onChange={(endsOn) => onChange({ ...week, endsOn })}
            />
          </div>
          <CustomSelect
            id={`${prefix}.host_school_id`}
            name={`${prefix}.host_school_id`}
            label="Ausrichtende OGS"
            required
            error={hostError}
            placeholder="Schule wählen"
            value={week.hostSchoolId}
            options={schools.map((school) => ({
              value: school.id,
              label: school.name,
            }))}
            onChange={(hostSchoolId) => onChange({ ...week, hostSchoolId })}
          />
        </div>
        {notice?.type === "info" ? (
          <Alert type="info" message={notice.message} />
        ) : notice ? (
          <Alert type="warning" message={notice.message} />
        ) : null}

        <div className="space-y-2">
          <SectionLabel as="p" id={`${prefix}.booking_unit-label`}>
            Buchung
          </SectionLabel>
          <SegmentedControl
            items={BOOKING_UNITS.map((item) => ({ ...item, disabled: booked }))}
            value={week.bookingUnit ?? "week"}
            ariaLabel={`Buchung von ${title}`}
            ariaDescribedBy={unitHintId}
            onChange={(bookingUnit) => onChange({ ...week, bookingUnit })}
          />
          <p id={unitHintId} className="text-sm text-gray-600">
            {bookingUnitHint(week.bookingUnit ?? "week", booked)}
          </p>
          {week.bookingUnit === "day" && week.startsOn && week.endsOn ? (
            <HolidayDayPlacesGrid
              dates={holidayCareDays(week.startsOn, week.endsOn)}
              noCareDays={noCareDays?.days ?? []}
              labelId={`${prefix}.care_days-label`}
            />
          ) : null}
        </div>

        <div className="space-y-3">
          {/* What a time model is stands once above the weeks. */}
          <h5 className="text-sm font-semibold text-gray-900">Zeitmodelle</h5>
          {week.timeModels.every((model) => model.archived) ? (
            <p className="text-sm text-gray-600">
              Diese Woche hat noch kein buchbares Zeitmodell. Zum
              Veröffentlichen braucht jede Woche mindestens eins.
            </p>
          ) : null}
          {week.timeModels.map((model, modelIndex) => (
            <TimeModelRow
              key={model.key}
              model={model}
              unit={week.bookingUnit ?? "week"}
              field={`${prefix}.time_models.${modelIndex}`}
              fieldError={fieldError}
              onChange={(next) => setTimeModel(modelIndex, next)}
              onRemove={() =>
                onChange({
                  ...week,
                  timeModels: week.timeModels.filter(
                    (_, i) => i !== modelIndex,
                  ),
                })
              }
            />
          ))}
          {week.timeModels.length < MAX_TIME_MODELS_PER_WEEK ? (
            <Button
              type="button"
              variant="outline"
              size="md"
              className="gap-2"
              onClick={() =>
                onChange({
                  ...week,
                  timeModels: [...week.timeModels, emptyTimeModel()],
                })
              }
            >
              <Plus className="h-4 w-4" aria-hidden="true" />
              Zeitmodell hinzufügen
            </Button>
          ) : (
            <p className="text-sm text-gray-600">
              Eine Woche hat höchstens {MAX_TIME_MODELS_PER_WEEK} Zeitmodelle.
            </p>
          )}
        </div>
      </div>
    </section>
  );
}
