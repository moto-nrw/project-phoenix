"use client";

import {
  useCallback,
  useEffect,
  useId,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useLocale, useTranslations } from "next-intl";
import { Loader2, Trash2 } from "lucide-react";
import type { MotoConceptKey } from "~/lib/moto-concepts";
import { Modal } from "~/components/ui/modal";
import { Button } from "~/components/ui/button";
import {
  RequestSharingControl,
  RequestSharingSelector,
} from "~/components/parent/request-sharing-control";
import { TimeField } from "~/components/ui/time-field";
import { ISODatePicker } from "~/components/ui/date-picker";
import { useLocalizedDatePicker } from "~/lib/hooks/use-localized-date-picker";
import {
  type CareException,
  type ChildCareSchedule,
  type ChildFeatures,
  type ChildToday,
  type ExcusedRequest,
  type PickupChangeRequest,
  ParentApiError,
  type StatusDay,
  type StudentStatusKind,
  deleteCareException,
  getChildCareSchedule,
  getChildFeatures,
  listCareExceptions,
  listPickupChangeRequests,
  listExcusedRequests,
  listSickDays,
  submitCareException,
  submitSickNote,
  updatePickupChangeRequest,
} from "~/lib/parent-api";
import {
  RequestEditModal,
  useRequestVersion,
} from "~/components/parent/request-edit-modal";
import { CustomSelect } from "~/components/ui/custom-select";
import { createLogger } from "~/lib/logger";
import { formatLocalizedDate } from "~/lib/localized-date-format";
import {
  berlinTodayISO,
  parseISODate,
  toISODate,
  todayISO,
} from "~/lib/date-helpers";
import { useMessagesActivity } from "~/lib/hooks/use-messages-activity";

const logger = createLogger({ component: "ChildCare" });

// Thrown by reportSick when a gated absence was created server-side but
// the follow-up reload that would surface the new pending request failed. The
// SickNoteModal maps it to a localized "submitted but couldn't refresh" hint
// rather than leaking a raw message — the submission itself succeeded (#1845).
class ChildCareRefreshError extends Error {
  constructor() {
    super("child care refresh failed");
    this.name = "ChildCareRefreshError";
  }
}

const MAX_SICK_DAYS = 60;
const MAX_NOTE_LEN = 2000;

export type AbsenceSubmissionOutcome = "applied" | "pending";

// Stable empty default for SickStatusSummary's optional excusedRequests prop —
// a fresh [] literal per render would break referential equality (oxlint
// react/no-object-type-as-default-prop).
const EMPTY_EXCUSED_REQUESTS: readonly ExcusedRequest[] = [];
const EMPTY_PICKUP_CHANGE_REQUESTS: PickupChangeRequest[] = [];

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

// True when `next` (YYYY-MM-DD) is exactly the calendar day after `prev`.
// Mirrors the staff review list's isNextDay so a Mon+Wed request is never shown
// as "Mon – Wed" (which would wrongly imply Tuesday is included too).
function isNextDayISO(prev: string, next: string): boolean {
  const d = parseISODate(prev);
  d.setDate(d.getDate() + 1);
  return toISODate(d) === next;
}

function formatLocaleDate(iso: string, locale: string): string {
  return formatLocalizedDate(iso, locale);
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
  pickup_manage_allowed: false,
  guardian_contact_manage_allowed: false,
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

// The child's effective pickup situation for TODAY, resolved from the weekly
// base plan, any date-specific override, and today's absence status. The
// "Heute → Abholung" tile renders this; every branch is a real state, never a
// fabricated value (#1725):
//   - "time":    a pickup time applies today (`changed` = a same-day override
//                differs from a KNOWN base-plan time; false when the times are
//                equal or the base plan is unavailable — we never claim a
//                difference we cannot verify)
//   - "absent":  the child is off today (sick / excused / class trip, any
//                source, per the backend today_absent signal), so no pickup
//   - "none":    it's a care day (or weekend) with no pickup configured — bus,
//                self-departure, or simply not set
//   - "unknown": an authoritative input couldn't be loaded, so we assert nothing
export type TodayPickup =
  | { readonly kind: "time"; readonly time: string; readonly changed: boolean }
  | { readonly kind: "absent" }
  | { readonly kind: "none" }
  | { readonly kind: "unknown" };

// ISO weekday (Mon=1 .. Sun=7) of a YYYY-MM-DD date, matching the care-schedule
// wire format where weekdays are 1..5.
function isoWeekday(dateISO: string): number {
  return ((parseISODate(dateISO).getDay() + 6) % 7) + 1;
}

// Pure merge of base plan + override + absence into today's pickup state. Kept
// separate from the hook so it stays trivially unit-testable.
//
// Correctness turns on which inputs actually LOADED, because a failed fetch is
// an empty array/false — indistinguishable from "nothing here" — and silently
// showing a base-plan time that a real (unloaded) override or absence would
// contradict is the bug this guards (#1725 review):
//   - `weekPlanLoaded` gates the base plan AND `todayAbsent` (both ride the
//     care-schedule response). Without it we know neither the base time nor
//     whether the child is off.
//   - `careExceptionsLoaded` gates the override list. Without it we cannot rule
//     out a same-day change, so we must not fall through to a base-plan time.
//
// Order matters: an absence (when known) wins over any configured time, and a
// LOADED same-day override with a pickup time is authoritative even if the base
// plan never loaded — we just can't call it "changed" without a base to compare.
export function resolveTodayPickup(params: {
  readonly weekdays: ChildCareSchedule["weekdays"];
  readonly weekPlanLoaded: boolean;
  // Authoritative all-source absence for today (sick / excused / class trip),
  // computed server-side. Only meaningful when weekPlanLoaded (same response).
  readonly todayAbsent: boolean;
  readonly careExceptions: readonly CareException[];
  readonly careExceptionsLoaded: boolean;
  readonly today: string;
}): TodayPickup {
  const {
    weekdays,
    weekPlanLoaded,
    todayAbsent,
    careExceptions,
    careExceptionsLoaded,
    today,
  } = params;

  // Absence wins over any configured time — but only when we actually loaded the
  // signal (it rides the care-schedule response).
  if (weekPlanLoaded && todayAbsent) return { kind: "absent" };

  // Without the override list we can't distinguish "no override" from "fetch
  // failed", so we can't safely assert a base-plan time or "none" (#1725 review).
  if (!careExceptionsLoaded) return { kind: "unknown" };

  // A CareException overrides arrival and pickup independently, so a missing
  // pickup_time normally means "pickup not changed" (fall back to the base
  // plan), NOT "no pickup". Absence is expressed through today_absent above —
  // EXCEPT a staff "not coming today" exception (a pickup row with no time,
  // pickup_absent, OR an arrival row with no time, arrival_absent). Neither
  // creates a status day, so today_absent misses both; either leg being absent
  // resolves as an absence here rather than showing the base-plan pickup — a
  // guardian must never be told to expect a pickup for a child who is not
  // coming (#1725 review).
  const override = careExceptions.find((entry) => entry.date === today);
  if (override?.pickup_absent || override?.arrival_absent)
    return { kind: "absent" };
  if (override?.pickup_time) {
    // "changed" only when a LOADED base plan exists to differ FROM and the times
    // actually differ; an override equal to the plan is no real change, and a
    // missing base plan means we can't claim a difference at all (#1725 review).
    // Gate on weekPlanLoaded, not merely `base !== undefined`: a failed or
    // midnight-stale care-schedule fetch keeps the PREVIOUS weekdays array live
    // (setWeekdays only runs on success), so comparing an override against that
    // stale plan could wrongly flag "changed". When the plan isn't loaded we
    // treat the base as unknown and never mark a difference (#1725 review).
    const base = weekPlanLoaded
      ? weekdays.find((entry) => entry.weekday === isoWeekday(today))?.pickup
      : undefined;
    const changed = base !== undefined && base !== override.pickup_time;
    return { kind: "time", time: override.pickup_time, changed };
  }

  if (!weekPlanLoaded) return { kind: "unknown" };

  const base = weekdays.find((entry) => entry.weekday === isoWeekday(today));
  if (base?.pickup) return { kind: "time", time: base.pickup, changed: false };
  return { kind: "none" };
}

export interface ChildCare {
  readonly sickDays: StatusDay[];
  // Absence requests that went through the OGS approval gate (pending +
  // recently decided). Empty for schools that gate neither absence type.
  readonly excusedRequests: ExcusedRequest[];
  readonly careExceptions: CareException[];
  readonly pickupChangeRequests: PickupChangeRequest[];
  // Whether the care-exception list actually loaded. A failed fetch leaves
  // careExceptions empty, which is indistinguishable from "no overrides exist"
  // — and submitCareException treats an omitted leg as an authoritative clear.
  // The pickup modal must block saving while this is false so a parent can't
  // silently wipe an existing override the UI never managed to prefill.
  readonly careExceptionsLoaded: boolean;
  readonly pickupChangeRequestsLoaded: boolean;
  // Today's resolved pickup state for the "Heute" tile (see TodayPickup).
  readonly todayPickup: TodayPickup;
  readonly features: ChildFeatures;
  readonly loading: boolean;
  reportSick(
    dates: string[],
    reason: string,
    status: StudentStatusKind,
    recipientGuardianProfileIds?: string[],
  ): Promise<AbsenceSubmissionOutcome | void>;
  saveCareException(params: {
    date: string;
    pickupTime: string;
    reason: string;
  }): Promise<void | string>;
  removeCareException(date: string): Promise<void>;
  /** Lädt die Daten des Kindes neu, etwa nach einer geänderten Anfrage. */
  refresh(): void;
}

export function useChildCare(studentId: string): ChildCare {
  const [sickDays, setSickDays] = useState<StatusDay[]>([]);
  const [excusedRequests, setExcusedRequests] = useState<ExcusedRequest[]>([]);
  const [careExceptions, setCareExceptions] = useState<CareException[]>([]);
  const [careExceptionsLoaded, setCareExceptionsLoaded] = useState(false);
  const [pickupChangeRequests, setPickupChangeRequests] = useState<
    PickupChangeRequest[]
  >([]);
  const [pickupChangeRequestsLoaded, setPickupChangeRequestsLoaded] =
    useState(false);
  // The child's standard weekly plan, used only to resolve today's base pickup
  // time for the "Heute" tile. weekPlanLoaded stays false on a failed fetch so
  // the tile shows a neutral state instead of falsely claiming "no pickup".
  const [weekdays, setWeekdays] = useState<ChildCareSchedule["weekdays"]>([]);
  const [weekPlanLoaded, setWeekPlanLoaded] = useState(false);
  // The child's authoritative all-source absence for today (sick / excused /
  // class trip), carried on the care-schedule response. Only trusted when
  // weekPlanLoaded — it rides that same fetch.
  const [todayAbsent, setTodayAbsent] = useState(false);
  // The Berlin day the week-plan signal (todayAbsent above) was resolved for.
  // The server computes today_absent against ITS current day, so a response is
  // only valid while that day is still "today". A tab left open across midnight
  // advances `today` (below) before the reload lands — without this stamp the
  // stale absence boolean would be applied to the new date, asserting a wrong
  // absence or pickup until the reload finishes, and indefinitely if it hangs
  // (#1725 review). null until the first successful load.
  const [weekPlanDate, setWeekPlanDate] = useState<string | null>(null);
  const [features, setFeatures] = useState<ChildFeatures>(DEFAULT_FEATURES);
  const [loading, setLoading] = useState(true);
  // Today's calendar day in the SCHOOL's timezone (Europe/Berlin) — the axis the
  // backend resolves absences/overrides against. A guardian's browser may sit in
  // another timezone, so deriving "today" from the local clock can land on the
  // wrong weekday around midnight. Kept in state (not recomputed inline) so a tab
  // left open across midnight advances instead of freezing on yesterday (#1725
  // review) — see the rollover effect below.
  const [today, setToday] = useState(() => berlinTodayISO());
  // Stale-response guard: load re-runs on every studentId change, and on a fast
  // child A→B switch (same hook instance reused) a late-resolving load(A) must
  // not overwrite B's data — one child's sick days / care exceptions / feature
  // flags shown under another would mis-set the pickup-modal safety gate. Each
  // run claims the next token; only the most-recently-started run may setState.
  // Mirrors OgsConversation.refresh. mountedRef additionally blocks post-unmount.
  const loadSeqRef = useRef(0);
  const mountedRef = useRef(true);
  // The child this hook instance currently serves. A mutation (report sick,
  // save/remove pickup override, edit request) captures the studentId in its
  // closure at call time, then awaits a POST; if the guardian navigates A→B
  // during that await the SAME hook instance is reused for B (only the prop
  // changes — see ChildDetailContent). Applying A's result would setState on B —
  // most damagingly mark B absent with A's just-submitted absence, which A's
  // later SSE event (filtered by studentId) never corrects. This ref is bumped
  // SYNCHRONOUSLY in the reset block below, so a resolving A-write reads the new
  // identity and bails before touching B's state (#1725 review).
  const currentStudentIdRef = useRef(studentId);
  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  // Reset all per-child state before paint when studentId changes. The returned
  // values are also masked below until this layout effect commits, so data from
  // the previous child can never render under the new child's name.
  const [loadedStudentId, setLoadedStudentId] = useState(studentId);
  // This state is an ownership guard for asynchronous writes, not UI-derived
  // state: it deliberately changes only after the old child's data is masked.
  useLayoutEffect(() => {
    if (loadedStudentId === studentId) return;
    setLoadedStudentId(studentId);
    loadSeqRef.current += 1;
    currentStudentIdRef.current = studentId;
    setSickDays([]);
    setExcusedRequests([]);
    setCareExceptions([]);
    setCareExceptionsLoaded(false);
    setPickupChangeRequests([]);
    setPickupChangeRequestsLoaded(false);
    setWeekdays([]);
    setWeekPlanLoaded(false);
    setTodayAbsent(false);
    setWeekPlanDate(null);
    setFeatures(DEFAULT_FEATURES);
    setLoading(true);
  }, [loadedStudentId, studentId]);

  const hasCurrentStudentData = loadedStudentId === studentId;

  const load = useCallback(async (): Promise<{
    requestsOk: boolean;
    requests: ExcusedRequest[];
  }> => {
    const seq = ++loadSeqRef.current;
    setLoading(true);
    // Track each fetch's success separately so a failed one PRESERVES the last
    // known list instead of wiping it to []. An empty list from a failed fetch
    // is indistinguishable from "nothing here" and would erase already-shown
    // sick days / pending requests / care overrides on a transient hiccup — the
    // pickup modal also relies on careExceptionsLoaded to avoid clearing a leg it
    // never prefilled, and a gated sick-note submit relies on requestsOk to know
    // whether the freshly created request was actually loaded (#1845 review).
    let sickOk = true;
    let requestsOk = true;
    let exceptionsOk = true;
    let pickupRequestsOk = true;
    let weekPlanOk = true;
    try {
      const [days, requests, exceptions, pickupRequests, plan, flags] =
        await Promise.all([
          listSickDays(studentId).catch(() => {
            sickOk = false;
            return [] as StatusDay[];
          }),
          // Always fetch: it's a cheap call and the response is empty for schools
          // without the approval gate, so gating it on the (separately fetched)
          // feature flag would only add an ordering dependency for no real saving.
          listExcusedRequests(studentId).catch(() => {
            requestsOk = false;
            return [] as ExcusedRequest[];
          }),
          listCareExceptions(studentId).catch(() => {
            exceptionsOk = false;
            return [] as CareException[];
          }),
          listPickupChangeRequests(studentId).catch(() => {
            pickupRequestsOk = false;
            return [] as PickupChangeRequest[];
          }),
          getChildCareSchedule(studentId).catch(() => {
            weekPlanOk = false;
            return null;
          }),
          // Approval settings have no safe fallback: keep them unknown when
          // the feature request fails so the modal uses neutral wording.
          getChildFeatures(studentId).catch(() => DEFAULT_FEATURES),
        ]);
      if (!mountedRef.current || seq !== loadSeqRef.current)
        return { requestsOk, requests };
      // Only overwrite a list whose fetch succeeded; keep the previous state
      // otherwise (see above).
      if (sickOk) setSickDays(days);
      if (requestsOk) setExcusedRequests(requests);
      if (exceptionsOk) setCareExceptions(exceptions);
      setCareExceptionsLoaded(exceptionsOk);
      if (pickupRequestsOk) setPickupChangeRequests(pickupRequests);
      setPickupChangeRequestsLoaded(pickupRequestsOk);
      if (weekPlanOk && plan) {
        setWeekdays(plan.weekdays);
        setTodayAbsent(plan.today_absent);
        // Stamp the day this signal describes with the SERVER-resolved date
        // (the day the backend computed today_absent against), not a
        // client-captured request-start day. If the request crosses Berlin
        // midnight — start just before, handled just after — the two disagree
        // and the client date would bind the new day's today_absent onto
        // yesterday's tile until the rollover poll runs. Fall back to the local
        // Berlin day only for an older backend that omits today_date (#1725
        // review).
        setWeekPlanDate(plan.today_date ?? berlinTodayISO());
      }
      setWeekPlanLoaded(weekPlanOk);
      setFeatures(flags);
      return { requestsOk, requests };
    } catch (err) {
      logger.warn("child_care_load_failed", {
        error: err instanceof Error ? err.message : String(err),
        student_id: studentId,
      });
      return { requestsOk: false, requests: [] };
    } finally {
      // Only the latest run owns the loading flag, so a stale load resolving
      // after a newer one can't flip it back off prematurely.
      if (mountedRef.current && seq === loadSeqRef.current) setLoading(false);
    }
  }, [studentId]);

  // Reload on mount, on a studentId change (load identity), AND when the Berlin
  // calendar day rolls over while the tab stays open — the windowed lists and
  // today_absent are all resolved against a specific day, so a stale `today`
  // would otherwise show yesterday's absence/override indefinitely (#1725 review).
  useEffect(() => {
    void load();
  }, [load, today]);

  // Advance `today` at the Berlin midnight boundary. A 60s poll (rather than a
  // single timeout to next midnight) keeps this correct regardless of the
  // browser's own timezone and across DST shifts; the functional updater returns
  // the previous value on an unchanged day so React bails out with no re-render.
  useEffect(() => {
    const id = setInterval(() => {
      const current = berlinTodayISO();
      setToday((prev) => (prev === current ? prev : current));
    }, 60_000);
    return () => clearInterval(id);
  }, []);

  // A parent write or a staff decision on an absence request flips the request's
  // state / writes status days server-side but emits only an SSE trigger — no
  // payload and no local state change here. The portal-wide ParentRealtimeBridge
  // turns the backend's message-INDEPENDENT parent_child_updated event into a
  // `parent-conversation-refresh` window event (carrying the affected studentId);
  // refetch on a match so an open parent tab drops a resolved "Freigabe ausstehend",
  // shows a rejection reason, or surfaces a newly approved absence in real time
  // without a manual reload. That event fans out to EVERY guardian of the child and
  // fires regardless of whether parent messaging is enabled (#1845 review), so it
  // reaches a second guardian's tab and a messaging-off school too — the gaps the
  // old parent_message-only path had (submitter-only, suppressed when messaging is
  // off). marksRead:false — load() reads no messages and advances no read cursor,
  // so it fires even in a background tab. refetchOnFocus:true stays as the fallback
  // for a tab whose SSE stream had dropped entirely (no event ever arrives), mirroring
  // the conversation views' focus healing.
  const refreshChildCare = useCallback(() => void load(), [load]);
  useMessagesActivity({
    eventName: "parent-conversation-refresh",
    studentId,
    onMatch: refreshChildCare,
    marksRead: false,
    refetchOnFocus: true,
  });

  const reportSick = useCallback(
    async (
      dates: string[],
      reason: string,
      status: StudentStatusKind,
      recipientGuardianProfileIds: string[] = [],
    ) => {
      const { status_days } = await submitSickNote(
        studentId,
        dates,
        reason,
        status,
        recipientGuardianProfileIds,
      );
      // If the guardian navigated to another child mid-submit, this hook now
      // serves child B: applying A's status days (above all the optimistic
      // setTodayAbsent below) would mark B absent, and A's own later SSE event
      // is filtered out by studentId so B would stay absent indefinitely. The
      // write already committed server-side for A; A's tab reflects it (#1725
      // review). Bail before any setState.
      if (currentStudentIdRef.current !== studentId) return;
      // A direct write (sick note, or an excused absence without the approval
      // gate) returns the just-recorded days. Merge them into the already-loaded
      // list (replacing any same-date entries) so previously reported absences
      // don't disappear after a non-overlapping submit.
      if (status_days.length > 0) {
        setSickDays((prev) => {
          const submittedDates = new Set(status_days.map((d) => d.date));
          return [
            ...prev.filter((d) => !submittedDates.has(d.date)),
            ...status_days,
          ].sort((a, b) => a.date.localeCompare(b.date));
        });
        // If the report covers today, the child is absent NOW. todayPickup reads
        // todayAbsent exclusively, so update it here too — otherwise the "Heute →
        // Abholung" tile keeps showing the normal pickup time next to the
        // just-reported absence until an unrelated reload. Both submittable
        // statuses (sick/excused) count as an absence, so a today status day is
        // sufficient; the authoritative server signal agrees on the next load
        // (#1725 review). Only meaningful once the week plan loaded, but setting
        // it early is harmless — the tile shows "unknown" until then. Re-stamp
        // the signal's date to today so the midnight-rollover guard keeps trusting
        // this optimistic absence (we're asserting today's, which we just wrote).
        if (status_days.some((d) => d.date === berlinTodayISO())) {
          setTodayAbsent(true);
          setWeekPlanDate(berlinTodayISO());
        }
        return "applied" as const;
      }
      // Empty status days means the school gated this absence: the backend
      // created a pending request instead. Reload authoritatively to surface
      // the new "Freigabe ausstehend" request. An authoritative reload (rather
      // than an optimistic prepend) also avoids the duplicate-row race the
      // prepend had with a load() that an after-commit parent_message could
      // trigger before this POST resolved.
      const { requestsOk } = await load();
      if (!requestsOk) {
        // The request WAS created server-side, but the reload that would surface
        // it failed — closing silently would leave the parent with no pending
        // confirmation, as if nothing happened. Propagate so the modal stays open
        // and says so. A retry is safe: the backend treats an identical resubmit
        // idempotently, so it won't create a duplicate request (#1845 review).
        throw new ChildCareRefreshError();
      }
      return "pending" as const;
    },
    [studentId, load],
  );

  const saveCareException = useCallback(
    async (params: { date: string; pickupTime: string; reason: string }) => {
      const saved = await submitCareException(studentId, params);
      // Bail if we navigated to another child mid-save — A's override must not
      // land in B's exception list (#1725 review).
      if (currentStudentIdRef.current !== studentId) return;
      setPickupChangeRequests((prev) => [
        saved,
        ...prev.filter((request) => request.id !== saved.id),
      ]);
      return saved.id;
    },
    [studentId],
  );

  const removeCareException = useCallback(
    async (date: string) => {
      await deleteCareException(studentId, date);
      // Bail if we navigated to another child mid-delete — the removal must not
      // apply to B's exception list (#1725 review).
      if (currentStudentIdRef.current !== studentId) return;
      setCareExceptions((prev) => prev.filter((e) => e.date !== date));
    },
    [studentId],
  );

  const todayPickup = useMemo(
    () =>
      resolveTodayPickup({
        weekdays: hasCurrentStudentData ? weekdays : [],
        // The week-plan signal (base plan AND todayAbsent) is only valid for the
        // day it was resolved for. If the tab crossed midnight, `today` advanced
        // ahead of the reload; treat the signal as not-yet-loaded for the new
        // date so the tile shows "unknown" (asserting nothing) instead of a stale
        // absence or pickup — safe even if the reload hangs (#1725 review).
        weekPlanLoaded:
          hasCurrentStudentData && weekPlanLoaded && weekPlanDate === today,
        todayAbsent: hasCurrentStudentData && todayAbsent,
        careExceptions: hasCurrentStudentData ? careExceptions : [],
        careExceptionsLoaded: hasCurrentStudentData && careExceptionsLoaded,
        today,
      }),
    [
      weekdays,
      weekPlanLoaded,
      weekPlanDate,
      todayAbsent,
      careExceptions,
      careExceptionsLoaded,
      hasCurrentStudentData,
      today,
    ],
  );

  return {
    sickDays: hasCurrentStudentData ? sickDays : [],
    excusedRequests: hasCurrentStudentData ? excusedRequests : [],
    careExceptions: hasCurrentStudentData ? careExceptions : [],
    pickupChangeRequests: hasCurrentStudentData ? pickupChangeRequests : [],
    careExceptionsLoaded: hasCurrentStudentData && careExceptionsLoaded,
    pickupChangeRequestsLoaded:
      hasCurrentStudentData && pickupChangeRequestsLoaded,
    todayPickup,
    features: hasCurrentStudentData ? features : DEFAULT_FEATURES,
    loading: !hasCurrentStudentData || loading,
    reportSick,
    saveCareException,
    removeCareException,
    refresh: refreshChildCare,
  };
}

// --- sick-note modal ---

function resolveSickError(
  err: unknown,
  refreshFailed: string,
  overlap: string,
  save: string,
): string {
  if (err instanceof ChildCareRefreshError) return refreshFailed;
  if (err instanceof ParentApiError && err.code === "excused_request_overlap") {
    return overlap;
  }
  return err instanceof Error ? err.message : save;
}

export function SickNoteModal({
  studentId,
  onClose,
  onSubmit,
  sickRequiresApproval,
  excusedRequiresApproval,
  reasonRequired = true,
}: Readonly<{
  studentId?: string;
  onClose: () => void;
  onSubmit: (
    dates: string[],
    reason: string,
    status: StudentStatusKind,
    recipientGuardianProfileIds?: string[],
  ) => Promise<AbsenceSubmissionOutcome | void>;
  sickRequiresApproval?: boolean;
  excusedRequiresApproval?: boolean;
  /** Ob die OGS einen Grund verlangt (requiresGuardianReason). */
  reasonRequired?: boolean;
}>) {
  const t = useTranslations("parentChildCare");
  const datePicker = useLocalizedDatePicker();
  const initial = todayISO();
  const [from, setFrom] = useState(initial);
  const [to, setTo] = useState(initial);
  const [status, setStatus] = useState<StudentStatusKind>("sick");
  const [reason, setReason] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [submitted, setSubmitted] = useState(false);
  const [recipientIds, setRecipientIds] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);
  const reasonRef = useRef<HTMLTextAreaElement>(null);
  const errorId = useId();
  const dates = useMemo(() => enumerateDates(from, to), [from, to]);
  const noteMissing = reason.trim() === "";
  const requiresApproval =
    status === "sick" ? sickRequiresApproval : excusedRequiresApproval;
  const kindHint =
    status === "sick"
      ? requiresApproval === false
        ? "sick.kindHintSick"
        : "sick.kindHintSickUnknown"
      : requiresApproval === false
        ? "sick.kindHintExcused"
        : "sick.kindHintExcusedUnknown";
  const dayCopy =
    requiresApproval === undefined
      ? t("sick.unknownDaysCount", { count: dates.length })
      : requiresApproval
        ? t("sick.requestDaysCount")
        : t("sick.daysCount", { count: dates.length });

  const handleSubmit = async () => {
    if (dates.length === 0) {
      setError(t("sick.invalidDate"));
      return;
    }
    if (reasonRequired && noteMissing) {
      setError(t("sick.reasonRequiredError"));
      reasonRef.current?.focus();
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      const outcome = await onSubmit(dates, reason, status, recipientIds);
      if (outcome === "pending") setSubmitted(true);
      else onClose();
    } catch (err) {
      setError(
        resolveSickError(
          err,
          t("sick.submittedButRefreshFailed"),
          t("sick.overlapError"),
          t("sick.saveError"),
        ),
      );
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      isOpen
      onClose={onClose}
      title={submitted ? t("sick.requestSentTitle") : t("sick.title")}
      closeLabel={t("close")}
      mobileSheet
      footer={
        submitted ? undefined : (
          <>
            <Button
              type="button"
              variant="outline"
              size="md"
              className="hidden sm:inline-flex"
              onClick={onClose}
            >
              {t("cancel")}
            </Button>
            <Button
              type="button"
              size="md"
              className="w-full gap-2 sm:w-auto"
              onClick={() => void handleSubmit()}
              disabled={submitting}
            >
              {submitting && (
                <Loader2 className="size-4 animate-spin" aria-hidden="true" />
              )}
              {status === "sick"
                ? t("sick.submitSick")
                : t("sick.submitExcused")}
            </Button>
          </>
        )
      }
    >
      {submitted ? (
        <p role="status" className="text-sm leading-6 text-gray-700">
          {t("sick.requestSentBody")}
        </p>
      ) : (
        <div className="space-y-4">
          <p className="text-sm leading-6 text-gray-700">{t("sick.intro")}</p>
          <label className="block">
            <span className="mb-1 block text-sm font-medium text-gray-700">
              {t("sick.kindLabel")}
            </span>
            <CustomSelect
              value={status}
              ariaLabel={t("sick.kindLabel")}
              onChange={(value) => {
                setStatus(value as StudentStatusKind);
                setError(null);
              }}
              options={[
                { value: "sick", label: t("sick.kindSick") },
                { value: "excused", label: t("sick.kindExcused") },
              ]}
            />
          </label>
          <p className="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm leading-5 text-gray-600">
            {t(kindHint)}
          </p>
          <fieldset className="space-y-2">
            <legend className="text-sm font-medium text-gray-700">
              {t("sick.periodLabel")}
            </legend>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <label className="block">
                <span className="mb-1 block text-sm font-medium text-gray-700">
                  {t("sick.from")}
                </span>
                <ISODatePicker
                  {...datePicker}
                  ariaLabel={t("sick.from")}
                  value={from}
                  min={initial}
                  onChange={(next) => {
                    setFrom(next);
                    if (next > to) setTo(next);
                  }}
                  calendarLayout="popover"
                  hideClearButton
                />
              </label>
              <label className="block">
                <span className="mb-1 block text-sm font-medium text-gray-700">
                  {t("sick.to")}
                </span>
                <ISODatePicker
                  {...datePicker}
                  ariaLabel={t("sick.to")}
                  value={to}
                  min={from}
                  onChange={setTo}
                  calendarLayout="popover"
                  hideClearButton
                />
              </label>
            </div>
          </fieldset>
          <p className="text-sm text-gray-600">{dayCopy}</p>
          <label className="block">
            <span className="mb-1 block text-sm font-medium text-gray-700">
              {t("sick.reasonLabelRequired")}
              {reasonRequired && <span aria-hidden="true"> *</span>}
            </span>
            <textarea
              ref={reasonRef}
              value={reason}
              maxLength={MAX_NOTE_LEN}
              onChange={(event) => {
                setReason(event.target.value);
                if (error) setError(null);
              }}
              name="absence-reason"
              autoComplete="off"
              required={reasonRequired}
              rows={3}
              placeholder={t("sick.reasonPlaceholder")}
              aria-invalid={noteMissing && Boolean(error)}
              aria-describedby={noteMissing && error ? errorId : undefined}
              className="min-h-20 w-full resize-none rounded-lg border border-gray-300 bg-white px-3 py-2 text-base shadow-sm transition-colors hover:border-gray-400 focus-visible:border-gray-400 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
            />
          </label>
          {requiresApproval === true && studentId && (
            <RequestSharingSelector
              studentId={studentId}
              selected={recipientIds}
              onChange={setRecipientIds}
            />
          )}
          {error && (
            <p
              id={errorId}
              role="alert"
              className="bg-parent-red-soft text-parent-red-strong rounded-lg px-3 py-2 text-sm"
            >
              {error}
            </p>
          )}
        </div>
      )}
    </Modal>
  );
}

// --- pickup-time change modal ---

export function PickupTimeModal({
  studentId,
  careExceptions,
  pickupChangeRequests = EMPTY_PICKUP_CHANGE_REQUESTS,
  careExceptionsLoaded,
  pickupChangeRequestsLoaded = true,
  pickupChangeEnabled,
  childFirstName,
  today: childToday,
  onClose,
  onSubmit,
  onRemove,
  reasonRequired = true,
}: Readonly<{
  studentId?: string;
  careExceptions: CareException[];
  pickupChangeRequests?: PickupChangeRequest[];
  careExceptionsLoaded: boolean;
  pickupChangeRequestsLoaded?: boolean;
  pickupChangeEnabled: boolean;
  childFirstName?: string;
  today?: ChildToday;
  onClose: () => void;
  onSubmit: (params: {
    date: string;
    pickupTime: string;
    reason: string;
    recipientGuardianProfileIds?: string[];
  }) => Promise<void | string>;
  onRemove: (date: string) => Promise<void>;
  /** Ob die OGS einen Grund verlangt (requiresGuardianReason). */
  reasonRequired?: boolean;
}>) {
  const t = useTranslations("parentChildCare");
  const locale = useLocale();
  const datePicker = useLocalizedDatePicker();
  const today = berlinTodayISO();
  // Ist das Aendern abgeschaltet, oeffnet der Dialog auf dem einzigen Tag, den
  // die Eltern noch bearbeiten koennen. Ein offener Antrag geht dabei vor: er
  // ist der Tag mit dem Zuruecknehmen-Knopf, und ohne diese Vorauswahl stuende
  // der Dialog auf heute, wo es zu dem Antrag nichts zu sehen gibt.
  const initial = useMemo(() => {
    if (pickupChangeEnabled) return today;
    const pendingRequest = pickupChangeRequests.find(
      (request) => request.status === "pending",
    );
    if (pendingRequest) return pendingRequest.date;
    return (
      careExceptions.find((entry) => entry.pickup_source === "guardian")
        ?.date ?? today
    );
  }, [careExceptions, pickupChangeEnabled, pickupChangeRequests, today]);
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
  const [reason, setReason] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [recipientIds, setRecipientIds] = useState<string[]>([]);
  const [editing, setEditing] = useState(false);
  const [invalidField, setInvalidField] = useState<
    "pickupTime" | "reason" | null
  >(null);
  const pickupTimeRef = useRef<HTMLInputElement>(null);
  const reasonRef = useRef<HTMLTextAreaElement>(null);
  const errorId = useId();

  const existing = useMemo(
    () => careExceptions.find((e) => e.date === date),
    [careExceptions, date],
  );
  const request = useMemo(
    () => pickupChangeRequests.find((entry) => entry.date === date),
    [date, pickupChangeRequests],
  );
  const pending = request?.status === "pending";
  const staffOwned = existing?.pickup_source === "staff";
  const alreadyHome = date === today && childToday?.state === "left";
  // Eine offene eigene Anfrage wird geändert, nicht zurückgezogen (#2267).
  const canEditRequest =
    pending && studentId !== undefined && request?.is_self !== false;
  const { version, editedAt } = useRequestVersion(
    studentId ?? "",
    "pickup_change",
    request?.id ?? "",
    canEditRequest === true,
  );

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
        case "care_exception_already_left":
          return childFirstName
            ? t("pickup.alreadyHome", { name: childFirstName })
            : t("pickup.alreadyHomeGeneric");
        case "care_exception_raced":
          return t("pickup.errorRaced");
        case "care_exception_past_date":
          return t("pickup.errorPastDate");
        case "care_exception_too_far":
          return t("pickup.errorTooFar");
        case "care_exception_no_time":
          return t("pickup.noTime");
        case "care_exception_reason_required":
          return t("pickup.reasonRequired");
        case "care_exception_reason_too_long":
          return t("pickup.reasonTooLong");
        case "care_request_already_pending":
          return t("pickup.statusPending");
      }
    }
    return err instanceof Error ? err.message : t("pickup.saveError");
  };

  // When the selected date already has an override, prefill the fields so the
  // parent edits rather than blindly overwrites.
  useEffect(() => {
    setPickupTime(request?.pickup_time ?? existing?.pickup_time ?? "");
    setReason(request?.reason ?? existing?.reason ?? "");
    setError(null);
    setInvalidField(null);
  }, [existing, request]);

  const handleSubmit = async () => {
    // Guard: if the existing overrides never loaded we can't trust the
    // prefilled fields, and a save would send the empty leg as an authoritative
    // clear. Block until the list is known (the page must be reloaded).
    if (!careExceptionsLoaded || !pickupChangeRequestsLoaded) {
      setError(t("pickup.loadError"));
      return;
    }
    if (!pickupTime) {
      setError(t("pickup.noTime"));
      setInvalidField("pickupTime");
      pickupTimeRef.current?.focus();
      return;
    }
    if (reasonRequired && !reason.trim()) {
      setError(t("pickup.reasonRequired"));
      setInvalidField("reason");
      reasonRef.current?.focus();
      return;
    }
    setSubmitting(true);
    setError(null);
    setInvalidField(null);
    try {
      await onSubmit({
        date,
        pickupTime,
        reason: reason.trim(),
        recipientGuardianProfileIds: recipientIds,
      });
      onClose();
    } catch (err) {
      setError(resolveError(err));
    } finally {
      setSubmitting(false);
    }
  };

  // Ändern einer noch offenen eigenen Anfrage. Die Empfänger bleiben, wie sie
  // beim Senden festgelegt wurden; sie werden über "Anfrage teilen" geändert.
  const handleSaveEdit = async () => {
    if (!studentId || !request) return;
    if (!pickupTime) {
      setError(t("pickup.noTime"));
      setInvalidField("pickupTime");
      pickupTimeRef.current?.focus();
      return;
    }
    if (reasonRequired && !reason.trim()) {
      setError(t("pickup.reasonRequired"));
      setInvalidField("reason");
      reasonRef.current?.focus();
      return;
    }
    setSubmitting(true);
    setError(null);
    setInvalidField(null);
    try {
      await updatePickupChangeRequest(studentId, request.id, {
        date,
        pickupTime,
        reason: reason.trim(),
        expectedVersion: version,
      });
      onClose();
    } catch (err) {
      setError(
        err instanceof ParentApiError && err.code === "change_request_stale"
          ? t("pickup.staleError")
          : resolveError(err),
      );
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
      mobileSheet
      footer={
        <>
          {!pending && existing?.pickup_source === "guardian" && (
            <Button
              type="button"
              variant="ghost"
              size="md"
              className="w-full gap-2 whitespace-nowrap sm:w-auto"
              onClick={() => void handleRemove()}
              disabled={submitting || alreadyHome}
            >
              <Trash2 className="size-4" aria-hidden="true" />
              {t("pickup.reset")}
            </Button>
          )}
          <Button
            type="button"
            variant="outline"
            size="md"
            className="hidden whitespace-nowrap shadow-sm sm:inline-flex"
            onClick={onClose}
          >
            {t("cancel")}
          </Button>
          {editing ? (
            <Button
              type="button"
              size="md"
              className="w-full gap-2 whitespace-nowrap shadow-sm max-sm:min-h-11 sm:w-auto"
              onClick={() => void handleSaveEdit()}
              disabled={submitting}
            >
              {submitting && (
                <Loader2 className="size-4 animate-spin" aria-hidden="true" />
              )}
              {t("pickup.saveEdit")}
            </Button>
          ) : pending ? null : (
            <Button
              type="button"
              size="md"
              className="w-full gap-2 whitespace-nowrap shadow-sm sm:w-auto"
              onClick={() => void handleSubmit()}
              disabled={
                submitting ||
                alreadyHome ||
                staffOwned ||
                !pickupChangeEnabled ||
                !careExceptionsLoaded ||
                !pickupChangeRequestsLoaded
              }
            >
              {submitting && (
                <Loader2 className="size-4 animate-spin" aria-hidden="true" />
              )}
              {t("pickup.submit")}
            </Button>
          )}
        </>
      }
    >
      <div className="space-y-4">
        <p className="text-sm leading-6 text-gray-700">{t("pickup.intro")}</p>
        {(!careExceptionsLoaded || !pickupChangeRequestsLoaded) && (
          <p className="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-600">
            {t("pickup.loadError")}
          </p>
        )}
        <label className="block">
          <span className="mb-1 block text-sm font-medium text-gray-700">
            {t("pickup.dateLabel")}
          </span>
          <ISODatePicker
            {...datePicker}
            ariaLabel={t("pickup.dateLabel")}
            value={date}
            min={today}
            max={maxSelectable}
            onChange={setDate}
            calendarLayout="popover"
            hideClearButton
          />
        </label>

        {pending && (
          <div className="bg-moto-orange-soft text-moto-orange-strong space-y-2 rounded-lg px-3 py-2.5">
            <p className="text-base font-semibold tabular-nums">
              {request.previous_pickup_time
                ? t("pickup.pendingChange", {
                    from: request.previous_pickup_time,
                    to: request.pickup_time,
                  })
                : t("pickup.pendingRequested", {
                    to: request.pickup_time,
                  })}
            </p>
            <p className="mt-0.5 text-sm">{t("pickup.statusPending")}</p>
            {editedAt && (
              <p className="text-sm">
                {t("pickup.editedAt", {
                  date: formatLocaleDate(editedAt.slice(0, 10), locale),
                })}
              </p>
            )}
            {canEditRequest && !editing && (
              <Button
                type="button"
                variant="outline"
                size="md"
                className="max-sm:min-h-11"
                onClick={() => setEditing(true)}
              >
                {t("pickup.edit")}
              </Button>
            )}
            {studentId && request && (
              <RequestSharingControl
                studentId={studentId}
                requestType="pickup_change"
                requestId={request.id}
                isSelf={request.is_self === true}
              />
            )}
          </div>
        )}
        {request?.status === "approved" && (
          <p className="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-600">
            {t("pickup.statusApproved")}
          </p>
        )}
        {request?.status === "rejected" && (
          <p className="bg-parent-red-soft text-parent-red-strong rounded-lg px-3 py-2 text-sm">
            {t("pickup.statusRejected", {
              reason: request.decision_reason ?? t("pickup.noDecisionReason"),
            })}
          </p>
        )}

        {pending && !editing ? null : alreadyHome && childFirstName ? (
          <p className="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-600">
            {t("pickup.alreadyHome", { name: childFirstName })}
          </p>
        ) : staffOwned ? (
          <p className="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-600">
            {t("pickup.staffSet", {
              pickup: existing?.pickup_time ?? "—",
            })}
          </p>
        ) : (
          <div className="space-y-4">
            <TimeField
              inputRef={pickupTimeRef}
              value={pickupTime}
              onChange={(next) => {
                setPickupTime(next);
                if (invalidField === "pickupTime") {
                  setInvalidField(null);
                  setError(null);
                }
              }}
              label={t("pickup.pickupLabel")}
              hint={t("timeFormatHint")}
              placeholder={t("timeExample")}
              required
              invalid={invalidField === "pickupTime"}
              describedBy={invalidField === "pickupTime" ? errorId : undefined}
            />
            <label className="block">
              <span className="mb-1 block text-sm font-medium text-gray-700">
                {t("pickup.reasonLabel")}
                {reasonRequired && <span aria-hidden="true"> *</span>}
              </span>
              <textarea
                ref={reasonRef}
                value={reason}
                onChange={(e) => {
                  setReason(e.target.value);
                  if (invalidField === "reason") {
                    setInvalidField(null);
                    setError(null);
                  }
                }}
                maxLength={255}
                required={reasonRequired}
                rows={3}
                placeholder={t("pickup.reasonPlaceholder")}
                aria-invalid={invalidField === "reason"}
                aria-describedby={
                  invalidField === "reason" ? errorId : undefined
                }
                className="min-h-20 w-full resize-y rounded-lg border border-gray-300 bg-white px-3 py-2 text-base shadow-sm transition-colors hover:border-gray-400 focus-visible:border-gray-400 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
              />
            </label>
            {studentId && !editing && (
              <RequestSharingSelector
                studentId={studentId}
                selected={recipientIds}
                onChange={setRecipientIds}
              />
            )}
          </div>
        )}

        {existing?.pickup_source === "guardian" && (
          <p className="text-sm text-gray-500">
            {t("pickup.existingHint", {
              date: formatLocaleDate(date, locale),
            })}
          </p>
        )}

        {error && (
          <p
            id={errorId}
            role="alert"
            className="bg-parent-red-soft text-parent-red-strong rounded-lg px-3 py-2 text-sm"
          >
            {error}
          </p>
        )}
      </div>
    </Modal>
  );
}

// --- read-only lists for the page body ---

export function SickStatusSummary({
  studentId,
  sickDays,
  excusedRequests = EMPTY_EXCUSED_REQUESTS,
  reasonRequired = true,
  onEdited,
}: Readonly<{
  studentId?: string;
  sickDays: StatusDay[];
  // Absence requests behind an OGS approval gate. Pending ones show as
  // "Freigabe ausstehend" and can still be changed by their author; rejected
  // ones show the decision reason. Confirmed (approved) requests arrive as
  // StatusDays instead.
  excusedRequests?: readonly ExcusedRequest[];
  /** Ob die OGS einen Grund verlangt (requiresGuardianReason). */
  reasonRequired?: boolean;
  /** Nach einer geänderten Anfrage: die Liste neu laden. */
  onEdited?: () => void;
}>) {
  const t = useTranslations("parentChildCare");
  const locale = useLocale();
  const [editing, setEditing] = useState<ExcusedRequest | null>(null);

  // Collapse to a "first – last" range ONLY when the days are actually
  // contiguous; otherwise list them comma-separated. The request endpoint accepts
  // an arbitrary date list, so a Mon+Wed request must never render as "Mon – Wed"
  // (which would wrongly imply Tuesday is included), matching the staff list
  // (#1845 review).
  const rangeLabel = (dates: readonly string[]): string => {
    const sorted = [...dates].sort((a, b) => a.localeCompare(b));
    if (sorted.length === 0) return "";
    if (sorted.length === 1) return formatLocaleDate(sorted[0]!, locale);
    const contiguous = sorted.every(
      (d, i) => i === 0 || isNextDayISO(sorted[i - 1]!, d),
    );
    if (contiguous) {
      return `${formatLocaleDate(sorted[0]!, locale)} – ${formatLocaleDate(sorted.at(-1)!, locale)}`;
    }
    return sorted.map((d) => formatLocaleDate(d, locale)).join(", ");
  };

  const sickDates = sickDays
    .filter((d) => d.status === "sick")
    .map((d) => d.date);
  const excusedConfirmedDates = sickDays
    .filter((d) => d.status === "excused")
    .map((d) => d.date);
  const pending = excusedRequests.filter((r) => r.status === "pending");
  const rejected = excusedRequests.filter((r) => r.status === "rejected");
  const requestLabel = (request: ExcusedRequest) =>
    request.absence_status === "sick"
      ? t("summary.sickLabel")
      : t("summary.excusedLabel");
  // Approved requests normally surface as status days above, but the
  // status-day fetch is windowed (today..+2 months, matching the backend's
  // listSickDays range). An approval for a past date (delayed decision) or one
  // more than two months ahead has NO status day here, so it would silently
  // vanish once it left the pending list — surface those from the approved
  // request instead.
  //
  // Critically, only surface dates that are genuinely OUTSIDE the fetched window.
  // A within-window date with no matching status day was NOT dropped for being
  // out of range — it was superseded or cleared. Inside the window the status
  // days are authoritative; approved requests only fill gaps outside it.
  const windowStart = todayISO();
  const windowEnd = (() => {
    const d = parseISODate(windowStart);
    d.setMonth(d.getMonth() + 2);
    return toISODate(d);
  })();
  const shownDatesByStatus = {
    sick: new Set(sickDates),
    excused: new Set(excusedConfirmedDates),
  };
  const isOutOfWindow = (d: string) => d < windowStart || d > windowEnd;
  const approvedOutOfWindow = excusedRequests
    .filter((r) => r.status === "approved")
    .map((r) => ({
      id: r.id,
      label: requestLabel(r),
      dates: r.dates.filter(
        (d) =>
          isOutOfWindow(d) &&
          !(
            r.absence_status === "sick"
              ? shownDatesByStatus.sick
              : shownDatesByStatus.excused
          ).has(d),
      ),
    }))
    .filter((r) => r.dates.length > 0);

  const hasAny =
    sickDates.length > 0 ||
    excusedConfirmedDates.length > 0 ||
    approvedOutOfWindow.length > 0 ||
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
      {approvedOutOfWindow.map((r) => (
        <p key={r.id} className="text-sm font-semibold text-gray-900">
          {r.label}: {rangeLabel(r.dates)}
        </p>
      ))}
      {pending.map((r) => (
        <div key={r.id} className="space-y-0.5">
          <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
            <p className="text-sm font-semibold text-gray-900">
              {requestLabel(r)}: {rangeLabel(r.dates)}
            </p>
            <span className="bg-moto-amber/15 text-moto-amber-strong inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium">
              {t("summary.pendingLabel")}
            </span>
            {studentId && r.is_self && (
              <Button
                type="button"
                variant="outline"
                size="md"
                className="max-sm:min-h-11"
                onClick={() => setEditing(r)}
              >
                {t("summary.edit")}
              </Button>
            )}
          </div>
          {studentId && (
            <RequestSharingControl
              studentId={studentId}
              requestType="excused"
              requestId={r.id}
              isSelf={r.is_self}
            />
          )}
        </div>
      ))}
      {rejected.map((r) => (
        <div key={r.id} className="space-y-0.5">
          <div className="flex flex-wrap items-center gap-x-2">
            <p className="text-sm font-semibold text-gray-900">
              {requestLabel(r)}: {rangeLabel(r.dates)}
            </p>
            <span className="bg-moto-red/10 text-moto-red-strong inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium">
              {t("summary.rejectedLabel")}
            </span>
          </div>
          {r.decision_reason && (
            <p className="text-xs text-gray-500">
              {t("summary.reasonPrefix")}: {r.decision_reason}
            </p>
          )}
          {studentId && (
            <RequestSharingControl
              studentId={studentId}
              requestType="excused"
              requestId={r.id}
              isSelf={r.is_self}
            />
          )}
        </div>
      ))}
      {studentId && editing && (
        <RequestEditModal
          studentId={studentId}
          reasonRequired={reasonRequired}
          request={{
            type: "excused",
            id: editing.id,
            dates: [...editing.dates],
            note: editing.note,
          }}
          onClose={() => setEditing(null)}
          onSaved={() => onEdited?.()}
        />
      )}
    </div>
  );
}

// --- OGS quick actions (parent self-service) -------------------------------
//
// The care-schedule change request FORM now lives in ./care-schedule-request-
// modal and is owned entirely by the Stammdaten page (#1803) — it is no longer
// reachable from the chat. Only the sick-note and one-day pickup-change actions
// remain here as quick-action pills above the composer.

export type OgsActionKey = "sick" | "pickup";

// A single parent self-service action available from the OGS chat. Whether it
// takes effect immediately or needs OGS confirmation depends on the action and
// the school's settings.
export interface OgsAction {
  readonly key: OgsActionKey;
  readonly concept: MotoConceptKey;
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
      concept: "sick",
      enabled: features.sick_note_enabled,
    },
    {
      key: "pickup",
      concept: "pickup",
      enabled: features.pickup_change_enabled,
    },
  ];
}
