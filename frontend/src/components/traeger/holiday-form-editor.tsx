"use client";

// Anmeldeformular eines Ferienzeitraums bauen (#5198): dieselben Bausteine
// wie der Baukasten der Online-Anmeldung, beschränkt auf einfache Feldtypen
// und Bedingungen auf andere Fragen. Jede Frage gilt je Familie oder je Kind
// und kann eine Gesundheitsangabe sein. Eine Frage ohne Gesundheitsangabe kann
// die ausrichtende OGS sehen (#5202). Der feste Baustein „Anschrift“ fragt
// die Anschrift ab (#5625), der feste Baustein „Gesundheit“ macht Allergien
// und Medikamente zur Pflicht (#6686). Speichern legt eine neue Version an.

import { FileText, Plus, TextCursorInput } from "lucide-react";
import { useCallback, useMemo, useRef, useState } from "react";
import {
  FieldEditorRow,
  FieldErrorContext,
} from "~/components/enrollment/form-field-builder";
import {
  FormPreviewFrame as FormPreview,
  FormPreviewHeading,
  PreviewFieldsSection,
  PreviewSection,
} from "~/components/enrollment/form-preview";
import { LegalBlocksSection } from "~/components/enrollment/legal-blocks-section";
import {
  DraftDocumentContext,
  useDraftDocuments,
  type DraftDocumentSave,
} from "~/components/enrollment/draft-documents";
import { Alert } from "~/components/ui/alert";
import { Button, buttonClassName } from "~/components/ui/button";
import { EditActions } from "~/components/ui/edit-actions";
import { BELOW_SM, useMediaQuery } from "~/lib/hooks/use-media-query";
import { EmptyState } from "~/components/ui/empty-state";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
// Die Vorschau nennt die Abschnitte wie das Elternformular (#5467).
import germanMessages from "~/i18n/messages/de/holidayRegistration.json";
import {
  blankField,
  blankInfoField,
  type ConditionSource,
  type FormField,
  type FormLegalBlock,
} from "~/lib/enrollment-form-schema-api";
import { draftDocumentURLsInLegalBlocks } from "~/lib/enrollment-legal-blocks";
import { createLogger } from "~/lib/logger";
import {
  HOLIDAY_FORM_FIELD_TYPES,
  type HolidayForm,
  type HolidayFormField,
  saveHolidayForm,
  uploadHolidayDocument,
  deleteHolidayDocument,
} from "~/lib/traeger-holiday-form-api";
import {
  copyStableObjectKey,
  getStableObjectKey,
} from "~/lib/stable-object-key";
import {
  HOLIDAY_ADDRESS_CHOICES,
  holidayAddressPreviewFields,
} from "./holiday-form-address";
import {
  HOLIDAY_HEALTH_CHOICES,
  holidayHealthPreviewFields,
  questionsRepeatingHealthFields,
} from "./holiday-form-health";
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
  text: "Die Antworten sehen Sie in der Anmeldung. Ohne Gesundheitsangabe stehen sie auch im Export.",
} as const;

/** Marks a health question and a question the hosting OGS sees in the preview. */
function questionBadge(field: FormField) {
  const question = field as HolidayFormField;
  const shared = question.share_with_host ? (
    <StatusBadge tone="blue" label="OGS sieht es" />
  ) : null;
  if (!question.health) return shared;
  return (
    <span className="inline-flex flex-wrap gap-1">
      <StatusBadge tone="orange" label="Gesundheit" />
      {shared}
    </span>
  );
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
  const { success } = useToast();
  const [fields, setFields] = useState<HolidayFormField[]>(() => [
    ...form.fields,
  ]);
  const [legalBlocks, setLegalBlocks] = useState<FormLegalBlock[]>(() => [
    ...(form.legalBlocks ?? []),
  ]);
  const [address, setAddress] = useState(form.addressRequirement);
  const [health, setHealth] = useState(form.healthRequirement);
  const adapter = useMemo(
    () => ({
      upload: (file: File) => uploadHolidayDocument(periodId, file),
      remove: (id: string, options?: { keepalive?: boolean }) =>
        deleteHolidayDocument(periodId, id, options),
    }),
    [periodId],
  );
  const draftDocuments = useDraftDocuments(adapter.remove);
  const uploading = draftDocuments.uploading;
  const [saving, setSaving] = useState(false);
  const { fieldError: readFieldError } = errorPath;
  const fieldError = useCallback(
    (name: string) => readFieldError(name) ?? undefined,
    [readFieldError],
  );
  const dirty =
    holidayFieldsSignature(fields) !== holidayFieldsSignature(form.fields) ||
    JSON.stringify(legalBlocks) !== JSON.stringify(form.legalBlocks ?? []) ||
    address !== form.addressRequirement ||
    health !== form.healthRequirement;
  const missingTemplateFields = templateFieldsMissing(fields, form.template);
  // On a phone a question folds to its head, so the long form stays
  // walkable; from tablet width on every question starts open, all fields
  // in view. A new question opens, and so does one the check marks.
  const isPhone = useMediaQuery(BELOW_SM);
  const foldByDefault = isPhone;
  const [foldChoice, setFoldChoice] = useState<ReadonlyMap<string, boolean>>(
    () => new Map(),
  );
  const fieldKey = (field: HolidayFormField) =>
    getStableObjectKey(field, "holiday-field");
  const isOpen = (key: string) => foldChoice.get(key) ?? !foldByDefault;
  const setOpen = (keys: readonly string[], open: boolean) =>
    setFoldChoice((current) => {
      const next = new Map(current);
      for (const key of keys) next.set(key, open);
      return next;
    });
  const addField = (field: HolidayFormField) => {
    setOpen([fieldKey(field)], true);
    setFields((current) => [...current, field]);
  };
  // With required health fields an own question about allergies or
  // medication asks parents a second time (#6686).
  const repeatedHealthQuestions =
    health === "required" ? questionsRepeatingHealthFields(fields) : [];

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
      const marked = new Set(
        Object.keys(errors).map((name) => /^fields\.(\d+)\./.exec(name)?.[1]),
      );
      setOpen(
        fields.filter((_, index) => marked.has(String(index))).map(fieldKey),
        true,
      );
      errorPath.invalid("Bitte prüfen Sie die markierten Fragen.", errors);
      return;
    }
    setSaving(true);
    errorPath.clear();
    let draftSave: DraftDocumentSave | null = null;
    try {
      draftSave = await draftDocuments.startSave(
        draftDocumentURLsInLegalBlocks(legalBlocks, draftDocuments.drafts()),
      );
      const saved = await saveHolidayForm(
        periodId,
        prepareHolidayFieldsForSave(fields),
        legalBlocks,
        address,
        health,
      );
      draftSave.commit();
      setFields([...saved.fields]);
      setLegalBlocks([...(saved.legalBlocks ?? [])]);
      setAddress(saved.addressRequirement);
      setHealth(saved.healthRequirement);
      success(
        `Das Anmeldeformular ist als Version ${saved.version} gespeichert.`,
      );
      onSaved(saved);
    } catch (err) {
      draftSave?.fail();
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

  const editActions = (
    <EditActions
      onCancel={() => {
        errorPath.clear();
        void draftDocuments.cleanup();
        setLegalBlocks([...(form.legalBlocks ?? [])]);
        setAddress(form.addressRequirement);
        setHealth(form.healthRequirement);
        // New row identities also discard row-local input drafts.
        setFields(form.fields.map((field) => ({ ...field })));
      }}
      onSave={() => void save()}
      saving={saving}
      cancelDisabled={uploading}
      disabled={!dirty || uploading}
      // A phone keeps both buttons on one line under the long list.
      cancelLabel={isPhone ? "Verwerfen" : "Änderungen verwerfen"}
      saveLabel="Als neue Version speichern"
    />
  );

  return (
    <DraftDocumentContext.Provider value={draftDocuments.lifecycle}>
      <FieldErrorContext.Provider value={fieldError}>
        <div
          ref={formRef}
          className="grid grid-cols-1 gap-6 xl:grid-cols-[minmax(0,1fr)_380px]"
        >
          <SectionCard
            title="Fragen"
            description={`Diese Fragen stehen in der Ferienanmeldung zusätzlich zu den festen Angaben. ${HOLIDAY_FREE_QUESTION_NOTE.text}`}
            // The save stays in the card and in view while the long list of
            // questions scrolls.
            stickyFooter
            footer={editActions}
            action={
              <div className="flex flex-wrap gap-2">
                <Button
                  type="button"
                  variant="outline"
                  size="md"
                  disabled={saving || uploading || missingTemplateFields === 0}
                  onClick={() =>
                    setFields((current) =>
                      insertTemplateFields(current, form.template),
                    )
                  }
                >
                  Standardvorlage einfügen
                </Button>
                {/* Below xl the preview stands after all questions: the
                    way there is one tap, not a long scroll. */}
                <a
                  href="#holiday-form-preview"
                  className={buttonClassName({
                    variant: "ghost",
                    size: "md",
                    className: "xl:hidden",
                  })}
                >
                  Zur Vorschau
                </a>
              </div>
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
                      key={fieldKey(field)}
                      fold={{
                        open: isOpen(fieldKey(field)),
                        onToggle: () =>
                          setOpen([fieldKey(field)], !isOpen(fieldKey(field))),
                      }}
                      field={field}
                      index={index}
                      total={fields.length}
                      allFields={fields}
                      onChange={(patch) => update(index, patch)}
                      onRemove={() => remove(index)}
                      onMoveUp={() => move(index, -1)}
                      onMoveDown={() => move(index, 1)}
                      disabled={saving || uploading}
                      fieldTypes={HOLIDAY_FORM_FIELD_TYPES}
                      conditionSources={HOLIDAY_CONDITION_SOURCES}
                      allowAttachments={false}
                      // Said once in the card's description, not on every
                      // question.
                      freeQuestionNote={null}
                      healthToggle={
                        field.type === "information"
                          ? undefined
                          : {
                              checked: Boolean(field.health),
                              onChange: (health) => update(index, { health }),
                            }
                      }
                      hostShareToggle={
                        field.type === "information"
                          ? undefined
                          : {
                              checked: Boolean(field.share_with_host),
                              onChange: (share_with_host) =>
                                update(index, { share_with_host }),
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
                  disabled={saving || uploading}
                  className="inline-flex items-center gap-2"
                  onClick={() =>
                    addField({
                      ...blankField(fields.length),
                      share_with_host: true,
                    })
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
                  disabled={saving || uploading}
                  className="inline-flex items-center gap-2"
                  onClick={() => addField(blankInfoField(fields.length))}
                >
                  <Plus className="h-4 w-4" aria-hidden="true" />
                  <FileText className="h-4 w-4" aria-hidden="true" />
                  Infotext hinzufügen
                </Button>
              </div>
              <SectionCard
                title="Gesundheit"
                headingLevel={3}
                namedRegion
                description={
                  health === "required"
                    ? "Eltern beantworten für jedes Kind Allergien und Medikamente. Hat das Kind keine, wählen sie „Nein, keine“. Weitere Hinweise bleiben freiwillig."
                    : "Eltern können Allergien, Medikamente und weitere Hinweise angeben. Sie dürfen die Felder leer lassen."
                }
              >
                <SegmentedControl
                  ariaLabel="Allergien und Medikamente abfragen"
                  items={HOLIDAY_HEALTH_CHOICES.map((item) => ({
                    ...item,
                    disabled: saving || uploading,
                  }))}
                  value={health}
                  onChange={setHealth}
                />
                {repeatedHealthQuestions.length > 0 ? (
                  <Alert
                    type="warning"
                    size="compact"
                    className="mt-3"
                    message={`Allergien und Medikamente fragt das Formular schon ab. Diese eigenen Fragen fragen es noch einmal: ${repeatedHealthQuestions
                      .map((field) => `„${field.label}“`)
                      .join(
                        ", ",
                      )}. Entfernen Sie sie, sonst antworten Eltern doppelt.`}
                  />
                ) : null}
              </SectionCard>
              <SectionCard
                title="Anschrift"
                headingLevel={3}
                namedRegion
                description="Eltern geben Straße, PLZ und Ort an. Rechnung und Bestätigungsschreiben übernehmen die Anschrift."
              >
                {/* Unter sm untereinander: drei Segmente nebeneinander sind breiter
                    als ein 320-px-Telefon abzüglich der beiden Kartenränder. */}
                <SegmentedControl
                  className="max-sm:grid max-sm:w-full max-sm:grid-flow-row"
                  ariaLabel="Anschrift abfragen"
                  items={HOLIDAY_ADDRESS_CHOICES.map((item) => ({
                    ...item,
                    disabled: saving || uploading,
                  }))}
                  value={address}
                  onChange={setAddress}
                />
              </SectionCard>
              <LegalBlocksSection
                blocks={legalBlocks}
                standardBlocks={[]}
                customOnly
                uploadAdapter={adapter}
                onChange={setLegalBlocks}
                disabled={saving || uploading}
              />
            </div>
          </SectionCard>

          <aside
            id="holiday-form-preview"
            aria-label="Vorschau für Eltern"
            className="scroll-mt-24"
          >
            <FormPreview
              title="Ferienanmeldung"
              headerInside
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
                  ...holidayHealthPreviewFields(health),
                ]}
              />
              <PreviewFieldsSection
                title={germanMessages.childQuestionsTitle}
                fields={childFields}
                emptyTitle="Keine Fragen oder Infos zum Kind"
                emptyText="Bei jedem Kind stehen nur die festen Angaben."
                fieldBadge={questionBadge}
              />
              <PreviewSection
                title="Elternteil"
                fields={["Vorname *", "Nachname *", "E-Mail *", "Telefon *"]}
              />
              {address === "off" ? null : (
                <PreviewSection
                  title={
                    address === "optional"
                      ? germanMessages.addressTitleOptional
                      : germanMessages.addressTitle
                  }
                  fields={holidayAddressPreviewFields(address)}
                />
              )}
              <PreviewFieldsSection
                title={germanMessages.familyQuestionsTitle}
                fields={familyFields}
                emptyTitle="Keine allgemeinen Fragen oder Infos"
                emptyText="Beim Elternteil stehen nur die festen Angaben."
                fieldBadge={questionBadge}
              />
              <PreviewSection
                title="Kontakte"
                fields={["Personen: Notfallkontakt und/oder Abholung"]}
              />
              <PreviewSection
                title="Hinweise und Einwilligungen"
                fields={legalBlocks
                  .filter((b) => b.enabled)
                  .map((b) => `${b.title}${b.required ? " *" : ""}`)}
              />
              <PreviewSection
                title="Zustimmungen"
                fields={[
                  "Datenschutz *",
                  health === "required"
                    ? "Gesundheitsangaben *"
                    : "Gesundheitsangaben (wenn angegeben) *",
                  "Verbindliche Anmeldung (wenn verbindlich) *",
                ]}
              />
            </FormPreview>
          </aside>
        </div>
      </FieldErrorContext.Provider>
    </DraftDocumentContext.Provider>
  );
}
