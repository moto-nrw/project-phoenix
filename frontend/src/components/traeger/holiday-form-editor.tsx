"use client";

// Anmeldeformular eines Ferienzeitraums bauen (#5198): dieselben Bausteine
// wie der Baukasten der Online-Anmeldung, beschränkt auf einfache Feldtypen
// und Bedingungen auf andere Fragen. Jede Frage gilt je Familie oder je Kind
// und kann eine Gesundheitsangabe sein. Speichern legt eine neue Version an.

import { FileText, Plus, TextCursorInput } from "lucide-react";
import { useCallback, useRef, useState } from "react";
import {
  FieldEditorRow,
  FieldErrorContext,
  FormPreview,
  FormPreviewHeading,
  PreviewFieldsSection,
  PreviewSection,
} from "~/components/enrollment/form-field-builder";
import { Button } from "~/components/ui/button";
import { EditActions } from "~/components/ui/edit-actions";
import { EmptyState } from "~/components/ui/empty-state";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiFormError } from "~/contexts/ToastContext";
import {
  blankField,
  blankInfoField,
  type ConditionSource,
  type FormField,
} from "~/lib/enrollment-form-schema-api";
import { createLogger } from "~/lib/logger";
import {
  HOLIDAY_FORM_FIELD_TYPES,
  type HolidayForm,
  type HolidayFormField,
  saveHolidayForm,
} from "~/lib/traeger-holiday-form-api";
import {
  copyStableObjectKey,
  getStableObjectKey,
} from "~/lib/stable-object-key";
import {
  holidayFieldsSignature,
  insertTemplateFields,
  prepareHolidayFieldsForSave,
  templateFieldsMissing,
  validateHolidayForm,
} from "./holiday-form-model";

const logger = createLogger({ component: "HolidayFormEditor" });

const HOLIDAY_CONDITION_SOURCES: readonly ConditionSource[] = ["field"];

const HOLIDAY_FREE_QUESTION_NOTE = {
  title: "Eltern beantworten diese Frage bei der Ferienanmeldung.",
  text: "Die Antwort sehen Sie in der Anmeldung. Ohne Gesundheitsangabe steht sie auch im Export.",
} as const;

/** Marks a health question in the preview. */
function healthBadge(field: FormField) {
  return (field as HolidayFormField).health ? (
    <StatusBadge tone="orange" label="Gesundheit" />
  ) : null;
}

export function HolidayFormEditor({
  periodId,
  form,
  onSaved,
}: Readonly<{
  periodId: string;
  form: HolidayForm;
  onSaved: (form: HolidayForm) => void;
}>) {
  const formRef = useRef<HTMLDivElement>(null);
  const errorPath = useApiFormError(formRef);
  const [fields, setFields] = useState<HolidayFormField[]>(() => [
    ...form.fields,
  ]);
  const [saving, setSaving] = useState(false);
  const { fieldError: readFieldError } = errorPath;
  const fieldError = useCallback(
    (name: string) => readFieldError(name) ?? undefined,
    [readFieldError],
  );
  const dirty =
    holidayFieldsSignature(fields) !== holidayFieldsSignature(form.fields);
  const missingTemplateFields = templateFieldsMissing(fields, form.template);

  const update = (index: number, patch: Partial<HolidayFormField>) =>
    setFields((current) =>
      current.map((field, i) =>
        i === index
          ? copyStableObjectKey(field, { ...field, ...patch })
          : field,
      ),
    );
  const move = (index: number, delta: number) =>
    setFields((current) => {
      const target = index + delta;
      if (target < 0 || target >= current.length) return current;
      const next = [...current];
      const [moved] = next.splice(index, 1);
      if (moved) next.splice(target, 0, moved);
      return next;
    });
  const remove = (index: number) =>
    setFields((current) => current.filter((_, i) => i !== index));

  const save = async () => {
    const errors = validateHolidayForm(fields);
    if (Object.keys(errors).length > 0) {
      errorPath.invalid("Bitte prüfen Sie die markierten Fragen.", errors);
      return;
    }
    setSaving(true);
    errorPath.clear();
    try {
      const saved = await saveHolidayForm(
        periodId,
        prepareHolidayFieldsForSave(fields),
      );
      setFields([...saved.fields]);
      onSaved(saved);
    } catch (err) {
      logger.error("traeger_holiday_form_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void errorPath.show(err, { object: "das Anmeldeformular" });
    } finally {
      setSaving(false);
    }
  };

  const familyFields = fields.filter((field) => !field.applies_to_child);
  const childFields = fields.filter((field) => field.applies_to_child);

  return (
    <FieldErrorContext.Provider value={fieldError}>
      <div
        ref={formRef}
        className="grid grid-cols-1 gap-6 xl:grid-cols-[minmax(0,1fr)_380px]"
      >
        <SectionCard
          title="Fragen"
          description="Diese Fragen stehen in der Ferienanmeldung zusätzlich zu den festen Angaben."
          action={
            <Button
              type="button"
              variant="outline"
              size="md"
              disabled={saving || missingTemplateFields === 0}
              onClick={() =>
                setFields((current) =>
                  insertTemplateFields(current, form.template),
                )
              }
            >
              Standardvorlage einfügen
            </Button>
          }
        >
          <div className="space-y-4">
            <FormErrorAlert message={errorPath.error} />
            {missingTemplateFields === 0 && form.template.length > 0 ? (
              <p className="text-sm text-gray-600">
                Alle Fragen der Standardvorlage sind im Formular.
              </p>
            ) : null}
            {fields.length === 0 ? (
              <EmptyState
                variant="compact"
                title="Noch keine Fragen"
                description="Eltern sehen nur die festen Angaben. Fügen Sie eine Frage oder die Standardvorlage hinzu."
              />
            ) : (
              <div className="space-y-3">
                {fields.map((field, index) => (
                  <FieldEditorRow
                    key={getStableObjectKey(field, "holiday-field")}
                    field={field}
                    index={index}
                    total={fields.length}
                    allFields={fields}
                    onChange={(patch) => update(index, patch)}
                    onRemove={() => remove(index)}
                    onMoveUp={() => move(index, -1)}
                    onMoveDown={() => move(index, 1)}
                    disabled={saving}
                    fieldTypes={HOLIDAY_FORM_FIELD_TYPES}
                    conditionSources={HOLIDAY_CONDITION_SOURCES}
                    allowAttachments={false}
                    freeQuestionNote={HOLIDAY_FREE_QUESTION_NOTE}
                    healthToggle={
                      field.type === "information"
                        ? undefined
                        : {
                            checked: Boolean(field.health),
                            onChange: (health) => update(index, { health }),
                          }
                    }
                  />
                ))}
              </div>
            )}
            <div className="flex flex-wrap gap-2">
              <Button
                type="button"
                variant="outline"
                size="md"
                disabled={saving}
                className="inline-flex items-center gap-2"
                onClick={() =>
                  setFields((current) => [
                    ...current,
                    blankField(current.length),
                  ])
                }
              >
                <Plus className="h-4 w-4" aria-hidden="true" />
                <TextCursorInput className="h-4 w-4" aria-hidden="true" />
                Frage hinzufügen
              </Button>
              <Button
                type="button"
                variant="outline"
                size="md"
                disabled={saving}
                className="inline-flex items-center gap-2"
                onClick={() =>
                  setFields((current) => [
                    ...current,
                    blankInfoField(current.length),
                  ])
                }
              >
                <Plus className="h-4 w-4" aria-hidden="true" />
                <FileText className="h-4 w-4" aria-hidden="true" />
                Infotext hinzufügen
              </Button>
            </div>
            <EditActions
              onCancel={() => {
                errorPath.clear();
                // New row identities also discard row-local input drafts.
                setFields(form.fields.map((field) => ({ ...field })));
              }}
              onSave={() => void save()}
              saving={saving}
              disabled={!dirty}
              cancelLabel="Änderungen verwerfen"
              saveLabel="Als neue Version speichern"
              className="border-t border-gray-100 pt-4"
            />
          </div>
        </SectionCard>

        <aside>
          <FormPreview
            title="Ferienanmeldung"
            header={
              <FormPreviewHeading
                title="Vorschau für Eltern"
                hint={
                  dirty
                    ? "Noch nicht gespeichert. Eltern sehen weiter die gespeicherte Version."
                    : "So sehen Eltern das Formular."
                }
              />
            }
          >
            <PreviewSection
              title="Kind"
              fields={[
                "Vorname *",
                "Nachname *",
                "Geburtsdatum *",
                "Schule *",
                "Wochen und Zeitmodell *",
                "Allergien",
                "Medikamente",
                "Weitere Hinweise zur Gesundheit",
              ]}
            />
            <PreviewFieldsSection
              title="Fragen je Kind"
              fields={childFields}
              emptyTitle="Keine Fragen je Kind"
              emptyText="Bei jedem Kind stehen nur die festen Angaben."
              fieldBadge={healthBadge}
            />
            <PreviewSection
              title="Elternteil"
              fields={["Vorname *", "Nachname *", "E-Mail *", "Telefon *"]}
            />
            <PreviewFieldsSection
              title="Fragen zur Familie"
              fields={familyFields}
              emptyTitle="Keine Fragen zur Familie"
              emptyText="Beim Elternteil stehen nur die festen Angaben."
              fieldBadge={healthBadge}
            />
            <PreviewSection
              title="Kontakte"
              fields={["Notfallkontakt *", "Abholberechtigte"]}
            />
            <PreviewSection
              title="Zustimmungen"
              fields={[
                "Datenschutz *",
                "Gesundheitsangaben (wenn angegeben) *",
                "Verbindliche Anmeldung (wenn verbindlich) *",
              ]}
            />
          </FormPreview>
        </aside>
      </div>
    </FieldErrorContext.Provider>
  );
}
