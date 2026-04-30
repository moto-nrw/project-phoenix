"use client";

import { useEffect, useMemo, useState } from "react";
import {
  type CalendarPeriodSummary,
  type CareOffering,
  type CareOfferingInput,
  type DaysOfWeekMode,
  cloneCareOffering,
  createCalendarPeriod,
  createCareOffering,
  deleteCareOffering,
  listCalendarPeriods,
  listCareOfferings,
  updateCareOffering,
} from "~/lib/care-offering-api";
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

function blankInput(periodId?: number): CareOfferingInput {
  return {
    calendar_period_id: periodId ?? null,
    activity_group_id: null,
    name: "",
    description: "",
    days_of_week_mode: "fixed",
    available_days: ["mon", "tue", "wed", "thu", "fri"],
    includes_holiday_care: false,
    includes_lunch: false,
    capacity: null,
    price_cents: null,
    application_window_start: null,
    application_window_end: null,
    service_start_date: null,
    service_end_date: null,
    is_active: true,
    sort_order: 0,
  };
}

function offeringToInput(o: CareOffering): CareOfferingInput {
  return {
    calendar_period_id: o.calendar_period_id
      ? Number(o.calendar_period_id)
      : null,
    activity_group_id: o.activity_group_id ? Number(o.activity_group_id) : null,
    name: o.name,
    description: o.description ?? "",
    days_of_week_mode: o.days_of_week_mode,
    available_days: o.available_days,
    includes_holiday_care: o.includes_holiday_care,
    includes_lunch: o.includes_lunch,
    capacity: o.capacity ?? null,
    price_cents: o.price_cents ?? null,
    application_window_start: o.application_window_start ?? null,
    application_window_end: o.application_window_end ?? null,
    service_start_date: o.service_start_date ?? null,
    service_end_date: o.service_end_date ?? null,
    is_active: o.is_active,
    sort_order: o.sort_order,
  };
}

export function CareOfferingsEditor() {
  const [periods, setPeriods] = useState<CalendarPeriodSummary[]>([]);
  const [selectedPeriodId, setSelectedPeriodId] = useState<string>("");
  const [offerings, setOfferings] = useState<CareOffering[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showCreatePeriodModal, setShowCreatePeriodModal] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState<CareOfferingInput | null>(null);
  const [saving, setSaving] = useState(false);

  const schoolYearPeriods = useMemo(
    () => periods.filter((p) => p.period_type === "school_year"),
    [periods],
  );
  const hasNoPeriods = periods.length === 0;

  const loadAll = async () => {
    setLoading(true);
    setError(null);
    try {
      const [periodsData, offeringsData] = await Promise.all([
        listCalendarPeriods(),
        listCareOfferings(selectedPeriodId || undefined),
      ]);
      setPeriods(periodsData);
      setOfferings(offeringsData);
      // Auto-select the first school year if nothing selected.
      if (!selectedPeriodId) {
        const first = periodsData.find((p) => p.period_type === "school_year");
        if (first) setSelectedPeriodId(first.id);
      }
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
  }, [selectedPeriodId]);

  const beginCreate = () => {
    const periodId = selectedPeriodId ? Number(selectedPeriodId) : undefined;
    setDraft(blankInput(periodId));
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
    if (!globalThis.window.confirm(`"${offering.name}" wirklich löschen?`)) {
      return;
    }
    setError(null);
    try {
      await deleteCareOffering(offering.id);
      await loadAll();
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Löschen fehlgeschlagen";
      setError(
        `${msg}. Tipp: Wenn das Angebot bereits in Anmeldungen verwendet wird, deaktiviere es stattdessen über "Bearbeiten → Aktiv ☐".`,
      );
    }
  };

  const handleClone = async (offering: CareOffering) => {
    const targetIDStr = globalThis.window.prompt(
      "ID des Ziel-Schuljahres (siehe Auswahl oben):",
      selectedPeriodId,
    );
    if (!targetIDStr) return;
    const offsetStr = globalThis.window.prompt(
      "Datums-Verschiebung in Tagen (z.B. 365 für ein Jahr, 0 für keine Verschiebung):",
      "365",
    );
    if (offsetStr === null) return;
    const targetID = Number(targetIDStr);
    const offset = Number(offsetStr);
    if (!targetID || Number.isNaN(offset)) {
      setError("Ungültige Werte für Klonen");
      return;
    }
    setError(null);
    try {
      await cloneCareOffering(offering.id, {
        target_calendar_period_id: targetID,
        days_offset: offset,
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
      {hasNoPeriods && (
        <div className="rounded-lg border border-amber-300 bg-amber-50 p-4">
          <h2 className="text-sm font-semibold text-amber-900">
            Noch kein Schuljahr angelegt
          </h2>
          <p className="mt-1 text-sm text-amber-800">
            Bevor du Betreuungsangebote anlegen kannst, brauchst du ein
            Schuljahr (Kalenderzeitraum). Lege jetzt eines an oder verwende
            einen vorhandenen Zeitraum.
          </p>
          <button
            type="button"
            onClick={() => setShowCreatePeriodModal(true)}
            className="mt-3 rounded-lg bg-amber-700 px-3 py-1.5 text-sm font-medium text-white hover:bg-amber-800"
          >
            Erstes Schuljahr anlegen
          </button>
        </div>
      )}

      {error && (
        <div className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-800">
          {error}
        </div>
      )}

      <div className="flex flex-wrap items-center gap-3">
        <label className="text-sm">
          <span className="mr-2 font-medium text-gray-700">Schuljahr:</span>
          <select
            value={selectedPeriodId}
            onChange={(e) => setSelectedPeriodId(e.target.value)}
            className="rounded-md border-gray-300 px-3 py-2 text-sm shadow-sm"
          >
            <option value="">Alle Zeiträume</option>
            {schoolYearPeriods.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name} ({p.id})
              </option>
            ))}
          </select>
        </label>
        <button
          type="button"
          onClick={() => setShowCreatePeriodModal(true)}
          className="rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
        >
          + Schuljahr
        </button>
        <button
          type="button"
          onClick={beginCreate}
          disabled={hasNoPeriods}
          className="ml-auto rounded-lg bg-gray-900 px-4 py-2 text-sm font-semibold text-white hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50"
        >
          + Neues Angebot
        </button>
      </div>

      {offerings.length === 0 ? (
        <p className="rounded-lg border border-dashed border-gray-300 bg-gray-50 p-6 text-center text-sm text-gray-500">
          Keine Angebote in diesem Zeitraum. Klicke auf "+ Neues Angebot".
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
          periods={periods}
          saving={saving}
          onChange={setDraft}
          onSave={handleSave}
          onCancel={cancelEdit}
        />
      )}

      {showCreatePeriodModal && (
        <CreateCalendarPeriodModal
          onClose={() => setShowCreatePeriodModal(false)}
          onCreated={async (created) => {
            setShowCreatePeriodModal(false);
            setSelectedPeriodId(created.id);
            await loadAll();
          }}
        />
      )}
    </div>
  );
}

interface CareOfferingFormProps {
  readonly draft: CareOfferingInput;
  readonly editing: boolean;
  readonly periods: CalendarPeriodSummary[];
  readonly saving: boolean;
  readonly onChange: (draft: CareOfferingInput) => void;
  readonly onSave: () => Promise<void>;
  readonly onCancel: () => void;
}

function CareOfferingForm({
  draft,
  editing,
  periods,
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
            placeholder="z.B. Regelbetreuung"
          />
        </label>
        <label className="block">
          <span className="block text-xs font-medium text-gray-600">
            Schuljahr *
          </span>
          <select
            value={draft.calendar_period_id?.toString() ?? ""}
            onChange={(e) =>
              update({
                calendar_period_id: e.target.value
                  ? Number(e.target.value)
                  : null,
              })
            }
            className="mt-1 w-full rounded-md border-gray-300 px-3 py-2 text-sm"
          >
            <option value="">– bitte wählen –</option>
            {periods.map((p) => (
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
                days_of_week_mode: e.target.checked ? "parent_choice" : "fixed",
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

      <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2">
        <label className="block">
          <span className="block text-xs font-medium text-gray-600">
            Anmeldung beginnt
          </span>
          <input
            type="datetime-local"
            value={toDatetimeLocal(draft.application_window_start)}
            onChange={(e) =>
              update({
                application_window_start: fromDatetimeLocal(e.target.value),
              })
            }
            className="mt-1 w-full rounded-md border-gray-300 px-3 py-2 text-sm"
          />
        </label>
        <label className="block">
          <span className="block text-xs font-medium text-gray-600">
            Anmeldung endet
          </span>
          <input
            type="datetime-local"
            value={toDatetimeLocal(draft.application_window_end)}
            onChange={(e) =>
              update({
                application_window_end: fromDatetimeLocal(e.target.value),
              })
            }
            className="mt-1 w-full rounded-md border-gray-300 px-3 py-2 text-sm"
          />
        </label>
      </div>

      <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2">
        <label className="block">
          <span className="block text-xs font-medium text-gray-600">
            Betreuung beginnt
          </span>
          <input
            type="date"
            value={draft.service_start_date ?? ""}
            onChange={(e) =>
              update({ service_start_date: e.target.value || null })
            }
            className="mt-1 w-full rounded-md border-gray-300 px-3 py-2 text-sm"
          />
        </label>
        <label className="block">
          <span className="block text-xs font-medium text-gray-600">
            Betreuung endet
          </span>
          <input
            type="date"
            value={draft.service_end_date ?? ""}
            onChange={(e) =>
              update({ service_end_date: e.target.value || null })
            }
            className="mt-1 w-full rounded-md border-gray-300 px-3 py-2 text-sm"
          />
        </label>
      </div>

      <div className="mt-4 flex flex-wrap gap-4 text-sm">
        <label className="inline-flex items-center gap-2">
          <input
            type="checkbox"
            checked={draft.includes_holiday_care}
            onChange={(e) =>
              update({ includes_holiday_care: e.target.checked })
            }
          />
          inkl. Ferienbetreuung
        </label>
        <label className="inline-flex items-center gap-2">
          <input
            type="checkbox"
            checked={draft.includes_lunch}
            onChange={(e) => update({ includes_lunch: e.target.checked })}
          />
          inkl. Mittagessen
        </label>
        <label className="inline-flex items-center gap-2">
          <input
            type="checkbox"
            checked={draft.is_active}
            onChange={(e) => update({ is_active: e.target.checked })}
          />
          Aktiv
        </label>
      </div>

      <div className="mt-5 flex gap-3">
        <button
          type="button"
          onClick={() => void onSave()}
          disabled={saving || !draft.name || !draft.calendar_period_id}
          className="rounded-lg bg-gray-900 px-4 py-2 text-sm font-semibold text-white hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {saving ? "Speichern..." : "Speichern"}
        </button>
        <button
          type="button"
          onClick={onCancel}
          disabled={saving}
          className="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
        >
          Abbrechen
        </button>
      </div>
    </div>
  );
}

function toDatetimeLocal(value: string | null | undefined): string {
  if (!value) return "";
  // Trim seconds + timezone for the <input type="datetime-local"> value.
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return "";
  const pad = (n: number) => n.toString().padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function fromDatetimeLocal(value: string): string | null {
  if (!value) return null;
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return null;
  return d.toISOString();
}

interface CreateCalendarPeriodModalProps {
  readonly onClose: () => void;
  readonly onCreated: (created: CalendarPeriodSummary) => Promise<void>;
}

function CreateCalendarPeriodModal({
  onClose,
  onCreated,
}: CreateCalendarPeriodModalProps) {
  const [name, setName] = useState("");
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");
  const [periodType, setPeriodType] = useState<
    "school_year" | "semester" | "holiday" | "custom"
  >("school_year");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async () => {
    setError(null);
    if (!name.trim() || !startDate || !endDate) {
      setError("Name, Start und Ende sind Pflichtfelder.");
      return;
    }
    setSubmitting(true);
    try {
      const created = await createCalendarPeriod({
        name: name.trim(),
        start_date: startDate,
        end_date: endDate,
        period_type: periodType,
      });
      await onCreated(created);
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Anlegen fehlgeschlagen";
      setError(msg);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4">
      <div className="w-full max-w-md rounded-xl bg-white p-6 shadow-2xl">
        <h2 className="text-lg font-semibold text-gray-900">
          Schuljahr anlegen
        </h2>
        <p className="mt-1 text-sm text-gray-600">
          Lege einen neuen Kalenderzeitraum an, in dem dann Betreuungsangebote
          eingerichtet werden können.
        </p>

        {error && (
          <div className="mt-3 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-800">
            {error}
          </div>
        )}

        <div className="mt-4 space-y-3">
          <label className="block">
            <span className="block text-xs font-medium text-gray-600">
              Name *
            </span>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="z.B. Schuljahr 2026/2027"
              className="mt-1 w-full rounded-md border-gray-300 px-3 py-2 text-sm"
            />
          </label>
          <div className="grid grid-cols-2 gap-3">
            <label className="block">
              <span className="block text-xs font-medium text-gray-600">
                Start *
              </span>
              <input
                type="date"
                value={startDate}
                onChange={(e) => setStartDate(e.target.value)}
                className="mt-1 w-full rounded-md border-gray-300 px-3 py-2 text-sm"
              />
            </label>
            <label className="block">
              <span className="block text-xs font-medium text-gray-600">
                Ende *
              </span>
              <input
                type="date"
                value={endDate}
                onChange={(e) => setEndDate(e.target.value)}
                className="mt-1 w-full rounded-md border-gray-300 px-3 py-2 text-sm"
              />
            </label>
          </div>
          <label className="block">
            <span className="block text-xs font-medium text-gray-600">
              Typ *
            </span>
            <select
              value={periodType}
              onChange={(e) =>
                setPeriodType(e.target.value as typeof periodType)
              }
              className="mt-1 w-full rounded-md border-gray-300 px-3 py-2 text-sm"
            >
              <option value="school_year">Schuljahr</option>
              <option value="holiday">Ferien</option>
              <option value="semester">Halbjahr</option>
              <option value="custom">Sonstiger Zeitraum</option>
            </select>
          </label>
        </div>

        <div className="mt-5 flex justify-end gap-3">
          <button
            type="button"
            onClick={onClose}
            disabled={submitting}
            className="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
          >
            Abbrechen
          </button>
          <button
            type="button"
            onClick={() => void handleSubmit()}
            disabled={submitting}
            className="rounded-lg bg-gray-900 px-4 py-2 text-sm font-semibold text-white hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {submitting ? "Speichern..." : "Anlegen"}
          </button>
        </div>
      </div>
    </div>
  );
}

// Suppress the "unused type" lint when DaysOfWeekMode is only referenced
// implicitly via the editor draft state.
void ({} as DaysOfWeekMode);
