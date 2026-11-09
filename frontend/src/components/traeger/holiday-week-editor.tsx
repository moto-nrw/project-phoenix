"use client";

// Eine buchbare Woche eines Ferienzeitraums (#4659): Tage, ausrichtende OGS
// und die Zeitmodelle mit ihrem Preis. Ein gebuchtes Zeitmodell lässt sich
// nicht entfernen, nur archivieren (#5994).

import { Archive, Plus, X } from "lucide-react";
import { useCallback } from "react";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { ISODateInput } from "~/components/ui/date-picker";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { Input } from "~/components/ui/input";
import { StatusBadge } from "~/components/ui/status-badge";
import { TimeField } from "~/components/ui/time-field";
import { formatDate } from "~/lib/date-helpers";
import type { TraegerSchool } from "~/lib/traeger-api";
import {
  emptyTimeModel,
  MAX_TIME_MODELS_PER_WEEK,
  type TimeModelDraft,
  type WeekDraft,
} from "./holiday-period-model";

type FieldError = (name: string) => string | null | undefined;

function weekTitle(index: number, week: WeekDraft): string {
  if (!week.startsOn || !week.endsOn) return `Woche ${index + 1}`;
  return `Woche ${index + 1}: ${formatDate(week.startsOn)} bis ${formatDate(week.endsOn)}`;
}

/**
 * An archived time model: shown, but no longer changed or offered. The days
 * booked on it keep its times.
 */
function ArchivedTimeModelRow({ model }: Readonly<{ model: TimeModelDraft }>) {
  return (
    <div className="flex flex-col gap-1 border-t border-gray-100 pt-3 first:border-t-0 first:pt-0">
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-sm font-medium text-gray-900">{model.name}</span>
        <StatusBadge tone="gray" size="xs" label="Archiviert" />
      </div>
      <p className="text-sm text-gray-600">
        {model.startsAt} bis {model.endsAt} Uhr, {model.price} Euro. Neu buchen
        kann es niemand mehr. Gebuchte Tage behalten es.
      </p>
    </div>
  );
}

function TimeModelRow({
  model,
  field,
  fieldError,
  onChange,
  onRemove,
}: Readonly<{
  model: TimeModelDraft;
  field: string;
  fieldError: FieldError;
  onChange: (model: TimeModelDraft) => void;
  onRemove: () => void;
}>) {
  if (model.archived) return <ArchivedTimeModelRow model={model} />;
  const startsError = fieldError(`${field}.starts_at`);
  const endsError = fieldError(`${field}.ends_at`);
  return (
    <div className="grid grid-cols-1 gap-3 border-t border-gray-100 pt-3 first:border-t-0 first:pt-0 sm:grid-cols-[minmax(0,2fr)_minmax(0,1fr)_minmax(0,1fr)_minmax(0,1fr)_7rem] sm:items-start">
      <Input
        name={`${field}.name`}
        label="Name"
        required
        controlSize="compact"
        placeholder="bis 13 Uhr"
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
          hint="z. B. 08:00"
          placeholder="08:00"
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
          hint="z. B. 13:00"
          placeholder="13:00"
          value={model.endsAt}
          invalid={Boolean(endsError)}
          describedBy={endsError ? `${field}.ends_at-error` : undefined}
          onChange={(endsAt) => onChange({ ...model, endsAt })}
        />
        <FieldErrorHint id={`${field}.ends_at-error`} message={endsError} />
      </div>
      <Input
        name={`${field}.price_cents`}
        label="Preis in Euro"
        required
        controlSize="compact"
        inputMode="decimal"
        placeholder="45,00"
        value={model.price}
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
  fieldError,
  onChange,
  onRemove,
  onDateValidity,
}: Readonly<{
  index: number;
  week: WeekDraft;
  schools: readonly TraegerSchool[];
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
  return (
    // Eine Zeile der Formularkarte, keine eigene Karte (#5146): die Wochen
    // trennt eine Linie wie die Zeitmodelle darunter.
    <section className="space-y-4 border-t border-gray-200 pt-6 first:border-t-0 first:pt-0">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h4 className="text-sm font-semibold text-gray-900">{title}</h4>
        <Button
          type="button"
          variant="outline_danger"
          size="md"
          onClick={onRemove}
        >
          Woche entfernen
        </Button>
      </div>
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

        <div className="space-y-3">
          <div>
            <h5 className="text-sm font-semibold text-gray-900">Zeitmodelle</h5>
            <p className="text-sm text-gray-500">
              Zum Beispiel „bis 13 Uhr“ und „bis 16 Uhr“, jedes mit eigenem
              Preis.
            </p>
          </div>
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
