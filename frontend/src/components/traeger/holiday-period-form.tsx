"use client";

// Formular eines Ferienzeitraums (#4659): Anmeldung, Hinweise für Eltern und
// die buchbaren Wochen mit ausrichtender OGS und Zeitmodellen.

import { Plus } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { ISODateInput } from "~/components/ui/date-picker";
import { EditActions } from "~/components/ui/edit-actions";
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
  MAX_HOLIDAY_WEEKS,
  validateHolidayPeriod,
  type WeekDraft,
} from "./holiday-period-model";
import { HolidayWeekEditor } from "./holiday-week-editor";

const logger = createLogger({ component: "HolidayPeriodForm" });

const INVALID_DATE_TEXT =
  "Bitte geben Sie ein gültiges Datum im Format TT.MM.JJJJ ein.";

/**
 * Date fields whose typed text is no valid date. The draft keeps the last
 * valid value of such a field, so saving waits until the text is fixed.
 * Week fields are keyed by the week's stable key, so removing a week does
 * not shift them onto its neighbour.
 */
function invalidDateFields(
  invalid: ReadonlySet<string>,
  draft: HolidayPeriodDraft,
): Record<string, string> {
  const fields: Record<string, string> = {};
  for (const name of ["registration_opens_on", "registration_closes_on"]) {
    if (invalid.has(name)) fields[name] = INVALID_DATE_TEXT;
  }
  draft.weeks.forEach((week, index) => {
    for (const field of ["starts_on", "ends_on"]) {
      if (invalid.has(`${week.key}:${field}`)) {
        fields[`weeks.${index}.${field}`] = INVALID_DATE_TEXT;
      }
    }
  });
  return fields;
}

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
  const [source, setSource] = useState(period);
  const [edited, setEdited] = useState(false);
  const [invalidDates, setInvalidDates] = useState<ReadonlySet<string>>(
    () => new Set(),
  );
  // Stable callbacks: the date field re-reports its validity whenever the
  // callback changes, which would clear a flag the typed text just set.
  const markDate = useCallback(
    (key: string, valid: boolean) =>
      setInvalidDates((current) => {
        if (current.has(key) === !valid) return current;
        const next = new Set(current);
        if (valid) next.delete(key);
        else next.add(key);
        return next;
      }),
    [],
  );
  const opensValidity = useCallback(
    (valid: boolean) => markDate("registration_opens_on", valid),
    [markDate],
  );
  const closesValidity = useCallback(
    (valid: boolean) => markDate("registration_closes_on", valid),
    [markDate],
  );
  // A date field whose text is no valid date holds an unsaved change: the
  // stored period no longer matches what the person sees.
  const hasInvalidDate = invalidDates.size > 0;
  // Revalidation may replace cached metadata after this form mounted. Adopt
  // it before rendering fields only while pristine, never over typed edits,
  // invalid date text or an in-flight save. Remember an ignored answer too.
  if (source !== period) {
    setSource(period);
    if (period && !edited && !hasInvalidDate && !saving) {
      setDraftState(draftFromPeriod(period));
    }
  }
  useEffect(() => {
    if (hasInvalidDate) onDirtyChange?.(true);
  }, [hasInvalidDate, onDirtyChange]);
  const weekDateValidity = useCallback(
    (weekKey: string, field: "starts_on" | "ends_on", valid: boolean) =>
      markDate(`${weekKey}:${field}`, valid),
    [markDate],
  );
  const errorPath = useApiFormError(formRef);
  const fieldError = (name: string) => errorPath.fieldError(name);

  const setDraft = (next: HolidayPeriodDraft) => {
    setDraftState(next);
    setEdited(true);
    onDirtyChange?.(true);
  };
  const setWeek = (index: number, week: WeekDraft) =>
    setDraft({
      ...draft,
      weeks: draft.weeks.map((current, i) => (i === index ? week : current)),
    });

  const save = async () => {
    const errors = {
      ...validateHolidayPeriod(draft),
      ...invalidDateFields(invalidDates, draft),
    };
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
      setEdited(false);
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

      {/* Locked while saving: the answer replaces the draft, so an edit made
          meanwhile would be lost. */}
      <fieldset disabled={saving} className="min-w-0 space-y-6">
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
                  error={fieldError("registration_opens_on")}
                  onValidityChange={opensValidity}
                  onChange={(registrationOpensOn) =>
                    setDraft({ ...draft, registrationOpensOn })
                  }
                />
              </div>
              <div>
                <ISODateInput
                  id="registration_closes_on"
                  label="Anmeldeschluss"
                  value={draft.registrationClosesOn}
                  error={fieldError("registration_closes_on")}
                  onValidityChange={closesValidity}
                  onChange={(registrationClosesOn) =>
                    setDraft({ ...draft, registrationClosesOn })
                  }
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
                  onDateValidity={weekDateValidity}
                  onRemove={() => {
                    // The week's date flags go with it, so a later invalid
                    // date of another week reports again.
                    setInvalidDates((current) => {
                      const prefix = `${week.key}:`;
                      if (![...current].some((key) => key.startsWith(prefix))) {
                        return current;
                      }
                      return new Set(
                        [...current].filter((key) => !key.startsWith(prefix)),
                      );
                    });
                    setDraft({
                      ...draft,
                      weeks: draft.weeks.filter((_, i) => i !== index),
                    });
                  }}
                />
              ))
            )}
            {draft.weeks.length < MAX_HOLIDAY_WEEKS ? (
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
            ) : (
              <p className="text-sm text-gray-600">
                Ein Ferienzeitraum hat höchstens {MAX_HOLIDAY_WEEKS} Wochen.
              </p>
            )}
          </div>
        </SectionCard>
      </fieldset>

      <EditActions onCancel={onCancel} saving={saving} saveType="submit" />
    </form>
  );
}
