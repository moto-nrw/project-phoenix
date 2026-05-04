"use client";

import { useEffect, useState } from "react";
import {
  type Phase,
  type PhaseInput,
  type PhaseKind,
  type PhaseCareOverflowMode,
  createPhase,
  deletePhase,
  listPhases,
  updatePhase,
} from "~/lib/enrollment-phase-api";
import { listSchemas, type FormSchema } from "~/lib/enrollment-form-schema-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "PhasesEditor" });

const KIND_LABELS: Record<PhaseKind, string> = {
  school_year: "Schuljahr",
  holiday: "Ferienbetreuung",
  custom: "Sonstiges",
};

const OVERFLOW_LABELS: Record<PhaseCareOverflowMode, string> = {
  waitlist: "Auf Warteliste setzen",
  reject: "Anmeldung blockieren",
  allow: "Ohne Hinweis akzeptieren",
};

// Schema-source mode:
//   "base"   → form_schema_id = null (core fields only)
//   "custom" → admin builds a brand-new schema in the form editor;
//              after publishing they come back here and pick it from
//              the dropdown — keeping that flow explicit avoids a
//              tangled inline-create UX
//   "reuse"  → pick an existing schema row (any version, any phase)
type SchemaSource = "base" | "reuse";

function blankInput(): PhaseInput {
  // Pre-fill service dates with the next school year so admins don't
  // start from "0001-01-01". They can edit before saving.
  const now = new Date();
  const startYear =
    now.getMonth() >= 6 ? now.getFullYear() : now.getFullYear() - 1;
  return {
    name: "",
    kind: "school_year",
    service_start_date: `${startYear}-09-01`,
    service_end_date: `${startYear + 1}-07-31`,
    enrollment_open_at: null,
    enrollment_close_at: null,
    form_schema_id: null,
    show_status_reason_to_parent: false,
    care_overflow_mode: "waitlist",
    is_active: true,
  };
}

function phaseToInput(p: Phase): PhaseInput {
  return {
    name: p.name,
    kind: p.kind,
    service_start_date: p.service_start_date,
    service_end_date: p.service_end_date,
    enrollment_open_at: p.enrollment_open_at ?? null,
    enrollment_close_at: p.enrollment_close_at ?? null,
    form_schema_id: p.form_schema_id ?? null,
    show_status_reason_to_parent: p.show_status_reason_to_parent,
    care_overflow_mode: p.care_overflow_mode,
    is_active: p.is_active,
  };
}

// RFC3339 ↔ datetime-local "YYYY-MM-DDTHH:MM" helpers. The HTML
// datetime-local input always works in the browser's local zone,
// which matches what an admin expects when they type "1. März 09:00".
function toLocalInputValue(rfc?: string | null): string {
  if (!rfc) return "";
  const d = new Date(rfc);
  if (Number.isNaN(d.getTime())) return "";
  const pad = (n: number) => n.toString().padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function fromLocalInputValue(local: string): string | null {
  if (!local) return null;
  const d = new Date(local);
  if (Number.isNaN(d.getTime())) return null;
  return d.toISOString();
}

export function PhasesEditor() {
  const [phases, setPhases] = useState<Phase[]>([]);
  const [schemas, setSchemas] = useState<FormSchema[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [info, setInfo] = useState<string | null>(null);

  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState<PhaseInput | null>(null);
  const [schemaSource, setSchemaSource] = useState<SchemaSource>("base");
  const [saving, setSaving] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  const loadAll = async () => {
    setLoading(true);
    setError(null);
    try {
      const [phasesData, schemasData] = await Promise.all([
        listPhases(),
        listSchemas().catch(() => [] as FormSchema[]),
      ]);
      setPhases(phasesData);
      setSchemas(schemasData);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("phases_load_failed", { error: message });
      setError(message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadAll();
  }, []);

  const startCreate = () => {
    setEditingId("new");
    setDraft(blankInput());
    setSchemaSource("base");
    setInfo(null);
    setError(null);
  };

  const startEdit = (phase: Phase) => {
    setEditingId(phase.id);
    setDraft(phaseToInput(phase));
    setSchemaSource(phase.form_schema_id ? "reuse" : "base");
    setInfo(null);
    setError(null);
  };

  const cancelEdit = () => {
    setEditingId(null);
    setDraft(null);
  };

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!draft) return;
    setSaving(true);
    setError(null);
    setInfo(null);
    try {
      // Normalise schema_source → form_schema_id.
      const payload: PhaseInput = {
        ...draft,
        form_schema_id: schemaSource === "base" ? null : draft.form_schema_id,
      };
      if (schemaSource === "reuse" && !payload.form_schema_id) {
        throw new Error("Bitte ein Formular auswählen oder 'Basis' wählen.");
      }
      if (editingId === "new") {
        const created = await createPhase(payload);
        setInfo(`Phase „${created.name}" erstellt.`);
      } else if (editingId) {
        const updated = await updatePhase(editingId, payload);
        setInfo(`Phase „${updated.name}" gespeichert.`);
      }
      cancelEdit();
      await loadAll();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("phase_save_failed", { error: message });
      setError(message);
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (phase: Phase) => {
    if (
      !window.confirm(
        `Phase „${phase.name}" wirklich löschen? Diese Aktion ist nicht umkehrbar.`,
      )
    ) {
      return;
    }
    setDeletingId(phase.id);
    setError(null);
    setInfo(null);
    try {
      await deletePhase(phase.id);
      setInfo(`Phase „${phase.name}" gelöscht.`);
      await loadAll();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("phase_delete_failed", { error: message });
      // The backend returns 409 with a German hint when the phase has
      // care offerings or submissions referencing it. The message
      // already says "deactivate instead" — surface as-is.
      setError(message);
    } finally {
      setDeletingId(null);
    }
  };

  const handleToggleActive = async (phase: Phase) => {
    setSaving(true);
    setError(null);
    setInfo(null);
    try {
      const updated = await updatePhase(phase.id, {
        ...phaseToInput(phase),
        is_active: !phase.is_active,
      });
      setInfo(
        updated.is_active
          ? `Phase „${updated.name}" ist jetzt aktiv.`
          : `Phase „${updated.name}" wurde deaktiviert.`,
      );
      await loadAll();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("phase_toggle_active_failed", { error: message });
      setError(message);
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return <p className="text-sm text-gray-500">Phasen werden geladen...</p>;
  }

  return (
    <div className="space-y-6">
      {error && (
        <div className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-800">
          {error}
        </div>
      )}
      {info && (
        <div className="rounded-lg border border-green-200 bg-green-50 p-3 text-sm text-green-800">
          {info}
        </div>
      )}

      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-gray-900">Phasen</h2>
        {!editingId && (
          <button
            type="button"
            onClick={startCreate}
            className="rounded-md bg-[#83CD2D] px-4 py-2 text-sm font-medium text-white hover:opacity-90"
          >
            + Neue Phase
          </button>
        )}
      </div>

      {editingId && draft && (
        <PhaseForm
          draft={draft}
          setDraft={setDraft}
          schemas={schemas}
          schemaSource={schemaSource}
          setSchemaSource={setSchemaSource}
          editing={editingId !== "new"}
          saving={saving}
          onSubmit={handleSave}
          onCancel={cancelEdit}
        />
      )}

      {phases.length === 0 ? (
        <div className="rounded-2xl border border-dashed border-gray-300 bg-gray-50 p-6 text-center text-sm text-gray-500">
          Noch keine Phasen angelegt. Klicke auf „Neue Phase", um mit der ersten
          Anmeldephase (z. B. einem Schuljahr) zu starten.
        </div>
      ) : (
        <div className="overflow-x-auto rounded-xl border border-gray-200">
          <table className="min-w-full divide-y divide-gray-200 text-sm">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-3 py-2 text-left text-xs font-semibold text-gray-600">
                  Name
                </th>
                <th className="px-3 py-2 text-left text-xs font-semibold text-gray-600">
                  Typ
                </th>
                <th className="px-3 py-2 text-left text-xs font-semibold text-gray-600">
                  Zeitraum
                </th>
                <th className="px-3 py-2 text-left text-xs font-semibold text-gray-600">
                  Anmeldefenster
                </th>
                <th className="px-3 py-2 text-left text-xs font-semibold text-gray-600">
                  Status
                </th>
                <th className="px-3 py-2 text-right text-xs font-semibold text-gray-600">
                  Aktionen
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200 bg-white">
              {phases.map((p) => (
                <tr key={p.id}>
                  <td className="px-3 py-2 font-medium text-gray-900">
                    {p.name}
                  </td>
                  <td className="px-3 py-2 text-gray-700">
                    {KIND_LABELS[p.kind]}
                  </td>
                  <td className="px-3 py-2 text-xs text-gray-600">
                    {p.service_start_date} – {p.service_end_date}
                  </td>
                  <td className="px-3 py-2 text-xs text-gray-600">
                    {p.enrollment_open_at || p.enrollment_close_at ? (
                      <>
                        {p.enrollment_open_at
                          ? new Date(p.enrollment_open_at).toLocaleString(
                              "de-DE",
                              {
                                day: "2-digit",
                                month: "2-digit",
                                year: "numeric",
                                hour: "2-digit",
                                minute: "2-digit",
                              },
                            )
                          : "—"}
                        {" – "}
                        {p.enrollment_close_at
                          ? new Date(p.enrollment_close_at).toLocaleString(
                              "de-DE",
                              {
                                day: "2-digit",
                                month: "2-digit",
                                year: "numeric",
                                hour: "2-digit",
                                minute: "2-digit",
                              },
                            )
                          : "—"}
                      </>
                    ) : (
                      <span className="text-gray-400">unbegrenzt</span>
                    )}
                  </td>
                  <td className="px-3 py-2">
                    {p.is_active ? (
                      <span className="inline-flex items-center rounded-full bg-[#83CD2D]/15 px-2 py-0.5 text-xs font-medium text-[#5a8e1f]">
                        Aktiv
                      </span>
                    ) : (
                      <span className="inline-flex items-center rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600">
                        Inaktiv
                      </span>
                    )}
                  </td>
                  <td className="space-x-2 px-3 py-2 text-right">
                    <button
                      type="button"
                      onClick={() => startEdit(p)}
                      className="text-xs font-medium text-[#5080D8] hover:underline"
                      disabled={saving}
                    >
                      Bearbeiten
                    </button>
                    <button
                      type="button"
                      onClick={() => void handleToggleActive(p)}
                      className="text-xs font-medium text-gray-700 hover:underline"
                      disabled={saving}
                    >
                      {p.is_active ? "Deaktivieren" : "Aktivieren"}
                    </button>
                    <button
                      type="button"
                      onClick={() => void handleDelete(p)}
                      className="text-xs font-medium text-[#FF3130] hover:underline disabled:opacity-50"
                      disabled={deletingId === p.id || saving}
                    >
                      {deletingId === p.id ? "Wird gelöscht..." : "Löschen"}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

interface PhaseFormProps {
  readonly draft: PhaseInput;
  readonly setDraft: React.Dispatch<React.SetStateAction<PhaseInput | null>>;
  readonly schemas: FormSchema[];
  readonly schemaSource: SchemaSource;
  readonly setSchemaSource: React.Dispatch<React.SetStateAction<SchemaSource>>;
  readonly editing: boolean;
  readonly saving: boolean;
  readonly onSubmit: (e: React.FormEvent) => void;
  readonly onCancel: () => void;
}

function PhaseForm(props: PhaseFormProps) {
  const {
    draft,
    setDraft,
    schemas,
    schemaSource,
    setSchemaSource,
    editing,
    saving,
    onSubmit,
    onCancel,
  } = props;

  const update = (patch: Partial<PhaseInput>) =>
    setDraft((prev) => (prev ? { ...prev, ...patch } : prev));

  return (
    <form
      onSubmit={onSubmit}
      className="space-y-5 rounded-2xl border border-gray-200 bg-white p-6 shadow-sm"
    >
      <h3 className="text-base font-semibold text-gray-900">
        {editing ? "Phase bearbeiten" : "Neue Phase"}
      </h3>

      <div className="grid gap-4 sm:grid-cols-2">
        <label className="block">
          <span className="text-xs font-medium text-gray-700">Name</span>
          <input
            type="text"
            required
            value={draft.name}
            onChange={(e) => update({ name: e.target.value })}
            placeholder="z. B. Schuljahr 2026/27"
            className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
          />
        </label>

        <label className="block">
          <span className="text-xs font-medium text-gray-700">Typ</span>
          <select
            value={draft.kind}
            onChange={(e) => update({ kind: e.target.value as PhaseKind })}
            className="mt-1 w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm"
          >
            <option value="school_year">{KIND_LABELS.school_year}</option>
            <option value="holiday">{KIND_LABELS.holiday}</option>
            <option value="custom">{KIND_LABELS.custom}</option>
          </select>
        </label>
      </div>

      <fieldset className="rounded-lg border border-gray-200 p-4">
        <legend className="px-1 text-xs font-medium text-gray-700">
          Betreuungszeitraum
        </legend>
        <div className="grid gap-3 sm:grid-cols-2">
          <label className="block">
            <span className="text-xs text-gray-600">Beginn</span>
            <input
              type="date"
              required
              value={draft.service_start_date}
              onChange={(e) => update({ service_start_date: e.target.value })}
              className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
            />
          </label>
          <label className="block">
            <span className="text-xs text-gray-600">Ende</span>
            <input
              type="date"
              required
              value={draft.service_end_date}
              onChange={(e) => update({ service_end_date: e.target.value })}
              className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
            />
          </label>
        </div>
      </fieldset>

      <fieldset className="rounded-lg border border-gray-200 p-4">
        <legend className="px-1 text-xs font-medium text-gray-700">
          Anmeldefenster (optional)
        </legend>
        <p className="mb-2 text-xs text-gray-500">
          Lass beide Felder leer, wenn das Anmeldeformular jederzeit erreichbar
          sein soll. Sonst kann die Phase nur in diesem Zeitraum eingereicht
          werden.
        </p>
        <div className="grid gap-3 sm:grid-cols-2">
          <label className="block">
            <span className="text-xs text-gray-600">Öffnung</span>
            <input
              type="datetime-local"
              value={toLocalInputValue(draft.enrollment_open_at)}
              onChange={(e) =>
                update({
                  enrollment_open_at: fromLocalInputValue(e.target.value),
                })
              }
              className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
            />
          </label>
          <label className="block">
            <span className="text-xs text-gray-600">Schließung</span>
            <input
              type="datetime-local"
              value={toLocalInputValue(draft.enrollment_close_at)}
              onChange={(e) =>
                update({
                  enrollment_close_at: fromLocalInputValue(e.target.value),
                })
              }
              className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
            />
          </label>
        </div>
      </fieldset>

      <fieldset className="rounded-lg border border-gray-200 p-4">
        <legend className="px-1 text-xs font-medium text-gray-700">
          Formular
        </legend>
        <div className="space-y-2 text-sm">
          <label
            htmlFor="schema-source-base"
            className="flex items-start gap-2"
          >
            <input
              id="schema-source-base"
              type="radio"
              name="schema-source"
              value="base"
              checked={schemaSource === "base"}
              onChange={() => {
                setSchemaSource("base");
                update({ form_schema_id: null });
              }}
              className="mt-0.5"
            />
            <span>
              <strong>Basis</strong> — nur die Standardfelder (Eltern, Kinder,
              Geburtsdatum, Klasse).
            </span>
          </label>
          <label
            htmlFor="schema-source-reuse"
            className="flex items-start gap-2"
          >
            <input
              id="schema-source-reuse"
              type="radio"
              name="schema-source"
              value="reuse"
              checked={schemaSource === "reuse"}
              onChange={() => setSchemaSource("reuse")}
              className="mt-0.5"
              disabled={schemas.length === 0}
            />
            <span className="flex-1">
              <strong>Bestehendes Formular wählen</strong> — eigene Felder aus
              einer anderen Phase übernehmen.
              {schemaSource === "reuse" && (
                <select
                  value={draft.form_schema_id ?? ""}
                  onChange={(e) =>
                    update({ form_schema_id: e.target.value || null })
                  }
                  className="mt-1 block w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm"
                  disabled={schemas.length === 0}
                  required={schemaSource === "reuse"}
                  aria-label="Formular auswählen"
                >
                  <option value="">— wählen —</option>
                  {schemas.map((s) => (
                    <option key={s.id} value={s.id}>
                      Version {s.version}
                      {s.is_active ? " (aktiv)" : ""} — {s.fields.length} Felder
                    </option>
                  ))}
                </select>
              )}
              {schemas.length === 0 && (
                <p className="mt-1 text-xs text-gray-500">
                  Noch keine eigenen Formulare vorhanden. Erstelle zunächst
                  eines auf der Seite „Anmeldeformular".
                </p>
              )}
            </span>
          </label>
        </div>
      </fieldset>

      <div className="grid gap-4 sm:grid-cols-2">
        <label className="block">
          <span className="text-xs font-medium text-gray-700">
            Verhalten bei voller Betreuung
          </span>
          <select
            value={draft.care_overflow_mode}
            onChange={(e) =>
              update({
                care_overflow_mode: e.target.value as PhaseCareOverflowMode,
              })
            }
            className="mt-1 w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm"
          >
            <option value="waitlist">{OVERFLOW_LABELS.waitlist}</option>
            <option value="reject">{OVERFLOW_LABELS.reject}</option>
            <option value="allow">{OVERFLOW_LABELS.allow}</option>
          </select>
        </label>

        <div className="flex flex-col gap-2 pt-5 text-sm text-gray-700">
          <label className="flex items-center gap-2">
            <input
              type="checkbox"
              checked={draft.show_status_reason_to_parent}
              onChange={(e) =>
                update({ show_status_reason_to_parent: e.target.checked })
              }
            />
            <span>
              Begründung für Eltern sichtbar
              <span className="ml-1 text-xs text-gray-500">
                (auf der Status-Seite + in Status-E-Mails)
              </span>
            </span>
          </label>
          <label className="flex items-center gap-2">
            <input
              type="checkbox"
              checked={draft.is_active}
              onChange={(e) => update({ is_active: e.target.checked })}
            />
            <span>
              Aktiv
              <span className="ml-1 text-xs text-gray-500">
                (für Eltern sichtbar, sobald das Anmeldefenster offen ist)
              </span>
            </span>
          </label>
        </div>
      </div>

      <div className="flex justify-end gap-2">
        <button
          type="button"
          onClick={onCancel}
          disabled={saving}
          className="rounded-md border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50"
        >
          Abbrechen
        </button>
        <button
          type="submit"
          disabled={saving}
          className="rounded-md bg-[#83CD2D] px-4 py-2 text-sm font-medium text-white hover:opacity-90 disabled:opacity-50"
        >
          {saving ? "Speichert..." : editing ? "Speichern" : "Erstellen"}
        </button>
      </div>
    </form>
  );
}
