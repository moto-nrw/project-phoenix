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
  ListPlus,
  Lock,
  MoreVertical,
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
};

const NEW_SCHEMA_VALUE = "__new__";
type EditorMode = "overview" | "builder" | "detail";

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
  const [phases, setPhases] = useState<Phase[]>([]);
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
    setError(null);
    setMode(nextMode);
  };

  const editSchema = (schema: FormSchema) => selectSchema(schema, "builder");

  const previewSchema = (schema: FormSchema) => selectSchema(schema, "detail");

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
      const refreshed = await loadAll();
      const stillThere = refreshed.find((s) => s.id === result.id);
      selectSchema(stillThere ?? result, "detail");
      toast.success(
        isCreating ? "Formularvorlage erstellt." : "Änderungen gespeichert.",
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
        phases={phases}
        onCreate={startNew}
        onEdit={editSchema}
        onPreview={previewSchema}
        error={error}
      />
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
                      key={`custom-field-${index}`}
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
                      : "Änderungen speichern"}
                </button>
              </div>
            </section>
          </div>

          <aside className="moto-dotted-background moto-dotted-background--split border-t border-gray-100 p-5 sm:p-6 lg:border-t-0 lg:border-l">
            <FormPreview
              fields={fields}
              templateName={name}
              isActive={currentSchema?.is_active ?? false}
              isSaved={currentSchema !== null}
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
    </div>
  );
}

function EnrollmentFormsOverview({
  templates,
  phases,
  onCreate,
  onEdit,
  onPreview,
  error,
}: Readonly<{
  templates: FormSchema[];
  phases: Phase[];
  onCreate: () => void;
  onEdit: (schema: FormSchema) => void;
  onPreview: (schema: FormSchema) => void;
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
                    eigene Vorlage mit Zusatzfragen.
                  </p>
                </div>
              </div>

              <TemplateOverviewList
                templates={templates}
                phases={phases}
                onEdit={onEdit}
                onPreview={onPreview}
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
}: Readonly<{
  templates: FormSchema[];
  phases: Phase[];
  onEdit: (schema: FormSchema) => void;
  onPreview: (schema: FormSchema) => void;
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
          <span>Keine Zusatzfragen</span>
        </div>
      </div>
      <div className="flex justify-start gap-2 md:justify-end">
        <a
          href="/enroll"
          target="_blank"
          rel="noreferrer"
          className="inline-flex h-9 items-center justify-center gap-2 rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        >
          <ExternalLink className="h-4 w-4" aria-hidden="true" />
          Elternansicht
        </a>
      </div>
    </article>
  );
}

function TemplateOverviewRow({
  schema,
  onEdit,
  onPreview,
  isAssigned,
}: Readonly<{
  schema: FormSchema;
  onEdit: () => void;
  onPreview: () => void;
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
          Eigene Vorlage für zusätzliche Fragen.
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
              label: "Bearbeiten",
              icon: <Pencil className="h-4 w-4" aria-hidden />,
              onClick: onEdit,
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
          <span className="h-4 w-4 text-gray-500">{item.icon}</span>
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
            title="Zusatzfragen nur bei Bedarf"
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
      <button
        type="button"
        onClick={onBack}
        className="inline-flex h-9 items-center justify-center gap-2 rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
      >
        <ArrowLeft className="h-4 w-4" aria-hidden="true" />
        Zurück zur Übersicht
      </button>

      <section className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm backdrop-blur-md">
        <div className="grid gap-0 lg:grid-cols-[minmax(0,1fr)_360px] xl:grid-cols-[minmax(0,1fr)_400px]">
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
                  Prüfe die Elternansicht und ordne diese Vorlage einer
                  Anmeldephase zu, wenn Eltern die Zusatzfragen sehen sollen.
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
                label="Pflichtfragen"
              />
              <FormMetric
                icon={<FileText className="h-4 w-4" aria-hidden="true" />}
                value={childFieldCount.toString()}
                label="Pro Kind"
              />
            </div>

            <FormPreview
              fields={schema.fields}
              templateName={schema.name}
              isActive={schema.is_active}
              isSaved
              assignedPhaseCount={assignedPhases.length}
              sticky={false}
            />
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
                href="/enrollment-phases"
                className="inline-flex h-10 w-full items-center justify-center gap-2 rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
              >
                <CalendarClock className="h-4 w-4" aria-hidden="true" />
                Anmeldephasen öffnen
              </Link>

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
                    Vorschau öffnen
                  </span>
                  <span className="mt-0.5 block text-xs leading-5 text-gray-500">
                    Öffnet die Elternansicht in einem neuen Tab.
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
                    title="Phase auswählen"
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
                value={optionsDraft}
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
  isActive,
  isSaved,
  assignedPhaseCount = 0,
  sticky = true,
}: Readonly<{
  fields: FormField[];
  templateName: string;
  isActive: boolean;
  isSaved: boolean;
  assignedPhaseCount?: number;
  sticky?: boolean;
}>) {
  const previewStatus = getPreviewStatus({
    assignedPhaseCount,
    isActive,
    isSaved,
  });

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
            Vorschau öffnen
          </span>
          <span className="mt-0.5 block text-xs leading-5 text-gray-500">
            Öffnet die Elternansicht in einem neuen Tab.
          </span>
        </span>
      </a>

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
