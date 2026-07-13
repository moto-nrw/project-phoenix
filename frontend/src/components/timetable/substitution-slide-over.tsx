"use client";

/**
 * SubstitutionSlideOver — the Vertretungsplan (#1840) block editor.
 *
 * Same shape as the Betreuungsplan edit surface (TimetableEventModal): a
 * SlideOver whose body is a <form> and whose footer holds Abbrechen + a single
 * "Speichern" submit button — nothing is applied until you save.
 *
 * Clarity first: each planned person is a status card whose colour and badge
 * say at a glance whether they are present or absent. Marking someone absent is
 * one click; the replacement ("Vertretung") and an optional reason are a
 * clearly-labelled, secondary step so the absent person is never mistaken for
 * their substitute.
 */

import { History, Plus, RotateCcw, UserMinus } from "lucide-react";
import { useEffect, useState } from "react";

import { berlinTodayISO, formatDate } from "~/lib/date-helpers";
import { getActivityTypeBadge, getStatusLabel } from "~/lib/timetable-helpers";
import type {
  ApplyDeviationsInput,
  EnrichedInstance,
} from "~/lib/timetable-types";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { CustomSelect } from "~/components/ui/custom-select";
import {
  SlideOver,
  SlideOverCloseButton,
  SlideOverContent,
  SlideOverDescription,
  SlideOverFooter,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";

import {
  timetableMutedSurface,
  timetableNestedSurface,
} from "./timetable-style";

interface StaffOption {
  id: string;
  name: string;
}

interface SubstitutionSlideOverProps {
  instance: EnrichedInstance | null;
  /** Every staff member in the tenant, the substitute picker source. */
  staffOptions: readonly StaffOption[];
  staffNames: Map<string, string>;
  /**
   * Staff ids already marked absent somewhere on this instance's date. They are
   * out of action for the whole day (absence is day-wide) and must not be
   * offered as substitutes — the backend rejects them too (#1840).
   */
  dayAbsentStaffIds: ReadonlySet<string>;
  /**
   * True when the tenant staff list failed to load. The substitute picker then
   * shows an explicit error instead of an empty list that reads like "no staff".
   */
  staffLoadError?: boolean;
  /**
   * True when the current user holds `schedules:manage`. The list endpoints
   * feeding this page are `schedules:read`, so a view-only user can open the
   * slide-over, but every deviation save requires `schedules:manage` and would
   * 403. Without this the editing controls render for a read-only user and each
   * save fails at the backend; gate the whole editing surface on it (#1840).
   */
  canManage: boolean;
  onClose: () => void;
  /**
   * Applies the ENTIRE form (absences, substitute, understaffed
   * acknowledgement, cancel) in one atomic backend call (#1840). The slide-over
   * no longer sequences independent mutations that could half-commit.
   *
   * Resolves `true` when the save committed and the slide-over may close,
   * `false` when it failed (403/409/500/network) so the form stays open with
   * the user's edits intact for a retry. It must NOT reject — the caller
   * surfaces the error as a toast and returns false.
   */
  onApply: (input: ApplyDeviationsInput) => Promise<boolean>;
  /** Öffnet den Verlauf (Änderungsprotokoll, #1886) für diesen Block. */
  onShowHistory?: () => void;
}

// Per-planned-person edit state.
interface PersonForm {
  absent: boolean;
  wasAbsent: boolean;
  reason: string;
  substituteId: string;
  showReason: boolean;
}

function staffLabel(staffNames: Map<string, string>, id: string): string {
  return staffNames.get(id) ?? `Personal #${id}`;
}

const FORM_ID = "vertretung-form";

export function SubstitutionSlideOver({
  instance,
  staffOptions,
  staffNames,
  dayAbsentStaffIds,
  staffLoadError = false,
  canManage,
  onClose,
  onApply,
  onShowHistory,
}: SubstitutionSlideOverProps) {
  // A materialized past occurrence can still carry status "planned"/"active",
  // but the backend rejects every deviation on a block whose date is before
  // today ("block date is in the past"). The page can browse past weeks, so
  // gate editing on the date too — otherwise we'd offer a save that always
  // 400s (#1840). Today itself is still editable (backend allows date == today).
  // Use the school (Berlin) calendar date, not the browser's: the backend
  // compares against timezone.TodayDate() (always Berlin), so a browser in
  // another timezone must not decide "past" on its own local day (#1840).
  const isPast = (instance?.date ?? "") < berlinTodayISO();
  // Editable only when the block is current/future AND the user may actually
  // save. `schedules:read` opens this page but every save needs
  // `schedules:manage`; without canManage the controls would render and every
  // save would 403 (#1840).
  const canEdit =
    !isPast &&
    canManage &&
    (instance?.status === "planned" || instance?.status === "active");

  const plannedStaff = (instance?.staff ?? []).filter((s) => !s.isSubstitute);
  const substitutes = (instance?.staff ?? []).filter((s) => s.isSubstitute);
  const assignedIds = new Set((instance?.staff ?? []).map((s) => s.staffId));
  const substituteOptions = staffOptions
    // Exclude staff already on this block and anyone marked absent elsewhere
    // that day — a day-absent person cannot cover a shift (#1840).
    .filter((s) => !assignedIds.has(s.id) && !dayAbsentStaffIds.has(s.id))
    .map((s) => ({ value: s.id, label: s.name }));
  // A block may hold several substitutes — one per absent planned position. What
  // the backend rejects (409 substitute_conflict) is re-substituting a position
  // that is ALREADY flagged absent while another active substitute exists. So the
  // picker is locked only on such already-absent rows (see substituteDisabled
  // below); a newly-absent position may still name its own distinct substitute.
  // Removing an existing substitute is the "Entfernen" button below.

  const [people, setPeople] = useState<Record<string, PersonForm>>({});
  const [cancel, setCancel] = useState(false);
  const [cancelReason, setCancelReason] = useState("");
  const [unstaffed, setUnstaffed] = useState(false);
  const [unstaffedReason, setUnstaffedReason] = useState("");
  // Substitute staff ids the admin marked for removal (#1840). An assigned
  // substitute who later becomes unavailable can be marked absent day-wide,
  // which frees the block so another replacement can be chosen — otherwise a
  // new pick for the original absent person 409s (substitute_conflict) because
  // the old substitute is still non-absent.
  const [removedSubs, setRemovedSubs] = useState<Set<string>>(new Set());
  // Substitutes who are already marked absent (removed on a prior save) and are
  // being brought back in this save. Clearing the persisted absence restores them
  // as the active substitute, so an accidental "Entfernen" is correctable without
  // a DB edit (#1840). Inverse of removedSubs; the two never touch the same row
  // (a row is either absent → restorable, or present → removable).
  const [restoredSubs, setRestoredSubs] = useState<Set<string>>(new Set());
  const [saving, setSaving] = useState(false);

  const wasUnstaffed = instance?.understaffedAck === true;

  // Re-seed the form whenever a different block is opened.
  useEffect(() => {
    if (!instance) return;
    const seed: Record<string, PersonForm> = {};
    for (const row of instance.staff) {
      if (row.isSubstitute) continue;
      seed[row.staffId] = {
        absent: row.isAbsent,
        wasAbsent: row.isAbsent,
        reason: row.absenceReason ?? "",
        substituteId: "",
        showReason: Boolean(row.absenceReason),
      };
    }
    setPeople(seed);
    setCancel(false);
    setCancelReason("");
    setUnstaffed(instance.understaffedAck === true);
    setUnstaffedReason(instance.understaffedNote ?? "");
    setRemovedSubs(new Set());
    setRestoredSubs(new Set());
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [instance?.id]);

  function toggleRemoveSub(id: string) {
    setRemovedSubs((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function toggleRestoreSub(id: string) {
    setRestoredSubs((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function updatePerson(id: string, patch: Partial<PersonForm>) {
    setPeople((prev) => ({ ...prev, [id]: { ...prev[id]!, ...patch } }));
  }

  // A substitute is still covering the block when it is either non-absent and not
  // staged for removal, or absent but staged for restore. Such a substitute
  // counts toward projected coverage; once coverage meets the planned position
  // count the persisted-absent pickers lock (see substituteDisabled).
  const isSubstituteActive = (row: {
    staffId: string;
    isAbsent: boolean;
  }): boolean =>
    row.isAbsent
      ? restoredSubs.has(row.staffId)
      : !removedSubs.has(row.staffId);

  // "Bewusst unbesetzt" (deliberately unstaffed) acknowledges that at least one
  // planned position is deliberately left unfilled (#1840). It is valid whenever
  // the block stays understaffed after this form is applied — fewer people
  // present than planned, or nobody at all — not only when the whole block is
  // empty. So compare the projected coverage (planned people still there +
  // existing non-absent substitutes + any replacement newly picked here) against
  // the number of planned (non-substitute) positions. The backend enforces the
  // identical rule (ErrUnderstaffedAckStillStaffed only when fully staffed).
  const plannedPositions = plannedStaff.length;
  const presentPlanned = plannedStaff.filter((row) => {
    const p = people[row.staffId];
    return p ? !p.absent : !row.isAbsent;
  }).length;
  const activeSubstitutes = substitutes.filter(isSubstituteActive).length;
  // Count UNIQUE newly-picked substitutes. The backend collapses the same
  // substitute selected for two absent positions into a single covering row (the
  // repeated (instance, substitute) pair classifies as "already on instance"), so
  // counting the raw selections overstates coverage — it would wrongly read the
  // block as fully staffed, disable "Bewusst unbesetzt", and clear a valid
  // acknowledgement even though the backend still reports a shortfall (#1840).
  const newReplacements = new Set(
    Object.values(people)
      .filter((p) => p.absent && p.substituteId !== "")
      .map((p) => p.substituteId),
  ).size;
  const projectedCoverage =
    presentPlanned + activeSubstitutes + newReplacements;
  const isUnderstaffed =
    projectedCoverage === 0 || projectedCoverage < plannedPositions;

  // The understaffed reason is editable even when the ack flag itself doesn't
  // change, so an edit to an already-acknowledged block's note must count as a
  // change and be dispatched.
  const noteEdited =
    unstaffed &&
    unstaffedReason.trim() !== (instance?.understaffedNote ?? "").trim();

  const hasChanges =
    cancel ||
    unstaffed !== wasUnstaffed ||
    noteEdited ||
    removedSubs.size > 0 ||
    restoredSubs.size > 0 ||
    Object.values(people).some(
      (p) =>
        (p.absent && !p.wasAbsent) ||
        (p.absent && p.substituteId !== "") ||
        // A persisted absence being cleared (marked present again) is a change.
        (!p.absent && p.wasAbsent),
    );

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault();
    if (!instance || !hasChanges || saving) return;
    setSaving(true);
    try {
      // Cancel is exclusive — it maps to the backend's cancel branch and ignores
      // every other field, mirroring the UI where "Block absagen" disables the
      // rest of the form.
      if (cancel) {
        const ok = await onApply({
          cancel: true,
          cancelReason: cancelReason.trim() || undefined,
        });
        // Only close on a committed save — a failed cancel keeps the form open
        // so the edits survive for a retry (#1840).
        if (ok) onClose();
        return;
      }

      // Build ONE payload. The backend applies absences, the substitute, the
      // acknowledgement, and the removed-substitute absences atomically, so the
      // frontend no longer has to order independent requests to avoid a
      // half-saved block.
      const absences: NonNullable<ApplyDeviationsInput["absences"]> = [];
      const substitutions: NonNullable<ApplyDeviationsInput["substitutions"]> =
        [];
      // Persisted absences the admin cleared — mark them present again (#1840).
      const presences: NonNullable<ApplyDeviationsInput["presences"]> = [];

      // Removed substitutes are marked absent day-wide, which frees the block so
      // the replacement below no longer conflicts with the old substitute.
      for (const staffId of removedSubs) {
        absences.push({ staffId });
      }
      // Restored substitutes were absent (removed on a prior save) and are brought
      // back by clearing their day-wide absence — the inverse of removal (#1840).
      for (const staffId of restoredSubs) {
        presences.push(staffId);
      }
      for (const [staffId, p] of Object.entries(people)) {
        const reason = p.reason.trim() || undefined;
        const newlyAbsent = p.absent && !p.wasAbsent;
        if (p.absent && p.substituteId) {
          substitutions.push({
            absentStaffId: staffId,
            substituteStaffId: p.substituteId,
            reason,
          });
        } else if (newlyAbsent) {
          absences.push({ staffId, reason });
        } else if (!p.absent && p.wasAbsent) {
          // Was absent in the DB, now marked present → clear the day-wide absence.
          presences.push(staffId);
        }
      }

      const input: ApplyDeviationsInput = {};
      if (absences.length > 0) input.absences = absences;
      if (substitutions.length > 0) input.substitutions = substitutions;
      if (presences.length > 0) input.presences = presences;

      // "Bewusst unbesetzt" only holds while the block stays understaffed after
      // the save. isUnderstaffed forces it off if the block ends up fully
      // staffed even though the (now-disabled) checkbox stayed checked, so the
      // backend never sees a contradictory ack=true + fully-staffed state.
      const effectiveUnstaffed = unstaffed && isUnderstaffed;
      if (effectiveUnstaffed !== wasUnstaffed || noteEdited) {
        input.understaffedAck = effectiveUnstaffed;
        if (effectiveUnstaffed) {
          input.understaffedNote = unstaffedReason.trim() || undefined;
        }
      }

      // Keep the slide-over open (edits preserved) unless the save committed.
      const ok = await onApply(input);
      if (ok) onClose();
    } finally {
      setSaving(false);
    }
  }

  const typeBadge = instance
    ? getActivityTypeBadge(instance.activityType)
    : null;

  return (
    <SlideOver
      open={instance !== null}
      onOpenChange={(o) => {
        if (!o) onClose();
      }}
    >
      {instance && (
        <SlideOverContent>
          <SlideOverHeader>
            <div className="flex items-start justify-between gap-2">
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-2">
                  <SlideOverTitle>{instance.title}</SlideOverTitle>
                  <span className="rounded-full bg-gray-100 px-2 py-0.5 text-[10px] font-semibold tracking-wide text-gray-600 uppercase">
                    {getStatusLabel(instance.status)}
                  </span>
                  {typeBadge && (
                    <span
                      className="rounded-full px-1.5 py-0.5 text-[9px] font-bold tracking-wide text-white uppercase"
                      style={{ backgroundColor: typeBadge.bg }}
                    >
                      {typeBadge.label}
                    </span>
                  )}
                </div>
                <SlideOverDescription>
                  {formatDate(instance.date)} • {instance.startTime} –{" "}
                  {instance.endTime} •{" "}
                  {instance.roomName || `Raum #${instance.roomId}`}
                </SlideOverDescription>
              </div>
              <div className="flex shrink-0 items-center gap-1">
                {onShowHistory ? (
                  <Button
                    type="button"
                    variant="ghost"
                    size="compact"
                    onClick={onShowHistory}
                    aria-label="Verlauf anzeigen"
                  >
                    <History className="h-4 w-4" aria-hidden />
                    Verlauf
                  </Button>
                ) : null}
                <SlideOverCloseButton />
              </div>
            </div>
          </SlideOverHeader>

          <form
            id={FORM_ID}
            onSubmit={(e) => void handleSubmit(e)}
            className="min-h-0 flex-1 space-y-6 overflow-y-auto px-5 py-4"
          >
            {/* Personal */}
            <section className="space-y-2">
              <h3 className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                Personal
              </h3>
              {plannedStaff.length === 0 ? (
                <p
                  className={`${timetableMutedSurface} p-3 text-sm text-gray-500`}
                >
                  Für diesen Block war niemand geplant.
                </p>
              ) : (
                <ul className="space-y-2">
                  {plannedStaff.map((row) => {
                    const p = people[row.staffId];
                    if (!p) return null;
                    const name = staffLabel(staffNames, row.staffId);
                    // Each absent planned position may name its own replacement.
                    // The backend applies a count-based rule: a substitute is
                    // accepted while the block still has an open absent slot
                    // (active coverage below the planned position count) and 409s
                    // only when every absent position is already covered. Mirror
                    // that here so the picker matches what the save will accept. A
                    // NEWLY-absent row (!wasAbsent) always opens its own picker; an
                    // ALREADY-absent (persisted) row keeps its picker open too as
                    // long as projected coverage is still below the planned count —
                    // so a still-open gap left by a previous save can be filled
                    // without first removing another position's valid replacement.
                    // Once coverage meets the planned count a further replacement
                    // would overstaff, so persisted-absent rows that DON'T already
                    // hold a pick lock; a row that already named a replacement stays
                    // editable (changing or clearing its own pick never overstaffs)
                    // (#1840).
                    const substituteDisabled =
                      unstaffed ||
                      substituteOptions.length === 0 ||
                      (p.wasAbsent &&
                        !p.substituteId &&
                        projectedCoverage >= plannedPositions);
                    return (
                      <li
                        key={row.staffId}
                        className={`rounded-xl border shadow-sm ${
                          p.absent
                            ? "border-[#FF3130]/25 bg-[#FF3130]/5"
                            : "border-gray-200 bg-white"
                        }`}
                      >
                        {/* Status row */}
                        <div className="flex items-center justify-between gap-2 p-3">
                          <div className="min-w-0">
                            <div
                              className={`truncate text-sm font-semibold ${
                                p.absent
                                  ? "text-gray-400 line-through"
                                  : "text-gray-900"
                              }`}
                            >
                              {name}
                            </div>
                            <div className="mt-0.5 flex items-center gap-1.5 text-[11px]">
                              {p.absent ? (
                                <span className="font-semibold text-[#CC2626]">
                                  Abwesend
                                </span>
                              ) : (
                                <span className="text-gray-500">Anwesend</span>
                              )}
                              {row.isPrimary && (
                                <span className="text-gray-400">
                                  • Zuständig
                                </span>
                              )}
                            </div>
                          </div>
                          {canEdit && (
                            <div className="shrink-0">
                              {!p.absent ? (
                                <Button
                                  type="button"
                                  variant="outline_danger"
                                  size="md"
                                  onClick={() =>
                                    updatePerson(row.staffId, { absent: true })
                                  }
                                >
                                  <UserMinus className="mr-1.5 h-4 w-4" />
                                  Abwesend
                                </Button>
                              ) : (
                                // Both a freshly-marked absence and a persisted
                                // one (wasAbsent) can be undone: the persisted
                                // case clears the saved absence day-wide so a
                                // wrongly-marked person can be corrected (#1840).
                                <Button
                                  type="button"
                                  variant="ghost"
                                  size="md"
                                  onClick={() =>
                                    updatePerson(row.staffId, {
                                      absent: false,
                                      substituteId: "",
                                    })
                                  }
                                >
                                  <RotateCcw className="mr-1.5 h-4 w-4" />
                                  {p.wasAbsent
                                    ? "Anwesend melden"
                                    : "Rückgängig"}
                                </Button>
                              )}
                            </div>
                          )}
                        </div>

                        {/* Absent detail: clearly-labelled Vertretung + optional reason */}
                        {p.absent && (
                          <div className="space-y-2 rounded-b-xl border-t border-[#FF3130]/15 bg-white/60 p-3">
                            <div>
                              <span className="mb-1 block text-[11px] font-semibold tracking-wide text-gray-500 uppercase">
                                Vertretung für {name}
                              </span>
                              {canEdit ? (
                                <CustomSelect
                                  value={p.substituteId}
                                  options={substituteOptions}
                                  ariaLabel={`Vertretung für ${name}`}
                                  placeholder="Ersatzperson wählen…"
                                  disabled={substituteDisabled}
                                  onChange={(value) =>
                                    updatePerson(row.staffId, {
                                      substituteId: value,
                                    })
                                  }
                                />
                              ) : (
                                <span className="text-sm text-gray-500">—</span>
                              )}
                              {unstaffed ? (
                                <p className="mt-1 text-[11px] text-gray-400">
                                  Keine Vertretung möglich, solange der Block
                                  als bewusst unbesetzt markiert ist.
                                </p>
                              ) : p.wasAbsent &&
                                !p.substituteId &&
                                projectedCoverage >= plannedPositions ? (
                                <p className="mt-1 text-[11px] text-gray-400">
                                  Der Block ist bereits vollständig vertreten.
                                  Zum Tauschen zuerst „Entfernen“.
                                </p>
                              ) : staffLoadError &&
                                substituteOptions.length === 0 ? (
                                <p className="mt-1 text-[11px] text-[#CC2626]">
                                  Personalliste konnte nicht geladen werden.
                                  Bitte die Seite neu laden.
                                </p>
                              ) : null}
                            </div>

                            {p.wasAbsent ? (
                              p.reason ? (
                                <p className="text-xs text-gray-500">
                                  Grund: {p.reason}
                                </p>
                              ) : null
                            ) : p.showReason ? (
                              <input
                                type="text"
                                value={p.reason}
                                maxLength={500}
                                onChange={(e) =>
                                  updatePerson(row.staffId, {
                                    reason: e.target.value,
                                  })
                                }
                                placeholder="Grund (optional)"
                                className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm text-gray-900 placeholder:text-gray-400 focus:border-gray-400 focus:outline-none"
                              />
                            ) : (
                              <button
                                type="button"
                                onClick={() =>
                                  updatePerson(row.staffId, {
                                    showReason: true,
                                  })
                                }
                                className="inline-flex items-center gap-1 text-xs font-medium text-gray-500 hover:text-gray-700"
                              >
                                <Plus className="h-3.5 w-3.5" />
                                Grund hinzufügen
                              </button>
                            )}
                          </div>
                        )}
                      </li>
                    );
                  })}
                </ul>
              )}

              {substitutes.length > 0 && (
                <div className="space-y-1 pt-1">
                  <h4 className="text-[11px] font-semibold tracking-wide text-gray-500 uppercase">
                    Aktuelle Vertretung
                  </h4>
                  <ul className="space-y-1">
                    {substitutes.map((row) => {
                      const removed = removedSubs.has(row.staffId);
                      const restored = restoredSubs.has(row.staffId);
                      // A substitute is inactive when marked absent (and not being
                      // restored) or staged for removal. A persisted-absent
                      // substitute the admin restores reads as active again (#1840).
                      const inactive = (row.isAbsent && !restored) || removed;
                      return (
                        <li
                          key={row.staffId}
                          className={`flex items-center justify-between gap-2 rounded-lg px-3 py-2 ${
                            inactive ? "bg-gray-100" : "bg-[#83CD2D]/10"
                          }`}
                        >
                          <span
                            className={`truncate text-sm font-medium ${
                              inactive
                                ? "text-gray-400 line-through"
                                : "text-gray-900"
                            }`}
                          >
                            {staffLabel(staffNames, row.staffId)}
                          </span>
                          <div className="flex shrink-0 items-center gap-2">
                            {row.isAbsent && !restored ? (
                              <span className="rounded-full bg-[#FF3130]/10 px-2 py-0.5 text-[10px] font-semibold text-[#CC2626]">
                                Abwesend
                              </span>
                            ) : removed ? (
                              <span className="rounded-full bg-gray-200 px-2 py-0.5 text-[10px] font-semibold text-gray-500">
                                Wird entfernt
                              </span>
                            ) : restored ? (
                              <span className="rounded-full bg-[#83CD2D]/20 px-2 py-0.5 text-[10px] font-semibold text-[#5A8E1F]">
                                Wird wiederhergestellt
                              </span>
                            ) : (
                              <span className="rounded-full bg-[#83CD2D]/20 px-2 py-0.5 text-[10px] font-semibold text-[#5A8E1F]">
                                Ersatz
                              </span>
                            )}
                            {canEdit &&
                              (row.isAbsent ? (
                                // A persisted-absent substitute (removed on a
                                // prior save) can be brought back so an accidental
                                // removal is correctable without a DB edit (#1840).
                                <Button
                                  type="button"
                                  variant="ghost"
                                  size="compact"
                                  onClick={() => toggleRestoreSub(row.staffId)}
                                >
                                  {restored ? (
                                    <>
                                      <UserMinus className="mr-1 h-3.5 w-3.5" />
                                      Rückgängig
                                    </>
                                  ) : (
                                    <>
                                      <RotateCcw className="mr-1 h-3.5 w-3.5" />
                                      Anwesend melden
                                    </>
                                  )}
                                </Button>
                              ) : (
                                <Button
                                  type="button"
                                  variant="ghost"
                                  size="compact"
                                  onClick={() => toggleRemoveSub(row.staffId)}
                                >
                                  {removed ? (
                                    <>
                                      <RotateCcw className="mr-1 h-3.5 w-3.5" />
                                      Rückgängig
                                    </>
                                  ) : (
                                    <>
                                      <UserMinus className="mr-1 h-3.5 w-3.5" />
                                      Entfernen
                                    </>
                                  )}
                                </Button>
                              ))}
                          </div>
                        </li>
                      );
                    })}
                  </ul>
                  {canEdit && substitutes.some((row) => !row.isAbsent) && (
                    <p className="text-[11px] leading-5 text-gray-400">
                      „Entfernen“ meldet die Vertretung für den ganzen Tag
                      abwesend und gibt den Block für eine andere Ersatzperson
                      frei.
                    </p>
                  )}
                  {canEdit && substitutes.some((row) => row.isAbsent) && (
                    <p className="text-[11px] leading-5 text-gray-400">
                      „Anwesend melden“ macht eine entfernte Vertretung wieder
                      verfügbar.
                    </p>
                  )}
                </div>
              )}

              {canEdit && (
                <p className="text-[11px] leading-5 text-gray-400">
                  Abwesenheit und Vertretung gelten für alle Termine dieser
                  Person am {formatDate(instance.date)}.
                </p>
              )}
            </section>

            {/* Block */}
            {canEdit && (
              <section className="space-y-2">
                <h3 className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                  Block
                </h3>
                <div className={`${timetableNestedSurface} space-y-3 p-3`}>
                  <label
                    htmlFor="vp-unstaffed"
                    className="flex items-start gap-2"
                  >
                    <Checkbox
                      id="vp-unstaffed"
                      checked={unstaffed}
                      disabled={cancel || !isUnderstaffed}
                      onChange={(e) => setUnstaffed(e.target.checked)}
                    />
                    <span className="text-sm text-gray-800">
                      Bewusst unbesetzt
                      <span className="block text-xs text-gray-500">
                        Eine geplante Position bleibt absichtlich unbesetzt und
                        zählt nicht als offene Lücke.
                        {!isUnderstaffed && (
                          <span className="mt-0.5 block text-[#CC2626]">
                            Nur möglich, wenn mindestens eine geplante Position
                            unbesetzt bleibt (weniger Personal als geplant).
                          </span>
                        )}
                      </span>
                    </span>
                  </label>
                  {unstaffed && !cancel && (
                    <input
                      type="text"
                      value={unstaffedReason}
                      maxLength={500}
                      onChange={(e) => setUnstaffedReason(e.target.value)}
                      placeholder="Grund (optional)"
                      className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm text-gray-900 placeholder:text-gray-400 focus:border-gray-400 focus:outline-none"
                    />
                  )}

                  <label
                    htmlFor="vp-cancel"
                    className="flex items-start gap-2 border-t border-gray-100 pt-3"
                  >
                    <Checkbox
                      id="vp-cancel"
                      checked={cancel}
                      disabled={unstaffed}
                      onChange={(e) => setCancel(e.target.checked)}
                    />
                    <span className="text-sm text-gray-800">
                      Block absagen
                      <span className="block text-xs text-gray-500">
                        Sagt den Termin ab. Die Halbjahresvorlage bleibt
                        unverändert.
                      </span>
                    </span>
                  </label>
                  {cancel && (
                    <input
                      type="text"
                      value={cancelReason}
                      maxLength={500}
                      onChange={(e) => setCancelReason(e.target.value)}
                      placeholder="Grund (optional)"
                      className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm text-gray-900 placeholder:text-gray-400 focus:border-gray-400 focus:outline-none"
                    />
                  )}
                </div>
              </section>
            )}

            {!canEdit &&
              (instance.status === "cancelled" ? (
                <div
                  className={`${timetableMutedSurface} space-y-1 p-3 text-sm text-gray-500`}
                >
                  <p className="font-medium text-gray-700">
                    Dieser Block wurde abgesagt.
                  </p>
                  {instance.cancelReason && (
                    <p>Grund: {instance.cancelReason}</p>
                  )}
                </div>
              ) : !canManage &&
                !isPast &&
                (instance.status === "planned" ||
                  instance.status === "active") ? (
                // The block is editable in principle, but the user only holds
                // read access to the plan — surface that instead of the
                // "past/completed" message, which would be misleading (#1840).
                <p
                  className={`${timetableMutedSurface} p-3 text-sm text-gray-500`}
                >
                  Sie haben nur Leserechte für den Vertretungsplan. Änderungen
                  können nur Personen mit Verwaltungsrechten vornehmen.
                </p>
              ) : (
                <p
                  className={`${timetableMutedSurface} p-3 text-sm text-gray-500`}
                >
                  Vergangene oder abgeschlossene Termine können nicht mehr
                  geändert werden.
                </p>
              ))}
          </form>

          {canEdit && (
            <SlideOverFooter>
              <div className="flex items-center justify-end gap-3">
                <Button
                  type="button"
                  variant="outline"
                  size="md"
                  disabled={saving}
                  onClick={onClose}
                >
                  Abbrechen
                </Button>
                <Button
                  type="submit"
                  form={FORM_ID}
                  variant="primary"
                  size="md"
                  isLoading={saving}
                  loadingText="Speichere …"
                  disabled={saving || !hasChanges}
                >
                  Speichern
                </Button>
              </div>
            </SlideOverFooter>
          )}
        </SlideOverContent>
      )}
    </SlideOver>
  );
}
