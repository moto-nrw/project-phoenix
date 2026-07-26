"use client";

import { useState } from "react";
import { Check } from "lucide-react";
import {
  createRollover,
  type Phase,
  type RolloverInput,
  type RolloverMode,
  type RolloverResult,
} from "~/lib/enrollment-phase-api";
import { parseISODate, toISODate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { CustomSelect } from "~/components/ui/custom-select";
import { ISODatePicker } from "~/components/ui/date-picker";
import { DateTimePicker } from "~/components/ui/date-time-picker";

const logger = createLogger({ component: "RolloverForm" });

// Pre-fill helper: the new phase defaults to one year after the
// source phase's service window. Admin can override every field.
function prefillFromSource(source: Phase): RolloverInput {
  const start = parseISODate(source.service_start_date);
  const end = parseISODate(source.service_end_date);
  const oneYear = (d: Date) => {
    const next = new Date(d);
    next.setFullYear(d.getFullYear() + 1);
    return next;
  };

  const nextStart = oneYear(start);
  const nextEnd = oneYear(end);

  // Deadline default: two weeks before the new service start.
  const deadline = new Date(nextStart);
  deadline.setDate(deadline.getDate() - 14);

  return {
    name: `${source.name} (Folgejahr)`,
    kind: source.kind,
    service_start_date: toISODate(nextStart),
    service_end_date: toISODate(nextEnd),
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
  const [nameError, setNameError] = useState<string | null>(null);

  const update = <K extends keyof RolloverInput>(
    key: K,
    value: RolloverInput[K],
  ) => {
    setDraft((d) => ({ ...d, [key]: value }));
    if (key === "name" && nameError !== null) {
      setNameError(null);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    setNameError(null);
    try {
      if (!draft.service_start_date || !draft.service_end_date) {
        throw new Error(
          "Bitte Beginn und Ende des Betreuungszeitraums angeben.",
        );
      }
      const payload: RolloverInput = {
        ...draft,
        rollover_deadline: localToRFC3339(deadlineLocal),
      };
      const result = await createRollover(source.id, payload);
      onSuccess(result);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      const code = (err as { code?: string } | undefined)?.code;
      logger.error("rollover_create_failed", { error: message, code });
      if (code === "rollover.duplicate_name") {
        setNameError(message);
      } else {
        setError(message);
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <form
      onSubmit={handleSubmit}
      className="moto-content-surface space-y-5 rounded-2xl border p-6 shadow-sm backdrop-blur-md"
    >
      <header className="border-b border-gray-100 pb-4">
        <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
          Anschlussphase
        </p>
        <h2 className="mt-1 text-base font-semibold text-gray-900">
          Anschlussphase für „{source.name}" erstellen
        </h2>
        <p className="mt-1 text-sm text-gray-600">
          Alle bestätigten Anmeldungen aus dieser Phase werden in eine neue
          Phase übernommen. Eltern erhalten eine E-Mail mit den nächsten
          Schritten.
        </p>
      </header>

      {error && (
        <div className="rounded-2xl border border-[#FF3130]/20 bg-[#FF3130]/10 p-4 text-sm text-[#CC2626]">
          {error}
        </div>
      )}

      <div className="grid gap-4 md:grid-cols-2">
        <label className="block">
          <span className="block text-xs font-semibold text-gray-700">
            Name der neuen Phase
          </span>
          <input
            id="rollover-name"
            type="text"
            required
            value={draft.name}
            onChange={(e) => update("name", e.target.value)}
            aria-invalid={nameError !== null}
            aria-describedby={nameError ? "rollover-name-error" : undefined}
            className={`mt-1 h-10 w-full rounded-lg border px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
              nameError !== null
                ? "border-[#FF3130] focus:border-[#FF3130] focus:ring-[#FF3130]"
                : "border-gray-200"
            }`}
          />
          {nameError !== null && (
            <p id="rollover-name-error" className="mt-1 text-xs text-[#FF3130]">
              {nameError}
            </p>
          )}
        </label>
        <label
          className="block"
          htmlFor="rollover-kind"
          id="rollover-kind-label"
        >
          <span className="block text-xs font-semibold text-gray-700">Typ</span>
          <CustomSelect
            ariaLabelledBy="rollover-kind-label"
            id="rollover-kind"
            value={draft.kind}
            onChange={(value) => update("kind", value as RolloverInput["kind"])}
            className="mt-1"
            options={[
              { value: "school_year", label: "Schuljahr" },
              { value: "holiday", label: "Ferienbetreuung" },
              { value: "custom", label: "Sonstiges" },
            ]}
          />
        </label>
      </div>

      <fieldset className="rounded-xl border border-gray-200 p-4">
        <legend className="px-1 text-xs font-medium text-gray-700">
          Betreuungszeitraum
        </legend>
        <div className="grid gap-3 sm:grid-cols-2">
          <div className="block">
            <label
              htmlFor="rollover-service-start"
              className="block text-xs font-semibold text-gray-700"
            >
              Betreuung von
            </label>
            <ISODatePicker
              id="rollover-service-start"
              controlSize="md"
              ariaLabel="Betreuung von"
              value={draft.service_start_date}
              onChange={(next) => update("service_start_date", next)}
              className="mt-1"
              calendarLayout="popover"
              // The required picker prevents deselection in the calendar; the
              // submit validation remains the safety net for programmatic edits.
              hideClearButton
              required
            />
          </div>
          <div className="block">
            <label
              htmlFor="rollover-service-end"
              className="block text-xs font-semibold text-gray-700"
            >
              Betreuung bis
            </label>
            <ISODatePicker
              id="rollover-service-end"
              controlSize="md"
              ariaLabel="Betreuung bis"
              min={draft.service_start_date || undefined}
              value={draft.service_end_date}
              onChange={(next) => update("service_end_date", next)}
              className="mt-1"
              calendarLayout="popover"
              hideClearButton
              required
            />
          </div>
        </div>
      </fieldset>

      <fieldset className="rounded-xl border border-gray-200 p-4">
        <legend className="px-1 text-xs font-medium text-gray-700">
          Elternrückmeldung
        </legend>
        <div className="grid gap-3 sm:grid-cols-2">
          <label className="block" htmlFor="rollover-mode">
            <span className="block text-xs font-semibold text-gray-700">
              Modus
            </span>
            <CustomSelect
              id="rollover-mode"
              value={draft.rollover_mode}
              onChange={(value) =>
                update("rollover_mode", value as RolloverMode)
              }
              className="mt-1"
              ariaLabel="Modus"
              options={[
                {
                  value: "opt_out",
                  label: "Opt-Out: Eltern müssen abmelden",
                },
                {
                  value: "opt_in",
                  label: "Opt-In: Eltern müssen aktiv bestätigen",
                },
              ]}
            />
            <p className="mt-1 text-xs text-gray-500">
              {draft.rollover_mode === "opt_out"
                ? "Anmeldungen werden automatisch übernommen. Ohne aktive Abmeldung bis zur Frist landet die Anmeldung in der Prüfung."
                : "Eltern müssen aktiv bestätigen. Ohne Bestätigung bis zur Frist wird die Anmeldung zurückgezogen."}
            </p>
          </label>
          <div className="block">
            <label
              htmlFor="rollover-deadline"
              className="block text-xs font-semibold text-gray-700"
            >
              Frist für die Eltern-Antwort
            </label>
            <DateTimePicker
              id="rollover-deadline"
              controlSize="md"
              dateAriaLabel="Frist für die Eltern-Antwort"
              timeAriaLabel="Frist Uhrzeit"
              className="mt-1"
              value={deadlineLocal}
              onChange={setDeadlineLocal}
              // A deadline without an explicit time should run to the end of the
              // chosen day, not expire at midnight.
              defaultTime="23:59"
              hideClearButton
            />
          </div>
        </div>
      </fieldset>

      <div className="space-y-2">
        <RolloverCheckbox
          checked={draft.rollover_bumps_grade ?? true}
          onChange={(checked) => update("rollover_bumps_grade", checked)}
          label="Klassenstufe automatisch um 1 erhöhen"
          hint="Für jährliche Anmeldephasen aktivieren. Für Halbjahre oder Zeiträume innerhalb eines Schuljahres deaktivieren."
        />
        <RolloverCheckbox
          checked={draft.rollover_auto_approve}
          onChange={(checked) => update("rollover_auto_approve", checked)}
          label="Vorgemerkte Anmeldungen automatisch genehmigen"
          hint="Nur für Opt-Out sinnvoll. Nach Ablauf der Frist werden vorgemerkte Anmeldungen direkt bestätigt."
        />
      </div>

      <div className="flex justify-end gap-2 pt-2">
        <button
          type="button"
          onClick={onCancel}
          className="inline-flex h-9 items-center justify-center rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50"
          disabled={submitting}
        >
          Abbrechen
        </button>
        <button
          type="submit"
          className="inline-flex h-9 items-center justify-center rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50"
          disabled={submitting}
        >
          {submitting ? "Wird erstellt..." : "Anschlussphase erstellen"}
        </button>
      </div>
    </form>
  );
}

function RolloverCheckbox({
  checked,
  onChange,
  label,
  hint,
}: Readonly<{
  checked: boolean;
  onChange: (checked: boolean) => void;
  label: string;
  hint: string;
}>) {
  return (
    <label
      className={`flex min-h-11 cursor-pointer items-center gap-3 rounded-2xl border px-3 py-2.5 text-sm font-medium text-gray-700 transition-colors focus-within:ring-2 focus-within:ring-gray-300 ${
        checked
          ? "border-gray-300 bg-gray-50"
          : "border-gray-100 bg-white hover:border-gray-200 hover:bg-gray-50"
      }`}
    >
      <input
        type="checkbox"
        checked={checked}
        onChange={(event) => onChange(event.target.checked)}
        className="sr-only"
      />
      <span
        className={`flex h-5 w-5 shrink-0 items-center justify-center rounded-md border shadow-sm transition-all ${
          checked ? "border-gray-900 bg-gray-900" : "border-gray-300 bg-white"
        }`}
        aria-hidden="true"
      >
        <Check
          className={`h-3.5 w-3.5 text-white transition-opacity ${
            checked ? "opacity-100" : "opacity-0"
          }`}
        />
      </span>
      <span className="min-w-0 flex-1 leading-snug">
        {label}
        <span className="ml-1 text-xs font-normal text-gray-500">{hint}</span>
      </span>
    </label>
  );
}
