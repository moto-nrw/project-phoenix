"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { HeartPulse, Loader2, MessageCircle, Send, X } from "lucide-react";
import {
  type ChildFeatures,
  type ParentNote,
  type StatusDay,
  addChildNote,
  getChildFeatures,
  listChildNotes,
  listSickDays,
  submitSickNote,
} from "~/lib/parent-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ChildCare" });

const MAX_SICK_DAYS = 60;
const MAX_NOTE_LEN = 2000;

// --- date helpers (native <input type=date> already yields YYYY-MM-DD) ---

function todayISO(): string {
  const now = new Date();
  const y = now.getFullYear();
  const m = `${now.getMonth() + 1}`.padStart(2, "0");
  const d = `${now.getDate()}`.padStart(2, "0");
  return `${y}-${m}-${d}`;
}

function enumerateDates(fromISO: string, toISO: string): string[] {
  const from = new Date(`${fromISO}T00:00:00Z`);
  const to = new Date(`${toISO}T00:00:00Z`);
  if (Number.isNaN(from.getTime()) || Number.isNaN(to.getTime())) return [];
  if (to.getTime() < from.getTime()) return [];
  const out: string[] = [];
  const cursor = new Date(from);
  while (cursor.getTime() <= to.getTime() && out.length < MAX_SICK_DAYS) {
    out.push(cursor.toISOString().slice(0, 10));
    cursor.setUTCDate(cursor.getUTCDate() + 1);
  }
  return out;
}

function formatGermanDate(iso: string): string {
  try {
    const d = iso.length === 10 ? new Date(`${iso}T00:00:00Z`) : new Date(iso);
    return new Intl.DateTimeFormat("de-DE", {
      day: "2-digit",
      month: "2-digit",
      year: "numeric",
      timeZone: iso.length === 10 ? "UTC" : undefined,
    }).format(d);
  } catch {
    return iso;
  }
}

function formatGermanDateTime(iso: string): string {
  try {
    return new Intl.DateTimeFormat("de-DE", {
      day: "2-digit",
      month: "2-digit",
      year: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    }).format(new Date(iso));
  } catch {
    return iso;
  }
}

// --- data hook ---

// Default both features on so a transient features-fetch failure doesn't
// lock a parent out of an action their school actually allows.
const DEFAULT_FEATURES: ChildFeatures = {
  sick_note_enabled: true,
  notes_enabled: true,
  // Capability flags default to false on fetch failure (least privilege —
  // hide invite/remove if we can't confirm they're enabled; the backend
  // enforces the gate regardless).
  related_accounts_invite_enabled: false,
  related_accounts_remove_enabled: false,
};

export interface ChildCare {
  readonly sickDays: StatusDay[];
  readonly notes: ParentNote[];
  readonly features: ChildFeatures;
  readonly loading: boolean;
  reportSick(dates: string[], reason: string): Promise<void>;
  postNote(body: string): Promise<ParentNote[]>;
}

export function useChildCare(studentId: string): ChildCare {
  const [sickDays, setSickDays] = useState<StatusDay[]>([]);
  const [notes, setNotes] = useState<ParentNote[]>([]);
  const [features, setFeatures] = useState<ChildFeatures>(DEFAULT_FEATURES);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [days, noteList, flags] = await Promise.all([
        listSickDays(studentId).catch(() => [] as StatusDay[]),
        listChildNotes(studentId).catch(() => [] as ParentNote[]),
        getChildFeatures(studentId).catch(() => DEFAULT_FEATURES),
      ]);
      setSickDays(days);
      setNotes(noteList);
      setFeatures(flags);
    } catch (err) {
      logger.warn("child_care_load_failed", {
        error: err instanceof Error ? err.message : String(err),
        student_id: studentId,
      });
    } finally {
      setLoading(false);
    }
  }, [studentId]);

  useEffect(() => {
    void load();
  }, [load]);

  const reportSick = useCallback(
    async (dates: string[], reason: string) => {
      const updated = await submitSickNote(studentId, dates, reason);
      // The POST only returns the just-submitted dates. Merge them into the
      // already-loaded list (replacing any same-date entries) so previously
      // reported sick days don't disappear after a non-overlapping submit.
      setSickDays((prev) => {
        const submittedDates = new Set(updated.map((d) => d.date));
        return [
          ...prev.filter((d) => !submittedDates.has(d.date)),
          ...updated,
        ].sort((a, b) => a.date.localeCompare(b.date));
      });
    },
    [studentId],
  );

  const postNote = useCallback(
    async (body: string) => {
      const updated = await addChildNote(studentId, body);
      setNotes(updated);
      return updated;
    },
    [studentId],
  );

  return { sickDays, notes, features, loading, reportSick, postNote };
}

// --- modal shell ---

function ModalShell({
  title,
  accent,
  icon,
  onClose,
  children,
}: Readonly<{
  title: string;
  accent: string;
  icon: React.ReactNode;
  onClose: () => void;
  children: React.ReactNode;
}>) {
  return (
    <div
      className="fixed inset-0 z-50 flex items-end justify-center bg-black/40 p-0 sm:items-center sm:p-4"
      role="dialog"
      aria-modal="true"
      aria-label={title}
    >
      <div className="w-full max-w-lg overflow-hidden rounded-t-3xl bg-white shadow-xl sm:rounded-2xl">
        <div className="flex items-center justify-between gap-3 border-b border-gray-100 px-5 py-4">
          <div className="flex items-center gap-3">
            <span
              className={`flex h-10 w-10 items-center justify-center rounded-xl ${accent}`}
            >
              {icon}
            </span>
            <h2 className="text-lg font-semibold text-gray-900">{title}</h2>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="flex h-9 w-9 items-center justify-center rounded-lg text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
            aria-label="Schließen"
          >
            <X className="h-5 w-5" aria-hidden="true" />
          </button>
        </div>
        <div className="px-5 py-5">{children}</div>
      </div>
    </div>
  );
}

// --- sick-note modal ---

export function SickNoteModal({
  onClose,
  onSubmit,
}: Readonly<{
  onClose: () => void;
  onSubmit: (dates: string[], reason: string) => Promise<void>;
}>) {
  const initial = todayISO();
  const [from, setFrom] = useState(initial);
  const [to, setTo] = useState(initial);
  const [reason, setReason] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const dates = useMemo(() => enumerateDates(from, to), [from, to]);

  const handleSubmit = async () => {
    if (dates.length === 0) {
      setError("Bitte ein gültiges Datum wählen (Bis-Datum nach Von-Datum).");
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      await onSubmit(dates, reason);
      onClose();
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Die Krankmeldung konnte nicht gespeichert werden.",
      );
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <ModalShell
      title="Krank melden"
      accent="bg-[#D6373E]/10 text-[#D6373E]"
      icon={<HeartPulse className="h-5 w-5" aria-hidden="true" />}
      onClose={onClose}
    >
      <div className="space-y-4">
        <p className="text-sm leading-6 text-gray-600">
          Melden Sie Ihr Kind für einen oder mehrere Tage krank. Die Betreuung
          sieht die Meldung sofort.
        </p>
        <div className="grid grid-cols-2 gap-3">
          <label className="block">
            <span className="mb-1 block text-xs font-semibold tracking-wide text-gray-500 uppercase">
              Von
            </span>
            <input
              type="date"
              value={from}
              min={initial}
              onChange={(e) => {
                setFrom(e.target.value);
                if (e.target.value > to) setTo(e.target.value);
              }}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus-visible:border-[#D6373E] focus-visible:ring-2 focus-visible:ring-[#D6373E]/30 focus-visible:outline-none"
            />
          </label>
          <label className="block">
            <span className="mb-1 block text-xs font-semibold tracking-wide text-gray-500 uppercase">
              Bis
            </span>
            <input
              type="date"
              value={to}
              min={from}
              onChange={(e) => setTo(e.target.value)}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus-visible:border-[#D6373E] focus-visible:ring-2 focus-visible:ring-[#D6373E]/30 focus-visible:outline-none"
            />
          </label>
        </div>
        <p className="text-xs text-gray-500">
          {dates.length === 1
            ? "1 Tag wird gemeldet."
            : `${dates.length} Tage werden gemeldet.`}
        </p>
        <label className="block">
          <span className="mb-1 block text-xs font-semibold tracking-wide text-gray-500 uppercase">
            Grund (optional)
          </span>
          <textarea
            value={reason}
            maxLength={MAX_NOTE_LEN}
            onChange={(e) => setReason(e.target.value)}
            rows={3}
            placeholder="z. B. Fieber, beim Arzt"
            className="w-full resize-none rounded-lg border border-gray-300 px-3 py-2 text-sm focus-visible:border-[#D6373E] focus-visible:ring-2 focus-visible:ring-[#D6373E]/30 focus-visible:outline-none"
          />
        </label>
        {error && (
          <p className="rounded-lg bg-[#FF3130]/10 px-3 py-2 text-sm text-[#CC2626]">
            {error}
          </p>
        )}
        <div className="flex justify-end gap-2 pt-1">
          <button
            type="button"
            onClick={onClose}
            className="inline-flex h-10 items-center rounded-lg border border-gray-300 bg-white px-4 text-sm font-semibold text-gray-700 transition-colors hover:bg-gray-50"
          >
            Abbrechen
          </button>
          <button
            type="button"
            onClick={() => void handleSubmit()}
            disabled={submitting}
            className="inline-flex h-10 items-center gap-2 rounded-lg bg-[#D6373E] px-4 text-sm font-semibold text-white transition-colors hover:bg-[#bb2f35] disabled:cursor-not-allowed disabled:opacity-60"
          >
            {submitting && (
              <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
            )}
            Krankmeldung senden
          </button>
        </div>
      </div>
    </ModalShell>
  );
}

// --- notes modal ---

export function NotesModal({
  notes,
  onClose,
  onSubmit,
}: Readonly<{
  notes: ParentNote[];
  onClose: () => void;
  onSubmit: (body: string) => Promise<ParentNote[]>;
}>) {
  const [body, setBody] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [list, setList] = useState<ParentNote[]>(notes);

  const handleSubmit = async () => {
    const trimmed = body.trim();
    if (trimmed.length === 0) {
      setError("Bitte eine Nachricht eingeben.");
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      const updated = await onSubmit(trimmed);
      setList(updated);
      setBody("");
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Die Nachricht konnte nicht gesendet werden.",
      );
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <ModalShell
      title="Nachricht an das Team"
      accent="bg-[#F78C10]/10 text-[#F78C10]"
      icon={<MessageCircle className="h-5 w-5" aria-hidden="true" />}
      onClose={onClose}
    >
      <div className="space-y-4">
        <label className="block">
          <span className="mb-1 block text-xs font-semibold tracking-wide text-gray-500 uppercase">
            Neue Nachricht
          </span>
          <textarea
            value={body}
            maxLength={MAX_NOTE_LEN}
            onChange={(e) => setBody(e.target.value)}
            rows={3}
            placeholder="Kurze Mitteilung an die Betreuung …"
            className="w-full resize-none rounded-lg border border-gray-300 px-3 py-2 text-sm focus-visible:border-[#F78C10] focus-visible:ring-2 focus-visible:ring-[#F78C10]/30 focus-visible:outline-none"
          />
          <span className="mt-1 block text-right text-xs text-gray-400">
            {body.length}/{MAX_NOTE_LEN}
          </span>
        </label>
        {error && (
          <p className="rounded-lg bg-[#FF3130]/10 px-3 py-2 text-sm text-[#CC2626]">
            {error}
          </p>
        )}
        <div className="flex justify-end">
          <button
            type="button"
            onClick={() => void handleSubmit()}
            disabled={submitting}
            className="inline-flex h-10 items-center gap-2 rounded-lg bg-[#F78C10] px-4 text-sm font-semibold text-white transition-colors hover:bg-[#dd7c0c] disabled:cursor-not-allowed disabled:opacity-60"
          >
            {submitting ? (
              <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
            ) : (
              <Send className="h-4 w-4" aria-hidden="true" />
            )}
            Senden
          </button>
        </div>
        {list.length > 0 && (
          <div className="border-t border-gray-100 pt-4">
            <p className="mb-2 text-xs font-semibold tracking-wide text-gray-500 uppercase">
              Zuletzt gesendet
            </p>
            <ul className="space-y-2">
              {list.map((note) => (
                <li
                  key={note.id}
                  className="rounded-xl border border-gray-200 bg-gray-50/70 p-3"
                >
                  <p className="text-sm whitespace-pre-wrap text-gray-900">
                    {note.body}
                  </p>
                  <p className="mt-1 text-xs text-gray-500">
                    {formatGermanDateTime(note.created_at)}
                  </p>
                </li>
              ))}
            </ul>
          </div>
        )}
      </div>
    </ModalShell>
  );
}

// --- read-only lists for the page body ---

export function SickStatusSummary({
  sickDays,
}: Readonly<{ sickDays: StatusDay[] }>) {
  if (sickDays.length === 0) {
    return <span className="text-sm text-gray-600">Keine Krankmeldung</span>;
  }
  const sorted = [...sickDays].sort((a, b) => a.date.localeCompare(b.date));
  const first = sorted.at(0)!;
  const last = sorted.at(-1)!;
  const label =
    sorted.length === 1
      ? `Krank am ${formatGermanDate(first.date)}`
      : `Krank ${formatGermanDate(first.date)} – ${formatGermanDate(last.date)}`;
  return <span className="text-sm font-semibold text-[#D6373E]">{label}</span>;
}

export function ParentNotesList({ notes }: Readonly<{ notes: ParentNote[] }>) {
  if (notes.length === 0) {
    return (
      <p className="text-sm leading-6 text-gray-600">
        Noch keine Nachrichten gesendet.
      </p>
    );
  }
  return (
    <ul className="space-y-2">
      {notes.map((note) => (
        <li
          key={note.id}
          className="rounded-xl border border-gray-200 bg-gray-50/70 p-3"
        >
          <p className="text-sm whitespace-pre-wrap text-gray-900">
            {note.body}
          </p>
          <p className="mt-1 text-xs text-gray-500">
            {formatGermanDateTime(note.created_at)}
          </p>
        </li>
      ))}
    </ul>
  );
}
