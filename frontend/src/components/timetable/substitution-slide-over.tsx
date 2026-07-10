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

import { Plus, RotateCcw, UserMinus } from "lucide-react";
import { useEffect, useState } from "react";

import { formatDate } from "~/lib/date-helpers";
import { getActivityTypeBadge, getStatusLabel } from "~/lib/timetable-helpers";
import type { EnrichedInstance } from "~/lib/timetable-types";
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

export interface StaffOption {
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
  onClose: () => void;
  onMarkAbsent: (
    absentStaffId: string,
    date: string,
    reason?: string,
  ) => Promise<void>;
  onSubstitute: (
    absentStaffId: string,
    substituteStaffId: string,
    date: string,
    reason?: string,
  ) => Promise<void>;
  onCancelBlock: (instance: EnrichedInstance, reason?: string) => Promise<void>;
  onAcknowledge: (
    instance: EnrichedInstance,
    ack: boolean,
    note?: string,
  ) => Promise<void>;
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
  onClose,
  onMarkAbsent,
  onSubstitute,
  onCancelBlock,
  onAcknowledge,
}: SubstitutionSlideOverProps) {
  const canEdit =
    instance?.status === "planned" || instance?.status === "active";

  const plannedStaff = (instance?.staff ?? []).filter((s) => !s.isSubstitute);
  const substitutes = (instance?.staff ?? []).filter((s) => s.isSubstitute);
  const assignedIds = new Set((instance?.staff ?? []).map((s) => s.staffId));
  const substituteOptions = staffOptions
    // Exclude staff already on this block and anyone marked absent elsewhere
    // that day — a day-absent person cannot cover a shift (#1840).
    .filter((s) => !assignedIds.has(s.id) && !dayAbsentStaffIds.has(s.id))
    .map((s) => ({ value: s.id, label: s.name }));

  const [people, setPeople] = useState<Record<string, PersonForm>>({});
  const [cancel, setCancel] = useState(false);
  const [cancelReason, setCancelReason] = useState("");
  const [unstaffed, setUnstaffed] = useState(false);
  const [unstaffedReason, setUnstaffedReason] = useState("");
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
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [instance?.id]);

  function updatePerson(id: string, patch: Partial<PersonForm>) {
    setPeople((prev) => ({ ...prev, [id]: { ...prev[id]!, ...patch } }));
  }

  // One Vertretung per block. The backend has no substitute→absence link and so
  // safely allows only a single substitute per instance; classifySubstitute
  // 409s a second one after the first already committed, leaving a half-saved
  // form. We therefore let at most one absent person name a replacement here —
  // the others stay absent-only (their position is left open). A block is
  // "covered" by that one replacement.
  function substituteChosenElsewhere(rowStaffId: string): boolean {
    return Object.entries(people).some(
      ([id, p]) => id !== rowStaffId && p.absent && p.substituteId !== "",
    );
  }

  // "Bewusst unbesetzt" (deliberately unstaffed) is only valid when nobody will
  // be covering the block after this form is applied. The backend rejects the
  // acknowledgement while any non-absent staff remain (ErrUnderstaffedAckStill
  // Staffed), so the derived flag below must account for ALL staffing that
  // survives the save — currently-present planned people, existing non-absent
  // substitutes, and any replacement newly selected here — not just new picks.
  const anyStaffRemaining =
    plannedStaff.some((row) => {
      const p = people[row.staffId];
      return p ? !p.absent : !row.isAbsent;
    }) ||
    substitutes.some((row) => !row.isAbsent) ||
    Object.values(people).some((p) => p.absent && p.substituteId !== "");

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
    Object.values(people).some(
      (p) => (p.absent && !p.wasAbsent) || (p.absent && p.substituteId !== ""),
    );

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault();
    if (!instance || !hasChanges || saving) return;
    setSaving(true);
    try {
      if (cancel) {
        await onCancelBlock(instance, cancelReason.trim() || undefined);
        onClose();
        return;
      }
      // Apply absences/substitutes first so staffing reflects the change before
      // we (maybe) acknowledge the block as unstaffed — the backend refuses that
      // acknowledgement while non-absent staff are still assigned.
      for (const [staffId, p] of Object.entries(people)) {
        const reason = p.reason.trim() || undefined;
        const newlyAbsent = p.absent && !p.wasAbsent;
        if (p.absent && p.substituteId) {
          await onSubstitute(staffId, p.substituteId, instance.date, reason);
        } else if (newlyAbsent) {
          await onMarkAbsent(staffId, instance.date, reason);
        }
      }
      if (unstaffed !== wasUnstaffed || noteEdited) {
        await onAcknowledge(
          instance,
          unstaffed,
          unstaffed ? unstaffedReason.trim() || undefined : undefined,
        );
      }
      onClose();
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
              <SlideOverCloseButton />
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
                    // Option A: only one Vertretung per block. Once another
                    // absent person has a replacement, this row's picker is
                    // locked so the save never sends a conflicting second one.
                    const otherHasSubstitute = substituteChosenElsewhere(
                      row.staffId,
                    );
                    const substituteDisabled =
                      unstaffed ||
                      otherHasSubstitute ||
                      substituteOptions.length === 0;
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
                              ) : p.wasAbsent ? null : (
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
                                  Rückgängig
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
                              ) : otherHasSubstitute && !p.substituteId ? (
                                <p className="mt-1 text-[11px] text-gray-400">
                                  Pro Block ist eine Vertretung möglich. Diese
                                  Position bleibt offen.
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
                    {substitutes.map((row) => (
                      <li
                        key={row.staffId}
                        className="flex items-center justify-between gap-2 rounded-lg bg-[#83CD2D]/10 px-3 py-2"
                      >
                        <span className="truncate text-sm font-medium text-gray-900">
                          {staffLabel(staffNames, row.staffId)}
                        </span>
                        <span className="rounded-full bg-[#83CD2D]/20 px-2 py-0.5 text-[10px] font-semibold text-[#5A8E1F]">
                          Ersatz
                        </span>
                      </li>
                    ))}
                  </ul>
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
                      disabled={cancel || anyStaffRemaining}
                      onChange={(e) => setUnstaffed(e.target.checked)}
                    />
                    <span className="text-sm text-gray-800">
                      Bewusst unbesetzt
                      <span className="block text-xs text-gray-500">
                        Läuft absichtlich ohne Personal und zählt nicht als
                        offene Lücke.
                        {anyStaffRemaining && (
                          <span className="mt-0.5 block text-[#CC2626]">
                            Nur möglich, wenn alle geplanten Personen abwesend
                            und keine Vertretung eingetragen ist.
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
