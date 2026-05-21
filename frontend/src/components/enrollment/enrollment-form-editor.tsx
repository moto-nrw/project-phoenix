"use client";

import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import {
  ArrowLeft,
  CalendarClock,
  Check,
  Eye,
  ExternalLink,
  FileText,
  GripVertical,
  HelpCircle,
  ListPlus,
  Lock,
  Pencil,
  Plus,
  ShieldCheck,
  Trash2,
} from "lucide-react";
import { useToast } from "~/contexts/ToastContext";
import {
  blankField,
  createSchema,
  listSchemas,
  updateSchema,
  type FormField,
  type FormFieldType,
  type FormSchema,
} from "~/lib/enrollment-form-schema-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "EnrollmentFormEditor" });

const fieldTypeLabels: Record<FormFieldType, string> = {
  boolean: "Ja/Nein",
  number: "Zahl",
  text: "Text",
  textarea: "Mehrzeiliger Text",
  date: "Datum",
  select: "Auswahl",
};

const NEW_SCHEMA_VALUE = "__new__";
type EditorMode = "overview" | "builder";

interface CoreField {
  readonly key: string;
  readonly label: string;
  readonly type: FormFieldType;
  readonly required: boolean;
  readonly appliesToChild: boolean;
}

const CORE_FIELDS: ReadonlyArray<CoreField> = [
  {
    key: "guardian_first_name",
    label: "Vorname (Elternteil)",
    type: "text",
    required: true,
    appliesToChild: false,
  },
  {
    key: "guardian_last_name",
    label: "Nachname (Elternteil)",
    type: "text",
    required: true,
    appliesToChild: false,
  },
  {
    key: "guardian_email",
    label: "E-Mail (Elternteil)",
    type: "text",
    required: true,
    appliesToChild: false,
  },
  {
    key: "guardian_phone",
    label: "Telefonnummer (Elternteil)",
    type: "text",
    required: false,
    appliesToChild: false,
  },
  {
    key: "first_name",
    label: "Vorname (Kind)",
    type: "text",
    required: true,
    appliesToChild: true,
  },
  {
    key: "last_name",
    label: "Nachname (Kind)",
    type: "text",
    required: true,
    appliesToChild: true,
  },
  {
    key: "date_of_birth",
    label: "Geburtsdatum (Kind)",
    type: "date",
    required: true,
    appliesToChild: true,
  },
  {
    key: "target_grade_level",
    label: "Klassenstufe (Kind)",
    type: "number",
    required: true,
    appliesToChild: true,
  },
];

export function EnrollmentFormEditor() {
  const toast = useToast();
  const [allSchemas, setAllSchemas] = useState<FormSchema[]>([]);
  const [selectedKey, setSelectedKey] = useState<string>(NEW_SCHEMA_VALUE);
  const [name, setName] = useState("");
  const [fields, setFields] = useState<FormField[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [mode, setMode] = useState<EditorMode>("overview");

  const latestByName = useMemo<FormSchema[]>(() => {
    const seen = new Map<string, FormSchema>();
    for (const s of allSchemas) {
      const prior = seen.get(s.name);
      if (!prior || s.version > prior.version) {
        seen.set(s.name, s);
      }
    }
    return Array.from(seen.values()).sort((a, b) =>
      a.name.localeCompare(b.name, "de"),
    );
  }, [allSchemas]);

  const loadAll = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const list = await listSchemas();
      setAllSchemas(list);
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

  const applySchema = (schema: FormSchema) => {
    setSelectedKey(schema.id);
    setName(schema.name);
    setFields(schema.fields);
    setError(null);
    setMode("builder");
  };

  const startNew = () => {
    setSelectedKey(NEW_SCHEMA_VALUE);
    setName("");
    setFields([]);
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

  const addField = () => {
    setFields((prev) => [...prev, blankField(prev.length)]);
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

  const handleSave = async () => {
    setSaving(true);
    setError(null);
    try {
      let result: FormSchema;
      if (isCreating) {
        if (name.trim() === "") {
          setError("Bitte einen Namen für das Formular vergeben.");
          setSaving(false);
          return;
        }
        result = await createSchema(name.trim(), fields);
      } else {
        result = await updateSchema(selectedKey, fields);
      }
      // Refresh list so the overview shows the new row, then jump to it.
      const refreshed = await loadAll();
      const stillThere = refreshed.find((s) => s.id === result.id);
      if (stillThere) applySchema(stillThere);
      else applySchema(result);
      toast.success(
        isCreating
          ? "Formularvorlage erstellt."
          : "Neue Formularversion gespeichert.",
      );
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Speichern fehlgeschlagen";
      logger.error("schema_save_failed", { error: message });
      setError(message);
      toast.error(message);
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return <p className="text-sm text-gray-500">Wird geladen...</p>;
  }

  const currentSchema = isCreating
    ? null
    : (latestByName.find((s) => s.id === selectedKey) ?? null);

  if (mode === "overview") {
    return (
      <EnrollmentFormsOverview
        templates={latestByName}
        onCreate={startNew}
        onEdit={applySchema}
        error={error}
      />
    );
  }

  return (
    <div className="space-y-5">
      <button
        type="button"
        onClick={backToOverview}
        disabled={saving}
        className="inline-flex h-9 items-center justify-center gap-2 rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
      >
        <ArrowLeft className="h-4 w-4" aria-hidden="true" />
        Zurück zur Übersicht
      </button>
      <section className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm backdrop-blur-md">
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

            <CoreFieldsSection />

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
                    Lege nur Fragen an, die du wirklich für diese Anmeldung
                    brauchst. Das Basisformular bleibt immer enthalten.
                  </p>
                </div>
                <button
                  type="button"
                  onClick={addField}
                  disabled={saving}
                  className="inline-flex h-9 items-center justify-center gap-2 rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <Plus className="h-4 w-4" aria-hidden="true" />
                  Zusatzfrage
                </button>
              </div>

              {error ? (
                <div className="rounded-lg border border-[#FF3130]/20 bg-[#FF3130]/10 p-3 text-sm text-[#CC2626]">
                  {error}
                </div>
              ) : null}

              {fields.length === 0 ? (
                <EmptyCustomFields onAdd={addField} disabled={saving} />
              ) : (
                <div className="space-y-3">
                  {fields.map((field, index) => (
                    <FieldEditorRow
                      key={`${field.key || "field"}-${index}`}
                      field={field}
                      index={index}
                      total={fields.length}
                      onChange={(patch) => updateField(index, patch)}
                      onRemove={() => removeField(index)}
                      onMoveUp={() => moveField(index, -1)}
                      onMoveDown={() => moveField(index, 1)}
                      disabled={saving}
                    />
                  ))}
                </div>
              )}

              <div className="flex flex-wrap items-center justify-end gap-2 border-t border-gray-100 pt-4">
                <button
                  type="button"
                  onClick={startNew}
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
                      : "Neue Version speichern"}
                </button>
              </div>
            </section>
          </div>

          <aside className="moto-dotted-background moto-dotted-background--split border-t border-gray-100 p-5 sm:p-6 lg:border-t-0 lg:border-l">
            <FormPreview
              fields={fields}
              templateName={name}
              currentVersion={currentSchema?.version ?? null}
              isActive={currentSchema?.is_active ?? false}
              isSaved={currentSchema !== null}
            />
          </aside>
        </div>
      </section>
    </div>
  );
}

function EnrollmentFormsOverview({
  templates,
  onCreate,
  onEdit,
  error,
}: Readonly<{
  templates: FormSchema[];
  onCreate: () => void;
  onEdit: (schema: FormSchema) => void;
  error: string | null;
}>) {
  const activeTemplates = templates.filter((schema) => schema.is_active).length;
  const totalQuestions = templates.reduce(
    (sum, schema) => sum + schema.fields.length,
    0,
  );
  const requiredQuestions = templates.reduce(
    (sum, schema) =>
      sum + schema.fields.filter((field) => Boolean(field.required)).length,
    0,
  );

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
                  du nur, wenn eine Anmeldephase zusätzliche Fragen braucht.
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

            <div className="grid grid-cols-2 gap-3 xl:grid-cols-4">
              <FormMetric
                icon={<Lock className="h-4 w-4" aria-hidden="true" />}
                value="1"
                label="Basisformular"
              />
              <FormMetric
                icon={<FileText className="h-4 w-4" aria-hidden="true" />}
                value={templates.length.toString()}
                label="Vorlagen"
              />
              <FormMetric
                icon={<ListPlus className="h-4 w-4" aria-hidden="true" />}
                value={totalQuestions.toString()}
                label="Zusatzfragen"
              />
              <FormMetric
                icon={<Check className="h-4 w-4" aria-hidden="true" />}
                value={requiredQuestions.toString()}
                label="Pflichtfragen"
              />
            </div>

            {error ? (
              <div className="rounded-lg border border-[#FF3130]/20 bg-[#FF3130]/10 p-3 text-sm text-[#CC2626]">
                {error}
              </div>
            ) : null}

            <div className="grid gap-4 xl:grid-cols-[340px_minmax(0,1fr)]">
              <BasisFormCard />
              <section className="space-y-3">
                <div className="flex items-end justify-between gap-3">
                  <div>
                    <h3 className="text-base font-semibold text-gray-900">
                      Eigene Vorlagen
                    </h3>
                    <p className="mt-1 text-sm text-gray-600">
                      Vorlagen werden später einer Anmeldephase zugeordnet.
                    </p>
                  </div>
                </div>

                {templates.length === 0 ? (
                  <EmptyTemplateState onCreate={onCreate} />
                ) : (
                  <div className="grid gap-3">
                    {templates.map((schema) => (
                      <TemplateOverviewCard
                        key={schema.id}
                        schema={schema}
                        onEdit={() => onEdit(schema)}
                      />
                    ))}
                  </div>
                )}
              </section>
            </div>
          </div>

          <aside className="moto-dotted-background moto-dotted-background--split border-t border-gray-100 p-5 sm:p-6 lg:border-t-0 lg:border-l">
            <OverviewGuide
              templateCount={templates.length}
              activeTemplates={activeTemplates}
              onCreate={onCreate}
            />
          </aside>
        </div>
      </section>
    </div>
  );
}

function BasisFormCard() {
  return (
    <article className="moto-content-surface flex h-full flex-col rounded-2xl border p-4 shadow-sm">
      <div className="flex items-start justify-between gap-3">
        <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-white text-gray-600 shadow-sm">
          <Lock className="h-4 w-4" aria-hidden="true" />
        </span>
        <span className="rounded-full bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-600">
          Immer vorhanden
        </span>
      </div>
      <div className="mt-4 flex-1">
        <h3 className="text-base font-semibold text-gray-900">Basisformular</h3>
        <p className="mt-2 text-sm leading-6 text-gray-600">
          Fragt Elternteil, Kind, Klassenstufe und gewünschtes Betreuungsangebot
          ab. Für viele Anmeldungen reicht das bereits.
        </p>
      </div>
      <div className="mt-4 flex flex-wrap gap-2 text-xs text-gray-600">
        <span className="rounded-full bg-gray-100 px-2.5 py-1 font-medium text-gray-700">
          Gesperrt
        </span>
        <span className="rounded-full bg-gray-100 px-2.5 py-1 font-medium text-gray-700">
          Systemformular
        </span>
      </div>
      <a
        href="/enroll"
        target="_blank"
        rel="noreferrer"
        className="mt-4 inline-flex h-9 items-center justify-center gap-2 rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
      >
        <ExternalLink className="h-4 w-4" aria-hidden="true" />
        Elternansicht öffnen
      </a>
    </article>
  );
}

function EmptyTemplateState({ onCreate }: Readonly<{ onCreate: () => void }>) {
  return (
    <button
      type="button"
      onClick={onCreate}
      className="group flex min-h-[210px] w-full flex-col items-center justify-center rounded-2xl border border-dashed border-gray-300 bg-white/70 px-6 py-10 text-center shadow-sm transition-colors hover:border-gray-400 hover:bg-white focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
    >
      <span className="flex h-11 w-11 items-center justify-center rounded-2xl bg-gray-100 text-gray-600 transition-colors group-hover:bg-gray-900 group-hover:text-white">
        <Plus className="h-5 w-5" aria-hidden="true" />
      </span>
      <span className="mt-3 text-sm font-semibold text-gray-900">
        Noch keine eigenen Formularvorlagen
      </span>
      <span className="mt-1 max-w-md text-sm leading-6 text-gray-500">
        Starte mit dem Basisformular oder erstelle eine Vorlage für zusätzliche
        Fragen, etwa für Ferienbetreuung oder spezielle Hinweise.
      </span>
    </button>
  );
}

function TemplateOverviewCard({
  schema,
  onEdit,
}: Readonly<{ schema: FormSchema; onEdit: () => void }>) {
  const requiredCount = schema.fields.filter((field) =>
    Boolean(field.required),
  ).length;
  const childFieldCount = schema.fields.filter((field) =>
    Boolean(field.applies_to_child),
  ).length;

  return (
    <article className="moto-content-surface rounded-2xl border p-4 shadow-sm">
      <div className="flex flex-col gap-4 md:flex-row md:flex-wrap md:items-start md:justify-between">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h4 className="text-base font-semibold text-gray-900">
              {schema.name}
            </h4>
            <span
              className={`inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-medium ${
                schema.is_active
                  ? "bg-[#83CD2D]/10 text-[#5F9F20]"
                  : "bg-gray-100 text-gray-600"
              }`}
            >
              <span
                className={`h-1.5 w-1.5 rounded-full ${
                  schema.is_active ? "bg-[#83CD2D]" : "bg-gray-300"
                }`}
                aria-hidden="true"
              />
              {schema.is_active ? "Aktiv" : "Gespeichert"}
            </span>
          </div>
          <p className="mt-2 text-sm leading-6 text-gray-600">
            Version {schema.version}, erstellt am{" "}
            {formatSchemaDate(schema.created_at)}
          </p>
          <div className="mt-3 flex flex-wrap gap-2 text-xs text-gray-600">
            <span className="rounded-full bg-gray-100 px-2.5 py-1 font-medium text-gray-700">
              {schema.fields.length} Zusatzfragen
            </span>
            <span className="rounded-full bg-gray-100 px-2.5 py-1 font-medium text-gray-700">
              {requiredCount} Pflichtfragen
            </span>
            <span className="rounded-full bg-gray-100 px-2.5 py-1 font-medium text-gray-700">
              {childFieldCount} pro Kind
            </span>
          </div>
        </div>
        <div className="flex shrink-0 flex-wrap gap-2">
          <a
            href="/enroll"
            target="_blank"
            rel="noreferrer"
            className="inline-flex h-9 items-center justify-center gap-2 rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          >
            <ExternalLink className="h-4 w-4" aria-hidden="true" />
            Vorschau
          </a>
          <button
            type="button"
            onClick={onEdit}
            className="inline-flex h-9 items-center justify-center gap-2 rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          >
            <Pencil className="h-4 w-4" aria-hidden="true" />
            Bearbeiten
          </button>
        </div>
      </div>
    </article>
  );
}

function OverviewGuide({
  templateCount,
  activeTemplates,
  onCreate,
}: Readonly<{
  templateCount: number;
  activeTemplates: number;
  onCreate: () => void;
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

      <button
        type="button"
        onClick={onCreate}
        className="inline-flex h-10 w-full items-center justify-center gap-2 rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
      >
        <Plus className="h-4 w-4" aria-hidden="true" />
        Vorlage erstellen
      </button>

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
            title="Zusatzfragen nur bei Bedarf"
            done={templateCount > 0}
          />
          <GuideStep
            icon={<CalendarClock className="h-4 w-4" aria-hidden="true" />}
            title="Vorlage in Anmeldephase wählen"
            done={activeTemplates > 0}
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

function FormBuilderIntro() {
  return (
    <header>
      <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
        Formular-Konfigurator
      </p>
      <h2 className="mt-1 text-xl font-semibold text-gray-900">
        Zusatzfragen bearbeiten
      </h2>
      <p className="mt-2 max-w-3xl text-sm leading-6 text-gray-600">
        Ergänze hier nur Fragen, die über das Basisformular hinausgehen. Die
        Auswahl der Vorlage passiert in der Übersicht, dieser Bereich ist nur
        zum Bearbeiten da.
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
              ? "Gib der Vorlage einen eindeutigen Namen. Danach kannst du Zusatzfragen anlegen."
              : "Beim Speichern entsteht eine neue Version dieser Vorlage."}
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
              Version {currentSchema?.version ?? 1}
            </span>
            <span className="rounded-full bg-gray-100 px-2.5 py-1 font-medium text-gray-700">
              {fields.length} Zusatzfragen
            </span>
            <span className="rounded-full bg-gray-100 px-2.5 py-1 font-medium text-gray-700">
              {requiredCount} Pflichtfragen
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

function CoreFieldsSection() {
  const guardian = CORE_FIELDS.filter((f) => !f.appliesToChild);
  const child = CORE_FIELDS.filter((f) => f.appliesToChild);
  return (
    <section className="rounded-2xl border border-gray-200 bg-gray-50/70 p-4">
      <div className="flex items-start gap-3">
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-white text-gray-500 shadow-sm">
          <Lock className="h-4 w-4" aria-hidden="true" />
        </span>
        <div>
          <h2 className="text-sm font-semibold text-gray-900">Basisformular</h2>
          <p className="mt-0.5 text-xs leading-5 text-gray-600">
            Diese Angaben sind immer Teil der Online-Anmeldung und werden hier
            nur als Orientierung angezeigt.
          </p>
        </div>
      </div>

      <div className="mt-4 grid gap-4 md:grid-cols-2">
        <CoreFieldGroup title="Eltern" fields={guardian} />
        <CoreFieldGroup title="Kind" fields={child} />
      </div>
    </section>
  );
}

function CoreFieldGroup({
  title,
  fields,
}: Readonly<{ title: string; fields: CoreField[] }>) {
  return (
    <div>
      <h3 className="text-xs font-semibold text-gray-700">{title}</h3>
      <ul className="mt-2 space-y-1.5">
        {fields.map((field) => (
          <CoreFieldRow key={field.key} field={field} />
        ))}
      </ul>
    </div>
  );
}

function CoreFieldRow({ field }: { readonly field: CoreField }) {
  return (
    <li className="moto-content-surface flex items-center gap-2 rounded-lg border px-3 py-2 text-xs shadow-sm">
      <span className="flex-1 font-medium text-gray-900">{field.label}</span>
      <span className="text-gray-500">{fieldTypeLabels[field.type]}</span>
      {field.required && (
        <span className="rounded-full bg-gray-100 px-2 py-0.5 text-[10px] font-medium text-gray-600">
          Pflicht
        </span>
      )}
    </li>
  );
}

function EmptyCustomFields({
  onAdd,
  disabled,
}: Readonly<{ onAdd: () => void; disabled: boolean }>) {
  return (
    <button
      type="button"
      onClick={onAdd}
      disabled={disabled}
      className="group flex w-full flex-col items-center justify-center rounded-2xl border border-dashed border-gray-300 bg-white/70 px-6 py-10 text-center shadow-sm transition-colors hover:border-gray-400 hover:bg-white focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
    >
      <span className="flex h-11 w-11 items-center justify-center rounded-2xl bg-gray-100 text-gray-600 transition-colors group-hover:bg-gray-900 group-hover:text-white">
        <Plus className="h-5 w-5" aria-hidden="true" />
      </span>
      <span className="mt-3 text-sm font-semibold text-gray-900">
        Keine Zusatzfragen
      </span>
      <span className="mt-1 max-w-md text-sm leading-6 text-gray-500">
        Das ist für viele Anmeldungen genau richtig. Füge nur Fragen hinzu, wenn
        Eltern wirklich zusätzliche Angaben machen sollen.
      </span>
    </button>
  );
}

interface FieldEditorRowProps {
  readonly field: FormField;
  readonly index: number;
  readonly total: number;
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
  onChange,
  onRemove,
  onMoveUp,
  onMoveDown,
  disabled,
}: FieldEditorRowProps) {
  const optionsText = (field.options ?? [])
    .map((option) => option.label)
    .join("\n");
  const updateOptions = (value: string) => {
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
              <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                Frage {index + 1}
              </p>
              <h3 className="mt-1 text-sm font-semibold text-gray-900">
                {field.label.trim() || "Neue Zusatzfrage"}
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
                placeholder="z. B. Allergien oder Hinweise"
                disabled={disabled}
                className="mt-1 h-10 w-full rounded-lg border border-gray-200 px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
              />
            </label>
            <label className="block">
              <span className="text-xs font-medium text-gray-700">Typ</span>
              <select
                value={field.type}
                onChange={(event) =>
                  onChange({ type: event.target.value as FormFieldType })
                }
                disabled={disabled}
                className="moto-select moto-content-surface mt-1 h-10 w-full rounded-lg border px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
              >
                {Object.entries(fieldTypeLabels).map(([value, label]) => (
                  <option key={value} value={value}>
                    {label}
                  </option>
                ))}
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
              onChange={(event) => onChange({ help_text: event.target.value })}
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
                value={optionsText}
                onChange={(event) => updateOptions(event.target.value)}
                placeholder={"Eine Option pro Zeile\nz. B. Ja\nz. B. Nein"}
                disabled={disabled}
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
              hint="Eltern müssen diese Frage beantworten."
              disabled={disabled}
            />
            <FormChoice
              checked={Boolean(field.applies_to_child)}
              onChange={(checked) => onChange({ applies_to_child: checked })}
              label="Pro Kind abfragen"
              hint="Die Frage erscheint für jedes angemeldete Kind."
              disabled={disabled}
            />
          </div>

          <details className="rounded-lg border border-gray-100 bg-gray-50/70 px-3 py-2 text-xs text-gray-500">
            <summary className="cursor-pointer font-medium text-gray-600">
              Technischer Schlüssel
            </summary>
            <label className="mt-2 block">
              <span className="sr-only">Technischer Schlüssel</span>
              <input
                type="text"
                value={field.key}
                onChange={(event) => onChange({ key: event.target.value })}
                placeholder="z. B. allergies"
                disabled={disabled}
                className="h-9 w-full rounded-lg border border-gray-200 bg-white px-3 font-mono text-xs shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
              />
            </label>
          </details>
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

function FormPreview({
  fields,
  templateName,
  currentVersion,
  isActive,
  isSaved,
}: Readonly<{
  fields: FormField[];
  templateName: string;
  currentVersion: number | null;
  isActive: boolean;
  isSaved: boolean;
}>) {
  return (
    <div className="sticky top-6 space-y-4">
      <div className="flex items-start justify-between gap-4">
        <div>
          <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
            Vorschau
          </p>
          <h2 className="mt-1 text-base font-semibold text-gray-900">
            Elternformular
          </h2>
        </div>
        <span className="inline-flex h-9 w-9 items-center justify-center rounded-xl bg-white text-gray-600 shadow-sm">
          <Eye className="h-4 w-4" aria-hidden="true" />
        </span>
      </div>

      <a
        href="/enroll"
        target="_blank"
        rel="noreferrer"
        className="moto-content-surface flex items-start gap-3 rounded-2xl border p-3 text-left shadow-sm transition-colors hover:border-gray-300 hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
      >
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-white text-gray-600 shadow-sm">
          <ExternalLink className="h-4 w-4" aria-hidden="true" />
        </span>
        <span className="min-w-0">
          <span className="block text-sm font-semibold text-gray-900">
            Echte Elternansicht öffnen
          </span>
          <span className="mt-0.5 block text-xs leading-5 text-gray-500">
            Öffnet die öffentliche Anmeldung in einem neuen Tab.
          </span>
        </span>
      </a>

      <div
        className={`moto-content-surface rounded-2xl border p-3 shadow-sm ${
          isActive
            ? "border-[#83CD2D]/30 bg-[#83CD2D]/10"
            : "border-gray-200 bg-white"
        }`}
      >
        <div className="flex items-start gap-3">
          <span
            className={`mt-1 h-2.5 w-2.5 shrink-0 rounded-full ${
              isActive ? "bg-[#83CD2D]" : "bg-gray-300"
            }`}
            aria-hidden="true"
          />
          <div>
            <p className="text-sm font-semibold text-gray-900">
              {isActive
                ? "Dieses Formular ist aktiv"
                : isSaved
                  ? "Dieses Formular ist gespeichert"
                  : "Dieses Formular ist noch nicht gespeichert"}
            </p>
            <p className="mt-1 text-xs leading-5 text-gray-500">
              {isActive
                ? "Eltern sehen diese Version, wenn sie der aktiven Anmeldephase zugeordnet ist."
                : isSaved
                  ? "Ordne die Vorlage in einer Anmeldephase zu, damit Eltern sie verwenden."
                  : "Speichere die Vorlage zuerst. Danach kannst du sie in einer Anmeldephase auswählen."}
            </p>
          </div>
        </div>
      </div>

      <div className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
        <div className="border-b border-gray-100 px-4 py-4">
          <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
            Online-Anmeldung
          </p>
          <h3 className="mt-1 text-lg font-semibold text-gray-900">
            {templateName.trim() || "Basisformular"}
          </h3>
          <p className="mt-1 text-sm text-gray-500">
            {currentVersion ? `Version ${currentVersion}` : "Neue Vorlage"}
          </p>
        </div>

        <div className="space-y-5 p-4">
          <PreviewSection
            title="Elternteil"
            fields={["Vorname", "Nachname", "E-Mail", "Telefonnummer"]}
          />
          <PreviewSection
            title="Kind"
            fields={["Vorname", "Nachname", "Geburtsdatum", "Klassenstufe"]}
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
                  Eltern sehen nur das Basisformular.
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

function PreviewCustomField({ field }: Readonly<{ field: FormField }>) {
  return (
    <div className="rounded-lg border border-gray-200 bg-white px-3 py-2">
      <div className="flex items-start justify-between gap-3">
        <span className="text-sm font-medium text-gray-900">
          {field.label.trim() || "Neue Zusatzfrage"}
        </span>
        {field.required ? (
          <span className="rounded-full bg-gray-100 px-2 py-0.5 text-[10px] font-medium text-gray-600">
            Pflicht
          </span>
        ) : null}
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
      <div className="mt-3 flex items-center gap-2 text-xs text-gray-500">
        <span className="h-4 w-4 rounded border border-gray-300 bg-white" />
        Ja
      </div>
    );
  }

  if (field.type === "select") {
    const firstOption = field.options?.[0]?.label ?? "Bitte wählen";
    return (
      <div className="mt-3 flex h-9 items-center justify-between rounded-lg border border-gray-200 bg-gray-50 px-3 text-xs text-gray-500">
        {firstOption}
        <span>⌄</span>
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

function normalizeFieldKey(value: string): string {
  return value
    .trim()
    .toLowerCase()
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^_+|_+$/g, "");
}

function formatSchemaDate(value: string): string {
  return new Date(value).toLocaleDateString("de-DE", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}
