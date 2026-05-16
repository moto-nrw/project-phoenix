"use client";

import { useState } from "react";
import {
  createRollover,
  type Phase,
  type RolloverInput,
  type RolloverMode,
  type RolloverResult,
} from "~/lib/enrollment-phase-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "RolloverForm" });

// Pre-fill helper: the new phase defaults to one year after the
// source phase's service window. Admin can override every field.
function prefillFromSource(source: Phase): RolloverInput {
  const start = new Date(source.service_start_date);
  const end = new Date(source.service_end_date);
  const oneYear = (d: Date) => {
    const next = new Date(d);
    next.setUTCFullYear(d.getUTCFullYear() + 1);
    return next;
  };
  const toDate = (d: Date) => d.toISOString().slice(0, 10);

  const nextStart = oneYear(start);
  const nextEnd = oneYear(end);

  // Deadline default: two weeks before the new service start.
  const deadline = new Date(nextStart);
  deadline.setUTCDate(deadline.getUTCDate() - 14);

  return {
    name: `${source.name} (Folgejahr)`,
    kind: source.kind,
    service_start_date: toDate(nextStart),
    service_end_date: toDate(nextEnd),
    enrollment_open_at: null,
    enrollment_close_at: null,
    form_schema_id: source.form_schema_id ?? null,
    rollover_mode: "opt_out",
    rollover_auto_approve: false,
    rollover_deadline: deadline.toISOString(),
    rollover_bumps_grade: true,
  };
}

// Format `YYYY-MM-DDTHH:MM` (datetime-local input value) ↔ RFC3339.
function localToRFC3339(local: string): string {
  if (!local) return "";
  return new Date(local).toISOString();
}

function rfc3339ToLocal(rfc: string | null | undefined): string {
  if (!rfc) return "";
  const d = new Date(rfc);
  const pad = (n: number) => String(n).padStart(2, "0");
  return (
    `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}` +
    `T${pad(d.getHours())}:${pad(d.getMinutes())}`
  );
}

interface Props {
  readonly source: Phase;
  readonly onCancel: () => void;
  readonly onSuccess: (result: RolloverResult) => void;
}

export function RolloverForm({ source, onCancel, onSuccess }: Props) {
  const [draft, setDraft] = useState<RolloverInput>(() =>
    prefillFromSource(source),
  );
  const [deadlineLocal, setDeadlineLocal] = useState(
    rfc3339ToLocal(draft.rollover_deadline),
  );
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const update = <K extends keyof RolloverInput>(
    key: K,
    value: RolloverInput[K],
  ) => {
    setDraft((d) => ({ ...d, [key]: value }));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      const payload: RolloverInput = {
        ...draft,
        rollover_deadline: localToRFC3339(deadlineLocal),
      };
      const result = await createRollover(source.id, payload);
      onSuccess(result);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("rollover_create_failed", { error: message });
      setError(message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <form
      onSubmit={handleSubmit}
      className="space-y-4 rounded-2xl border border-gray-200 bg-white p-6 shadow-sm"
    >
      <header>
        <h2 className="text-lg font-semibold text-gray-900">
          Anschlussphase für „{source.name}" erstellen
        </h2>
        <p className="mt-1 text-sm text-gray-600">
          Alle bestätigten Anmeldungen aus dieser Phase werden in eine neue
          Phase übernommen. Eltern erhalten eine E-Mail mit den nächsten
          Schritten.
        </p>
      </header>

      {error && (
        <div className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-800">
          {error}
        </div>
      )}

      <div className="grid gap-4 md:grid-cols-2">
        <div>
          <label
            htmlFor="rollover-name"
            className="block text-xs font-semibold text-gray-700"
          >
            Name der neuen Phase
          </label>
          <input
            id="rollover-name"
            type="text"
            required
            value={draft.name}
            onChange={(e) => update("name", e.target.value)}
            className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
          />
        </div>
        <div>
          <label
            htmlFor="rollover-kind"
            className="block text-xs font-semibold text-gray-700"
          >
            Typ
          </label>
          <select
            id="rollover-kind"
            value={draft.kind}
            onChange={(e) =>
              update("kind", e.target.value as RolloverInput["kind"])
            }
            className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
          >
            <option value="school_year">Schuljahr</option>
            <option value="holiday">Ferienbetreuung</option>
            <option value="custom">Sonstiges</option>
          </select>
        </div>
        <div>
          <label
            htmlFor="rollover-service-start"
            className="block text-xs font-semibold text-gray-700"
          >
            Betreuung von
          </label>
          <input
            id="rollover-service-start"
            type="date"
            required
            value={draft.service_start_date}
            onChange={(e) => update("service_start_date", e.target.value)}
            className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
          />
        </div>
        <div>
          <label
            htmlFor="rollover-service-end"
            className="block text-xs font-semibold text-gray-700"
          >
            Betreuung bis
          </label>
          <input
            id="rollover-service-end"
            type="date"
            required
            value={draft.service_end_date}
            onChange={(e) => update("service_end_date", e.target.value)}
            className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
          />
        </div>
        <div>
          <label
            htmlFor="rollover-mode"
            className="block text-xs font-semibold text-gray-700"
          >
            Modus
          </label>
          <select
            id="rollover-mode"
            value={draft.rollover_mode}
            onChange={(e) =>
              update("rollover_mode", e.target.value as RolloverMode)
            }
            className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
          >
            <option value="opt_out">Opt-Out — Eltern müssen abmelden</option>
            <option value="opt_in">
              Opt-In — Eltern müssen aktiv bestätigen
            </option>
          </select>
          <p className="mt-1 text-xs text-gray-500">
            {draft.rollover_mode === "opt_out"
              ? "Anmeldungen werden automatisch übernommen. Ohne aktive Abmeldung bis zur Frist landet die Anmeldung in der Prüfung."
              : "Eltern müssen aktiv bestätigen. Ohne Bestätigung bis zur Frist wird die Anmeldung zurückgezogen."}
          </p>
        </div>
        <div>
          <label
            htmlFor="rollover-deadline"
            className="block text-xs font-semibold text-gray-700"
          >
            Frist für die Eltern-Antwort
          </label>
          <input
            id="rollover-deadline"
            type="datetime-local"
            required
            value={deadlineLocal}
            onChange={(e) => setDeadlineLocal(e.target.value)}
            className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
          />
        </div>
      </div>

      <div className="space-y-2">
        <label className="flex items-start gap-2 text-sm text-gray-700">
          <input
            type="checkbox"
            checked={draft.rollover_bumps_grade ?? true}
            onChange={(e) => update("rollover_bumps_grade", e.target.checked)}
            className="mt-1"
          />
          <span>
            Klassenstufe automatisch um 1 erhöhen
            <span className="block text-xs text-gray-500">
              Aktivieren für jährliche Phasen; deaktivieren für Halbjahre oder
              andere Zeiträume innerhalb eines Schuljahres.
            </span>
          </span>
        </label>
        <label className="flex items-start gap-2 text-sm text-gray-700">
          <input
            type="checkbox"
            checked={draft.rollover_auto_approve}
            onChange={(e) => update("rollover_auto_approve", e.target.checked)}
            className="mt-1"
          />
          <span>
            Vorgemerkte Anmeldungen automatisch genehmigen
            <span className="block text-xs text-gray-500">
              Nur für Opt-Out-Modus sinnvoll. Nach Ablauf der Frist werden
              vorgemerkte Anmeldungen direkt bestätigt, ohne erneute Prüfung
              durch die Schulleitung.
            </span>
          </span>
        </label>
      </div>

      <div className="flex justify-end gap-2 pt-2">
        <button
          type="button"
          onClick={onCancel}
          className="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
          disabled={submitting}
        >
          Abbrechen
        </button>
        <button
          type="submit"
          className="rounded-md bg-[#83CD2D] px-4 py-2 text-sm font-medium text-white hover:opacity-90 disabled:opacity-50"
          disabled={submitting}
        >
          {submitting ? "Wird erstellt..." : "Anschlussphase erstellen"}
        </button>
      </div>
    </form>
  );
}
