"use client";

import { useEffect, useMemo, useState } from "react";
import {
  type CareOffering,
  type CareOfferingInput,
  type DaysOfWeekMode,
  cloneCareOffering,
  createCareOffering,
  deleteCareOffering,
  listCareOfferings,
  updateCareOffering,
} from "~/lib/care-offering-api";
import { type Phase, listPhases } from "~/lib/enrollment-phase-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "CareOfferingsEditor" });

const DAY_LABELS: Record<string, string> = {
  mon: "Mo",
  tue: "Di",
  wed: "Mi",
  thu: "Do",
  fri: "Fr",
  sat: "Sa",
  sun: "So",
};
const ALL_DAYS = ["mon", "tue", "wed", "thu", "fri", "sat", "sun"];

const KIND_LABELS: Record<Phase["kind"], string> = {
  school_year: "Schuljahr",
  holiday: "Ferienbetreuung",
  custom: "Sonstiges",
};

function blankInput(phaseId: number): CareOfferingInput {
  return {
    phase_id: phaseId,
    activity_group_id: null,
    name: "",
    description: "",
    days_of_week_mode: "fixed",
    available_days: ["mon", "tue", "wed", "thu", "fri"],
    includes_holiday_care: false,
    includes_lunch: false,
    capacity: null,
    price_cents: null,
    is_active: true,
    sort_order: 0,
  };
}

function offeringToInput(o: CareOffering): CareOfferingInput {
  return {
    phase_id: Number(o.phase_id),
    activity_group_id: o.activity_group_id ? Number(o.activity_group_id) : null,
    name: o.name,
    description: o.description ?? "",
    days_of_week_mode: o.days_of_week_mode,
    available_days: o.available_days,
    includes_holiday_care: o.includes_holiday_care,
    includes_lunch: o.includes_lunch,
    capacity: o.capacity ?? null,
    price_cents: o.price_cents ?? null,
    is_active: o.is_active,
    sort_order: o.sort_order,
  };
}

export function CareOfferingsEditor() {
  const [phases, setPhases] = useState<Phase[]>([]);
  const [selectedPhaseId, setSelectedPhaseId] = useState<string>("");
  const [offerings, setOfferings] = useState<CareOffering[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState<CareOfferingInput | null>(null);
  const [saving, setSaving] = useState(false);

  const hasNoPhases = phases.length === 0;
  const selectedPhase = useMemo(
    () => phases.find((p) => p.id === selectedPhaseId) ?? null,
    [phases, selectedPhaseId],
  );

  const loadAll = async () => {
    setLoading(true);
    setError(null);
    try {
      const phasesData = await listPhases();
      setPhases(phasesData);

      // Auto-select first phase if nothing selected and at least one
      // phase exists. Prefer is_active over inactive.
      let activePhaseId = selectedPhaseId;
      if (!activePhaseId && phasesData.length > 0) {
        const first = phasesData.find((p) => p.is_active) ?? phasesData[0];
        if (first) {
          activePhaseId = first.id;
          setSelectedPhaseId(first.id);
        }
      }

      const offeringsData = activePhaseId
        ? await listCareOfferings(activePhaseId)
        : [];
      setOfferings(offeringsData);
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("care_offerings_load_failed", { error: msg });
      setError(msg);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadAll();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedPhaseId]);

  const beginCreate = () => {
    if (!selectedPhaseId) return;
    setDraft(blankInput(Number(selectedPhaseId)));
    setEditingId(null);
  };

  const beginEdit = (o: CareOffering) => {
    setDraft(offeringToInput(o));
    setEditingId(o.id);
  };

  const cancelEdit = () => {
    setDraft(null);
    setEditingId(null);
  };

  const handleSave = async () => {
    if (!draft) return;
    setSaving(true);
    setError(null);
    try {
      if (editingId) {
        await updateCareOffering(editingId, draft);
      } else {
        await createCareOffering(draft);
      }
      setDraft(null);
      setEditingId(null);
      await loadAll();
    } catch (err) {
      const msg =
        err instanceof Error ? err.message : "Speichern fehlgeschlagen";
      setError(msg);
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (offering: CareOffering) => {
    if (!globalThis.window.confirm(`„${offering.name}" wirklich löschen?`)) {
      return;
    }
    setError(null);
    try {
      await deleteCareOffering(offering.id);
      await loadAll();
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Löschen fehlgeschlagen";
      setError(
        `${msg}. Tipp: Wenn das Angebot bereits in Anmeldungen verwendet wird, deaktiviere es stattdessen über „Bearbeiten → Aktiv ☐".`,
      );
    }
  };

  const handleClone = async (offering: CareOffering) => {
    const targetIDStr = globalThis.window.prompt(
      `ID der Ziel-Phase (siehe Phasenübersicht):\n\nVerfügbare Phasen:\n${phases
        .map((p) => `  ${p.id}: ${p.name}`)
        .join("\n")}`,
      selectedPhaseId,
    );
    if (!targetIDStr) return;
    const targetID = Number(targetIDStr);
    if (!targetID) {
      setError("Ungültige Phase-ID");
      return;
    }
    setError(null);
    try {
      await cloneCareOffering(offering.id, {
        target_phase_id: targetID,
      });
      await loadAll();
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Klonen fehlgeschlagen";
      setError(msg);
    }
  };

  if (loading) {
    return <p className="text-sm text-gray-500">Wird geladen...</p>;
  }

  return (
    <div className="space-y-6">
      {hasNoPhases && (
        <div className="rounded-lg border border-amber-300 bg-amber-50 p-4">
          <h2 className="text-sm font-semibold text-amber-900">
            Noch keine Anmeldephase angelegt
          </h2>
          <p className="mt-1 text-sm text-amber-800">
            Bevor du Betreuungsangebote anlegen kannst, brauchst du mindestens
            eine Anmeldephase (z. B. ein Schuljahr). Lege diese zuerst auf der
            Seite „Anmeldephasen" an.
          </p>
        </div>
      )}

      {error && (
        <div className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-800">
          {error}
        </div>
      )}

      <div className="flex flex-wrap items-center gap-3">
        <label className="text-sm">
          <span className="mr-2 font-medium text-gray-700">Phase:</span>
          <select
            value={selectedPhaseId}
            onChange={(e) => setSelectedPhaseId(e.target.value)}
            disabled={hasNoPhases}
            className="rounded-md border-gray-300 px-3 py-2 text-sm shadow-sm disabled:bg-gray-100"
          >
            {phases.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name} ({KIND_LABELS[p.kind]})
                {!p.is_active ? " — inaktiv" : ""}
              </option>
            ))}
          </select>
        </label>
        <button
          type="button"
          onClick={beginCreate}
          disabled={hasNoPhases || !selectedPhaseId}
          className="ml-auto rounded-lg bg-gray-900 px-4 py-2 text-sm font-semibold text-white hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50"
        >
          + Neues Angebot
        </button>
      </div>

      {selectedPhase && (
        <p className="text-xs text-gray-500">
          Betreuungszeitraum dieser Phase: {selectedPhase.service_start_date} –{" "}
          {selectedPhase.service_end_date}
        </p>
      )}

      {selectedPhaseId && offerings.length === 0 ? (
        <p className="rounded-lg border border-dashed border-gray-300 bg-gray-50 p-6 text-center text-sm text-gray-500">
          Keine Angebote in dieser Phase. Klicke auf „+ Neues Angebot".
        </p>
      ) : (
        <ul className="space-y-3">
          {offerings.map((o) => (
            <li
              key={o.id}
              className="rounded-lg border border-gray-200 bg-white p-4 shadow-sm"
            >
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <h3 className="text-base font-semibold text-gray-900">
                    {o.name}
                    {!o.is_active && (
                      <span className="ml-2 rounded bg-gray-200 px-2 py-0.5 text-xs font-normal text-gray-700">
                        deaktiviert
                      </span>
                    )}
                  </h3>
                  {o.description && (
                    <p className="mt-1 text-sm text-gray-600">
                      {o.description}
                    </p>
                  )}
                  <p className="mt-1 text-xs text-gray-500">
                    Tage:{" "}
                    {o.available_days.map((d) => DAY_LABELS[d] ?? d).join(", ")}
                    {o.includes_holiday_care && " · inkl. Ferienbetreuung"}
                    {o.includes_lunch && " · inkl. Mittagessen"}
                    {o.capacity != null && ` · max. ${o.capacity} Plätze`}
                  </p>
                </div>
                <div className="flex gap-2">
                  <button
                    type="button"
                    onClick={() => beginEdit(o)}
                    className="rounded border border-gray-300 px-3 py-1 text-xs text-gray-700 hover:bg-gray-50"
                  >
                    Bearbeiten
                  </button>
                  <button
                    type="button"
                    onClick={() => void handleClone(o)}
                    className="rounded border border-gray-300 px-3 py-1 text-xs text-gray-700 hover:bg-gray-50"
                  >
                    Klonen
                  </button>
                  <button
                    type="button"
                    onClick={() => void handleDelete(o)}
                    className="rounded border border-red-300 px-3 py-1 text-xs text-red-700 hover:bg-red-50"
                  >
                    Löschen
                  </button>
                </div>
              </div>
            </li>
          ))}
        </ul>
      )}

      {draft && (
        <CareOfferingForm
          draft={draft}
          editing={Boolean(editingId)}
          phases={phases}
          saving={saving}
          onChange={setDraft}
          onSave={handleSave}
          onCancel={cancelEdit}
        />
      )}
    </div>
  );
}

interface CareOfferingFormProps {
  readonly draft: CareOfferingInput;
  readonly editing: boolean;
  readonly phases: Phase[];
  readonly saving: boolean;
  readonly onChange: (draft: CareOfferingInput) => void;
  readonly onSave: () => Promise<void>;
  readonly onCancel: () => void;
}

function CareOfferingForm({
  draft,
  editing,
  phases,
  saving,
  onChange,
  onSave,
  onCancel,
}: CareOfferingFormProps) {
  const update = (patch: Partial<CareOfferingInput>) =>
    onChange({ ...draft, ...patch });
  const toggleDay = (day: string) => {
    const set = new Set(draft.available_days);
    if (set.has(day)) set.delete(day);
    else set.add(day);
    update({ available_days: ALL_DAYS.filter((d) => set.has(d)) });
  };

  return (
    <div className="rounded-lg border border-gray-300 bg-white p-5 shadow">
      <h3 className="mb-4 text-base font-semibold text-gray-900">
        {editing ? "Angebot bearbeiten" : "Neues Angebot"}
      </h3>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <label className="block">
          <span className="block text-xs font-medium text-gray-600">
            Name *
          </span>
          <input
            type="text"
            value={draft.name}
            onChange={(e) => update({ name: e.target.value })}
            className="mt-1 w-full rounded-md border-gray-300 px-3 py-2 text-sm"
            placeholder="z. B. Regelbetreuung"
          />
        </label>
        <label className="block">
          <span className="block text-xs font-medium text-gray-600">
            Phase *
          </span>
          <select
            value={draft.phase_id?.toString() ?? ""}
            onChange={(e) =>
              update({
                phase_id: e.target.value ? Number(e.target.value) : 0,
              })
            }
            className="mt-1 w-full rounded-md border-gray-300 px-3 py-2 text-sm"
          >
            <option value="">— bitte wählen —</option>
            {phases.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </select>
        </label>
      </div>

      <label className="mt-4 block">
        <span className="block text-xs font-medium text-gray-600">
          Beschreibung
        </span>
        <textarea
          value={draft.description ?? ""}
          onChange={(e) => update({ description: e.target.value })}
          rows={2}
          className="mt-1 w-full rounded-md border-gray-300 px-3 py-2 text-sm"
        />
      </label>

      <div className="mt-4">
        <span className="block text-xs font-medium text-gray-600">Tage</span>
        <div className="mt-1 flex flex-wrap gap-2">
          {ALL_DAYS.map((d) => {
            const active = draft.available_days.includes(d);
            return (
              <button
                key={d}
                type="button"
                onClick={() => toggleDay(d)}
                className={`rounded-md border px-3 py-1.5 text-xs font-medium ${
                  active
                    ? "border-gray-900 bg-gray-900 text-white"
                    : "border-gray-300 bg-white text-gray-700 hover:bg-gray-50"
                }`}
              >
                {DAY_LABELS[d]}
              </button>
            );
          })}
        </div>
        <label className="mt-2 inline-flex items-center gap-2 text-xs text-gray-600">
          <input
            type="checkbox"
            checked={draft.days_of_week_mode === "parent_choice"}
            onChange={(e) =>
              update({
                days_of_week_mode: e.target.checked
                  ? "parent_choice"
                  : ("fixed" as DaysOfWeekMode),
              })
            }
          />
          Eltern können einzelne Tage auswählen
        </label>
      </div>

      <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-3">
        <label className="block">
          <span className="block text-xs font-medium text-gray-600">
            Kapazität
          </span>
          <input
            type="number"
            min={0}
            value={draft.capacity ?? ""}
            onChange={(e) =>
              update({
                capacity: e.target.value === "" ? null : Number(e.target.value),
              })
            }
            placeholder="leer = unbegrenzt"
            className="mt-1 w-full rounded-md border-gray-300 px-3 py-2 text-sm"
          />
        </label>
        <label className="block">
          <span className="block text-xs font-medium text-gray-600">
            Preis (Cent)
          </span>
          <input
            type="number"
            min={0}
            value={draft.price_cents ?? ""}
            onChange={(e) =>
              update({
                price_cents:
                  e.target.value === "" ? null : Number(e.target.value),
              })
            }
            placeholder="optional, nur informativ"
            className="mt-1 w-full rounded-md border-gray-300 px-3 py-2 text-sm"
          />
        </label>
        <label className="block">
          <span className="block text-xs font-medium text-gray-600">
            Sortierung
          </span>
          <input
            type="number"
            value={draft.sort_order}
            onChange={(e) => update({ sort_order: Number(e.target.value) })}
            className="mt-1 w-full rounded-md border-gray-300 px-3 py-2 text-sm"
          />
        </label>
      </div>

      <div className="mt-4 flex flex-wrap gap-4">
        <label className="inline-flex items-center gap-2 text-sm text-gray-700">
          <input
            type="checkbox"
            checked={draft.includes_holiday_care}
            onChange={(e) =>
              update({ includes_holiday_care: e.target.checked })
            }
          />
          Inkl. Ferienbetreuung
        </label>
        <label className="inline-flex items-center gap-2 text-sm text-gray-700">
          <input
            type="checkbox"
            checked={draft.includes_lunch}
            onChange={(e) => update({ includes_lunch: e.target.checked })}
          />
          Inkl. Mittagessen
        </label>
        <label className="inline-flex items-center gap-2 text-sm text-gray-700">
          <input
            type="checkbox"
            checked={draft.is_active}
            onChange={(e) => update({ is_active: e.target.checked })}
          />
          Aktiv (für Eltern sichtbar)
        </label>
      </div>

      <div className="mt-5 flex justify-end gap-2">
        <button
          type="button"
          onClick={onCancel}
          disabled={saving}
          className="rounded-md border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50"
        >
          Abbrechen
        </button>
        <button
          type="button"
          onClick={() => void onSave()}
          disabled={saving || !draft.phase_id || !draft.name.trim()}
          className="rounded-md bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 disabled:opacity-50"
        >
          {saving ? "Speichert..." : editing ? "Speichern" : "Erstellen"}
        </button>
      </div>
    </div>
  );
}
