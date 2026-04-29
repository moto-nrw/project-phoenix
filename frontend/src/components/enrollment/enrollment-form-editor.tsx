"use client";

import { useEffect, useState } from "react";
import {
  blankField,
  fetchActiveSchema,
  publishSchema,
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

/**
 * Admin form-schema editor.
 *
 * Renders a table of custom fields with inline edits + add/remove
 * buttons. Save publishes a new schema version (POST /api/enrollment/
 * schema). The previous version is automatically deactivated by the
 * backend service.
 *
 * Core fields (guardian name/email/phone, child name/dob/grade) are
 * NOT shown here — they're hardcoded into the form on the parent side.
 * The "Reserved keys" hint warns admins which keys cannot be reused.
 */
export function EnrollmentFormEditor() {
  const [activeSchema, setActiveSchema] = useState<FormSchema | null>(null);
  const [fields, setFields] = useState<FormField[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [savedAt, setSavedAt] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      setLoading(true);
      setError(null);
      try {
        const schema = await fetchActiveSchema();
        if (cancelled) return;
        setActiveSchema(schema);
        setFields(schema?.fields ?? []);
      } catch (err) {
        if (cancelled) return;
        const message =
          err instanceof Error ? err.message : "Unbekannter Fehler";
        logger.error("schema_load_failed", { error: message });
        setError(message);
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    void load();
    return () => {
      cancelled = true;
    };
  }, []);

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

  const handleSave = async () => {
    setSaving(true);
    setError(null);
    setSavedAt(null);
    try {
      const result = await publishSchema(fields);
      setActiveSchema(result);
      setFields(result.fields);
      setSavedAt(new Date().toLocaleTimeString("de-DE"));
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Speichern fehlgeschlagen";
      logger.error("schema_publish_failed", { error: message });
      setError(message);
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return <p className="text-sm text-gray-500">Wird geladen...</p>;
  }

  return (
    <div className="space-y-6">
      <div className="rounded-lg border border-gray-200 bg-white p-4 text-sm text-gray-700">
        <h2 className="mb-2 text-base font-semibold text-gray-900">
          Aktuelle Version
        </h2>
        {activeSchema ? (
          <p>
            Version <span className="font-medium">{activeSchema.version}</span>{" "}
            · {activeSchema.fields.length} benutzerdefinierte Felder · zuletzt
            gespeichert{" "}
            {new Date(activeSchema.created_at).toLocaleString("de-DE")}
          </p>
        ) : (
          <p>
            Noch keine Version veröffentlicht — speichere unten zum Anlegen.
          </p>
        )}
      </div>

      <div className="rounded-lg border border-blue-100 bg-blue-50 p-3 text-xs text-blue-900">
        <strong>Reservierte Schlüssel</strong> (nicht erlaubt, da bereits
        Kernfeld): guardian_first_name, guardian_last_name, guardian_email,
        guardian_phone, first_name, last_name, date_of_birth,
        target_grade_level.
      </div>

      {error && (
        <div className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-800">
          {error}
        </div>
      )}
      {savedAt && !error && (
        <div className="rounded-lg border border-green-200 bg-green-50 p-3 text-sm text-green-800">
          Neue Version gespeichert ({savedAt}).
        </div>
      )}

      {fields.length === 0 ? (
        <p className="rounded-lg border border-dashed border-gray-300 bg-gray-50 p-6 text-center text-sm text-gray-500">
          Keine benutzerdefinierten Felder. Klicke auf "Feld hinzufügen", um zu
          beginnen.
        </p>
      ) : (
        <div className="space-y-3">
          {fields.map((field, index) => (
            <FieldEditorRow
              key={`${index}-${field.key}`}
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
          + Feld hinzufügen
        </button>
        <button
          type="button"
          onClick={handleSave}
          disabled={saving}
          className="rounded-lg bg-gray-900 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {saving ? "Speichern..." : "Neue Version veröffentlichen"}
        </button>
      </div>
    </div>
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
    <div className="rounded-lg border border-gray-200 bg-white p-4 shadow-sm">
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
            className="mt-1 block w-full rounded-md border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-gray-500 focus:ring-gray-500"
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
