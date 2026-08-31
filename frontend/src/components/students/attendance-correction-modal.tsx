"use client";

// Korrektur eines abgeschlossenen Betreuungsblocks (#2898).
//
// Der Block ist abgeschlossen — die laufende Erfassung ist hier vorbei. Diese
// Maske ist deshalb bewusst kein zweites Erfassungsformular: sie verlangt einen
// Grund, sie sagt, dass die Änderung protokolliert wird, und sie zeigt die
// bisherigen Korrekturen desselben Eintrags direkt darunter.

import { useCallback, useEffect, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { FormModal } from "~/components/ui/form-modal";
import { createLogger } from "~/lib/logger";
import { getCachedSession } from "~/lib/session-cache";

const logger = createLogger({ component: "AttendanceCorrectionModal" });

const REASON_MAX_LENGTH = 500;
const NOTE_MAX_LENGTH = 500;

const STATUS_OPTIONS = [
  { value: "present", label: "Anwesend" },
  { value: "absent", label: "Abwesend" },
  { value: "expected", label: "Erwartet" },
] as const;

const SUBSTATUS_OPTIONS = [
  { value: "", label: "Kein Hinweis" },
  { value: "late", label: "Verspätet" },
  { value: "excused", label: "Entschuldigt" },
  { value: "sick", label: "Krank" },
  { value: "field_trip", label: "Klassenfahrt" },
  { value: "other", label: "Sonstiges" },
] as const;

export interface CorrectableSlot {
  readonly instanceId: string;
  readonly title: string;
  readonly date: string;
  readonly startTime: string;
  readonly endTime: string;
  readonly status: string;
  readonly substatus: string | null;
  readonly note: string | null;
}

interface CorrectionEntry {
  readonly field_name: string;
  readonly old_value?: string | null;
  readonly new_value?: string | null;
  readonly reason: string;
  readonly actor_name?: string | null;
  readonly corrected_at: string;
}

interface AttendanceCorrectionModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly studentId: string;
  readonly slot: CorrectableSlot;
  readonly onCorrected: () => void;
}

const FIELD_LABELS: Record<string, string> = {
  status: "Anwesenheit",
  substatus: "Hinweis",
  note: "Bemerkung",
};

function describeValue(value: string | null | undefined): string {
  if (value === null || value === undefined || value === "") return "leer";
  const status = STATUS_OPTIONS.find((option) => option.value === value);
  if (status) return status.label;
  const substatus = SUBSTATUS_OPTIONS.find((option) => option.value === value);
  if (substatus) return substatus.label;
  return value;
}

async function authorizedHeaders(): Promise<Record<string, string>> {
  const session = await getCachedSession();
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };
  if (session?.user?.token) {
    headers.Authorization = `Bearer ${session.user.token}`;
  }
  return headers;
}

export function AttendanceCorrectionModal({
  isOpen,
  onClose,
  studentId,
  slot,
  onCorrected,
}: AttendanceCorrectionModalProps) {
  const [status, setStatus] = useState(slot.status);
  const [substatus, setSubstatus] = useState(slot.substatus ?? "");
  const [note, setNote] = useState(slot.note ?? "");
  const [reason, setReason] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [history, setHistory] = useState<CorrectionEntry[]>([]);

  // Reset whenever a different slot opens the modal — otherwise the previous
  // block's values would be offered as this one's starting point.
  useEffect(() => {
    if (!isOpen) return;
    setStatus(slot.status);
    setSubstatus(slot.substatus ?? "");
    setNote(slot.note ?? "");
    setReason("");
    setError(null);
  }, [isOpen, slot]);

  const loadHistory = useCallback(async () => {
    try {
      const response = await fetch(
        `/api/timetable/instances/${slot.instanceId}/students/${studentId}/corrections`,
        { credentials: "include", headers: await authorizedHeaders() },
      );
      if (!response.ok) return;
      const payload = (await response.json()) as {
        data?: { corrections?: CorrectionEntry[] };
        corrections?: CorrectionEntry[];
      };
      setHistory(payload.data?.corrections ?? payload.corrections ?? []);
    } catch (err) {
      // The trail is context, not the purpose of this dialog: failing to load
      // it must not block the correction itself.
      logger.warn("Korrekturverlauf konnte nicht geladen werden", { err });
    }
  }, [slot.instanceId, studentId]);

  useEffect(() => {
    if (isOpen) void loadHistory();
  }, [isOpen, loadHistory]);

  const trimmedReason = reason.trim();
  const nothingChanged =
    status === slot.status &&
    substatus === (slot.substatus ?? "") &&
    note === (slot.note ?? "");

  const handleSubmit = async () => {
    setError(null);
    if (trimmedReason === "") {
      setError("Bitte geben Sie einen Grund für die Korrektur an.");
      return;
    }
    if (nothingChanged) {
      setError("Es gibt nichts zu speichern. Bitte ändern Sie zuerst etwas.");
      return;
    }

    setSaving(true);
    try {
      const response = await fetch(
        `/api/timetable/instances/${slot.instanceId}/students/${studentId}/correction`,
        {
          method: "POST",
          credentials: "include",
          headers: await authorizedHeaders(),
          body: JSON.stringify({
            status,
            substatus: substatus === "" ? null : substatus,
            note: note.trim() === "" ? null : note.trim(),
            reason: trimmedReason,
          }),
        },
      );
      if (!response.ok) {
        setError(
          response.status === 409
            ? "Dieser Termin lässt sich nicht korrigieren. Nur abgeschlossene Termine können Sie nachträglich ändern."
            : "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.",
        );
        return;
      }
      onCorrected();
      onClose();
    } catch (err) {
      logger.error("Korrektur fehlgeschlagen", { err });
      setError(
        "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.",
      );
    } finally {
      setSaving(false);
    }
  };

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title="Eintrag korrigieren"
      size="md"
      footer={
        <div className="flex justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={onClose}
            disabled={saving}
          >
            Abbrechen
          </Button>
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={() => void handleSubmit()}
            disabled={saving}
          >
            {saving ? "Wird gespeichert …" : "Speichern"}
          </Button>
        </div>
      }
    >
      <div className="space-y-4">
        <p className="text-sm text-gray-600">
          {slot.title} am {slot.date}, {slot.startTime}–{slot.endTime}. Dieser
          Termin ist abgeschlossen. Ihre Änderung wird mit Ihrem Namen, der
          Uhrzeit und dem Grund gespeichert.
        </p>

        {error ? <Alert type="error" message={error} /> : null}

        <div>
          <label
            id="correction-status-label"
            htmlFor="correction-status"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            Anwesenheit
          </label>
          <CustomSelect
            id="correction-status"
            value={status}
            options={[...STATUS_OPTIONS]}
            onChange={setStatus}
            labelId="correction-status-label"
          />
        </div>

        <div>
          <label
            id="correction-substatus-label"
            htmlFor="correction-substatus"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            Hinweis
          </label>
          <CustomSelect
            id="correction-substatus"
            value={substatus}
            options={[...SUBSTATUS_OPTIONS]}
            onChange={setSubstatus}
            labelId="correction-substatus-label"
          />
        </div>

        <div>
          <label
            htmlFor="correction-note"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            Bemerkung
          </label>
          <textarea
            id="correction-note"
            value={note}
            onChange={(event) => setNote(event.target.value)}
            maxLength={NOTE_MAX_LENGTH}
            rows={3}
            className="moto-content-surface w-full rounded-md border px-4 py-3 text-sm"
            placeholder="Keine Bemerkung"
          />
        </div>

        <div>
          <label
            htmlFor="correction-reason"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            Grund der Korrektur <span aria-hidden="true">*</span>
          </label>
          <textarea
            id="correction-reason"
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            maxLength={REASON_MAX_LENGTH}
            rows={2}
            required
            className="moto-content-surface w-full rounded-md border px-4 py-3 text-sm"
            placeholder="Warum wird der Eintrag geändert?"
          />
          <p className="mt-1 text-xs text-gray-500">
            Bitte ausfüllen. Der Grund bleibt dauerhaft gespeichert.
          </p>
        </div>

        {history.length > 0 ? (
          <div className="border-t border-gray-100 pt-3">
            <p className="mb-2 text-sm font-medium text-gray-700">
              Bisherige Korrekturen
            </p>
            <ul className="space-y-2">
              {history.map((entry) => (
                <li
                  key={`${entry.corrected_at}-${entry.field_name}`}
                  className="text-xs text-gray-600"
                >
                  <span className="font-medium">
                    {FIELD_LABELS[entry.field_name] ?? entry.field_name}
                  </span>
                  : {describeValue(entry.old_value)} →{" "}
                  {describeValue(entry.new_value)}
                  <br />
                  <span className="text-gray-500">
                    {new Date(entry.corrected_at).toLocaleString("de-DE")}
                    {entry.actor_name ? ` · ${entry.actor_name}` : ""} ·{" "}
                    {entry.reason}
                  </span>
                </li>
              ))}
            </ul>
          </div>
        ) : null}
      </div>
    </FormModal>
  );
}
