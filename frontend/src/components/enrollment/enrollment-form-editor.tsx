"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
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

// Sentinel for the "new schema" entry in the picker. Anything that
// isn't this value is the id of an existing schema row.
const NEW_SCHEMA_VALUE = "__new__";

// CORE_FIELDS mirrors backend/models/enrollment/form_schema.go's
// CoreFieldKeys. These are always rendered on the parent form by
// dedicated columns on enrollment.requests / .request_children, so
// the schema editor must never declare them, but admins should see
// them here for a complete picture of what parents will fill in.
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

/**
 * Admin form-schema editor.
 *
 * Top-of-component picker lists every existing schema (one entry per
 * latest version of each name) plus a "+ Neues Formular" option that
 * unlocks the name input. Save dispatches:
 *   - createSchema(name, fields) when creating a fresh one
 *   - updateSchema(id, fields)   when editing an existing one (the
 *     row's name is inherited from the source on the backend, so the
 *     name input is read-only in edit mode)
 *
 * Core fields (guardian name/email/phone, child name/dob/grade) are
 * NOT shown here, they are hardcoded into the form on the parent side.
 */
export function EnrollmentFormEditor() {
  const [allSchemas, setAllSchemas] = useState<FormSchema[]>([]);
  const [selectedKey, setSelectedKey] = useState<string>(NEW_SCHEMA_VALUE);
  const [name, setName] = useState("");
  const [fields, setFields] = useState<FormField[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [savedAt, setSavedAt] = useState<string | null>(null);

  // Group by name and keep only the latest version per name for the
  // picker. Older versions are still on the backend and bound phases
  // keep their pinned schema_id, but the editor only ever writes new
  // versions on top of the latest.
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
    void (async () => {
      const list = await loadAll();
      // Auto-select the most recently created schema so reopening the
      // page lands on something meaningful. Empty list keeps "Neues
      // Formular" stays the default.
      if (list.length > 0) {
        const newest = list.reduce((a, b) =>
          new Date(a.created_at) > new Date(b.created_at) ? a : b,
        );
        applySchema(newest);
      }
    })();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [loadAll]);

  const applySchema = (schema: FormSchema) => {
    setSelectedKey(schema.id);
    setName(schema.name);
    setFields(schema.fields);
    setError(null);
    setSavedAt(null);
  };

  const startNew = () => {
    setSelectedKey(NEW_SCHEMA_VALUE);
    setName("");
    setFields([]);
    setError(null);
    setSavedAt(null);
  };

  const handlePickerChange = (value: string) => {
    if (value === NEW_SCHEMA_VALUE) {
      startNew();
      return;
    }
    const picked = latestByName.find((s) => s.id === value);
    if (picked) applySchema(picked);
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
    setSavedAt(null);
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
      // Refresh list so the picker shows the new row, then jump to it.
      const refreshed = await loadAll();
      const stillThere = refreshed.find((s) => s.id === result.id);
      if (stillThere) applySchema(stillThere);
      else applySchema(result);
      setSavedAt(new Date().toLocaleTimeString("de-DE"));
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Speichern fehlgeschlagen";
      logger.error("schema_save_failed", { error: message });
      setError(message);
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

  return (
    <div className="space-y-6">
      <section className="moto-content-surface rounded-2xl border p-4 shadow-sm backdrop-blur-md sm:p-5">
        <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
          Optional
        </p>
        <h2 className="mt-1 text-lg font-semibold text-gray-900">
          Zusatzfelder zum Basisformular
        </h2>
        <p className="mt-1 max-w-2xl text-sm text-gray-600">
          Du musst hier nichts anlegen, um eine Online-Anmeldung zu starten. Das
          Basisformular fragt Eltern, Kind, Klassenstufe, Zustimmungen und
          Betreuungsangebote bereits ab. Nutze diese Seite nur für zusätzliche
          Fragen, zum Beispiel Allergien, Abholberechtigte oder Notfallkontakt.
        </p>
      </section>

      <div className="moto-content-surface rounded-lg border p-4">
        <label className="block">
          <span className="block text-xs font-medium text-gray-600">
            Zusatzfelder-Konfiguration auswählen
          </span>
          <select
            value={selectedKey}
            onChange={(e) => handlePickerChange(e.target.value)}
            disabled={saving}
            className="moto-select mt-1 block w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm focus:border-gray-500 focus:ring-gray-500"
          >
            <option value={NEW_SCHEMA_VALUE}>
              + Neue Zusatzfelder anlegen
            </option>
            {latestByName.length > 0 && (
              <optgroup label="Bestehende Konfigurationen">
                {latestByName.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name} (v{s.version})
                  </option>
                ))}
              </optgroup>
            )}
          </select>
        </label>

        <label className="mt-3 block">
          <span className="block text-xs font-medium text-gray-600">
            Name der Konfiguration
          </span>
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="z. B. Schuljahr, Ferienbetreuung"
            disabled={saving || !isCreating}
            className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-gray-500 focus:ring-gray-500 disabled:bg-gray-100 disabled:text-gray-600"
          />
          {!isCreating && (
            <p className="mt-1 text-xs text-gray-500">
              Beim Speichern wird eine neue Version unter dem gleichen Namen
              angelegt. Bestehende Anmeldungen behalten ihre alte Version.
            </p>
          )}
        </label>

        {currentSchema && (
          <p className="mt-3 text-xs text-gray-600">
            Aktuelle Version:{" "}
            <span className="font-medium">v{currentSchema.version}</span>
            {" · "}
            {currentSchema.fields.length} benutzerdefinierte Felder{" · "}
            zuletzt gespeichert{" "}
            {new Date(currentSchema.created_at).toLocaleString("de-DE")}
          </p>
        )}
      </div>

      <CoreFieldsSection />

      <div className="border-t border-gray-200 pt-4">
        <h2 className="text-sm font-semibold text-gray-900">
          Benutzerdefinierte Felder
        </h2>
        <p className="mt-0.5 text-xs text-gray-600">
          Diese Felder erscheinen zusätzlich zum Basisformular, sobald du sie in
          einer Anmeldephase auswählst.
        </p>
      </div>

      {error && (
        <div className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-800">
          {error}
        </div>
      )}
      {savedAt && !error && (
        <div className="rounded-lg border border-green-200 bg-green-50 p-3 text-sm text-green-800">
          Gespeichert ({savedAt}).
        </div>
      )}

      {fields.length === 0 ? (
        <p className="rounded-lg border border-dashed border-gray-300 bg-gray-50 p-6 text-center text-sm text-gray-500">
          Keine Zusatzfelder. Das ist in Ordnung, wenn das Basisformular für
          deine Anmeldung reicht.
        </p>
      ) : (
        <div className="space-y-3">
          {fields.map((field, index) => (
            <FieldEditorRow
              key={index}
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

      <div className="flex flex-wrap items-center gap-3">
        <button
          type="button"
          onClick={addField}
          disabled={saving}
          className="rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50"
        >
          + Zusatzfeld hinzufügen
        </button>
        <button
          type="button"
          onClick={handleSave}
          disabled={saving}
          className="rounded-lg bg-gray-900 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {saving
            ? "Speichern..."
            : isCreating
              ? "Zusatzfelder erstellen"
              : "Neue Version speichern"}
        </button>
      </div>
    </div>
  );
}

// CoreFieldsSection renders the always-present core fields (read-only)
// so admins see the full parent form at a glance. Two groups: Eltern,
// Kind. Each row mirrors the FieldEditorRow shape so the visual
// rhythm carries over from the custom fields below.
function CoreFieldsSection() {
  const guardian = CORE_FIELDS.filter((f) => !f.appliesToChild);
  const child = CORE_FIELDS.filter((f) => f.appliesToChild);
  return (
    <div className="rounded-lg border border-gray-200 bg-gray-50 p-4">
      <h2 className="text-sm font-semibold text-gray-900">Kernfelder</h2>
      <p className="mt-0.5 text-xs text-gray-600">
        Diese Felder werden immer abgefragt und können hier nicht bearbeitet
        werden.
      </p>

      <div className="mt-3">
        <h3 className="text-xs font-medium text-gray-700">Eltern</h3>
        <ul className="mt-1.5 space-y-1.5">
          {guardian.map((f) => (
            <CoreFieldRow key={f.key} field={f} />
          ))}
        </ul>
      </div>

      <div className="mt-4">
        <h3 className="text-xs font-medium text-gray-700">Kind</h3>
        <ul className="mt-1.5 space-y-1.5">
          {child.map((f) => (
            <CoreFieldRow key={f.key} field={f} />
          ))}
        </ul>
      </div>
    </div>
  );
}

function CoreFieldRow({ field }: { readonly field: CoreField }) {
  return (
    <li className="moto-content-surface flex items-center gap-3 rounded-md border px-3 py-2 text-xs">
      <code className="rounded bg-gray-100 px-1.5 py-0.5 font-mono text-[11px] text-gray-700">
        {field.key}
      </code>
      <span className="flex-1 font-medium text-gray-900">{field.label}</span>
      <span className="text-gray-500">{fieldTypeLabels[field.type]}</span>
      {field.required && (
        <span className="rounded-full bg-[#FF3130]/10 px-2 py-0.5 text-[10px] font-medium text-[#FF3130]">
          Pflicht
        </span>
      )}
    </li>
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
  return (
    <div className="moto-content-surface rounded-lg border p-4 shadow-sm">
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <label className="block">
          <span className="block text-xs font-medium text-gray-600">
            Schlüssel (intern)
          </span>
          <input
            type="text"
            value={field.key}
            onChange={(e) => onChange({ key: e.target.value })}
            placeholder="z.B. allergies"
            disabled={disabled}
            className="mt-1 block w-full rounded-md border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-gray-500 focus:ring-gray-500"
          />
        </label>
        <label className="block">
          <span className="block text-xs font-medium text-gray-600">
            Bezeichnung (sichtbar)
          </span>
          <input
            type="text"
            value={field.label}
            onChange={(e) => onChange({ label: e.target.value })}
            placeholder="z.B. Allergien"
            disabled={disabled}
            className="mt-1 block w-full rounded-md border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-gray-500 focus:ring-gray-500"
          />
        </label>
        <label className="block">
          <span className="block text-xs font-medium text-gray-600">Typ</span>
          <select
            value={field.type}
            onChange={(e) =>
              onChange({ type: e.target.value as FormFieldType })
            }
            disabled={disabled}
            className="moto-select mt-1 block w-full rounded-md border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-gray-500 focus:ring-gray-500"
          >
            {Object.entries(fieldTypeLabels).map(([value, label]) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </select>
        </label>
      </div>

      <div className="mt-3 flex flex-wrap items-center gap-3 text-sm">
        <label className="flex items-center gap-2">
          <input
            type="checkbox"
            checked={Boolean(field.required)}
            onChange={(e) => onChange({ required: e.target.checked })}
            disabled={disabled}
          />
          <span className="text-gray-700">Pflichtfeld</span>
        </label>
        <label className="flex items-center gap-2">
          <input
            type="checkbox"
            checked={Boolean(field.applies_to_child)}
            onChange={(e) => onChange({ applies_to_child: e.target.checked })}
            disabled={disabled}
          />
          <span className="text-gray-700">Pro Kind</span>
        </label>

        <div className="ml-auto flex gap-2">
          <button
            type="button"
            onClick={onMoveUp}
            disabled={disabled || index === 0}
            className="rounded border border-gray-300 px-2 py-1 text-xs text-gray-700 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-40"
          >
            ↑
          </button>
          <button
            type="button"
            onClick={onMoveDown}
            disabled={disabled || index === total - 1}
            className="rounded border border-gray-300 px-2 py-1 text-xs text-gray-700 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-40"
          >
            ↓
          </button>
          <button
            type="button"
            onClick={onRemove}
            disabled={disabled}
            className="rounded border border-red-300 px-2 py-1 text-xs text-red-700 hover:bg-red-50 disabled:cursor-not-allowed disabled:opacity-40"
          >
            Entfernen
          </button>
        </div>
      </div>
    </div>
  );
}
