"use client";

import Link from "next/link";
import { createPortal } from "react-dom";
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import {
  ArrowLeft,
  CalendarClock,
  Check,
  ChevronDown,
  Eye,
  ExternalLink,
  FileText,
  GripVertical,
  HelpCircle,
  Info,
  ListPlus,
  Lock,
  MoreVertical,
  Pencil,
  Plus,
  ShieldCheck,
  Trash2,
} from "lucide-react";
import { useToast } from "~/contexts/ToastContext";
import { ConfirmationModal } from "~/components/ui/modal";
import { BooleanField } from "~/components/settings/fields/boolean-field";
import {
  blankField,
  blankInfoField,
  createSchema,
  deleteSchema,
  latestSchemasByName,
  listSchemas,
  updateSchema,
  RESERVED_TARGETS,
  type ConditionOperator,
  type ConditionSource,
  type FormField,
  type FormFieldTarget,
  type FormFieldType,
  type FormSchema,
  type CoreRequirementKey,
  type CoreRequirements,
  type VisibilityCondition,
} from "~/lib/enrollment-form-schema-api";
import { listPhases, type Phase } from "~/lib/enrollment-phase-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "EnrollmentFormEditor" });

const fieldTypeLabels: Record<FormFieldType, string> = {
  boolean: "Ja/Nein",
  number: "Zahl",
  text: "Text",
  textarea: "Mehrzeiliger Text",
  date: "Datum",
  select: "Auswahl",
  information: "Infotext",
  phone_list: "Telefonliste",
  weekday_schedule: "Wochenzeiten",
  weekday_boolean: "Wochentage",
  contact_list: "Kontaktliste",
};

// German labels for the visibility-condition editor.
const conditionSourceLabels: Record<ConditionSource, string> = {
  field: "Antwort auf eine andere Frage",
  grade_level: "Klassenstufe des Kindes",
  care_offering: "Gewähltes Betreuungsangebot",
};

const conditionOperatorLabels: Record<ConditionOperator, string> = {
  eq: "ist gleich",
  neq: "ist nicht gleich",
  not_empty: "ist ausgefüllt",
  includes: "enthält",
};

const freeFieldTypes = [
  "boolean",
  "number",
  "text",
  "textarea",
  "date",
  "select",
] satisfies FormFieldType[];

const structuredFieldTypes = new Set<FormFieldType>([
  "phone_list",
  "weekday_schedule",
  "weekday_boolean",
  "contact_list",
]);

// Labels for the answer storage picker. Admins can still rename the
// displayed question; the target decides whether the answer is copied
// into student data, schedule data, or contacts at approval time.
const targetPickerLabels: Record<Exclude<FormFieldTarget, "">, string> = {
  "student.health_info": "Gesundheitsinformationen beim Kind speichern",
  "student.extra_info": "Hinweise für die Betreuung beim Kind speichern",
  "student.bus_days": "Buskind beim Kind speichern",
  "student.bus": "Buskind beim Kind speichern",
  "student.pickup_status": "Abholregelung beim Kind speichern",
  "schedule.pickup": "Abholzeiten im Stundenplan speichern",
  "schedule.arrival": "Ankunftszeiten im Stundenplan speichern",
  "student.contacts":
    "Weitere Kontakte, Abholberechtigte und Notfallkontakte speichern",
};

// Targets sorted alphabetically by label for the picker, keeps the
// dropdown stable across renders even if the underlying map order
// changes. student.bus is a legacy alias of student.bus_days (#1582) and is
// excluded so new fields can only pick the canonical target.
const TARGET_PICKER_ORDER: Array<Exclude<FormFieldTarget, "">> = (
  Object.keys(targetPickerLabels) as Array<Exclude<FormFieldTarget, "">>
)
  .filter((target) => target !== "student.bus")
  .sort((a, b) =>
    targetPickerLabels[a].localeCompare(targetPickerLabels[b], "de"),
  );

const targetSuggestionDescriptions: Record<
  Exclude<FormFieldTarget, "">,
  string
> = {
  "student.health_info":
    "Für Allergien, Medikamente oder andere Gesundheitsangaben.",
  "student.extra_info":
    "Für wichtige Hinweise, die im Alltag der Betreuung sichtbar sein sollen.",
  "student.bus_days":
    "Für die Information, an welchen Wochentagen ein Kind mit dem Bus fährt.",
  "student.bus": "Für die Information, ob ein Kind mit dem Bus fährt.",
  "student.pickup_status":
    "Für die Wochentage, an denen ein Kind abgeholt wird. Nicht gewählte Tage bedeuten, dass es alleine nach Hause geht.",
  "schedule.pickup": "Für regelmäßige Abholzeiten je Wochentag.",
  "schedule.arrival": "Für regelmäßige Ankunftszeiten je Wochentag.",
  "student.contacts":
    "Für weitere Kontakte, Abholberechtigte oder Notfallkontakte.",
};

const NEW_SCHEMA_VALUE = "__new__";
type EditorMode = "overview" | "builder" | "detail";
type PendingNavigation = "overview" | "new" | "preview";

interface CoreField {
  readonly key: string;
  readonly label: string;
  readonly type: FormFieldType;
  readonly required: boolean;
  readonly group: "guardian" | "child";
  readonly requirementKey?: CoreRequirementKey;
  readonly requirementLabel?: string;
  readonly requirementHint?: string;
}

const CORE_FIELDS: ReadonlyArray<CoreField> = [
  {
    key: "guardian_first_name",
    label: "Vorname (Elternteil)",
    type: "text",
    required: true,
    group: "guardian",
  },
  {
    key: "guardian_last_name",
    label: "Nachname (Elternteil)",
    type: "text",
    required: true,
    group: "guardian",
  },
  {
    key: "guardian_email",
    label: "E-Mail (Elternteil)",
    type: "text",
    required: true,
    group: "guardian",
  },
  {
    key: "guardian_phone",
    label: "Telefonnummer (Elternteil)",
    type: "text",
    required: false,
    group: "guardian",
    requirementKey: "guardian_phone",
    requirementLabel: "Telefonnummer verpflichtend",
    requirementHint: "Eltern müssen eine gültige Telefonnummer angeben.",
  },
  {
    key: "first_name",
    label: "Vorname (Kind)",
    type: "text",
    required: true,
    group: "child",
  },
  {
    key: "last_name",
    label: "Nachname (Kind)",
    type: "text",
    required: true,
    group: "child",
  },
  {
    key: "date_of_birth",
    label: "Geburtsdatum (Kind)",
    type: "date",
    required: true,
    group: "child",
  },
  {
    key: "target_grade_level",
    label: "Klassenstufe (Kind)",
    type: "number",
    required: true,
    group: "child",
  },
];

export function EnrollmentFormEditor() {
  const toast = useToast();
  const [allSchemas, setAllSchemas] = useState<FormSchema[]>([]);
  const [phases, setPhases] = useState<Phase[]>([]);
  const [selectedKey, setSelectedKey] = useState<string>(NEW_SCHEMA_VALUE);
  const [name, setName] = useState("");
  const [fields, setFields] = useState<FormField[]>([]);
  const [coreRequirements, setCoreRequirements] = useState<CoreRequirements>(
    {},
  );
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [mode, setMode] = useState<EditorMode>("overview");
  const [pendingNavigation, setPendingNavigation] =
    useState<PendingNavigation | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<FormSchema | null>(null);
  const [deletingSchemaId, setDeletingSchemaId] = useState<string | null>(null);

  const latestByName = useMemo(
    () => latestSchemasByName(allSchemas),
    [allSchemas],
  );

  const loadAll = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [list, phaseList] = await Promise.all([
        listSchemas(),
        listPhases().catch(() => [] as Phase[]),
      ]);
      setAllSchemas(list);
      setPhases(phaseList);
      return list;
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("schema_list_failed", { error: message });
      setError(message);
      return [];
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadAll();
  }, [loadAll]);

  const selectSchema = (schema: FormSchema, nextMode: EditorMode) => {
    setSelectedKey(schema.id);
    setName(schema.name);
    setFields(schema.fields);
    setCoreRequirements(schema.core_requirements ?? {});
    setError(null);
    setMode(nextMode);
  };

  const editSchema = (schema: FormSchema) => selectSchema(schema, "builder");

  const previewSchema = (schema: FormSchema) => selectSchema(schema, "detail");

  const requestRemoveSchema = (schema: FormSchema) => {
    setDeleteTarget(schema);
  };

  const confirmRemoveSchema = async () => {
    if (!deleteTarget) return;
    setError(null);
    setDeletingSchemaId(deleteTarget.id);
    try {
      await deleteSchema(deleteTarget.id);
      await loadAll();
      toast.success("Formularvorlage gelöscht.");
      if (selectedKey === deleteTarget.id) {
        backToOverview();
      }
      setDeleteTarget(null);
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Löschen fehlgeschlagen";
      logger.error("schema_delete_failed", { error: message });
      setError(message);
      toast.error(message);
    } finally {
      setDeletingSchemaId(null);
    }
  };

  const startNew = () => {
    setSelectedKey(NEW_SCHEMA_VALUE);
    setName("");
    setFields([]);
    setCoreRequirements({});
    setError(null);
    setMode("builder");
  };

  const backToOverview = () => {
    setError(null);
    setMode("overview");
  };

  const updateField = (index: number, patch: Partial<FormField>) => {
    setFields((prev) =>
      prev.map((f, i) => (i === index ? { ...f, ...patch } : f)),
    );
  };

  const updateCoreRequirement = (
    key: CoreRequirementKey,
    required: boolean,
  ) => {
    setCoreRequirements((prev) => ({ ...prev, [key]: required }));
  };

  const addField = () => {
    setFields((prev) => [...prev, blankField(prev.length)]);
  };

  const addInfoField = () => {
    setFields((prev) => [...prev, blankInfoField(prev.length)]);
  };

  const addTargetField = (target: Exclude<FormFieldTarget, "">) => {
    setFields((prev) => {
      if (prev.some((field) => field.target === target)) return prev;
      return [...prev, createTargetField(target, prev.length)];
    });
  };

  const removeField = (index: number) => {
    setFields((prev) =>
      prev
        .filter((_, i) => i !== index)
        .map((f, i) => ({ ...f, sort_order: i })),
    );
  };

  const moveField = (index: number, direction: -1 | 1) => {
    setFields((prev) => {
      const target = index + direction;
      if (target < 0 || target >= prev.length) return prev;
      const next = [...prev];
      const tmp = next[index];
      next[index] = next[target]!;
      next[target] = tmp!;
      return next.map((f, i) => ({ ...f, sort_order: i }));
    });
  };

  const isCreating = selectedKey === NEW_SCHEMA_VALUE;
  const currentSchema = isCreating
    ? null
    : (latestByName.find((s) => s.id === selectedKey) ?? null);
  const savedFieldSignature = useMemo(
    () => JSON.stringify(currentSchema?.fields ?? []),
    [currentSchema?.fields],
  );
  const currentFieldSignature = useMemo(() => JSON.stringify(fields), [fields]);
  const savedCoreRequirementSignature = useMemo(
    () => coreRequirementsSignature(currentSchema?.core_requirements ?? {}),
    [currentSchema?.core_requirements],
  );
  const currentCoreRequirementSignature = useMemo(
    () => coreRequirementsSignature(coreRequirements),
    [coreRequirements],
  );
  const hasUnsavedChanges =
    mode === "builder" &&
    (isCreating
      ? name.trim() !== "" ||
        fields.length > 0 ||
        currentCoreRequirementSignature !== "{}"
      : currentFieldSignature !== savedFieldSignature ||
        savedCoreRequirementSignature !== currentCoreRequirementSignature);
  const saveBlockedMessage = getSchemaDraftValidationMessage({
    fields,
    isCreating,
    name,
  });

  const saveSchema = async (
    nextMode: EditorMode = "detail",
  ): Promise<FormSchema | null> => {
    setSaving(true);
    setError(null);
    try {
      const validationMessage = getSchemaDraftValidationMessage({
        fields,
        isCreating,
        name,
      });
      if (validationMessage) {
        setError(validationMessage);
        toast.error(validationMessage);
        return null;
      }

      const fieldsForSave = prepareFieldsForSave(fields);
      let result: FormSchema;
      if (isCreating) {
        result = await createSchema(
          name.trim(),
          fieldsForSave,
          coreRequirements,
        );
      } else {
        result = await updateSchema(
          selectedKey,
          fieldsForSave,
          coreRequirements,
        );
      }
      const refreshed = await loadAll();
      const stillThere = refreshed.find((s) => s.id === result.id);
      const savedSchema = stillThere ?? result;
      selectSchema(savedSchema, nextMode);
      toast.success(
        isCreating ? "Formularvorlage erstellt." : "Änderungen gespeichert.",
      );
      return savedSchema;
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Speichern fehlgeschlagen";
      logger.error("schema_save_failed", { error: message });
      setError(message);
      toast.error(message);
      return null;
    } finally {
      setSaving(false);
    }
  };

  const handleSave = async () => {
    await saveSchema("detail");
  };

  const requestBackToOverview = () => {
    if (hasUnsavedChanges) {
      setPendingNavigation("overview");
      return;
    }
    backToOverview();
  };

  const requestStartNew = () => {
    if (hasUnsavedChanges) {
      setPendingNavigation("new");
      return;
    }
    startNew();
  };

  const openPreviewWindow = (schemaId: string) => {
    window.open(
      `/enroll/preview?schemaId=${encodeURIComponent(schemaId)}`,
      "_blank",
      "noopener,noreferrer",
    );
  };

  const requestExternalPreview = () => {
    if (!hasUnsavedChanges && currentSchema) {
      openPreviewWindow(currentSchema.id);
      return;
    }
    setPendingNavigation("preview");
  };

  const discardPendingNavigation = () => {
    const pending = pendingNavigation;
    setPendingNavigation(null);
    if (pending === "overview") {
      backToOverview();
      return;
    }
    if (pending === "new") startNew();
  };

  const savePendingNavigation = async () => {
    const pending = pendingNavigation;
    if (!pending) return;

    let previewWindow: Window | null = null;
    if (pending === "preview") {
      previewWindow = window.open("about:blank", "_blank");
      if (previewWindow) {
        previewWindow.opener = null;
        previewWindow.document.title = "Vorschau wird geöffnet";
        previewWindow.document.body.textContent = "Vorschau wird geöffnet...";
      }
    }

    const savedSchema = await saveSchema(
      pending === "overview" ? "overview" : "detail",
    );
    if (!savedSchema) {
      previewWindow?.close();
      return;
    }

    setPendingNavigation(null);
    if (pending === "new") {
      startNew();
      return;
    }
    if (pending === "preview") {
      const href = `/enroll/preview?schemaId=${encodeURIComponent(savedSchema.id)}`;
      if (previewWindow) {
        previewWindow.location.href = href;
      } else {
        window.open(href, "_blank", "noopener,noreferrer");
      }
    }
  };

  if (loading) {
    return <p className="text-sm text-gray-500">Wird geladen...</p>;
  }

  if (mode === "overview") {
    return (
      <>
        <EnrollmentFormsOverview
          templates={latestByName}
          phases={phases}
          onCreate={startNew}
          onEdit={editSchema}
          onPreview={previewSchema}
          onDelete={requestRemoveSchema}
          error={error}
        />
        <DeleteSchemaDialog
          schema={deleteTarget}
          deleting={deletingSchemaId === deleteTarget?.id}
          onClose={() => setDeleteTarget(null)}
          onConfirm={confirmRemoveSchema}
        />
      </>
    );
  }

  if (mode === "detail" && currentSchema) {
    return (
      <FormTemplateDetail
        schema={currentSchema}
        onBack={backToOverview}
        onEdit={() => editSchema(currentSchema)}
        assignedPhases={phases.filter(
          (phase) => phase.form_schema_id === currentSchema.id,
        )}
      />
    );
  }

  return (
    <div className="space-y-5">
      <section className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm backdrop-blur-md">
        <div className="border-b border-gray-100 px-5 py-3 sm:px-6">
          <button
            type="button"
            onClick={requestBackToOverview}
            disabled={saving}
            className="inline-flex h-8 items-center gap-2 rounded-lg px-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-50 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
          >
            <ArrowLeft className="h-4 w-4" aria-hidden="true" />
            Zurück zur Übersicht
          </button>
        </div>
        <div className="grid gap-0 lg:grid-cols-[minmax(0,1fr)_380px] xl:grid-cols-[minmax(0,1fr)_430px]">
          <div className="space-y-6 p-5 sm:p-6">
            <FormBuilderIntro />

            <BuilderTemplateSummary
              name={name}
              isCreating={isCreating}
              saving={saving}
              currentSchema={currentSchema}
              onNameChange={setName}
              fields={fields}
            />

            <CoreFieldsSection
              coreRequirements={coreRequirements}
              onRequirementChange={updateCoreRequirement}
              disabled={saving}
            />

            <section className="space-y-4">
              <div className="flex flex-col gap-3 border-t border-gray-100 pt-5 sm:flex-row sm:items-end sm:justify-between">
                <div>
                  <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
                    Zusatzfragen
                  </p>
                  <h2 className="mt-1 text-base font-semibold text-gray-900">
                    Was Eltern zusätzlich beantworten sollen
                  </h2>
                  <p className="mt-1 max-w-2xl text-sm text-gray-600">
                    Wähle feste Vorschläge, wenn die Antwort später in den
                    Stammdaten stehen soll. Freie Zusatzfragen bleiben nur bei
                    der Anmeldung.
                  </p>
                </div>
                <div className="flex flex-wrap gap-2">
                  <button
                    type="button"
                    onClick={addInfoField}
                    disabled={saving}
                    className="inline-flex h-9 items-center justify-center gap-2 rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    <Info className="h-4 w-4" aria-hidden="true" />
                    Infotext
                  </button>
                  <button
                    type="button"
                    onClick={addField}
                    disabled={saving}
                    className="inline-flex h-9 items-center justify-center gap-2 rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    <Plus className="h-4 w-4" aria-hidden="true" />
                    Freie Zusatzfrage
                  </button>
                </div>
              </div>

              {error ? (
                <div className="rounded-lg border border-[#FF3130]/20 bg-[#FF3130]/10 p-3 text-sm text-[#CC2626]">
                  {error}
                </div>
              ) : null}

              <TargetSuggestions
                fields={fields}
                onAdd={addTargetField}
                disabled={saving}
              />

              {fields.length > 0 ? (
                <div className="space-y-3">
                  {fields.map((field, index) => (
                    <FieldEditorRow
                      key={`custom-field-${index}`}
                      field={field}
                      index={index}
                      total={fields.length}
                      allFields={fields}
                      onChange={(patch) => updateField(index, patch)}
                      onRemove={() => removeField(index)}
                      onMoveUp={() => moveField(index, -1)}
                      onMoveDown={() => moveField(index, 1)}
                      disabled={saving}
                    />
                  ))}
                </div>
              ) : null}

              <div className="flex flex-wrap items-center justify-end gap-2 border-t border-gray-100 pt-4">
                <button
                  type="button"
                  onClick={requestStartNew}
                  disabled={saving}
                  className="inline-flex h-9 items-center justify-center rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50"
                >
                  Zurücksetzen
                </button>
                <button
                  type="button"
                  onClick={handleSave}
                  disabled={saving}
                  className="inline-flex h-9 items-center justify-center rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {saving
                    ? "Speichert..."
                    : isCreating
                      ? "Formularvorlage erstellen"
                      : "Änderungen speichern"}
                </button>
              </div>
            </section>
          </div>

          <aside className="moto-dotted-background moto-dotted-background--split border-t border-gray-100 p-5 sm:p-6 lg:border-t-0 lg:border-l">
            <FormPreview
              fields={fields}
              coreRequirements={coreRequirements}
              templateName={name}
              isActive={currentSchema?.is_active ?? false}
              isSaved={currentSchema !== null}
              previewHref={
                currentSchema && !hasUnsavedChanges
                  ? `/enroll/preview?schemaId=${encodeURIComponent(currentSchema.id)}`
                  : undefined
              }
              onPreviewClick={requestExternalPreview}
              assignedPhaseCount={
                currentSchema
                  ? phases.filter(
                      (phase) => phase.form_schema_id === currentSchema.id,
                    ).length
                  : 0
              }
            />
          </aside>
        </div>
      </section>
      <UnsavedChangesDialog
        pendingNavigation={pendingNavigation}
        saving={saving}
        saveBlockedMessage={saveBlockedMessage}
        onCancel={() => setPendingNavigation(null)}
        onDiscard={discardPendingNavigation}
        onSave={savePendingNavigation}
      />
    </div>
  );
}

function EnrollmentFormsOverview({
  templates,
  phases,
  onCreate,
  onEdit,
  onPreview,
  onDelete,
  error,
}: Readonly<{
  templates: FormSchema[];
  phases: Phase[];
  onCreate: () => void;
  onEdit: (schema: FormSchema) => void;
  onPreview: (schema: FormSchema) => void;
  onDelete: (schema: FormSchema) => void;
  error: string | null;
}>) {
  const assignedTemplateCount = templates.filter((schema) =>
    phases.some((phase) => phase.form_schema_id === schema.id),
  ).length;

  return (
    <div className="space-y-5">
      <section className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm backdrop-blur-md">
        <div className="grid gap-0 lg:grid-cols-[minmax(0,1fr)_360px] xl:grid-cols-[minmax(0,1fr)_400px]">
          <div className="space-y-6 p-5 sm:p-6">
            <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
              <div>
                <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
                  Formularübersicht
                </p>
                <h2 className="mt-1 text-xl font-semibold text-gray-900">
                  Anmeldeformulare verwalten
                </h2>
                <p className="mt-2 max-w-3xl text-sm leading-6 text-gray-600">
                  Das Basisformular ist immer vorhanden. Eigene Vorlagen nutzt
                  du, wenn eine Anmeldephase abweichende Pflichtangaben oder
                  Zusatzfragen braucht.
                </p>
              </div>
              <button
                type="button"
                onClick={onCreate}
                className="inline-flex h-9 items-center justify-center gap-2 rounded-lg bg-gray-900 px-3 text-sm font-medium whitespace-nowrap text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
              >
                <Plus className="h-4 w-4" aria-hidden="true" />
                Neue Vorlage
              </button>
            </div>

            {error ? (
              <div className="rounded-lg border border-[#FF3130]/20 bg-[#FF3130]/10 p-3 text-sm text-[#CC2626]">
                {error}
              </div>
            ) : null}

            <section className="space-y-3">
              <div className="flex items-end justify-between gap-3">
                <div>
                  <h3 className="text-base font-semibold text-gray-900">
                    Formularvorlagen
                  </h3>
                  <p className="mt-1 text-sm text-gray-600">
                    Jede Anmeldephase nutzt entweder das Basisformular oder eine
                    eigene Vorlage mit angepassten Pflichtangaben und
                    Zusatzfragen.
                  </p>
                </div>
              </div>

              <TemplateOverviewList
                templates={templates}
                phases={phases}
                onEdit={onEdit}
                onPreview={onPreview}
                onDelete={onDelete}
              />
            </section>
          </div>

          <aside className="moto-dotted-background moto-dotted-background--split border-t border-gray-100 p-5 sm:p-6 lg:border-t-0 lg:border-l">
            <OverviewGuide
              templateCount={templates.length}
              assignedTemplateCount={assignedTemplateCount}
            />
          </aside>
        </div>
      </section>
    </div>
  );
}

function TemplateOverviewList({
  templates,
  phases,
  onEdit,
  onPreview,
  onDelete,
}: Readonly<{
  templates: FormSchema[];
  phases: Phase[];
  onEdit: (schema: FormSchema) => void;
  onPreview: (schema: FormSchema) => void;
  onDelete: (schema: FormSchema) => void;
}>) {
  return (
    <div className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
      <div className="divide-y divide-gray-100">
        <BaseTemplateOverviewRow />
        {templates.map((schema) => (
          <TemplateOverviewRow
            key={schema.id}
            schema={schema}
            onEdit={() => onEdit(schema)}
            onPreview={() => onPreview(schema)}
            onDelete={() => onDelete(schema)}
            isAssigned={phases.some(
              (phase) => phase.form_schema_id === schema.id,
            )}
          />
        ))}
      </div>
    </div>
  );
}

function BaseTemplateOverviewRow() {
  return (
    <article className="grid gap-4 px-4 py-4 md:grid-cols-[minmax(0,1fr)_auto] md:items-center">
      <div className="min-w-0">
        <div className="flex items-center gap-2">
          <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-gray-100 text-gray-500">
            <Lock className="h-4 w-4" aria-hidden="true" />
          </span>
          <h4 className="truncate text-sm font-semibold text-gray-900">
            Basisformular
          </h4>
          <span className="rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-medium text-gray-600">
            System
          </span>
        </div>
        <p className="mt-1 text-xs leading-5 text-gray-500">
          Elternteil, Kind, Klassenstufe und gewünschtes Betreuungsangebot.
        </p>
        <div className="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-xs text-gray-500">
          <UsageLine title="Standard" status="base" />
          <span>Systemformular</span>
          <span>Keine Anpassungen</span>
        </div>
      </div>
      <div className="flex justify-start gap-2 md:justify-end">
        <a
          href="/enroll/preview?base=1"
          target="_blank"
          rel="noreferrer"
          className="inline-flex h-9 items-center justify-center gap-2 rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        >
          <ExternalLink className="h-4 w-4" aria-hidden="true" />
          Vorschau
        </a>
      </div>
    </article>
  );
}

function TemplateOverviewRow({
  schema,
  onEdit,
  onPreview,
  onDelete,
  isAssigned,
}: Readonly<{
  schema: FormSchema;
  onEdit: () => void;
  onPreview: () => void;
  onDelete: () => void;
  isAssigned: boolean;
}>) {
  const requiredCount = schema.fields.filter((field) =>
    Boolean(field.required),
  ).length;
  const usageStatus = isAssigned ? "assigned" : "ready";
  const questionLabel =
    schema.fields.length === 1 ? "1 Frage" : `${schema.fields.length} Fragen`;
  const usageTitle = isAssigned ? "In Phase verwendet" : "Nicht verwendet";

  return (
    <article className="grid gap-4 px-4 py-4 md:grid-cols-[minmax(0,1fr)_auto] md:items-center">
      <div className="min-w-0">
        <div className="flex items-center gap-2">
          <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-gray-100 text-gray-500">
            <FileText className="h-4 w-4" aria-hidden="true" />
          </span>
          <h4 className="truncate text-sm font-semibold text-gray-900">
            {schema.name}
          </h4>
        </div>
        <p className="mt-1 text-xs leading-5 text-gray-500">
          Eigene Vorlage für Pflichtangaben und Zusatzfragen.
        </p>
        <div className="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-xs text-gray-500">
          <UsageLine title={usageTitle} status={usageStatus} />
          <span>Erstellt {formatSchemaDate(schema.created_at)}</span>
          <span>{questionLabel}</span>
          {requiredCount > 0 ? <span>{requiredCount} Pflicht</span> : null}
        </div>
      </div>
      <div className="flex justify-start gap-2 md:justify-end">
        <TemplateActionsMenu
          label={`Aktionen für ${schema.name}`}
          items={[
            {
              label: "Prüfen",
              icon: <Eye className="h-4 w-4" aria-hidden />,
              onClick: onPreview,
            },
            {
              label: "Vorschau",
              icon: <ExternalLink className="h-4 w-4" aria-hidden />,
              onClick: () => {
                window.open(
                  `/enroll/preview?schemaId=${encodeURIComponent(schema.id)}`,
                  "_blank",
                  "noopener,noreferrer",
                );
              },
            },
            {
              label: "Bearbeiten",
              icon: <Pencil className="h-4 w-4" aria-hidden />,
              onClick: onEdit,
            },
            {
              label: "Löschen",
              icon: <Trash2 className="h-4 w-4" aria-hidden />,
              onClick: onDelete,
              variant: "danger",
            },
          ]}
        />
      </div>
    </article>
  );
}

interface TemplateActionItem {
  readonly label: string;
  readonly icon: ReactNode;
  readonly onClick: () => void;
  readonly variant?: "default" | "danger";
}

interface TemplateActionsMenuPosition {
  top: number;
  left: number;
  alignRight: boolean;
}

function TemplateActionsMenu({
  label,
  items,
}: Readonly<{
  label: string;
  items: TemplateActionItem[];
}>) {
  const [open, setOpen] = useState(false);
  const [mounted, setMounted] = useState(false);
  const [position, setPosition] = useState<TemplateActionsMenuPosition>({
    top: 0,
    left: 0,
    alignRight: false,
  });
  const buttonRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    setMounted(true);
  }, []);

  useEffect(() => {
    if (!open || !buttonRef.current) return;
    const rect = buttonRef.current.getBoundingClientRect();
    const menuWidth = 224;
    const alignRight = rect.left + menuWidth > window.innerWidth - 16;
    setPosition({
      top: rect.bottom + 6,
      left: alignRight ? rect.right : rect.left,
      alignRight,
    });
  }, [open]);

  useEffect(() => {
    if (!open) return;

    function handleClickOutside(event: MouseEvent) {
      if (
        menuRef.current &&
        event.target instanceof Node &&
        !menuRef.current.contains(event.target)
      ) {
        setOpen(false);
      }
    }

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") setOpen(false);
    }

    document.addEventListener("mousedown", handleClickOutside);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [open]);

  useEffect(() => {
    if (!open) return;

    function handleScroll() {
      setOpen(false);
    }

    window.addEventListener("scroll", handleScroll, true);
    return () => window.removeEventListener("scroll", handleScroll, true);
  }, [open]);

  const menu = open && mounted && (
    <div
      ref={menuRef}
      role="menu"
      tabIndex={-1}
      aria-label={label}
      className="fixed z-[9999] w-56 overflow-hidden rounded-lg border border-gray-200 bg-white py-1 text-left shadow-lg"
      style={{
        top: position.top,
        left: position.alignRight ? "auto" : position.left,
        right: position.alignRight ? window.innerWidth - position.left : "auto",
      }}
    >
      {items.map((item) => (
        <button
          key={item.label}
          type="button"
          role="menuitem"
          onClick={() => {
            setOpen(false);
            item.onClick();
          }}
          className="flex w-full items-center gap-2 px-3 py-2 text-sm text-gray-700 transition-colors hover:bg-gray-50"
        >
          <span
            className={`h-4 w-4 ${
              item.variant === "danger" ? "text-[#FF3130]" : "text-gray-500"
            }`}
          >
            {item.icon}
          </span>
          {item.label}
        </button>
      ))}
    </div>
  );

  return (
    <div className="flex justify-end">
      <button
        ref={buttonRef}
        type="button"
        onClick={() => setOpen((prev) => !prev)}
        aria-label={label}
        aria-haspopup="menu"
        aria-expanded={open}
        className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-gray-200 bg-white text-gray-500 shadow-sm transition-colors hover:bg-gray-50 hover:text-gray-800 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
      >
        <MoreVertical className="h-4 w-4" aria-hidden="true" />
      </button>
      {mounted ? createPortal(menu, document.body) : null}
    </div>
  );
}

type TemplateStatus = "base" | "assigned" | "ready";

function UsageLine({
  title,
  status,
}: Readonly<{
  title: string;
  status: TemplateStatus;
}>) {
  const statusMap: Record<
    TemplateStatus,
    { dotClassName: string; textClassName: string }
  > = {
    base: {
      dotClassName: "bg-gray-300",
      textClassName: "text-gray-700",
    },
    assigned: {
      dotClassName: "bg-[#83CD2D]",
      textClassName: "text-[#5F9F20]",
    },
    ready: {
      dotClassName: "bg-gray-300",
      textClassName: "text-gray-700",
    },
  };
  const config = statusMap[status];

  return (
    <span
      className={`inline-flex items-center gap-2 font-medium ${config.textClassName}`}
    >
      <span
        className={`h-2 w-2 shrink-0 rounded-full ${config.dotClassName}`}
        aria-hidden="true"
      />
      {title}
    </span>
  );
}

function OverviewGuide({
  templateCount,
  assignedTemplateCount,
}: Readonly<{
  templateCount: number;
  assignedTemplateCount: number;
}>) {
  return (
    <div className="sticky top-6 space-y-4">
      <div>
        <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
          Startpunkt
        </p>
        <h2 className="mt-1 text-base font-semibold text-gray-900">
          Erst prüfen, dann erweitern
        </h2>
        <p className="mt-2 text-sm leading-6 text-gray-600">
          Lege nur dann eine eigene Vorlage an, wenn das Basisformular nicht
          reicht. So bleibt die Elternansicht kurz und verständlich.
        </p>
      </div>

      <div className="moto-content-surface rounded-2xl border p-4 shadow-sm">
        <h3 className="text-sm font-semibold text-gray-900">
          Nächste Schritte
        </h3>
        <div className="mt-4 space-y-3">
          <GuideStep
            icon={<ShieldCheck className="h-4 w-4" aria-hidden="true" />}
            title="Basisformular prüfen"
            done
          />
          <GuideStep
            icon={<ListPlus className="h-4 w-4" aria-hidden="true" />}
            title="Vorlage nur bei Bedarf"
            done={templateCount > 0}
          />
          <GuideStep
            icon={<CalendarClock className="h-4 w-4" aria-hidden="true" />}
            title="Vorlage in Anmeldephase wählen"
            done={assignedTemplateCount > 0}
          />
        </div>
      </div>

      <p className="text-sm leading-6 text-gray-500">
        Für eine neue Halbjahresanmeldung brauchst du oft keine eigene
        Formularvorlage. Anmeldephase und Betreuungsangebote steuern den
        eigentlichen Ablauf.
      </p>
    </div>
  );
}

function FormTemplateDetail({
  schema,
  onBack,
  onEdit,
  assignedPhases,
}: Readonly<{
  schema: FormSchema;
  onBack: () => void;
  onEdit: () => void;
  assignedPhases: Phase[];
}>) {
  const requiredCount = schema.fields.filter((field) =>
    Boolean(field.required),
  ).length;
  const childFieldCount = schema.fields.filter((field) =>
    Boolean(field.applies_to_child),
  ).length;

  return (
    <div className="space-y-5">
      <section className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm backdrop-blur-md">
        <div className="grid gap-0 lg:grid-cols-[minmax(0,1fr)_360px] xl:grid-cols-[minmax(0,1fr)_400px]">
          <div>
            <div className="border-b border-gray-100 px-5 py-3 sm:px-6">
              <button
                type="button"
                onClick={onBack}
                className="inline-flex h-8 items-center gap-2 rounded-lg px-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-50 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
              >
                <ArrowLeft className="h-4 w-4" aria-hidden="true" />
                Zurück zur Übersicht
              </button>
            </div>
            <div className="space-y-5 p-5 sm:p-6">
              <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                <div>
                  <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
                    Formular prüfen
                  </p>
                  <h2 className="mt-1 text-xl font-semibold text-gray-900">
                    {schema.name}
                  </h2>
                  <p className="mt-2 max-w-3xl text-sm leading-6 text-gray-600">
                    Prüfe die Formularvorschau und ordne diese Vorlage einer
                    Anmeldephase zu, wenn Eltern die angepassten Pflichtangaben
                    oder Zusatzfragen sehen sollen.
                  </p>
                </div>
                <button
                  type="button"
                  onClick={onEdit}
                  className="inline-flex h-9 items-center justify-center gap-2 rounded-lg bg-gray-900 px-3 text-sm font-medium whitespace-nowrap text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                >
                  <Pencil className="h-4 w-4" aria-hidden="true" />
                  Bearbeiten
                </button>
              </div>

              <div className="grid grid-cols-2 gap-3 xl:grid-cols-4">
                <FormMetric
                  icon={<FileText className="h-4 w-4" aria-hidden="true" />}
                  value={formatSchemaDate(schema.created_at)}
                  label="Zuletzt gespeichert"
                />
                <FormMetric
                  icon={<ListPlus className="h-4 w-4" aria-hidden="true" />}
                  value={schema.fields.length.toString()}
                  label="Zusatzfragen"
                />
                <FormMetric
                  icon={<Check className="h-4 w-4" aria-hidden="true" />}
                  value={requiredCount.toString()}
                  label="Pflicht-Zusatzfragen"
                />
                <FormMetric
                  icon={<FileText className="h-4 w-4" aria-hidden="true" />}
                  value={childFieldCount.toString()}
                  label="Pro Kind"
                />
              </div>

              <FormPreview
                fields={schema.fields}
                coreRequirements={schema.core_requirements ?? {}}
                templateName={schema.name}
                isActive={schema.is_active}
                isSaved
                previewHref={`/enroll/preview?schemaId=${encodeURIComponent(schema.id)}`}
                assignedPhaseCount={assignedPhases.length}
                sticky={false}
              />
            </div>
          </div>

          <aside className="moto-dotted-background moto-dotted-background--split border-t border-gray-100 p-5 sm:p-6 lg:border-t-0 lg:border-l">
            <div className="sticky top-6 space-y-4">
              <div>
                <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                  Nächster Schritt
                </p>
                <h2 className="mt-1 text-base font-semibold text-gray-900">
                  {assignedPhases.length > 0
                    ? "In Anmeldephase verwendet"
                    : "In Anmeldephase verwenden"}
                </h2>
                <p className="mt-2 text-sm leading-6 text-gray-600">
                  {assignedPhases.length > 0
                    ? `Diese Vorlage ist in ${assignedPhases.length} Anmeldephase ausgewählt.`
                    : "Eine Vorlage wird erst für Eltern relevant, wenn sie in einer Anmeldephase ausgewählt ist. Ohne Auswahl nutzt die Phase das Basisformular."}
                </p>
              </div>

              <Link
                href={
                  assignedPhases.length > 0
                    ? "/enrollment-phases"
                    : `/enrollment-phases?assignForm=${encodeURIComponent(schema.id)}`
                }
                className="inline-flex h-10 w-full items-center justify-center gap-2 rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
              >
                <CalendarClock className="h-4 w-4" aria-hidden="true" />
                {assignedPhases.length > 0
                  ? "Anmeldephasen öffnen"
                  : "In Anmeldephase auswählen"}
              </Link>

              <a
                href={`/enroll/preview?schemaId=${encodeURIComponent(schema.id)}`}
                target="_blank"
                rel="noreferrer"
                className="moto-content-surface flex items-start gap-3 rounded-2xl border p-3 text-left shadow-sm transition-colors hover:border-gray-300 hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
              >
                <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-white text-gray-600 shadow-sm">
                  <ExternalLink className="h-4 w-4" aria-hidden="true" />
                </span>
                <span className="min-w-0">
                  <span className="block text-sm font-semibold text-gray-900">
                    Vorschau öffnen
                  </span>
                  <span className="mt-0.5 block text-xs leading-5 text-gray-500">
                    Öffnet die Formularvorschau in einem neuen Tab.
                  </span>
                </span>
              </a>

              <div className="moto-content-surface rounded-2xl border p-4 shadow-sm">
                <h3 className="text-sm font-semibold text-gray-900">Status</h3>
                <div className="mt-4 space-y-3">
                  <GuideStep
                    icon={<Check className="h-4 w-4" aria-hidden="true" />}
                    title="Vorlage gespeichert"
                    done
                  />
                  <GuideStep
                    icon={<Eye className="h-4 w-4" aria-hidden="true" />}
                    title="Vorschau prüfen"
                    done
                  />
                  <GuideStep
                    icon={
                      <CalendarClock className="h-4 w-4" aria-hidden="true" />
                    }
                    title="In Anmeldephase auswählen"
                    done={assignedPhases.length > 0}
                  />
                </div>
              </div>
            </div>
          </aside>
        </div>
      </section>
    </div>
  );
}

function GuideStep({
  icon,
  title,
  done,
}: Readonly<{ icon: ReactNode; title: string; done: boolean }>) {
  return (
    <div className="flex items-center gap-3">
      <span
        className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-xl ${
          done ? "bg-[#83CD2D]/15 text-[#5F9F20]" : "bg-gray-100 text-gray-500"
        }`}
      >
        {icon}
      </span>
      <span className="min-w-0 flex-1 text-sm font-medium text-gray-700">
        {title}
      </span>
      <span
        className={`h-2 w-2 rounded-full ${
          done ? "bg-[#83CD2D]" : "bg-gray-300"
        }`}
        aria-hidden="true"
      />
    </div>
  );
}

function DeleteSchemaDialog({
  schema,
  deleting,
  onClose,
  onConfirm,
}: Readonly<{
  schema: FormSchema | null;
  deleting: boolean;
  onClose: () => void;
  onConfirm: () => void;
}>) {
  return (
    <ConfirmationModal
      isOpen={schema !== null}
      onClose={onClose}
      onConfirm={onConfirm}
      title="Formularvorlage löschen?"
      confirmText="Löschen"
      cancelText="Abbrechen"
      isConfirmLoading={deleting}
      confirmButtonClass="bg-[#FF3130] hover:bg-[#CC2626]"
    >
      <div className="space-y-3 text-sm leading-6 text-gray-600">
        <p>
          Die Vorlage{" "}
          <span className="font-semibold text-gray-900">{schema?.name}</span>{" "}
          wird dauerhaft gelöscht.
        </p>
        <p>
          Dabei werden alle Versionen dieser Vorlage entfernt. Bereits
          verwendete Vorlagen können nicht gelöscht werden.
        </p>
      </div>
    </ConfirmationModal>
  );
}

function UnsavedChangesDialog({
  pendingNavigation,
  saving,
  saveBlockedMessage,
  onCancel,
  onDiscard,
  onSave,
}: Readonly<{
  pendingNavigation: PendingNavigation | null;
  saving: boolean;
  saveBlockedMessage: string | null;
  onCancel: () => void;
  onDiscard: () => void;
  onSave: () => void;
}>) {
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  useEffect(() => {
    if (!pendingNavigation) return;
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") onCancel();
    }
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [onCancel, pendingNavigation]);

  if (!mounted || !pendingNavigation) return null;

  const isPreview = pendingNavigation === "preview";
  const dialog = (
    <div
      className="fixed inset-0 z-[10000] flex items-center justify-center bg-gray-950/35 p-4 backdrop-blur-sm"
      role="presentation"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onCancel();
      }}
    >
      <section
        role="dialog"
        aria-modal="true"
        aria-labelledby="unsaved-form-dialog-title"
        className="moto-content-surface w-full max-w-lg rounded-2xl border p-5 shadow-xl"
      >
        <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
          Ungespeicherte Änderungen
        </p>
        <h2
          id="unsaved-form-dialog-title"
          className="mt-1 text-lg font-semibold text-gray-900"
        >
          Änderungen speichern?
        </h2>
        <p className="mt-2 text-sm leading-6 text-gray-600">
          {isPreview
            ? "Für die externe Vorschau muss die Vorlage zuerst gespeichert werden."
            : "Du hast Änderungen an dieser Vorlage. Speichere sie, bevor du den Bereich verlässt, oder verwirf sie bewusst."}
        </p>
        {saveBlockedMessage ? (
          <div className="mt-4 rounded-lg border border-[#FF3130]/20 bg-[#FF3130]/10 px-3 py-2 text-sm font-medium text-[#9F1F1E]">
            {saveBlockedMessage}
          </div>
        ) : null}
        <div className="mt-5 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <button
            type="button"
            onClick={onCancel}
            disabled={saving}
            className="inline-flex h-9 items-center justify-center rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
          >
            Abbrechen
          </button>
          {!isPreview ? (
            <button
              type="button"
              onClick={onDiscard}
              disabled={saving}
              className="inline-flex h-9 items-center justify-center rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
            >
              Verwerfen
            </button>
          ) : null}
          <button
            type="button"
            onClick={onSave}
            disabled={saving || Boolean(saveBlockedMessage)}
            className="inline-flex h-9 items-center justify-center rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
          >
            {saving
              ? "Speichert..."
              : isPreview
                ? "Speichern und Vorschau öffnen"
                : "Speichern und fortfahren"}
          </button>
        </div>
      </section>
    </div>
  );

  return createPortal(dialog, document.body);
}

function FormBuilderIntro() {
  return (
    <header>
      <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
        Formular-Konfigurator
      </p>
      <h2 className="mt-1 text-xl font-semibold text-gray-900">
        Formularvorlage bearbeiten
      </h2>
      <p className="mt-2 max-w-3xl text-sm leading-6 text-gray-600">
        Lege fest, welche optionalen Basisfelder verpflichtend sind, und ergänze
        bei Bedarf Zusatzfragen. Die Vorlage wird später in einer Anmeldephase
        ausgewählt.
      </p>
    </header>
  );
}

function FormMetric({
  icon,
  value,
  label,
}: Readonly<{
  icon: ReactNode;
  value: string;
  label: string;
}>) {
  return (
    <div className="moto-content-surface flex items-center gap-3 rounded-2xl border px-4 py-3 shadow-sm">
      <span className="flex h-9 w-9 items-center justify-center rounded-xl bg-white text-gray-600 shadow-sm">
        {icon}
      </span>
      <span>
        <span className="block text-lg leading-none font-semibold text-gray-900">
          {value}
        </span>
        <span className="mt-1 block text-xs font-medium text-gray-500">
          {label}
        </span>
      </span>
    </div>
  );
}

function BuilderTemplateSummary({
  name,
  isCreating,
  saving,
  currentSchema,
  onNameChange,
  fields,
}: Readonly<{
  name: string;
  isCreating: boolean;
  saving: boolean;
  currentSchema: FormSchema | null;
  onNameChange: (value: string) => void;
  fields: FormField[];
}>) {
  const requiredCount = fields.filter((field) =>
    Boolean(field.required),
  ).length;
  const childFieldCount = fields.filter((field) =>
    Boolean(field.applies_to_child),
  ).length;

  return (
    <section className="moto-content-surface rounded-2xl border p-4 shadow-sm">
      <div className="grid gap-4 md:grid-cols-[minmax(0,1fr)_280px]">
        <div className="min-w-0">
          <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
            Vorlage
          </p>
          <h2 className="mt-1 text-base font-semibold text-gray-900">
            {isCreating ? "Neue Formularvorlage" : name}
          </h2>
          <p className="mt-1 max-w-2xl text-sm text-gray-600">
            {isCreating
              ? "Gib der Vorlage einen eindeutigen Namen. Danach kannst du Pflichtangaben und Zusatzfragen festlegen."
              : "Beim Speichern bleiben bestehende Anmeldungen nachvollziehbar."}
          </p>
        </div>

        {isCreating ? (
          <label className="block">
            <span className="text-xs font-medium text-gray-700">
              Name der Vorlage
            </span>
            <input
              type="text"
              value={name}
              onChange={(event) => onNameChange(event.target.value)}
              placeholder="z. B. Ferienbetreuung Sommer 2026"
              disabled={saving}
              className="mt-1 h-10 w-full rounded-lg border border-gray-200 px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:bg-gray-100 disabled:text-gray-600"
            />
          </label>
        ) : (
          <div className="flex flex-wrap content-start gap-2 text-xs text-gray-600">
            <span className="rounded-full bg-gray-100 px-2.5 py-1 font-medium text-gray-700">
              {fields.length} Zusatzfragen
            </span>
            <span className="rounded-full bg-gray-100 px-2.5 py-1 font-medium text-gray-700">
              {requiredCount} Pflicht-Zusatzfragen
            </span>
            <span className="rounded-full bg-gray-100 px-2.5 py-1 font-medium text-gray-700">
              {childFieldCount} pro Kind
            </span>
          </div>
        )}
      </div>

      {currentSchema ? (
        <div className="mt-4 flex flex-wrap gap-2 text-xs text-gray-600">
          <span className="rounded-full bg-gray-100 px-2.5 py-1 font-medium text-gray-700">
            {new Date(currentSchema.created_at).toLocaleString("de-DE")}
          </span>
        </div>
      ) : null}
    </section>
  );
}

function CoreFieldsSection({
  coreRequirements,
  onRequirementChange,
  disabled,
}: Readonly<{
  coreRequirements: CoreRequirements;
  onRequirementChange: (key: CoreRequirementKey, required: boolean) => void;
  disabled: boolean;
}>) {
  const guardian = CORE_FIELDS.filter((f) => f.group === "guardian");
  const child = CORE_FIELDS.filter((f) => f.group === "child");
  const groups = [
    { title: "Eltern", fields: guardian },
    { title: "Kind", fields: child },
  ] satisfies ReadonlyArray<{ title: string; fields: CoreField[] }>;

  return (
    <section className="moto-content-surface rounded-2xl border p-5 shadow-sm">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
            Basisformular
          </p>
          <h2 className="mt-1 text-base font-semibold text-gray-900">
            Pflichtstatus der Basisfelder
          </h2>
          <p className="mt-1 max-w-2xl text-sm leading-6 text-gray-600">
            Fest gesetzte Felder bleiben immer Pflicht. Optionale Basisfelder
            kannst du hier verpflichtend machen.
          </p>
        </div>
        <span className="inline-flex w-fit items-center gap-1.5 rounded-full bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-600">
          <Lock className="h-3.5 w-3.5" aria-hidden="true" />
          Basisfelder
        </span>
      </div>

      <div className="mt-4 overflow-hidden rounded-xl border border-gray-200">
        {groups.map((group) => (
          <CoreFieldGroup
            key={group.title}
            title={group.title}
            fields={group.fields}
            coreRequirements={coreRequirements}
            onRequirementChange={onRequirementChange}
            disabled={disabled}
          />
        ))}
      </div>
    </section>
  );
}

function CoreFieldGroup({
  title,
  fields,
  coreRequirements,
  onRequirementChange,
  disabled,
}: Readonly<{
  title: string;
  fields: CoreField[];
  coreRequirements: CoreRequirements;
  onRequirementChange: (key: CoreRequirementKey, required: boolean) => void;
  disabled: boolean;
}>) {
  return (
    <div className="border-b border-gray-100 last:border-b-0">
      <div className="bg-gray-50/80 px-4 py-2">
        <h3 className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
          {title}
        </h3>
      </div>
      <ul className="divide-y divide-gray-100">
        {fields.map((field) => (
          <CoreFieldRow
            key={field.key}
            field={field}
            required={Boolean(
              field.requirementKey
                ? coreRequirements[field.requirementKey]
                : field.required,
            )}
            onRequirementChange={onRequirementChange}
            disabled={disabled}
          />
        ))}
      </ul>
    </div>
  );
}

function CoreFieldRow({
  field,
  required,
  onRequirementChange,
  disabled,
}: {
  readonly field: CoreField;
  readonly required: boolean;
  readonly onRequirementChange: (
    key: CoreRequirementKey,
    required: boolean,
  ) => void;
  readonly disabled: boolean;
}) {
  return (
    <li className="grid gap-3 bg-white px-4 py-3 sm:grid-cols-[minmax(0,1fr)_220px] sm:items-center">
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-2">
          <p className="text-sm font-medium text-gray-900">{field.label}</p>
          <span className="rounded bg-gray-100 px-1.5 py-0.5 text-xs text-gray-500">
            {fieldTypeLabels[field.type]}
          </span>
        </div>
        <p className="mt-0.5 text-xs leading-5 text-gray-500">
          {field.requirementKey
            ? (field.requirementHint ?? "Kann verpflichtend gemacht werden.")
            : "Immer erforderlich und deshalb nicht änderbar."}
        </p>
      </div>
      {field.requirementKey ? (
        <div className="flex items-center justify-between gap-3 sm:justify-end">
          <span className="text-xs font-medium text-gray-600">
            {required ? "Pflicht" : "Optional"}
          </span>
          <BooleanField
            value={required}
            onChange={(checked) =>
              onRequirementChange(field.requirementKey!, checked)
            }
            disabled={disabled}
            ariaLabel={field.requirementLabel ?? `${field.label} Pflichtfeld`}
          />
        </div>
      ) : (
        <div className="flex justify-start sm:justify-end">
          <span className="inline-flex items-center gap-1.5 rounded-full bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-600">
            <Lock className="h-3.5 w-3.5" aria-hidden="true" />
            Immer Pflicht
          </span>
        </div>
      )}
    </li>
  );
}

function TargetSuggestions({
  fields,
  onAdd,
  disabled,
}: Readonly<{
  fields: FormField[];
  onAdd: (target: Exclude<FormFieldTarget, "">) => void;
  disabled: boolean;
}>) {
  const selectedTargets = useMemo(
    () => new Set(fields.map((field) => field.target).filter(Boolean)),
    [fields],
  );

  return (
    <section className="rounded-2xl border border-gray-200 bg-gray-50/70 p-4">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h3 className="text-sm font-semibold text-gray-900">
            Stammdaten-Vorschläge
          </h3>
          <p className="mt-1 max-w-2xl text-xs leading-5 text-gray-600">
            Diese Fragen sind mit vorhandenen Stammdaten verbunden. Du kannst
            sie hinzufügen oder entfernen; Label und Typ sind fest vorgegeben.
          </p>
        </div>
        <span className="inline-flex w-fit items-center gap-1.5 rounded-full bg-white px-2.5 py-1 text-xs font-medium text-gray-600 shadow-sm">
          <Lock className="h-3.5 w-3.5" aria-hidden="true" />
          Stammdaten
        </span>
      </div>

      <div className="mt-4 grid gap-2 md:grid-cols-2">
        {TARGET_PICKER_ORDER.map((target) => {
          const selected = selectedTargets.has(target);
          return (
            <button
              key={target}
              type="button"
              onClick={() => onAdd(target)}
              disabled={disabled || selected}
              className={`flex min-h-24 items-start gap-3 rounded-xl border p-3 text-left shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-default ${
                selected
                  ? "border-gray-200 bg-white text-gray-500"
                  : "border-gray-200 bg-white text-gray-700 hover:border-gray-300 hover:bg-gray-50"
              }`}
            >
              <span
                className={`mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-lg ${
                  selected
                    ? "bg-gray-100 text-gray-500"
                    : "bg-gray-900 text-white"
                }`}
              >
                {selected ? (
                  <Check className="h-4 w-4" aria-hidden="true" />
                ) : (
                  <Plus className="h-4 w-4" aria-hidden="true" />
                )}
              </span>
              <span className="min-w-0">
                <span className="block text-sm font-semibold text-gray-900">
                  {RESERVED_TARGETS[target].label}
                </span>
                <span className="mt-1 block text-xs leading-5 text-gray-500">
                  {targetSuggestionDescriptions[target]}
                </span>
                <span className="mt-2 inline-flex rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-medium text-gray-600">
                  {selected ? "Ist drin" : "Hinzufügen"}
                </span>
              </span>
            </button>
          );
        })}
      </div>
    </section>
  );
}

interface FieldEditorRowProps {
  readonly field: FormField;
  readonly index: number;
  readonly total: number;
  readonly allFields: FormField[];
  readonly onChange: (patch: Partial<FormField>) => void;
  readonly onRemove: () => void;
  readonly onMoveUp: () => void;
  readonly onMoveDown: () => void;
  readonly disabled: boolean;
}

function FieldEditorRow({
  field,
  index,
  total,
  allFields,
  onChange,
  onRemove,
  onMoveUp,
  onMoveDown,
  disabled,
}: FieldEditorRowProps) {
  const target = field.target || null;
  const isTargetField = target !== null;
  const isInfo = field.type === "information";
  const displayLabel = isInfo
    ? field.label.trim() || "Infotext"
    : target
      ? field.label.trim() || RESERVED_TARGETS[target].label
      : field.label.trim() || "Neue Zusatzfrage";
  const optionSignature = useMemo(
    () => (field.options ?? []).map((option) => option.label).join("\n"),
    [field.options],
  );
  const [optionsDraft, setOptionsDraft] = useState(optionSignature);

  useEffect(() => {
    setOptionsDraft(optionSignature);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [field.key, field.type]);

  const updateOptions = (value: string) => {
    setOptionsDraft(value);
    const options = value
      .split("\n")
      .map((line) => line.trim())
      .filter(Boolean)
      .map((line) => ({
        label: line,
        value: normalizeFieldKey(line),
      }));
    onChange({ options });
  };

  return (
    <article className="moto-content-surface rounded-2xl border p-4 shadow-sm">
      <div className="flex items-start gap-3">
        <span className="mt-1 hidden text-gray-300 sm:block">
          <GripVertical className="h-5 w-5" aria-hidden="true" />
        </span>
        <div className="min-w-0 flex-1 space-y-4">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <div className="flex flex-wrap items-center gap-2">
                <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                  {isInfo ? "Infotext" : "Frage"} {index + 1}
                </p>
                {isInfo ? (
                  <span className="inline-flex items-center gap-1 rounded-full bg-[#5080D8]/10 px-2 py-0.5 text-[11px] font-medium text-[#3D63B0]">
                    <Info className="h-3 w-3" aria-hidden="true" />
                    Hinweis
                  </span>
                ) : isTargetField ? (
                  <span className="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-medium text-gray-600">
                    <Lock className="h-3 w-3" aria-hidden="true" />
                    Fester Vorschlag
                  </span>
                ) : (
                  <span className="rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-medium text-gray-600">
                    Freie Zusatzfrage
                  </span>
                )}
              </div>
              <h3 className="mt-1 text-sm font-semibold text-gray-900">
                {displayLabel}
              </h3>
            </div>
            <div className="flex gap-1.5">
              <button
                type="button"
                onClick={onMoveUp}
                disabled={disabled || index === 0}
                className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-gray-200 bg-white text-sm font-medium text-gray-500 shadow-sm transition-colors hover:bg-gray-50 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-40"
                aria-label="Frage nach oben verschieben"
              >
                ↑
              </button>
              <button
                type="button"
                onClick={onMoveDown}
                disabled={disabled || index === total - 1}
                className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-gray-200 bg-white text-sm font-medium text-gray-500 shadow-sm transition-colors hover:bg-gray-50 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-40"
                aria-label="Frage nach unten verschieben"
              >
                ↓
              </button>
              <button
                type="button"
                onClick={onRemove}
                disabled={disabled}
                className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-gray-200 bg-white text-gray-500 shadow-sm transition-colors hover:border-[#FF3130]/30 hover:bg-[#FF3130]/10 hover:text-[#CC2626] focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-40"
                aria-label="Frage entfernen"
              >
                <Trash2 className="h-4 w-4" aria-hidden="true" />
              </button>
            </div>
          </div>

          {isInfo ? (
            <div className="rounded-xl border border-[#5080D8]/20 bg-[#5080D8]/5 px-3 py-2 text-xs leading-5 text-gray-600">
              <p className="font-medium text-[#3D63B0]">
                Wird Eltern als Hinweis angezeigt.
              </p>
              <p className="mt-0.5">
                Ein Infotext sammelt keine Antwort. Nutze ihn für Erklärungen,
                Hinweise oder Zwischenüberschriften.
              </p>
            </div>
          ) : isTargetField ? (
            <div className="rounded-xl border border-gray-200 bg-gray-50/80 px-3 py-2 text-xs leading-5 text-gray-600">
              <p className="font-medium text-gray-800">
                Wird bei bestätigter Anmeldung in die Stammdaten übernommen.
              </p>
              <p className="mt-0.5">
                Du kannst die angezeigte Frage umbenennen. Inhalt und Typ sind
                fest vorgegeben. Entferne den Vorschlag, wenn diese Angabe nicht
                abgefragt werden soll.
              </p>
            </div>
          ) : (
            <div className="rounded-xl border border-gray-200 bg-gray-50/80 px-3 py-2 text-xs leading-5 text-gray-600">
              <p className="font-medium text-gray-800">
                Bleibt nur als Zusatzangabe bei der Anmeldung.
              </p>
              <p className="mt-0.5">
                Für Angaben, die nicht automatisch in die Stammdaten übernommen
                werden sollen.
              </p>
            </div>
          )}

          {isInfo ? (
            <>
              <label className="block">
                <span className="text-xs font-medium text-gray-700">
                  Titel (optional)
                </span>
                <input
                  type="text"
                  value={field.label}
                  onChange={(event) => onChange({ label: event.target.value })}
                  placeholder="z. B. Wichtiger Hinweis"
                  disabled={disabled}
                  className="mt-1 h-10 w-full rounded-lg border border-gray-200 px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                />
              </label>
              <label className="block">
                <span className="text-xs font-medium text-gray-700">
                  Infotext für Eltern
                </span>
                <textarea
                  value={field.content ?? ""}
                  onChange={(event) =>
                    onChange({ content: event.target.value })
                  }
                  placeholder="Dieser Text wird Eltern im Formular angezeigt."
                  disabled={disabled}
                  rows={3}
                  className="mt-1 w-full rounded-lg border border-gray-200 px-3 py-2 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                />
              </label>
              <div className="grid gap-2 sm:grid-cols-2">
                <FormChoice
                  checked={Boolean(field.applies_to_child)}
                  onChange={(checked) =>
                    onChange({ applies_to_child: checked })
                  }
                  label="Pro Kind anzeigen"
                  hint="Der Hinweis erscheint bei jedem angemeldeten Kind."
                  disabled={disabled}
                />
              </div>
            </>
          ) : (
            <>
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <label className="block">
                  <span className="text-xs font-medium text-gray-700">
                    Frage im Elternformular
                  </span>
                  <input
                    type="text"
                    value={field.label}
                    onChange={(event) => {
                      const nextLabel = event.target.value;
                      if (isTargetField) {
                        // Suggested fields keep their fixed key + type; only
                        // the displayed question text is editable.
                        onChange({ label: nextLabel });
                        return;
                      }
                      const currentAutoKey = normalizeFieldKey(field.label);
                      const shouldUpdateKey =
                        field.key.trim() === "" || field.key === currentAutoKey;
                      onChange({
                        label: nextLabel,
                        key: shouldUpdateKey
                          ? normalizeFieldKey(nextLabel)
                          : field.key,
                      });
                    }}
                    placeholder={
                      isTargetField && target
                        ? RESERVED_TARGETS[target].label
                        : "z. B. Allergien oder Hinweise"
                    }
                    disabled={disabled}
                    className="mt-1 h-10 w-full rounded-lg border border-gray-200 px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                  />
                </label>
                <label className="block">
                  <span className="text-xs font-medium text-gray-700">
                    Typ
                    {isTargetField ? (
                      <span className="ml-1 text-[11px] font-normal text-gray-500">
                        (automatisch festgelegt)
                      </span>
                    ) : null}
                  </span>
                  <select
                    value={field.type}
                    onChange={(event) =>
                      onChange({ type: event.target.value as FormFieldType })
                    }
                    disabled={disabled || isTargetField}
                    className="moto-select moto-content-surface mt-1 h-10 w-full rounded-lg border px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:bg-gray-100 disabled:text-gray-600"
                  >
                    {(isTargetField ? [field.type] : freeFieldTypes).map(
                      (value) => (
                        <option key={value} value={value}>
                          {fieldTypeLabels[value]}
                        </option>
                      ),
                    )}
                  </select>
                </label>
              </div>

              <label className="block">
                <span className="text-xs font-medium text-gray-700">
                  Hilfetext für Eltern
                </span>
                <input
                  type="text"
                  value={field.help_text ?? ""}
                  onChange={(event) =>
                    onChange({ help_text: event.target.value })
                  }
                  placeholder="Optionaler kurzer Hinweis unter der Frage"
                  disabled={disabled}
                  className="mt-1 h-10 w-full rounded-lg border border-gray-200 px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                />
              </label>

              {field.type === "select" ? (
                <label className="block">
                  <span className="text-xs font-medium text-gray-700">
                    Auswahloptionen
                  </span>
                  <textarea
                    value={optionsDraft}
                    onChange={(event) => updateOptions(event.target.value)}
                    placeholder={"Eine Option pro Zeile\nz. B. Ja\nz. B. Nein"}
                    disabled={disabled || isTargetField}
                    rows={3}
                    className="mt-1 w-full rounded-lg border border-gray-200 px-3 py-2 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                  />
                </label>
              ) : null}

              <div className="grid gap-2 sm:grid-cols-2">
                <FormChoice
                  checked={Boolean(field.required)}
                  onChange={(checked) => onChange({ required: checked })}
                  label="Pflichtfrage"
                  hint={getRequiredHint(field)}
                  disabled={disabled}
                />
                <FormChoice
                  checked={Boolean(field.applies_to_child)}
                  onChange={(checked) =>
                    onChange({ applies_to_child: checked })
                  }
                  label="Pro Kind abfragen"
                  hint="Die Frage erscheint für jedes angemeldete Kind."
                  disabled={disabled || isTargetField}
                />
              </div>
            </>
          )}

          <ConditionEditor
            field={field}
            index={index}
            allFields={allFields}
            onChange={onChange}
            disabled={disabled}
          />
        </div>
      </div>
    </article>
  );
}

function FormChoice({
  checked,
  onChange,
  label,
  hint,
  disabled,
}: Readonly<{
  checked: boolean;
  onChange: (checked: boolean) => void;
  label: string;
  hint: string;
  disabled: boolean;
}>) {
  return (
    <button
      type="button"
      role="checkbox"
      aria-checked={checked}
      onClick={() => onChange(!checked)}
      disabled={disabled}
      className={`flex cursor-pointer items-start gap-3 rounded-lg border p-3 transition-colors ${
        checked
          ? "border-[#83CD2D]/40 bg-[#83CD2D]/10"
          : "border-gray-200 bg-white hover:bg-gray-50"
      } ${disabled ? "cursor-not-allowed opacity-60" : ""}`}
    >
      <span
        className={`mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-md border ${
          checked
            ? "border-[#83CD2D] bg-[#83CD2D] text-white"
            : "border-gray-300 bg-white"
        }`}
        aria-hidden="true"
      >
        {checked ? <Check className="h-3.5 w-3.5" /> : null}
      </span>
      <span className="min-w-0">
        <span className="block text-sm font-medium text-gray-900">{label}</span>
        <span className="mt-0.5 block text-xs text-gray-500">{hint}</span>
      </span>
    </button>
  );
}

const conditionInputClass =
  "moto-select moto-content-surface mt-1 h-10 w-full rounded-lg border px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:bg-gray-100 disabled:text-gray-600";

// Field-source operators offered in the condition editor. "includes" is
// reserved for the care-offering source and is not listed here.
const FIELD_CONDITION_OPERATORS: ConditionOperator[] = [
  "eq",
  "neq",
  "not_empty",
];

function defaultConditionForController(
  controller: FormField,
): VisibilityCondition {
  if (controller.type === "select") {
    return {
      source: "field",
      field: controller.key,
      operator: "eq",
      value: controller.options?.[0]?.value ?? "",
    };
  }
  return {
    source: "field",
    field: controller.key,
    operator: "eq",
    value: true,
  };
}

function defaultConditionForSource(
  source: ConditionSource,
  controllers: FormField[],
): VisibilityCondition {
  if (source === "grade_level") {
    return { source: "grade_level", operator: "eq", value: 1 };
  }
  if (source === "care_offering") {
    return { source: "care_offering", operator: "includes", value: "" };
  }
  const controller = controllers[0];
  return controller
    ? defaultConditionForController(controller)
    : { source: "field", field: "", operator: "eq", value: "" };
}

/**
 * Per-field visibility editor. Lets an admin gate a question or info
 * block behind another answer, the child's grade level, or a chosen
 * care offering. Eligible controlling questions are limited to yes/no
 * and selection fields observable from this field's scope.
 */
function ConditionEditor({
  field,
  index,
  allFields,
  onChange,
  disabled,
}: Readonly<{
  field: FormField;
  index: number;
  allFields: FormField[];
  onChange: (patch: Partial<FormField>) => void;
  disabled: boolean;
}>) {
  const condition = field.visible_when ?? null;

  const controllers = allFields.filter(
    (f, i) =>
      i !== index &&
      f.key.trim() !== "" &&
      (f.type === "boolean" || f.type === "select") &&
      (field.applies_to_child || !f.applies_to_child),
  );

  const sources: ConditionSource[] = [];
  if (controllers.length > 0) sources.push("field");
  if (field.applies_to_child) {
    sources.push("grade_level", "care_offering");
  }
  const canEnable = sources.length > 0;

  const setCondition = (next: VisibilityCondition | null) =>
    onChange({ visible_when: next });
  const patchCondition = (patch: Partial<VisibilityCondition>) => {
    if (condition) setCondition({ ...condition, ...patch });
  };

  if (!canEnable && !condition) {
    return (
      <p className="rounded-lg border border-dashed border-gray-200 bg-gray-50/60 px-3 py-2 text-xs leading-5 text-gray-500">
        Eine Sichtbarkeitsregel ist möglich, sobald es eine Ja/Nein- oder
        Auswahlfrage gibt
        {field.applies_to_child
          ? "."
          : " (oder dieses Feld pro Kind angezeigt wird)."}
      </p>
    );
  }

  return (
    <div className="rounded-xl border border-gray-200 bg-white p-3">
      <FormChoice
        checked={condition !== null}
        onChange={(checked) =>
          setCondition(
            checked
              ? defaultConditionForSource(sources[0]!, controllers)
              : null,
          )
        }
        label="Nur unter einer Bedingung anzeigen"
        hint="Das Feld erscheint nur, wenn die Bedingung erfüllt ist."
        disabled={disabled || !canEnable}
      />

      {condition ? (
        <div className="mt-3 space-y-2">
          <label className="block">
            <span className="text-xs font-medium text-gray-700">
              Sichtbar wenn
            </span>
            <select
              value={condition.source}
              onChange={(event) =>
                setCondition(
                  defaultConditionForSource(
                    event.target.value as ConditionSource,
                    controllers,
                  ),
                )
              }
              disabled={disabled}
              className={conditionInputClass}
            >
              {sources.map((source) => (
                <option key={source} value={source}>
                  {conditionSourceLabels[source]}
                </option>
              ))}
            </select>
          </label>

          {condition.source === "field" ? (
            <ConditionFieldControls
              condition={condition}
              controllers={controllers}
              onPatch={patchCondition}
              onReplace={setCondition}
              disabled={disabled}
            />
          ) : condition.source === "grade_level" ? (
            <ConditionGradeControls
              condition={condition}
              onPatch={patchCondition}
              disabled={disabled}
            />
          ) : (
            <ConditionOfferingControls
              condition={condition}
              onPatch={patchCondition}
              disabled={disabled}
            />
          )}
        </div>
      ) : null}
    </div>
  );
}

function ConditionFieldControls({
  condition,
  controllers,
  onPatch,
  onReplace,
  disabled,
}: Readonly<{
  condition: VisibilityCondition;
  controllers: FormField[];
  onPatch: (patch: Partial<VisibilityCondition>) => void;
  onReplace: (next: VisibilityCondition) => void;
  disabled: boolean;
}>) {
  const controller = controllers.find((c) => c.key === condition.field);

  const changeController = (key: string) => {
    const next = controllers.find((c) => c.key === key);
    if (!next) return;
    const base = defaultConditionForController(next);
    onReplace(
      condition.operator === "not_empty"
        ? { ...base, operator: "not_empty", value: undefined }
        : base,
    );
  };

  const changeOperator = (operator: ConditionOperator) => {
    if (operator === "not_empty") {
      onPatch({ operator, value: undefined });
      return;
    }
    const fallback = controller
      ? defaultConditionForController(controller).value
      : "";
    onPatch({ operator, value: condition.value ?? fallback });
  };

  return (
    <>
      <label className="block">
        <span className="text-xs font-medium text-gray-700">Frage</span>
        <select
          value={condition.field ?? ""}
          onChange={(event) => changeController(event.target.value)}
          disabled={disabled}
          className={conditionInputClass}
        >
          {controllers.map((c) => (
            <option key={c.key} value={c.key}>
              {c.label.trim() || c.key}
            </option>
          ))}
        </select>
      </label>

      <label className="block">
        <span className="text-xs font-medium text-gray-700">Vergleich</span>
        <select
          value={condition.operator}
          onChange={(event) =>
            changeOperator(event.target.value as ConditionOperator)
          }
          disabled={disabled}
          className={conditionInputClass}
        >
          {FIELD_CONDITION_OPERATORS.map((op) => (
            <option key={op} value={op}>
              {conditionOperatorLabels[op]}
            </option>
          ))}
        </select>
      </label>

      {condition.operator !== "not_empty" && controller ? (
        <label className="block">
          <span className="text-xs font-medium text-gray-700">Wert</span>
          {controller.type === "boolean" ? (
            <select
              value={condition.value === true ? "true" : "false"}
              onChange={(event) =>
                onPatch({ value: event.target.value === "true" })
              }
              disabled={disabled}
              className={conditionInputClass}
            >
              <option value="true">Ja</option>
              <option value="false">Nein</option>
            </select>
          ) : (
            <select
              value={String(condition.value ?? "")}
              onChange={(event) => onPatch({ value: event.target.value })}
              disabled={disabled}
              className={conditionInputClass}
            >
              {(controller.options ?? []).map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </select>
          )}
        </label>
      ) : null}
    </>
  );
}

function ConditionGradeControls({
  condition,
  onPatch,
  disabled,
}: Readonly<{
  condition: VisibilityCondition;
  onPatch: (patch: Partial<VisibilityCondition>) => void;
  disabled: boolean;
}>) {
  return (
    <>
      <label className="block">
        <span className="text-xs font-medium text-gray-700">Vergleich</span>
        <select
          value={condition.operator}
          onChange={(event) =>
            onPatch({ operator: event.target.value as ConditionOperator })
          }
          disabled={disabled}
          className={conditionInputClass}
        >
          <option value="eq">{conditionOperatorLabels.eq}</option>
          <option value="neq">{conditionOperatorLabels.neq}</option>
        </select>
      </label>
      <label className="block">
        <span className="text-xs font-medium text-gray-700">Klassenstufe</span>
        <input
          type="number"
          min={1}
          value={String(condition.value ?? "")}
          onChange={(event) =>
            onPatch({
              value:
                event.target.value === "" ? "" : Number(event.target.value),
            })
          }
          disabled={disabled}
          className={conditionInputClass}
        />
      </label>
    </>
  );
}

function ConditionOfferingControls({
  condition,
  onPatch,
  disabled,
}: Readonly<{
  condition: VisibilityCondition;
  onPatch: (patch: Partial<VisibilityCondition>) => void;
  disabled: boolean;
}>) {
  return (
    <label className="block">
      <span className="text-xs font-medium text-gray-700">
        Angebotsname (enthält)
      </span>
      <input
        type="text"
        value={String(condition.value ?? "")}
        onChange={(event) => onPatch({ value: event.target.value })}
        placeholder="z. B. Mittagessen"
        disabled={disabled}
        className={conditionInputClass}
      />
      <span className="mt-1 block text-[11px] leading-4 text-gray-500">
        Sichtbar, wenn ein gewähltes Betreuungsangebot diesen Namen trägt.
      </span>
    </label>
  );
}

function FormPreview({
  fields,
  coreRequirements,
  templateName,
  isActive,
  isSaved,
  previewHref,
  onPreviewClick,
  assignedPhaseCount = 0,
  sticky = true,
}: Readonly<{
  fields: FormField[];
  coreRequirements: CoreRequirements;
  templateName: string;
  isActive: boolean;
  isSaved: boolean;
  previewHref?: string;
  onPreviewClick?: () => void;
  assignedPhaseCount?: number;
  sticky?: boolean;
}>) {
  const previewStatus = getPreviewStatus({
    assignedPhaseCount,
    isActive,
    isSaved,
  });
  const guardianFields = [
    "Vorname *",
    "Nachname *",
    "E-Mail *",
    coreRequirements.guardian_phone ? "Telefonnummer *" : "Telefonnummer",
  ];

  return (
    <div className={sticky ? "sticky top-6 space-y-4" : "space-y-4"}>
      <div className="flex items-start justify-between gap-4">
        <div>
          <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
            Vorschau
          </p>
          <div className="mt-1 flex flex-wrap items-center gap-2">
            <h2 className="text-base font-semibold text-gray-900">
              Elternformular
            </h2>
            <span
              className={`inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-medium ${previewStatus.className}`}
            >
              <span
                className={`h-1.5 w-1.5 rounded-full ${previewStatus.dotClassName}`}
                aria-hidden="true"
              />
              {previewStatus.label}
            </span>
          </div>
          <p className="mt-1 max-w-sm text-xs leading-5 text-gray-500">
            {previewStatus.hint}
          </p>
        </div>
        <span className="inline-flex h-9 w-9 items-center justify-center rounded-xl bg-white text-gray-600 shadow-sm">
          <Eye className="h-4 w-4" aria-hidden="true" />
        </span>
      </div>

      {previewHref ? (
        <a
          href={previewHref}
          target="_blank"
          rel="noreferrer"
          className="moto-content-surface flex items-start gap-3 rounded-2xl border p-3 text-left shadow-sm transition-colors hover:border-gray-300 hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        >
          <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-white text-gray-600 shadow-sm">
            <ExternalLink className="h-4 w-4" aria-hidden="true" />
          </span>
          <span className="min-w-0">
            <span className="block text-sm font-semibold text-gray-900">
              Vorschau öffnen
            </span>
            <span className="mt-0.5 block text-xs leading-5 text-gray-500">
              Öffnet diese Formularvorlage in einem neuen Tab.
            </span>
          </span>
        </a>
      ) : onPreviewClick ? (
        <button
          type="button"
          onClick={onPreviewClick}
          className="moto-content-surface flex w-full items-start gap-3 rounded-2xl border p-3 text-left shadow-sm transition-colors hover:border-gray-300 hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        >
          <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-white text-gray-600 shadow-sm">
            <ExternalLink className="h-4 w-4" aria-hidden="true" />
          </span>
          <span className="min-w-0">
            <span className="block text-sm font-semibold text-gray-900">
              Vorschau öffnen
            </span>
            <span className="mt-0.5 block text-xs leading-5 text-gray-500">
              Speichert die Vorlage und öffnet die Vorschau in einem neuen Tab.
            </span>
          </span>
        </button>
      ) : (
        <div className="moto-content-surface flex items-start gap-3 rounded-2xl border p-3 text-left opacity-70 shadow-sm">
          <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-white text-gray-600 shadow-sm">
            <ExternalLink className="h-4 w-4" aria-hidden="true" />
          </span>
          <span className="min-w-0">
            <span className="block text-sm font-semibold text-gray-900">
              Vorschau öffnen
            </span>
            <span className="mt-0.5 block text-xs leading-5 text-gray-500">
              Speichere die Vorlage zuerst.
            </span>
          </span>
        </div>
      )}

      <div className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
        <div className="border-b border-gray-100 px-4 py-4">
          <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
            Online-Anmeldung
          </p>
          <h3 className="mt-1 text-lg font-semibold text-gray-900">
            {templateName.trim() || "Basisformular"}
          </h3>
          <p className="mt-1 text-sm leading-6 text-gray-500">
            Bitte füllen Sie das Formular vollständig aus.
          </p>
        </div>

        <div className="space-y-5 p-4">
          <PreviewSection title="Elternteil" fields={guardianFields} />
          <PreviewSection
            title="Kind"
            fields={[
              "Vorname *",
              "Nachname *",
              "Geburtsdatum *",
              "Klassenstufe *",
            ]}
          />
          <PreviewSection
            title="Betreuungsangebot"
            fields={["Gewünschtes Angebot", "Betreuungstage"]}
          />

          <section>
            <div className="mb-3 flex items-center justify-between gap-3">
              <h4 className="text-sm font-semibold text-gray-900">
                Zusatzfragen
              </h4>
              <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600">
                {fields.length}
              </span>
            </div>
            {fields.length === 0 ? (
              <div className="rounded-xl border border-dashed border-gray-200 bg-gray-50/80 px-4 py-5 text-center">
                <HelpCircle className="mx-auto h-5 w-5 text-gray-400" />
                <p className="mt-2 text-sm font-medium text-gray-700">
                  Keine Zusatzfragen
                </p>
                <p className="mt-1 text-xs leading-5 text-gray-500">
                  Eltern sehen nur das Basisformular mit den gewählten
                  Pflichtangaben.
                </p>
              </div>
            ) : (
              <div className="space-y-3">
                {fields.map((field, index) => (
                  <PreviewCustomField
                    key={`${field.key || "preview"}-${index}`}
                    field={field}
                  />
                ))}
              </div>
            )}
          </section>

          <button
            type="button"
            className="h-9 w-full rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm"
          >
            Anmeldung absenden
          </button>
        </div>
      </div>
    </div>
  );
}

function PreviewSection({
  title,
  fields,
}: Readonly<{ title: string; fields: string[] }>) {
  return (
    <section>
      <h4 className="text-sm font-semibold text-gray-900">{title}</h4>
      <div className="mt-3 grid gap-2">
        {fields.map((field) => (
          <div
            key={field}
            className="rounded-lg border border-gray-200 bg-white px-3 py-2"
          >
            <span className="block text-[11px] font-medium text-gray-500">
              {field}
            </span>
            <span className="mt-1 block h-2 w-2/3 rounded-full bg-gray-100" />
          </div>
        ))}
      </div>
    </section>
  );
}

function ConditionalBadge({ field }: Readonly<{ field: FormField }>) {
  if (!field.visible_when) return null;
  return (
    <span className="inline-flex items-center gap-1 rounded-full bg-[#5080D8]/10 px-2 py-0.5 text-[10px] font-medium text-[#3D63B0]">
      bedingt
    </span>
  );
}

function PreviewCustomField({ field }: Readonly<{ field: FormField }>) {
  if (field.type === "information") {
    return (
      <div className="rounded-lg border border-[#5080D8]/20 bg-[#5080D8]/5 px-3 py-2">
        <div className="flex items-start justify-between gap-3">
          {field.label.trim() ? (
            <span className="text-sm font-semibold text-gray-900">
              {field.label}
            </span>
          ) : (
            <span className="text-xs font-medium text-[#3D63B0]">Infotext</span>
          )}
          <ConditionalBadge field={field} />
        </div>
        {field.content?.trim() ? (
          <p className="mt-1 text-xs leading-5 whitespace-pre-line text-gray-600">
            {field.content}
          </p>
        ) : (
          <p className="mt-1 text-xs leading-5 text-gray-400 italic">
            Noch kein Text hinterlegt.
          </p>
        )}
      </div>
    );
  }

  return (
    <div className="rounded-lg border border-gray-200 bg-white px-3 py-2">
      <div className="flex items-start justify-between gap-3">
        <span className="text-sm font-medium text-gray-900">
          {field.label.trim() || "Neue Zusatzfrage"}
        </span>
        <div className="flex shrink-0 items-center gap-1.5">
          <ConditionalBadge field={field} />
          {field.required ? (
            <span className="rounded-full bg-gray-100 px-2 py-0.5 text-[10px] font-medium text-gray-600">
              Pflicht
            </span>
          ) : null}
        </div>
      </div>
      {field.help_text ? (
        <p className="mt-1 text-xs leading-5 text-gray-500">
          {field.help_text}
        </p>
      ) : null}
      <PreviewInput field={field} />
    </div>
  );
}

function PreviewInput({ field }: Readonly<{ field: FormField }>) {
  if (field.type === "boolean") {
    return (
      <div className="mt-3 grid grid-cols-2 gap-2">
        <span className="flex h-9 items-center justify-center rounded-lg border border-gray-200 bg-gray-50 text-xs font-medium text-gray-500">
          Ja
        </span>
        <span className="flex h-9 items-center justify-center rounded-lg border border-gray-200 bg-gray-50 text-xs font-medium text-gray-500">
          Nein
        </span>
      </div>
    );
  }

  if (field.type === "select") {
    const firstOption = field.options?.[0]?.label ?? "Bitte wählen";
    return (
      <div className="mt-3 flex h-10 items-center justify-between rounded-lg border border-gray-200 bg-white px-3 text-sm text-gray-500 shadow-sm">
        <span className="min-w-0 truncate">{firstOption}</span>
        <ChevronDown
          className="h-4 w-4 shrink-0 text-gray-400"
          aria-hidden="true"
        />
      </div>
    );
  }

  if (field.type === "textarea") {
    return (
      <div className="mt-3 h-16 rounded-lg border border-gray-200 bg-gray-50" />
    );
  }

  return (
    <div className="mt-3 h-9 rounded-lg border border-gray-200 bg-gray-50" />
  );
}

function getPreviewStatus({
  assignedPhaseCount,
  isActive,
  isSaved,
}: Readonly<{
  assignedPhaseCount: number;
  isActive: boolean;
  isSaved: boolean;
}>) {
  if (assignedPhaseCount > 0) {
    return {
      label: "In Phase verwendet",
      hint: `Diese Vorlage ist in ${assignedPhaseCount} Anmeldephase ausgewählt.`,
      className: "bg-[#83CD2D]/10 text-[#5F9F20]",
      dotClassName: "bg-[#83CD2D]",
    };
  }

  if (isActive) {
    return {
      label: "Bereit zur Zuordnung",
      hint: "Eltern sehen diese Vorlage erst, wenn sie einer Anmeldephase zugeordnet ist.",
      className: "bg-[#83CD2D]/10 text-[#5F9F20]",
      dotClassName: "bg-[#83CD2D]",
    };
  }

  if (isSaved) {
    return {
      label: "Gespeichert",
      hint: "Ordne die Vorlage in einer Anmeldephase zu, damit Eltern sie verwenden.",
      className: "bg-gray-100 text-gray-600",
      dotClassName: "bg-gray-300",
    };
  }

  return {
    label: "Entwurf",
    hint: "Speichere die Vorlage zuerst. Danach kannst du sie in einer Anmeldephase auswählen.",
    className: "bg-gray-100 text-gray-600",
    dotClassName: "bg-gray-300",
  };
}

// Snake-case-ify a free-form label into a schema-safe field key.
//
// Implementation is character-loop based on purpose: SonarCloud
// (rule S5852) flags `+` / `*` quantifiers on character classes as
// potentially super-linear under backtracking. The patterns here
// aren't actually catastrophic (single char classes, no overlapping
// alternation), but the loop has the same O(n) cost and avoids the
// scanner warning. Inputs are also capped \u2014 admin labels never
// legitimately exceed a few hundred chars.
function normalizeFieldKey(value: string): string {
  const MAX_INPUT = 256;
  const source = value
    .slice(0, MAX_INPUT)
    .trim()
    .toLowerCase()
    .normalize("NFD");

  let out = "";
  let pendingUnderscore = false;
  for (let i = 0; i < source.length; i++) {
    const code = source.charCodeAt(i);
    // Combining diacritical marks introduced by NFD \u2014 skip.
    if (code >= 0x0300 && code <= 0x036f) continue;
    const isLower = code >= 0x61 && code <= 0x7a; // a-z
    const isDigit = code >= 0x30 && code <= 0x39; // 0-9
    if (isLower || isDigit) {
      if (pendingUnderscore && out.length > 0) {
        out += "_";
      }
      out += source[i];
      pendingUnderscore = false;
    } else {
      // Any other character \u2192 collapse into a single underscore
      // (deferred so we don't leave a trailing one).
      pendingUnderscore = true;
    }
  }
  return out;
}

function createTargetField(
  target: Exclude<FormFieldTarget, "">,
  sortOrder: number,
  existing?: FormField,
): FormField {
  const spec = RESERVED_TARGETS[target];
  return {
    key: target.replace(/\./g, "_"),
    label: spec.label,
    type: spec.type,
    required: Boolean(existing?.required),
    help_text: "",
    options: getTargetOptions(target),
    sort_order: sortOrder,
    applies_to_child: spec.appliesToChild,
    target,
  };
}

function getTargetOptions(
  _target: Exclude<FormFieldTarget, "">,
): FormField["options"] {
  // No reserved target uses static select options anymore. Abholregelung is
  // a weekday_boolean (the parent picks the pickup weekdays, just like the
  // Buskind field); structured types carry no options.
  return [];
}

function prepareFieldsForSave(fields: FormField[]): FormField[] {
  return fields.map((field, index) => {
    if (field.target) {
      // Suggested field: type/options/scope are fixed by the spec, but
      // the admin-edited label + help text + required flag + visibility
      // ride along.
      const base = createTargetField(field.target, index, field);
      return {
        ...base,
        label: field.label.trim() || base.label,
        help_text: field.help_text?.trim() ?? "",
        required: Boolean(field.required),
        visible_when: field.visible_when ?? undefined,
      };
    }
    if (field.type === "information") {
      return {
        key:
          field.key.trim() ||
          normalizeFieldKey(field.label) ||
          `infotext_${index + 1}`,
        label: field.label.trim(),
        type: "information",
        content: field.content?.trim() ?? "",
        sort_order: index,
        applies_to_child: Boolean(field.applies_to_child),
        visible_when: field.visible_when ?? undefined,
      };
    }
    return {
      ...field,
      key: field.key.trim() || normalizeFieldKey(field.label),
      label: field.label.trim(),
      help_text: field.help_text?.trim() ?? "",
      sort_order: index,
      visible_when: field.visible_when ?? undefined,
    };
  });
}

function coreRequirementsSignature(value: CoreRequirements): string {
  const enabled = Object.entries(value)
    .filter(([, required]) => required)
    .map(([key]) => key)
    .sort((a, b) => a.localeCompare(b));
  return JSON.stringify(Object.fromEntries(enabled.map((key) => [key, true])));
}

function getRequiredHint(field: FormField): string {
  if (field.type === "boolean") {
    return "Eltern müssen Ja oder Nein auswählen.";
  }
  if (field.type === "weekday_schedule") {
    return "Eltern müssen mindestens eine Uhrzeit angeben.";
  }
  if (field.type === "weekday_boolean") {
    return "Eltern müssen mindestens einen Wochentag auswählen.";
  }
  if (field.type === "contact_list") {
    return "Eltern müssen mindestens einen vollständigen Kontakt angeben.";
  }
  if (field.type === "phone_list") {
    return "Eltern müssen mindestens eine Telefonnummer angeben.";
  }
  return "Eltern müssen diese Frage beantworten.";
}

function getSchemaDraftValidationMessage({
  fields,
  isCreating,
  name,
}: Readonly<{
  fields: FormField[];
  isCreating: boolean;
  name: string;
}>): string | null {
  if (isCreating && name.trim() === "") {
    return "Bitte gib zuerst einen Namen für die Vorlage ein.";
  }

  const seenKeys = new Set<string>();
  for (const [index, field] of fields.entries()) {
    const position = index + 1;

    if (field.type === "information") {
      if ((field.content ?? "").trim() === "") {
        return `Bitte gib für den Infotext ${position} einen Text ein.`;
      }
      const infoKey =
        field.key.trim() ||
        normalizeFieldKey(field.label) ||
        `infotext_${position}`;
      if (seenKeys.has(infoKey)) {
        return `Bitte ändere Infotext ${position}. Es entsteht ein doppeltes internes Feld.`;
      }
      seenKeys.add(infoKey);
      continue;
    }

    if (field.label.trim() === "") {
      return `Bitte gib für Frage ${position} einen Fragetext ein.`;
    }

    const key = field.key.trim() || normalizeFieldKey(field.label);
    if (key === "") {
      return `Bitte ändere Frage ${position}. Aus dem Fragetext konnte kein internes Feld erzeugt werden.`;
    }
    if (seenKeys.has(key)) {
      return `Bitte ändere Frage ${position}. Zwei Zusatzfragen haben denselben oder einen zu ähnlichen Fragetext.`;
    }
    seenKeys.add(key);

    if (!field.target && structuredFieldTypes.has(field.type)) {
      return `Bitte wähle für Frage ${position} einen einfachen Typ. Telefonlisten, Wochenzeiten und Kontaktlisten sind nur als feste Vorschläge verfügbar.`;
    }

    if (field.type === "select" && (field.options ?? []).length === 0) {
      return `Bitte ergänze für Frage ${position} mindestens eine Auswahloption.`;
    }
  }

  return null;
}

function formatSchemaDate(value: string): string {
  return new Date(value).toLocaleDateString("de-DE", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}
