"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import {
  CalendarClock,
  CalendarRange,
  HeartPulse,
  IdCard,
  Loader2,
  type LucideIcon,
  Trash2,
} from "lucide-react";
import { Modal } from "~/components/ui/modal";
import { Button } from "~/components/ui/button";
import { Alert } from "~/components/ui/alert";
import {
  type CareException,
  type ChildFeatures,
  ParentApiError,
  type StatusDay,
  type StudentStatusKind,
  deleteCareException,
  getChildFeatures,
  listCareExceptions,
  listSickDays,
  submitCareException,
  submitSickNote,
} from "~/lib/parent-api";
import { CustomSelect } from "~/components/ui/custom-select";
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

// --- data hook ---

// Sick notes and team messages default ON so a transient features-fetch
// failure doesn't lock a parent out of an action their school allows. The
// pickup-time change is ON by default at the school level too, but here we fall
// back to OFF on a features-fetch failure — hiding a button on a transient
// error beats showing one the backend might reject with 403.
const DEFAULT_FEATURES: ChildFeatures = {
  sick_note_enabled: true,
  // Default false on fetch failure (least privilege), consistent with the other
  // consequential flags below: the features fetch .catch returns DEFAULT_FEATURES,
  // and a school with messaging turned OFF would otherwise show an enabled
  // composer on a transient hiccup → send → 403. The backend enforces the gate
  // regardless; this just keeps the UI from dead-ending on an action it can see.
  notes_enabled: false,
  // Consequential capability: default false on fetch failure (least privilege),
  // so a transient hiccup hides the request actions rather than dead-ending on
  // a 403; the backend enforces the gate regardless.
  request_submit_enabled: false,
  pickup_change_enabled: false,
  // Capability flags default to false on fetch failure (least privilege —
  // hide invite/remove if we can't confirm they're enabled; the backend
  // enforces the gate regardless).
  related_accounts_invite_enabled: false,
  related_accounts_remove_enabled: false,
};

export interface ChildCare {
  readonly sickDays: StatusDay[];
  readonly careExceptions: CareException[];
  // Whether the care-exception list actually loaded. A failed fetch leaves
  // careExceptions empty, which is indistinguishable from "no overrides exist"
  // — and submitCareException treats an omitted leg as an authoritative clear.
  // The pickup modal must block saving while this is false so a parent can't
  // silently wipe an existing override the UI never managed to prefill.
  readonly careExceptionsLoaded: boolean;
  readonly features: ChildFeatures;
  readonly loading: boolean;
  reportSick(
    dates: string[],
    reason: string,
    status: StudentStatusKind,
  ): Promise<void>;
  saveCareException(params: {
    date: string;
    pickupTime?: string;
    arrivalTime?: string;
  }): Promise<void>;
  removeCareException(date: string): Promise<void>;
}

export function useChildCare(studentId: string): ChildCare {
  const [sickDays, setSickDays] = useState<StatusDay[]>([]);
  const [careExceptions, setCareExceptions] = useState<CareException[]>([]);
  const [careExceptionsLoaded, setCareExceptionsLoaded] = useState(false);
  const [features, setFeatures] = useState<ChildFeatures>(DEFAULT_FEATURES);
  const [loading, setLoading] = useState(true);
  // Stale-response guard: load re-runs on every studentId change, and on a fast
  // child A→B switch (same hook instance reused) a late-resolving load(A) must
  // not overwrite B's data — one child's sick days / care exceptions / feature
  // flags shown under another would mis-set the pickup-modal safety gate. Each
  // run claims the next token; only the most-recently-started run may setState.
  // Mirrors OgsConversation.refresh. mountedRef additionally blocks post-unmount.
  const loadSeqRef = useRef(0);
  const mountedRef = useRef(true);
  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  const load = useCallback(async () => {
    const seq = ++loadSeqRef.current;
    setLoading(true);
    // Track the care-exception fetch separately: an empty list from a failed
    // fetch must NOT be treated as "no overrides", or the pickup modal could
    // clear a leg it never prefilled (see careExceptionsLoaded above).
    let exceptionsOk = true;
    try {
      const [days, exceptions, flags] = await Promise.all([
        listSickDays(studentId).catch(() => [] as StatusDay[]),
        listCareExceptions(studentId).catch(() => {
          exceptionsOk = false;
          return [] as CareException[];
        }),
        getChildFeatures(studentId).catch(() => DEFAULT_FEATURES),
      ]);
      if (!mountedRef.current || seq !== loadSeqRef.current) return;
      setSickDays(days);
      setCareExceptions(exceptions);
      setCareExceptionsLoaded(exceptionsOk);
      setFeatures(flags);
    } catch (err) {
      logger.warn("child_care_load_failed", {
        error: err instanceof Error ? err.message : String(err),
        student_id: studentId,
      });
    } finally {
      // Only the latest run owns the loading flag, so a stale load resolving
      // after a newer one can't flip it back off prematurely.
      if (mountedRef.current && seq === loadSeqRef.current) setLoading(false);
    }
  }, [studentId]);

  useEffect(() => {
    void load();
  }, [load]);

  const reportSick = useCallback(
    async (dates: string[], reason: string, status: StudentStatusKind) => {
      const updated = await submitSickNote(studentId, dates, reason, status);
      // The POST only returns the just-submitted dates. Merge them into the
      // already-loaded list (replacing any same-date entries) so previously
      // reported absences don't disappear after a non-overlapping submit.
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
    careExceptions,
    careExceptionsLoaded,
    features,
    loading,
    reportSick,
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
  onSubmit: (
    dates: string[],
    reason: string,
    status: StudentStatusKind,
  ) => Promise<void>;
}>) {
  const t = useTranslations("parentChildCare");
  const initial = todayISO();
  const [from, setFrom] = useState(initial);
  const [to, setTo] = useState(initial);
  const [status, setStatus] = useState<StudentStatusKind>("sick");
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
      await onSubmit(dates, reason, status);
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
        <label className="block">
          <span className="mb-1 block text-xs font-semibold tracking-wide text-gray-500 uppercase">
            {t("sick.kindLabel")}
          </span>
          <CustomSelect
            value={status}
            ariaLabel={t("sick.kindLabel")}
            onChange={(value) => setStatus(value as StudentStatusKind)}
            options={[
              { value: "sick", label: t("sick.kindSick") },
              { value: "excused", label: t("sick.kindExcused") },
            ]}
          />
        </label>
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
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus-visible:border-gray-400 focus-visible:ring-2 focus-visible:ring-gray-400/40 focus-visible:outline-none"
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
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus-visible:border-gray-400 focus-visible:ring-2 focus-visible:ring-gray-400/40 focus-visible:outline-none"
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
            className="w-full resize-none rounded-lg border border-gray-300 px-3 py-2 text-sm focus-visible:border-gray-400 focus-visible:ring-2 focus-visible:ring-gray-400/40 focus-visible:outline-none"
          />
        </label>
        {error && (
          <p className="rounded-lg bg-[#FF3130]/10 px-3 py-2 text-sm text-[#CC2626]">
            {error}
          </p>
        )}
        <div className="flex justify-end gap-2 pt-1">
          <Button type="button" variant="outline" size="md" onClick={onClose}>
            {t("cancel")}
          </Button>
          <Button
            type="button"
            size="md"
            className="gap-2"
            onClick={() => void handleSubmit()}
            disabled={submitting}
          >
            {submitting && (
              <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
            )}
            {t("sick.submit")}
          </Button>
        </div>
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
          <p className="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-600">
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
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus-visible:border-gray-400 focus-visible:ring-2 focus-visible:ring-gray-400/40 focus-visible:outline-none"
          />
        </label>

        {staffOwned ? (
          <p className="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-600">
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
                className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus-visible:border-gray-400 focus-visible:ring-2 focus-visible:ring-gray-400/40 focus-visible:outline-none"
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
                className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus-visible:border-gray-400 focus-visible:ring-2 focus-visible:ring-gray-400/40 focus-visible:outline-none"
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
            <Button
              type="button"
              variant="outline_danger"
              size="md"
              className="gap-2"
              onClick={() => void handleRemove()}
              disabled={submitting}
            >
              <Trash2 className="h-4 w-4" aria-hidden="true" />
              {t("pickup.reset")}
            </Button>
          ) : (
            <span />
          )}
          <div className="flex justify-end gap-2">
            <Button type="button" variant="outline" size="md" onClick={onClose}>
              {t("cancel")}
            </Button>
            <Button
              type="button"
              size="md"
              className="gap-2"
              onClick={() => void handleSubmit()}
              disabled={
                submitting ||
                staffOwned ||
                !pickupChangeEnabled ||
                !careExceptionsLoaded
              }
            >
              {submitting && (
                <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
              )}
              {t("pickup.submit")}
            </Button>
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
  return <span className="text-sm font-semibold text-gray-900">{label}</span>;
}

// --- OGS request modals (parent -> OGS structured requests) ---------------
//
// These mirror the sick/pickup self-service modals above (same Modal, same
// field/footer markup) so the whole parent action surface looks identical: calm
// neutral palette, no brand colour — the "Anfrage senden" wording and the
// chooser carry the request meaning, not colour. They own form state and call
// onSubmit(payload); the caller performs the API request and closes on success
// (throwing surfaces the error here).

// Weekday numbers match the backend (ISO: Monday=1 .. Friday=5). Labels are
// localized per-locale via t(`request.weekday.${num}`).
const REQUEST_WEEKDAYS = [1, 2, 3, 4, 5] as const;

// Empty value = "leave this weekday's departure mode unchanged". Labels are
// localized per-locale via t(`request.careMode.${key}`).
const REQUEST_CARE_MODES = [
  { value: "", key: "unchanged" },
  { value: "alone", key: "alone" },
  { value: "bus", key: "bus" },
  { value: "pickup", key: "pickup" },
] as const;

interface CareWeekdayDraft {
  mode: string;
  arrival: string;
  pickup: string;
}

// RequestModalFooter renders the shared cancel/submit buttons for the request
// modals using the kit Button (so disabled/loading states and brand styling
// come from the kit, not a hand-rolled button). It is passed to the Modal's
// `footer` slot — a sticky bar OUTSIDE the scrollable content — so "Anfrage
// senden" stays visible on short viewports instead of being clipped below the
// modal's scroll area.
function RequestModalFooter({
  submitting,
  onCancel,
  onSubmit,
}: Readonly<{
  submitting: boolean;
  onCancel: () => void;
  onSubmit: () => void;
}>) {
  const t = useTranslations("parentChildCare");
  return (
    <>
      <Button type="button" variant="outline" size="md" onClick={onCancel}>
        {t("cancel")}
      </Button>
      <Button
        type="button"
        size="md"
        className="gap-2"
        onClick={onSubmit}
        disabled={submitting}
      >
        {submitting && (
          <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
        )}
        {t("request.submit")}
      </Button>
    </>
  );
}

export function CareScheduleRequestModal({
  onClose,
  onSubmit,
}: Readonly<{
  onClose: () => void;
  onSubmit: (payload: Record<string, unknown>) => Promise<void>;
}>) {
  const t = useTranslations("parentChildCare");
  const [rows, setRows] = useState<Record<number, CareWeekdayDraft>>(() =>
    Object.fromEntries(
      REQUEST_WEEKDAYS.map((num) => [
        num,
        { mode: "", arrival: "", pickup: "" },
      ]),
    ),
  );
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const careModeOptions = REQUEST_CARE_MODES.map((m) => ({
    value: m.value,
    label: t(`request.careMode.${m.key}`),
  }));

  const setField = (
    num: number,
    field: keyof CareWeekdayDraft,
    value: string,
  ) =>
    setRows((prev) => ({ ...prev, [num]: { ...prev[num]!, [field]: value } }));

  const handleSubmit = async () => {
    const weekdays = REQUEST_WEEKDAYS.flatMap((num) => {
      const row = rows[num]!;
      if (!row.mode && !row.arrival && !row.pickup) return [];
      const entry: {
        weekday: number;
        mode?: string;
        arrival?: string;
        pickup?: string;
      } = { weekday: num };
      if (row.mode) entry.mode = row.mode;
      if (row.arrival) entry.arrival = row.arrival;
      if (row.pickup) entry.pickup = row.pickup;
      return [entry];
    });
    if (weekdays.length === 0) {
      setError(t("request.careSchedule.noChange"));
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      await onSubmit({ weekdays });
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("request.sendError"));
    } finally {
      setSubmitting(false);
    }
  };

  const timeClass =
    "w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus-visible:border-gray-400 focus-visible:ring-2 focus-visible:ring-gray-300 focus-visible:outline-none";

  return (
    <Modal
      isOpen
      onClose={onClose}
      title={t("request.careSchedule.title")}
      footer={
        <RequestModalFooter
          submitting={submitting}
          onCancel={onClose}
          onSubmit={() => void handleSubmit()}
        />
      }
    >
      <div className="space-y-4">
        <p className="text-sm leading-6 text-gray-600">
          {t("request.careSchedule.intro")}
        </p>
        <div className="space-y-3">
          {REQUEST_WEEKDAYS.map((num) => {
            const row = rows[num]!;
            const weekdayLabel = t(`request.weekday.${num}`);
            return (
              <div key={num} className="rounded-xl border border-gray-200 p-3">
                <p className="mb-2 text-sm font-semibold text-gray-900">
                  {weekdayLabel}
                </p>
                <div className="space-y-2">
                  <CustomSelect
                    value={row.mode}
                    options={careModeOptions}
                    onChange={(v) => setField(num, "mode", v)}
                    ariaLabel={t("request.careSchedule.modeAria", {
                      day: weekdayLabel,
                    })}
                  />
                  <div className="grid grid-cols-2 gap-2">
                    <label className="block">
                      <span className="mb-1 block text-xs font-medium text-gray-500">
                        {t("request.careSchedule.arrival")}
                      </span>
                      <input
                        type="time"
                        value={row.arrival}
                        onChange={(e) =>
                          setField(num, "arrival", e.target.value)
                        }
                        className={timeClass}
                      />
                    </label>
                    <label className="block">
                      <span className="mb-1 block text-xs font-medium text-gray-500">
                        {t("request.careSchedule.pickup")}
                      </span>
                      <input
                        type="time"
                        value={row.pickup}
                        onChange={(e) =>
                          setField(num, "pickup", e.target.value)
                        }
                        className={timeClass}
                      />
                    </label>
                  </div>
                </div>
              </div>
            );
          })}
        </div>
        {error && <Alert type="error" message={error} />}
      </div>
    </Modal>
  );
}

export function MasterDataRequestModal({
  onClose,
  onSubmit,
}: Readonly<{
  onClose: () => void;
  onSubmit: (payload: Record<string, unknown>) => Promise<void>;
}>) {
  const t = useTranslations("parentChildCare");
  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [birthday, setBirthday] = useState("");
  const [email, setEmail] = useState("");
  const [phone, setPhone] = useState("");
  const [note, setNote] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async () => {
    const fields: Record<string, string> = {};
    if (firstName.trim()) fields.first_name = firstName.trim();
    if (lastName.trim()) fields.last_name = lastName.trim();
    if (birthday.trim()) fields.birthday = birthday.trim();
    if (email.trim()) fields.guardian_email = email.trim();
    if (phone.trim()) fields.guardian_phone = phone.trim();
    if (note.trim()) fields.extra_info = note.trim();
    if (Object.keys(fields).length === 0) {
      setError(t("request.masterData.noField"));
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      await onSubmit({ fields });
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("request.sendError"));
    } finally {
      setSubmitting(false);
    }
  };

  const fieldClass =
    "w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus-visible:border-gray-400 focus-visible:ring-2 focus-visible:ring-gray-300 focus-visible:outline-none";

  return (
    <Modal
      isOpen
      onClose={onClose}
      title={t("request.masterData.title")}
      footer={
        <RequestModalFooter
          submitting={submitting}
          onCancel={onClose}
          onSubmit={() => void handleSubmit()}
        />
      }
    >
      <div className="space-y-4">
        <p className="text-sm leading-6 text-gray-600">
          {t("request.masterData.intro")}
        </p>
        <div className="grid gap-3 sm:grid-cols-2">
          <label className="block">
            <span className="mb-1 block text-xs font-semibold tracking-wide text-gray-500 uppercase">
              {t("request.masterData.firstName")}
            </span>
            <input
              type="text"
              value={firstName}
              onChange={(e) => setFirstName(e.target.value)}
              placeholder={t("request.masterData.firstNamePlaceholder")}
              className={fieldClass}
            />
          </label>
          <label className="block">
            <span className="mb-1 block text-xs font-semibold tracking-wide text-gray-500 uppercase">
              {t("request.masterData.lastName")}
            </span>
            <input
              type="text"
              value={lastName}
              onChange={(e) => setLastName(e.target.value)}
              placeholder={t("request.masterData.lastNamePlaceholder")}
              className={fieldClass}
            />
          </label>
        </div>
        <label className="block">
          <span className="mb-1 block text-xs font-semibold tracking-wide text-gray-500 uppercase">
            {t("request.masterData.birthday")}
          </span>
          <input
            type="date"
            value={birthday}
            onChange={(e) => setBirthday(e.target.value)}
            className={fieldClass}
          />
        </label>
        <label className="block">
          <span className="mb-1 block text-xs font-semibold tracking-wide text-gray-500 uppercase">
            {t("request.masterData.email")}
          </span>
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder={t("request.masterData.emailPlaceholder")}
            className={fieldClass}
          />
        </label>
        <label className="block">
          <span className="mb-1 block text-xs font-semibold tracking-wide text-gray-500 uppercase">
            {t("request.masterData.phone")}
          </span>
          <input
            type="tel"
            value={phone}
            onChange={(e) => setPhone(e.target.value)}
            placeholder={t("request.masterData.phonePlaceholder")}
            className={fieldClass}
          />
        </label>
        <label className="block">
          <span className="mb-1 block text-xs font-semibold tracking-wide text-gray-500 uppercase">
            {t("request.masterData.note")}
          </span>
          <textarea
            value={note}
            maxLength={MAX_NOTE_LEN}
            onChange={(e) => setNote(e.target.value)}
            rows={3}
            placeholder={t("request.masterData.notePlaceholder")}
            className={`resize-none ${fieldClass}`}
          />
        </label>
        {error && <Alert type="error" message={error} />}
      </div>
    </Modal>
  );
}

export type OgsActionKey =
  | "sick"
  | "pickup"
  | "care_schedule"
  | "student_master_data";

// A single parent action available from the OGS chat. Two deliberately separate
// groups so a parent never confuses a one-off exception with a permanent change:
// the "direct" group is self-service and takes effect immediately for a single
// day; the "request" group is an Anfrage the OGS must confirm before it changes
// anything permanently.
export interface OgsAction {
  readonly key: OgsActionKey;
  readonly Icon: LucideIcon;
  readonly enabled: boolean;
  readonly group: "direct" | "request";
}

// The SINGLE source of truth for the actions a parent can take from the OGS
// chat. Consumed by the always-visible quick-action chips above the composer
// (OgsConversation) — keep it here so the chips and any future menu can never
// drift apart. The self-service actions are gated on the school's feature flags;
// the two change-requests are gated on request_submit_enabled (the guardian's
// parent_portal.request.submit permission) so a chat-only guardian never sees an
// action the backend would reject with a 403. Display strings (label /
// shortLabel / hint) are NOT carried here — this function is not a hook, so
// consumers localize each key via t(`actions.${key}.{label,shortLabel,hint}`)
// in the "parentChildCare" namespace (mirrors the child-detail action pattern).
export function getOgsActions(features: ChildFeatures): OgsAction[] {
  return [
    {
      key: "sick",
      Icon: HeartPulse,
      enabled: features.sick_note_enabled,
      group: "direct",
    },
    {
      key: "pickup",
      Icon: CalendarClock,
      enabled: features.pickup_change_enabled,
      group: "direct",
    },
    {
      key: "care_schedule",
      Icon: CalendarRange,
      enabled: features.request_submit_enabled,
      group: "request",
    },
    {
      key: "student_master_data",
      Icon: IdCard,
      enabled: features.request_submit_enabled,
      group: "request",
    },
  ];
}

// The chooser behind the calm "Anfrage" link in the OGS chat. Self-service
// (immediate) actions live as pills next to the composer; the change requests
// are deliberately one step removed because they are rarer and consequential.
// This is where the "the OGS confirms before it takes effect" expectation is set
// in full — there is room here, unlike the cramped composer strip — so the
// distinction can never be misread. Lists the request actions from the shared
// getOgsActions source and hands the chosen key back to open the matching form.
export function RequestChooserModal({
  features,
  onPick,
  onClose,
}: Readonly<{
  features: ChildFeatures;
  onPick: (key: OgsActionKey) => void;
  onClose: () => void;
}>) {
  const t = useTranslations("parentChildCare");
  const requests = getOgsActions(features).filter(
    (action) => action.group === "request" && action.enabled,
  );
  return (
    <Modal isOpen onClose={onClose} title={t("request.chooserTitle")}>
      <div className="space-y-3">
        <p className="text-sm leading-6 text-gray-500">
          {t("request.chooserIntro")}
        </p>
        <div className="space-y-2">
          {requests.map((action) => (
            <button
              key={action.key}
              type="button"
              onClick={() => onPick(action.key)}
              className="flex w-full items-center gap-3 rounded-xl border border-gray-200 bg-white px-4 py-3 text-left transition-colors hover:border-gray-300 hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
            >
              <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-gray-100 text-gray-500">
                <action.Icon className="h-5 w-5" aria-hidden="true" />
              </span>
              <span className="min-w-0">
                <span className="block text-sm font-semibold text-gray-900">
                  {t(`actions.${action.key}.label`)}
                </span>
                <span className="block text-xs text-gray-500">
                  {t(`actions.${action.key}.hint`)}
                </span>
              </span>
            </button>
          ))}
        </div>
      </div>
    </Modal>
  );
}
