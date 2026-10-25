"use client";

// Formular eines Ferienzeitraums (#4659): Anmeldung, Hinweise für Eltern und
// die buchbaren Wochen mit ausrichtender OGS und Zeitmodellen.

import { Plus } from "lucide-react";
import { useRef, useState } from "react";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { ISODateInput } from "~/components/ui/date-picker";
import { EditActions } from "~/components/ui/edit-actions";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { SectionCard } from "~/components/ui/section-card";
import { Textarea } from "~/components/ui/textarea";
import { useApiFormError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  createHolidayPeriod,
  type HolidayPeriod,
  type TraegerSchool,
  updateHolidayPeriod,
} from "~/lib/traeger-api";
import {
  draftFromPeriod,
  emptyHolidayPeriod,
  emptyWeek,
  type HolidayPeriodDraft,
  inputFromDraft,
  validateHolidayPeriod,
  type WeekDraft,
} from "./holiday-period-model";
import { HolidayWeekEditor } from "./holiday-week-editor";

const logger = createLogger({ component: "HolidayPeriodForm" });

export function HolidayPeriodForm({
  period,
  schools,
  onSaved,
  onCancel,
  onDirtyChange,
}: Readonly<{
  /** The stored period; without it the form creates a new one. */
  period?: HolidayPeriod;
  schools: readonly TraegerSchool[];
  onSaved: (period: HolidayPeriod) => void;
  onCancel: () => void;
  /** Reports whether the form holds unsaved changes. */
  onDirtyChange?: (dirty: boolean) => void;
}>) {
  const formRef = useRef<HTMLFormElement>(null);
  const [draft, setDraftState] = useState<HolidayPeriodDraft>(() =>
    period ? draftFromPeriod(period) : emptyHolidayPeriod(),
  );
  const [saving, setSaving] = useState(false);
  const errorPath = useApiFormError(formRef);
  const fieldError = (name: string) => errorPath.fieldError(name);

  const setDraft = (next: HolidayPeriodDraft) => {
    setDraftState(next);
    onDirtyChange?.(true);
  };
  const setWeek = (index: number, week: WeekDraft) =>
    setDraft({
      ...draft,
      weeks: draft.weeks.map((current, i) => (i === index ? week : current)),
    });

  const save = async () => {
    const errors = validateHolidayPeriod(draft);
    if (Object.keys(errors).length > 0) {
      errorPath.invalid("Bitte prüfen Sie die markierten Felder.", errors);
      return;
    }
    errorPath.clear();
    setSaving(true);
    try {
      const input = inputFromDraft(draft);
      const saved = period
        ? await updateHolidayPeriod(period.id, input)
        : await createHolidayPeriod(input);
      setDraftState(draftFromPeriod(saved));
      onDirtyChange?.(false);
      onSaved(saved);
    } catch (err) {
      logger.error("holiday_period_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void errorPath.show(err, { object: "den Ferienzeitraum" });
    } finally {
      setSaving(false);
    }
  };

  return (
    <form
      ref={formRef}
      noValidate
      className="space-y-6"
      onSubmit={(event) => {
        event.preventDefault();
        void save();
      }}
    >
      <FormErrorAlert message={errorPath.error} />

      <SectionCard title="Anmeldung">
        <div className="space-y-4">
          <Input
            name="name"
            label="Name des Ferienzeitraums"
            placeholder="Osterferien 2027"
            value={draft.name}
            maxLength={120}
            error={fieldError("name") ?? undefined}
            onChange={(event) =>
              setDraft({ ...draft, name: event.target.value })
            }
          />
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div>
              <ISODateInput
                id="registration_opens_on"
                label="Anmeldebeginn"
                value={draft.registrationOpensOn}
                onChange={(registrationOpensOn) =>
                  setDraft({ ...draft, registrationOpensOn })
                }
              />
              <FieldErrorHint
                id="registration_opens_on-error"
                message={fieldError("registration_opens_on")}
              />
            </div>
            <div>
              <ISODateInput
                id="registration_closes_on"
                label="Anmeldeschluss"
                value={draft.registrationClosesOn}
                onChange={(registrationClosesOn) =>
                  setDraft({ ...draft, registrationClosesOn })
                }
              />
              <FieldErrorHint
                id="registration_closes_on-error"
                message={fieldError("registration_closes_on")}
              />
            </div>
          </div>
          <label htmlFor="binding" className="flex items-start gap-3">
            <Checkbox
              id="binding"
              name="binding"
              checked={draft.binding}
              onChange={(event) =>
                setDraft({ ...draft, binding: event.target.checked })
              }
            />
            <span className="text-sm text-gray-900">
              Die Anmeldung ist verbindlich.
            </span>
          </label>
          <Textarea
            name="notes"
            label="Hinweise für Eltern"
            rows={3}
            maxLength={2000}
            placeholder="Zum Beispiel: Mittagessen gibt es vor Ort. Oder: Bitte geben Sie ein Lunchpaket mit."
            value={draft.notes}
            error={fieldError("notes") ?? undefined}
            onChange={(event) =>
              setDraft({ ...draft, notes: event.target.value })
            }
          />
        </div>
      </SectionCard>

      <SectionCard
        title="Wochen"
        description="Jede Woche findet an einer Schule Ihres Trägers statt."
      >
        <div className="space-y-4">
          {draft.weeks.length === 0 ? (
            <p className="text-sm text-gray-600">
              Noch keine Woche. Zum Veröffentlichen braucht der Ferienzeitraum
              mindestens eine Woche.
            </p>
          ) : (
            draft.weeks.map((week, index) => (
              <HolidayWeekEditor
                key={week.key}
                index={index}
                week={week}
                schools={schools}
                fieldError={fieldError}
                onChange={(next) => setWeek(index, next)}
                onRemove={() =>
                  setDraft({
                    ...draft,
                    weeks: draft.weeks.filter((_, i) => i !== index),
                  })
                }
              />
            ))
          )}
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={() =>
              setDraft({ ...draft, weeks: [...draft.weeks, emptyWeek()] })
            }
          >
            <Plus className="h-4 w-4" aria-hidden="true" />
            Woche hinzufügen
          </Button>
        </div>
      </SectionCard>

      <EditActions onCancel={onCancel} saving={saving} saveType="submit" />
    </form>
  );
}
