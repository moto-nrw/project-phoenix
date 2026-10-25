"use client";

// Eine buchbare Woche eines Ferienzeitraums (#4659): Tage, ausrichtende OGS
// und die Zeitmodelle mit ihrem Preis.

import { Plus, X } from "lucide-react";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { ISODateInput } from "~/components/ui/date-picker";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { Input } from "~/components/ui/input";
import { TimeField } from "~/components/ui/time-field";
import { formatDate } from "~/lib/date-helpers";
import type { TraegerSchool } from "~/lib/traeger-api";
import {
  emptyTimeModel,
  type TimeModelDraft,
  type WeekDraft,
} from "./holiday-period-model";

type FieldError = (name: string) => string | null | undefined;

function weekTitle(index: number, week: WeekDraft): string {
  if (!week.startsOn || !week.endsOn) return `Woche ${index + 1}`;
  return `Woche ${index + 1}: ${formatDate(week.startsOn)} bis ${formatDate(week.endsOn)}`;
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
  const startsError = fieldError(`${field}.starts_at`);
  const endsError = fieldError(`${field}.ends_at`);
  return (
    <div className="grid grid-cols-1 gap-3 border-t border-gray-100 pt-3 first:border-t-0 first:pt-0 sm:grid-cols-[minmax(0,2fr)_minmax(0,1fr)_minmax(0,1fr)_minmax(0,1fr)_auto] sm:items-start">
      <Input
        name={`${field}.name`}
        label="Name"
        controlSize="compact"
        placeholder="bis 13 Uhr"
        value={model.name}
        maxLength={80}
        error={fieldError(`${field}.name`) ?? undefined}
        onChange={(event) => onChange({ ...model, name: event.target.value })}
      />
      <div>
        <TimeField
          label="Beginn"
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
          label="Ende"
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
        controlSize="compact"
        inputMode="decimal"
        placeholder="45,00"
        value={model.price}
        error={fieldError(`${field}.price_cents`) ?? undefined}
        onChange={(event) => onChange({ ...model, price: event.target.value })}
      />
      <div className="flex sm:pt-7">
        <Button
          type="button"
          variant="ghost"
          size="icon"
          aria-label={`Zeitmodell ${model.name || "ohne Namen"} entfernen`}
          onClick={onRemove}
        >
          <X className="h-4 w-4" aria-hidden="true" />
        </Button>
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
}: Readonly<{
  index: number;
  week: WeekDraft;
  schools: readonly TraegerSchool[];
  fieldError: FieldError;
  onChange: (week: WeekDraft) => void;
  onRemove: () => void;
}>) {
  const prefix = `weeks.${index}`;
  const hostError = fieldError(`${prefix}.host_school_id`);
  const setTimeModel = (modelIndex: number, model: TimeModelDraft) =>
    onChange({
      ...week,
      timeModels: week.timeModels.map((current, i) =>
        i === modelIndex ? model : current,
      ),
    });

  return (
    <fieldset className="moto-content-surface space-y-4 rounded-xl border p-4">
      <div className="flex items-center justify-between gap-3">
        <legend className="text-base font-semibold text-gray-900">
          {weekTitle(index, week)}
        </legend>
        <Button
          type="button"
          variant="outline"
          size="compact"
          onClick={onRemove}
        >
          Woche entfernen
        </Button>
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <div>
          <ISODateInput
            id={`${prefix}.starts_on`}
            label="Erster Tag"
            value={week.startsOn}
            onChange={(startsOn) => onChange({ ...week, startsOn })}
          />
          <FieldErrorHint
            id={`${prefix}.starts_on-error`}
            message={fieldError(`${prefix}.starts_on`)}
          />
        </div>
        <div>
          <ISODateInput
            id={`${prefix}.ends_on`}
            label="Letzter Tag"
            value={week.endsOn}
            min={week.startsOn || undefined}
            minDateError="Der letzte Tag liegt vor dem ersten Tag."
            onChange={(endsOn) => onChange({ ...week, endsOn })}
          />
          <FieldErrorHint
            id={`${prefix}.ends_on-error`}
            message={fieldError(`${prefix}.ends_on`)}
          />
        </div>
        <div>
          <span
            id={`${prefix}.host_school_id-label`}
            className="mb-2 block text-sm font-medium text-gray-700"
          >
            Ausrichtende OGS
          </span>
          <CustomSelect
            name={`${prefix}.host_school_id`}
            ariaLabelledBy={`${prefix}.host_school_id-label`}
            ariaDescribedBy={
              hostError ? `${prefix}.host_school_id-error` : undefined
            }
            invalid={Boolean(hostError)}
            placeholder="Schule wählen"
            value={week.hostSchoolId}
            options={schools.map((school) => ({
              value: school.id,
              label: school.name,
            }))}
            onChange={(hostSchoolId) => onChange({ ...week, hostSchoolId })}
          />
          <FieldErrorHint
            id={`${prefix}.host_school_id-error`}
            message={hostError}
          />
        </div>
      </div>

      <div className="space-y-3">
        <div>
          <h3 className="text-sm font-semibold text-gray-900">Zeitmodelle</h3>
          <p className="text-sm text-gray-500">
            Zum Beispiel „bis 13 Uhr“ und „bis 16 Uhr“, jedes mit eigenem Preis.
          </p>
        </div>
        {week.timeModels.length === 0 ? (
          <p className="text-sm text-gray-600">
            Diese Woche hat noch kein Zeitmodell. Zum Veröffentlichen braucht
            jede Woche mindestens eins.
          </p>
        ) : (
          week.timeModels.map((model, modelIndex) => (
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
          ))
        )}
        <Button
          type="button"
          variant="ghost"
          size="compact"
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
      </div>
    </fieldset>
  );
}
