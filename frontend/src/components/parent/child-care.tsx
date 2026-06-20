"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { Loader2, Send, Trash2 } from "lucide-react";
import { Modal } from "~/components/ui/modal";
import {
  type CareException,
  type ChildFeatures,
  type ParentNote,
  ParentApiError,
  type StatusDay,
  addChildNote,
  deleteCareException,
  getChildFeatures,
  listCareExceptions,
  listChildNotes,
  listSickDays,
  submitCareException,
  submitSickNote,
} from "~/lib/parent-api";
import { createLogger } from "~/lib/logger";
import { parseISODate, toISODate, todayISO } from "~/lib/date-helpers";

const logger = createLogger({ component: "ChildCare" });

const MAX_SICK_DAYS = 60;
const MAX_NOTE_LEN = 2000;

// --- date helpers (native <input type=date> already yields YYYY-MM-DD) ---

const DATE_ONLY_RE = /^\d{4}-\d{2}-\d{2}$/;

function enumerateDates(fromISO: string, toISO: string): string[] {
  if (!DATE_ONLY_RE.test(fromISO) || !DATE_ONLY_RE.test(toISO)) return [];
  const from = parseISODate(fromISO);
  const to = parseISODate(toISO);
  if (Number.isNaN(from.getTime()) || Number.isNaN(to.getTime())) return [];
  if (to.getTime() < from.getTime()) return [];
  const out: string[] = [];
  const cursor = new Date(from);
  while (cursor.getTime() <= to.getTime() && out.length < MAX_SICK_DAYS) {
    out.push(toISODate(cursor));
    cursor.setDate(cursor.getDate() + 1);
  }
  return out;
}

function formatLocaleDate(iso: string, locale: string): string {
  try {
    const d = iso.length === 10 ? new Date(`${iso}T00:00:00Z`) : new Date(iso);
    return new Intl.DateTimeFormat(locale, {
      day: "2-digit",
      month: "2-digit",
      year: "numeric",
      timeZone: iso.length === 10 ? "UTC" : undefined,
    }).format(d);
  } catch {
    return iso;
  }
}

function formatLocaleDateTime(iso: string, locale: string): string {
  try {
    return new Intl.DateTimeFormat(locale, {
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

// Sick notes and team messages default ON so a transient features-fetch
// failure doesn't lock a parent out of an action their school allows. The
// pickup-time change is ON by default at the school level too, but here we fall
// back to OFF on a features-fetch failure — hiding a button on a transient
// error beats showing one the backend might reject with 403.
const DEFAULT_FEATURES: ChildFeatures = {
  sick_note_enabled: true,
  notes_enabled: true,
  pickup_change_enabled: false,
  // Capability flags default to false on fetch failure (least privilege —
  // hide invite/remove if we can't confirm they're enabled; the backend
  // enforces the gate regardless).
  related_accounts_invite_enabled: false,
  related_accounts_remove_enabled: false,
};

export interface ChildCare {
  readonly sickDays: StatusDay[];
  readonly notes: ParentNote[];
  readonly careExceptions: CareException[];
  // Whether the care-exception list actually loaded. A failed fetch leaves
  // careExceptions empty, which is indistinguishable from "no overrides exist"
  // — and submitCareException treats an omitted leg as an authoritative clear.
  // The pickup modal must block saving while this is false so a parent can't
  // silently wipe an existing override the UI never managed to prefill.
  readonly careExceptionsLoaded: boolean;
  readonly features: ChildFeatures;
  readonly loading: boolean;
  reportSick(dates: string[], reason: string): Promise<void>;
  postNote(body: string): Promise<ParentNote[]>;
  saveCareException(params: {
    date: string;
    pickupTime?: string;
    arrivalTime?: string;
  }): Promise<void>;
  removeCareException(date: string): Promise<void>;
}

export function useChildCare(studentId: string): ChildCare {
  const [sickDays, setSickDays] = useState<StatusDay[]>([]);
  const [notes, setNotes] = useState<ParentNote[]>([]);
  const [careExceptions, setCareExceptions] = useState<CareException[]>([]);
  const [careExceptionsLoaded, setCareExceptionsLoaded] = useState(false);
  const [features, setFeatures] = useState<ChildFeatures>(DEFAULT_FEATURES);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    // Track the care-exception fetch separately: an empty list from a failed
    // fetch must NOT be treated as "no overrides", or the pickup modal could
    // clear a leg it never prefilled (see careExceptionsLoaded above).
    let exceptionsOk = true;
    try {
      const [days, noteList, exceptions, flags] = await Promise.all([
        listSickDays(studentId).catch(() => [] as StatusDay[]),
        listChildNotes(studentId).catch(() => [] as ParentNote[]),
        listCareExceptions(studentId).catch(() => {
          exceptionsOk = false;
          return [] as CareException[];
        }),
        getChildFeatures(studentId).catch(() => DEFAULT_FEATURES),
      ]);
      setSickDays(days);
      setNotes(noteList);
      setCareExceptions(exceptions);
      setCareExceptionsLoaded(exceptionsOk);
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

  const saveCareException = useCallback(
    async (params: {
      date: string;
      pickupTime?: string;
      arrivalTime?: string;
    }) => {
      const saved = await submitCareException(studentId, params);
      // Replace any same-date entry with the just-saved one, keep sorted.
      setCareExceptions((prev) =>
        [...prev.filter((e) => e.date !== saved.date), saved].sort((a, b) =>
          a.date.localeCompare(b.date),
        ),
      );
    },
    [studentId],
  );

  const removeCareException = useCallback(
    async (date: string) => {
      await deleteCareException(studentId, date);
      setCareExceptions((prev) => prev.filter((e) => e.date !== date));
    },
    [studentId],
  );

  return {
    sickDays,
    notes,
    careExceptions,
    careExceptionsLoaded,
    features,
    loading,
    reportSick,
    postNote,
    saveCareException,
    removeCareException,
  };
}

// --- sick-note modal ---

export function SickNoteModal({
  onClose,
  onSubmit,
}: Readonly<{
  onClose: () => void;
  onSubmit: (dates: string[], reason: string) => Promise<void>;
}>) {
  const t = useTranslations("parentChildCare");
  const initial = todayISO();
  const [from, setFrom] = useState(initial);
  const [to, setTo] = useState(initial);
  const [reason, setReason] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const dates = useMemo(() => enumerateDates(from, to), [from, to]);

  const handleSubmit = async () => {
    if (dates.length === 0) {
      setError(t("sick.invalidDate"));
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      await onSubmit(dates, reason);
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("sick.saveError"));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      isOpen
      onClose={onClose}
      title={t("sick.title")}
      closeLabel={t("close")}
    >
      <div className="space-y-4">
        <p className="text-sm leading-6 text-gray-600">{t("sick.intro")}</p>
        <div className="grid grid-cols-2 gap-3">
          <label className="block">
            <span className="mb-1 block text-xs font-semibold tracking-wide text-gray-500 uppercase">
              {t("sick.from")}
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
              {t("sick.to")}
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
          {t("sick.daysCount", { count: dates.length })}
        </p>
        <label className="block">
          <span className="mb-1 block text-xs font-semibold tracking-wide text-gray-500 uppercase">
            {t("sick.reasonLabel")}
          </span>
          <textarea
            value={reason}
            maxLength={MAX_NOTE_LEN}
            onChange={(e) => setReason(e.target.value)}
            rows={3}
            placeholder={t("sick.reasonPlaceholder")}
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
            {t("cancel")}
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
            {t("sick.submit")}
          </button>
        </div>
      </div>
    </Modal>
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
  const t = useTranslations("parentChildCare");
  const locale = useLocale();
  const [body, setBody] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [list, setList] = useState<ParentNote[]>(notes);

  const handleSubmit = async () => {
    const trimmed = body.trim();
    if (trimmed.length === 0) {
      setError(t("notes.empty"));
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      const updated = await onSubmit(trimmed);
      setList(updated);
      setBody("");
    } catch (err) {
      setError(err instanceof Error ? err.message : t("notes.sendError"));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      isOpen
      onClose={onClose}
      title={t("notes.title")}
      closeLabel={t("close")}
    >
      <div className="space-y-4">
        <label className="block">
          <span className="mb-1 block text-xs font-semibold tracking-wide text-gray-500 uppercase">
            {t("notes.newLabel")}
          </span>
          <textarea
            value={body}
            maxLength={MAX_NOTE_LEN}
            onChange={(e) => setBody(e.target.value)}
            rows={3}
            placeholder={t("notes.placeholder")}
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
            {t("notes.send")}
          </button>
        </div>
        {list.length > 0 && (
          <div className="border-t border-gray-100 pt-4">
            <p className="mb-2 text-xs font-semibold tracking-wide text-gray-500 uppercase">
              {t("notes.lastSent")}
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
                    {formatLocaleDateTime(note.created_at, locale)}
                  </p>
                </li>
              ))}
            </ul>
          </div>
        )}
      </div>
    </Modal>
  );
}

// --- pickup-time change modal ---

export function PickupTimeModal({
  careExceptions,
  careExceptionsLoaded,
  pickupChangeEnabled,
  onClose,
  onSubmit,
  onRemove,
}: Readonly<{
  careExceptions: CareException[];
  careExceptionsLoaded: boolean;
  pickupChangeEnabled: boolean;
  onClose: () => void;
  onSubmit: (params: {
    date: string;
    pickupTime?: string;
    arrivalTime?: string;
  }) => Promise<void>;
  onRemove: (date: string) => Promise<void>;
}>) {
  const t = useTranslations("parentChildCare");
  const locale = useLocale();
  const today = todayISO();
  const initial = useMemo(() => {
    if (pickupChangeEnabled) return today;
    return (
      careExceptions.find((entry) => entry.source === "guardian")?.date ?? today
    );
  }, [careExceptions, pickupChangeEnabled, today]);
  // Two calendar months ahead — mirrors the backend cap in SubmitCareException
  // and the parent-portal list window, so the picker can't offer a date the
  // server would reject.
  const maxSelectable = (() => {
    const d = parseISODate(today);
    d.setMonth(d.getMonth() + 2);
    return toISODate(d);
  })();
  const [date, setDate] = useState(initial);
  const [pickupTime, setPickupTime] = useState("");
  const [arrivalTime, setArrivalTime] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const existing = useMemo(
    () => careExceptions.find((e) => e.date === date),
    [careExceptions, date],
  );
  const staffOwned = existing?.source === "staff";

  // Map the backend's stable error code to a localized message; fall back to
  // the raw message (or a generic one) when the code is missing/unknown so the
  // German parent UI never surfaces an English backend string.
  const resolveError = (err: unknown): string => {
    if (err instanceof ParentApiError) {
      switch (err.code) {
        case "pickup_change_disabled":
          return t("pickup.errorDisabled");
        case "care_exception_conflict":
          return t("pickup.errorStaffConflict");
        case "care_exception_raced":
          return t("pickup.errorRaced");
        case "care_exception_past_date":
          return t("pickup.errorPastDate");
        case "care_exception_too_far":
          return t("pickup.errorTooFar");
        case "care_exception_no_time":
          return t("pickup.noTime");
      }
    }
    return err instanceof Error ? err.message : t("pickup.saveError");
  };

  // When the selected date already has an override, prefill the fields so the
  // parent edits rather than blindly overwrites.
  useEffect(() => {
    setPickupTime(existing?.pickup_time ?? "");
    setArrivalTime(existing?.arrival_time ?? "");
    setError(null);
  }, [existing]);

  const handleSubmit = async () => {
    // Guard: if the existing overrides never loaded we can't trust the
    // prefilled fields, and a save would send the empty leg as an authoritative
    // clear. Block until the list is known (the page must be reloaded).
    if (!careExceptionsLoaded) {
      setError(t("pickup.loadError"));
      return;
    }
    if (!pickupTime && !arrivalTime) {
      setError(t("pickup.noTime"));
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      await onSubmit({
        date,
        pickupTime: pickupTime || undefined,
        arrivalTime: arrivalTime || undefined,
      });
      onClose();
    } catch (err) {
      setError(resolveError(err));
    } finally {
      setSubmitting(false);
    }
  };

  const handleRemove = async () => {
    setSubmitting(true);
    setError(null);
    try {
      await onRemove(date);
      onClose();
    } catch (err) {
      setError(resolveError(err));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      isOpen
      onClose={onClose}
      title={t("pickup.title")}
      closeLabel={t("close")}
    >
      <div className="space-y-4">
        <p className="text-sm leading-6 text-gray-600">{t("pickup.intro")}</p>
        {!careExceptionsLoaded && (
          <p className="rounded-lg bg-[#F78C10]/10 px-3 py-2 text-sm text-[#9a5a08]">
            {t("pickup.loadError")}
          </p>
        )}
        <label className="block">
          <span className="mb-1 block text-xs font-semibold tracking-wide text-gray-500 uppercase">
            {t("pickup.dateLabel")}
          </span>
          <input
            type="date"
            value={date}
            min={today}
            max={maxSelectable}
            onChange={(e) => setDate(e.target.value)}
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus-visible:border-[#5080D8] focus-visible:ring-2 focus-visible:ring-[#5080D8]/30 focus-visible:outline-none"
          />
        </label>

        {staffOwned ? (
          <p className="rounded-lg bg-[#5080D8]/10 px-3 py-2 text-sm text-[#3a63b0]">
            {t("pickup.staffSet", {
              pickup: existing?.pickup_time ?? "—",
              arrival: existing?.arrival_time ?? "—",
            })}
          </p>
        ) : (
          <div className="grid grid-cols-2 gap-3">
            <label className="block">
              <span className="mb-1 block text-xs font-semibold tracking-wide text-gray-500 uppercase">
                {t("pickup.arrivalLabel")}
              </span>
              <input
                type="time"
                value={arrivalTime}
                onChange={(e) => setArrivalTime(e.target.value)}
                className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus-visible:border-[#5080D8] focus-visible:ring-2 focus-visible:ring-[#5080D8]/30 focus-visible:outline-none"
              />
            </label>
            <label className="block">
              <span className="mb-1 block text-xs font-semibold tracking-wide text-gray-500 uppercase">
                {t("pickup.pickupLabel")}
              </span>
              <input
                type="time"
                value={pickupTime}
                onChange={(e) => setPickupTime(e.target.value)}
                className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus-visible:border-[#5080D8] focus-visible:ring-2 focus-visible:ring-[#5080D8]/30 focus-visible:outline-none"
              />
            </label>
          </div>
        )}

        {existing && !staffOwned && (
          <p className="text-xs text-gray-500">
            {t("pickup.existingHint", {
              date: formatLocaleDate(date, locale),
            })}
          </p>
        )}

        {error && (
          <p className="rounded-lg bg-[#FF3130]/10 px-3 py-2 text-sm text-[#CC2626]">
            {error}
          </p>
        )}

        <div className="flex items-center justify-between gap-2 pt-1">
          {existing && !staffOwned ? (
            <button
              type="button"
              onClick={() => void handleRemove()}
              disabled={submitting}
              className="inline-flex h-10 items-center gap-2 rounded-lg border border-gray-300 bg-white px-3 text-sm font-semibold text-[#CC2626] transition-colors hover:bg-[#FF3130]/5 disabled:cursor-not-allowed disabled:opacity-60"
            >
              <Trash2 className="h-4 w-4" aria-hidden="true" />
              {t("pickup.reset")}
            </button>
          ) : (
            <span />
          )}
          <div className="flex justify-end gap-2">
            <button
              type="button"
              onClick={onClose}
              className="inline-flex h-10 items-center rounded-lg border border-gray-300 bg-white px-4 text-sm font-semibold text-gray-700 transition-colors hover:bg-gray-50"
            >
              {t("cancel")}
            </button>
            <button
              type="button"
              onClick={() => void handleSubmit()}
              disabled={
                submitting ||
                staffOwned ||
                !pickupChangeEnabled ||
                !careExceptionsLoaded
              }
              className="inline-flex h-10 items-center gap-2 rounded-lg bg-[#5080D8] px-4 text-sm font-semibold text-white transition-colors hover:bg-[#4069b8] disabled:cursor-not-allowed disabled:opacity-60"
            >
              {submitting && (
                <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
              )}
              {t("pickup.submit")}
            </button>
          </div>
        </div>
      </div>
    </Modal>
  );
}

// --- read-only lists for the page body ---

export function SickStatusSummary({
  sickDays,
}: Readonly<{ sickDays: StatusDay[] }>) {
  const t = useTranslations("parentChildCare");
  const locale = useLocale();
  if (sickDays.length === 0) {
    return <span className="text-sm text-gray-600">{t("summary.none")}</span>;
  }
  const sorted = [...sickDays].sort((a, b) => a.date.localeCompare(b.date));
  const first = sorted.at(0)!;
  const last = sorted.at(-1)!;
  const label =
    sorted.length === 1
      ? t("summary.oneDay", { date: formatLocaleDate(first.date, locale) })
      : t("summary.range", {
          from: formatLocaleDate(first.date, locale),
          to: formatLocaleDate(last.date, locale),
        });
  return <span className="text-sm font-semibold text-[#D6373E]">{label}</span>;
}

export function ParentNotesList({ notes }: Readonly<{ notes: ParentNote[] }>) {
  const t = useTranslations("parentChildCare");
  const locale = useLocale();
  if (notes.length === 0) {
    return (
      <p className="text-sm leading-6 text-gray-600">{t("notes.listEmpty")}</p>
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
            {formatLocaleDateTime(note.created_at, locale)}
          </p>
        </li>
      ))}
    </ul>
  );
}
