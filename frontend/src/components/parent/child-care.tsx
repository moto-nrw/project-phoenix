"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import {
  CalendarClock,
  HeartPulse,
  Loader2,
  type LucideIcon,
  Trash2,
} from "lucide-react";
import { Modal } from "~/components/ui/modal";
import { Button } from "~/components/ui/button";
import {
  type CareException,
  type ChildFeatures,
  type ExcusedRequest,
  ParentApiError,
  type StatusDay,
  type StudentStatusKind,
  deleteCareException,
  getChildFeatures,
  listCareExceptions,
  listExcusedRequests,
  listSickDays,
  submitCareException,
  submitSickNote,
  withdrawExcusedRequest,
} from "~/lib/parent-api";
import { CustomSelect } from "~/components/ui/custom-select";
import { createLogger } from "~/lib/logger";
import { parseISODate, toISODate, todayISO } from "~/lib/date-helpers";

const logger = createLogger({ component: "ChildCare" });

const MAX_SICK_DAYS = 60;
const MAX_NOTE_LEN = 2000;

// Stable empty default for SickStatusSummary's optional excusedRequests prop —
// a fresh [] literal per render would break referential equality (oxlint
// react/no-object-type-as-default-prop).
const EMPTY_EXCUSED_REQUESTS: readonly ExcusedRequest[] = [];

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
  // Default false on fetch failure: without a confirmed override we treat excused
  // absences as immediate (the pre-feature behavior), so a transient hiccup never
  // makes the UI claim an OGS confirmation is pending when it isn't. The backend
  // enforces the real gate regardless.
  excused_requires_approval: false,
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
  master_data_edit_enabled: false,
  master_data_contact_edit_enabled: false,
  master_data_request_enabled: false,
  meal_plan_enabled: false,
  // State flag defaults false so a fetch failure never shows a phantom
  // "Anfrage offen" badge on the overview.
  has_open_change_request: false,
  // Default false on fetch failure (least privilege): hide the Neuigkeiten
  // panel rather than showing an empty one for a school that has news off; the
  // backend feed enforces the gate regardless.
  parent_news_enabled: false,
};

export interface ChildCare {
  readonly sickDays: StatusDay[];
  // Excused-absence requests that went through the OGS approval gate (pending +
  // recently-decided). Empty for schools that don't gate excused absences.
  readonly excusedRequests: ExcusedRequest[];
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
  withdrawExcused(requestId: string): Promise<void>;
  saveCareException(params: {
    date: string;
    pickupTime?: string;
    arrivalTime?: string;
  }): Promise<void>;
  removeCareException(date: string): Promise<void>;
}

export function useChildCare(studentId: string): ChildCare {
  const [sickDays, setSickDays] = useState<StatusDay[]>([]);
  const [excusedRequests, setExcusedRequests] = useState<ExcusedRequest[]>([]);
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
      const [days, requests, exceptions, flags] = await Promise.all([
        listSickDays(studentId).catch(() => [] as StatusDay[]),
        // Always fetch: it's a cheap call and the response is empty for schools
        // without the approval gate, so gating it on the (separately fetched)
        // feature flag would only add an ordering dependency for no real saving.
        listExcusedRequests(studentId).catch(() => [] as ExcusedRequest[]),
        listCareExceptions(studentId).catch(() => {
          exceptionsOk = false;
          return [] as CareException[];
        }),
        getChildFeatures(studentId).catch(() => DEFAULT_FEATURES),
      ]);
      if (!mountedRef.current || seq !== loadSeqRef.current) return;
      setSickDays(days);
      setExcusedRequests(requests);
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
      const { status_days, pending_request } = await submitSickNote(
        studentId,
        dates,
        reason,
        status,
      );
      // An excused submission behind the OGS approval gate returns no status
      // days and a pending request instead — the child stays expected until the
      // OGS confirms. Prepend it so the summary shows it as "Freigabe ausstehend".
      if (pending_request) {
        setExcusedRequests((prev) => [pending_request, ...prev]);
      }
      // The POST returns only the just-recorded days (empty for a gated excused
      // submission). Merge them into the already-loaded list (replacing any
      // same-date entries) so previously reported absences don't disappear after
      // a non-overlapping submit.
      if (status_days.length > 0) {
        setSickDays((prev) => {
          const submittedDates = new Set(status_days.map((d) => d.date));
          return [
            ...prev.filter((d) => !submittedDates.has(d.date)),
            ...status_days,
          ].sort((a, b) => a.date.localeCompare(b.date));
        });
      }
    },
    [studentId],
  );

  const withdrawExcused = useCallback(
    async (requestId: string) => {
      const updated = await withdrawExcusedRequest(studentId, requestId);
      // Replace the request in place with its withdrawn form; the summary then
      // stops offering the withdraw action and drops it from the pending list.
      setExcusedRequests((prev) =>
        prev.map((r) => (r.id === updated.id ? updated : r)),
      );
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
    excusedRequests,
    careExceptions,
    careExceptionsLoaded,
    features,
    loading,
    reportSick,
    withdrawExcused,
    saveCareException,
    removeCareException,
  };
}

// --- sick-note modal ---

export function SickNoteModal({
  onClose,
  onSubmit,
  excusedRequiresApproval = false,
}: Readonly<{
  onClose: () => void;
  onSubmit: (
    dates: string[],
    reason: string,
    status: StudentStatusKind,
  ) => Promise<void>;
  // When true AND the parent picks "Entschuldigt", the absence must be confirmed
  // by the OGS before it takes effect — the modal shows a hint saying so. Mirrors
  // the backend operations.parent_excused_requires_approval gate.
  excusedRequiresApproval?: boolean;
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
  // For an excused absence the note is mandatory (the OGS needs the reason); a
  // sick note keeps it optional.
  const noteRequired = status === "excused";
  const noteMissing = noteRequired && reason.trim() === "";

  const handleSubmit = async () => {
    if (dates.length === 0) {
      setError(t("sick.invalidDate"));
      return;
    }
    if (noteMissing) {
      setError(t("sick.reasonRequiredError"));
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
        <p className="text-sm leading-6 text-gray-600">
          {excusedRequiresApproval ? t("sick.introApproval") : t("sick.intro")}
        </p>
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
        {noteRequired && excusedRequiresApproval && (
          <p className="rounded-lg border border-[#EAB308]/30 bg-[#EAB308]/10 px-3 py-2 text-sm text-[#92710b]">
            {t("sick.approvalHint")}
          </p>
        )}
        <label className="block">
          <span className="mb-1 block text-xs font-semibold tracking-wide text-gray-500 uppercase">
            {noteRequired
              ? t("sick.reasonLabelRequired")
              : t("sick.reasonLabel")}
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
            disabled={submitting || noteMissing}
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
  excusedRequests = EMPTY_EXCUSED_REQUESTS,
  onWithdraw,
}: Readonly<{
  sickDays: StatusDay[];
  // Excused requests behind the OGS approval gate. Pending ones show as
  // "Freigabe ausstehend" with a withdraw action; rejected ones show the
  // decision reason. Confirmed (approved) requests arrive as StatusDays instead.
  excusedRequests?: readonly ExcusedRequest[];
  onWithdraw?: (requestId: string) => Promise<void>;
}>) {
  const t = useTranslations("parentChildCare");
  const locale = useLocale();
  const [withdrawingId, setWithdrawingId] = useState<string | null>(null);

  const rangeLabel = (dates: readonly string[]): string => {
    const sorted = [...dates].sort((a, b) => a.localeCompare(b));
    const first = sorted.at(0);
    const last = sorted.at(-1);
    if (!first || !last) return "";
    return first === last
      ? formatLocaleDate(first, locale)
      : `${formatLocaleDate(first, locale)} – ${formatLocaleDate(last, locale)}`;
  };

  const handleWithdraw = async (requestId: string) => {
    if (!onWithdraw) return;
    setWithdrawingId(requestId);
    try {
      await onWithdraw(requestId);
    } catch (err) {
      logger.warn("excused_request_withdraw_failed", {
        error: err instanceof Error ? err.message : String(err),
        request_id: requestId,
      });
    } finally {
      setWithdrawingId(null);
    }
  };

  const sickDates = sickDays
    .filter((d) => d.status === "sick")
    .map((d) => d.date);
  const excusedConfirmedDates = sickDays
    .filter((d) => d.status === "excused")
    .map((d) => d.date);
  const pending = excusedRequests.filter((r) => r.status === "pending");
  const rejected = excusedRequests.filter((r) => r.status === "rejected");

  const hasAny =
    sickDates.length > 0 ||
    excusedConfirmedDates.length > 0 ||
    pending.length > 0 ||
    rejected.length > 0;
  if (!hasAny) {
    return <span className="text-sm text-gray-600">{t("summary.none")}</span>;
  }

  return (
    <div className="space-y-1.5">
      {sickDates.length > 0 && (
        <p className="text-sm font-semibold text-gray-900">
          {t("summary.sickLabel")}: {rangeLabel(sickDates)}
        </p>
      )}
      {excusedConfirmedDates.length > 0 && (
        <p className="text-sm font-semibold text-gray-900">
          {t("summary.excusedLabel")}: {rangeLabel(excusedConfirmedDates)}
        </p>
      )}
      {pending.map((r) => (
        <div key={r.id} className="flex flex-wrap items-center gap-x-2 gap-y-1">
          <p className="text-sm font-semibold text-gray-900">
            {t("summary.excusedLabel")}: {rangeLabel(r.dates)}
          </p>
          <span className="inline-flex items-center rounded-full bg-[#EAB308]/15 px-2 py-0.5 text-xs font-medium text-[#92710b]">
            {t("summary.pendingLabel")}
          </span>
          {onWithdraw && (
            <button
              type="button"
              disabled={withdrawingId === r.id}
              onClick={() => void handleWithdraw(r.id)}
              className="text-xs font-medium text-gray-500 underline underline-offset-2 transition-colors hover:text-gray-700 disabled:opacity-50"
            >
              {t("summary.withdraw")}
            </button>
          )}
        </div>
      ))}
      {rejected.map((r) => (
        <div key={r.id} className="space-y-0.5">
          <div className="flex flex-wrap items-center gap-x-2">
            <p className="text-sm font-semibold text-gray-900">
              {t("summary.excusedLabel")}: {rangeLabel(r.dates)}
            </p>
            <span className="inline-flex items-center rounded-full bg-[#FF3130]/10 px-2 py-0.5 text-xs font-medium text-[#CC2626]">
              {t("summary.rejectedLabel")}
            </span>
          </div>
          {r.decision_reason && (
            <p className="text-xs text-gray-500">
              {t("summary.reasonPrefix")}: {r.decision_reason}
            </p>
          )}
        </div>
      ))}
    </div>
  );
}

// --- OGS quick actions (parent self-service, immediate) --------------------
//
// The care-schedule change request FORM now lives in ./care-schedule-request-
// modal and is owned entirely by the Stammdaten page (#1803) — it is no longer
// reachable from the chat. Only the immediate self-service actions (sick note,
// one-day pickup change) remain here as quick-action pills above the composer.

export type OgsActionKey = "sick" | "pickup";

// A single parent self-service action available from the OGS chat. Each takes
// effect immediately for a single day (no OGS confirmation).
export interface OgsAction {
  readonly key: OgsActionKey;
  readonly Icon: LucideIcon;
  readonly enabled: boolean;
}

// The SINGLE source of truth for the actions a parent can take from the OGS
// chat. Consumed by the always-visible quick-action chips above the composer
// (OgsConversation) — keep it here so the chips and any future menu can never
// drift apart. The actions are gated on the school's feature flags. Display
// strings (label / shortLabel / hint) are NOT carried here — this function is
// not a hook, so consumers localize each key via
// t(`actions.${key}.{label,shortLabel,hint}`) in the "parentChildCare"
// namespace (mirrors the child-detail action pattern).
export function getOgsActions(features: ChildFeatures): OgsAction[] {
  return [
    {
      key: "sick",
      Icon: HeartPulse,
      enabled: features.sick_note_enabled,
    },
    {
      key: "pickup",
      Icon: CalendarClock,
      enabled: features.pickup_change_enabled,
    },
  ];
}
