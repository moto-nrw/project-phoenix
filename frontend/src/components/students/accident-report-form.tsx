"use client";

import { Checkbox } from "~/components/ui/checkbox";
import { DateTimePicker } from "~/components/ui/date-time-picker";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { FieldLabel } from "~/components/ui/field-label";
import { Input } from "~/components/ui/input";
import { Textarea } from "~/components/ui/textarea";
import {
  berlinClockFromISO,
  berlinDateTimeISO,
  berlinTodayISO,
  parseISODate,
} from "~/lib/date-helpers";
import type { AccidentReportFields } from "~/lib/student-accident-reports-api";

// The fields of an accident report (Unfallanzeige, #4771) in the order of the
// official form: when and where, what happened, the injury, who saw it, the
// doctor; then what the OGS records for itself: first aid, parents and
// supervision. Create dialog and edit state share this component.

/** The draft as the form holds it: times as "YYYY-MM-DDTHH:mm" in Berlin. */
export interface AccidentReportDraft {
  occurredAt: string;
  location: string;
  description: string;
  injuredBodyParts: string;
  injuryKind: string;
  witnesses: string;
  firstAid: string;
  doctorVisited: boolean;
  doctor: string;
  parentsNotified: string;
  parentsNotifiedAt: string;
  supervisor: string;
}

export function emptyAccidentReportDraft(): AccidentReportDraft {
  return {
    occurredAt: "",
    location: "",
    description: "",
    injuredBodyParts: "",
    injuryKind: "",
    witnesses: "",
    firstAid: "",
    doctorVisited: false,
    doctor: "",
    parentsNotified: "",
    parentsNotifiedAt: "",
    supervisor: "",
  };
}

function localFromInstant(iso: string | null): string {
  if (!iso) return "";
  const clock = berlinClockFromISO(iso);
  return clock ? `${berlinTodayISO(new Date(iso))}T${clock}` : "";
}

function instantFromLocal(value: string): string | null {
  const [date, time] = value.split("T");
  if (!date || !time) return null;
  try {
    return berlinDateTimeISO(parseISODate(date), time);
  } catch {
    // An impossible wall-clock time (daylight saving switch) counts as
    // missing; the form names the field.
    return null;
  }
}

export function draftFromReport(
  report: AccidentReportFields,
): AccidentReportDraft {
  return {
    ...report,
    occurredAt: localFromInstant(report.occurredAt),
    parentsNotifiedAt: localFromInstant(report.parentsNotifiedAt),
  };
}

/** The draft's missing required fields, keyed by field name. */
export function accidentReportDraftErrors(
  draft: AccidentReportDraft,
): Record<string, string> {
  const errors: Record<string, string> = {};
  if (!instantFromLocal(draft.occurredAt)) {
    errors.occurredAt = "Bitte geben Sie Tag und Uhrzeit des Unfalls an.";
  }
  if (!draft.location.trim()) {
    errors.location = "Bitte geben Sie den Unfallort an.";
  }
  if (!draft.description.trim()) {
    errors.description = "Bitte beschreiben Sie, was passiert ist.";
  }
  if (!draft.injuryKind.trim()) {
    errors.injuryKind = "Bitte geben Sie die Art der Verletzung an.";
  }
  if (draft.doctorVisited && !draft.doctor.trim()) {
    errors.doctor = "Bitte geben Sie Arztpraxis oder Krankenhaus an.";
  }
  if (!draft.supervisor.trim()) {
    errors.supervisor = "Bitte geben Sie die Aufsichtsperson an.";
  }
  return errors;
}

/** Turns a checked draft into the request fields. */
export function fieldsFromDraft(
  draft: AccidentReportDraft,
): AccidentReportFields {
  return {
    ...draft,
    occurredAt: instantFromLocal(draft.occurredAt) ?? "",
    parentsNotifiedAt: draft.parentsNotified.trim()
      ? instantFromLocal(draft.parentsNotifiedAt)
      : null,
  };
}

export function AccidentReportFormFields({
  draft,
  onChange,
  fieldError,
  disabled = false,
}: Readonly<{
  draft: AccidentReportDraft;
  onChange: (draft: AccidentReportDraft) => void;
  fieldError: (name: string) => string | undefined;
  disabled?: boolean;
}>) {
  const set = <K extends keyof AccidentReportDraft>(
    key: K,
    value: AccidentReportDraft[K],
  ) => onChange({ ...draft, [key]: value });

  return (
    <div className="space-y-4">
      <div>
        <FieldLabel htmlFor="unfall-zeitpunkt" required>
          Wann ist der Unfall passiert?
        </FieldLabel>
        <DateTimePicker
          id="unfall-zeitpunkt"
          value={draft.occurredAt}
          onChange={(value) => set("occurredAt", value)}
          invalid={Boolean(fieldError("occurredAt"))}
          disabled={disabled}
          required
          dateAriaLabel="Tag des Unfalls"
          timeAriaLabel="Uhrzeit des Unfalls"
        />
        <FieldErrorHint
          id="unfall-zeitpunkt-fehler"
          message={fieldError("occurredAt")}
        />
      </div>
      <Input
        name="location"
        label="Unfallort"
        placeholder="zum Beispiel Schulhof, am Klettergerüst"
        value={draft.location}
        onChange={(e) => set("location", e.target.value)}
        error={fieldError("location")}
        disabled={disabled}
        required
        maxLength={500}
      />
      <Textarea
        name="description"
        label="Was ist passiert?"
        placeholder="Beschreiben Sie den Hergang: Ort, Tätigkeit und wie es zum Unfall kam."
        value={draft.description}
        onChange={(e) => set("description", e.target.value)}
        error={fieldError("description")}
        disabled={disabled}
        required
        rows={4}
        maxLength={5000}
      />
      <div className="grid gap-4 sm:grid-cols-2">
        <Input
          name="injuredBodyParts"
          label="Verletzte Körperteile"
          placeholder="zum Beispiel linker Unterarm"
          value={draft.injuredBodyParts}
          onChange={(e) => set("injuredBodyParts", e.target.value)}
          error={fieldError("injuredBodyParts")}
          disabled={disabled}
          maxLength={500}
        />
        <Input
          name="injuryKind"
          label="Art der Verletzung"
          placeholder="zum Beispiel Prellung"
          value={draft.injuryKind}
          onChange={(e) => set("injuryKind", e.target.value)}
          error={fieldError("injuryKind")}
          disabled={disabled}
          required
          maxLength={500}
        />
      </div>
      <Input
        name="firstAid"
        label="Erste Hilfe"
        placeholder="zum Beispiel gekühlt, Pflaster"
        value={draft.firstAid}
        onChange={(e) => set("firstAid", e.target.value)}
        error={fieldError("firstAid")}
        disabled={disabled}
        maxLength={500}
      />
      <div className="space-y-2">
        <label
          htmlFor="unfall-arztbesuch"
          className="flex items-center gap-2 text-sm text-gray-700"
        >
          <Checkbox
            id="unfall-arztbesuch"
            checked={draft.doctorVisited}
            onChange={(e) => set("doctorVisited", e.target.checked)}
            disabled={disabled}
          />
          Das Kind war beim Arzt oder im Krankenhaus.
        </label>
        {draft.doctorVisited && (
          <Input
            name="doctor"
            label="Arztpraxis oder Krankenhaus"
            value={draft.doctor}
            onChange={(e) => set("doctor", e.target.value)}
            error={fieldError("doctor")}
            disabled={disabled}
            required
            maxLength={500}
          />
        )}
      </div>
      <Input
        name="witnesses"
        label="Zeugen"
        placeholder="Wer hat den Unfall gesehen?"
        value={draft.witnesses}
        onChange={(e) => set("witnesses", e.target.value)}
        error={fieldError("witnesses")}
        disabled={disabled}
        maxLength={500}
      />
      <div className="grid gap-4 sm:grid-cols-2">
        <Input
          name="parentsNotified"
          label="Benachrichtigte Eltern"
          placeholder="zum Beispiel Mutter, telefonisch"
          value={draft.parentsNotified}
          onChange={(e) => set("parentsNotified", e.target.value)}
          error={fieldError("parentsNotified")}
          disabled={disabled}
          maxLength={500}
        />
        <div>
          <FieldLabel htmlFor="unfall-eltern-zeit">
            Wann wurden die Eltern benachrichtigt?
          </FieldLabel>
          <DateTimePicker
            id="unfall-eltern-zeit"
            value={draft.parentsNotifiedAt}
            onChange={(value) => set("parentsNotifiedAt", value)}
            disabled={disabled || !draft.parentsNotified.trim()}
            dateAriaLabel="Tag der Benachrichtigung"
            timeAriaLabel="Uhrzeit der Benachrichtigung"
          />
        </div>
      </div>
      <Input
        name="supervisor"
        label="Aufsichtsperson"
        placeholder="Wer hatte die Aufsicht?"
        value={draft.supervisor}
        onChange={(e) => set("supervisor", e.target.value)}
        error={fieldError("supervisor")}
        disabled={disabled}
        required
        maxLength={500}
      />
    </div>
  );
}
