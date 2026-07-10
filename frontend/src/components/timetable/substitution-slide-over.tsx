"use client";

/**
 * SubstitutionSlideOver — the Vertretungsplan (#1840) block editor.
 *
 * Same shape as the Betreuungsplan edit surface (TimetableEventModal): a
 * SlideOver whose body is a <form> and whose footer holds Abbrechen + a single
 * "Speichern" submit button. You edit the block's staffing — mark people
 * absent, pick a substitute, cancel the block, or accept it unstaffed, each
 * with an optional reason — and nothing is applied until you save.
 */

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
}

function staffLabel(staffNames: Map<string, string>, id: string): string {
  return staffNames.get(id) ?? `Personal #${id}`;
}

const FORM_ID = "vertretung-form";

export function SubstitutionSlideOver({
  instance,
  staffOptions,
  staffNames,
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
    .filter((s) => !assignedIds.has(s.id))
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

  // Whether anything would actually be dispatched on save.
  const hasChanges =
    cancel ||
    unstaffed !== wasUnstaffed ||
    Object.values(people).some(
      (p) => (p.absent && !p.wasAbsent) || (p.absent && p.substituteId !== ""),
    );

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault();
    if (!instance || !hasChanges || saving) return;
    setSaving(true);
    try {
      // Cancelling the block supersedes every other change.
      if (cancel) {
        await onCancelBlock(instance, cancelReason.trim() || undefined);
        onClose();
        return;
      }

      if (unstaffed !== wasUnstaffed) {
        await onAcknowledge(
          instance,
          unstaffed,
          unstaffed ? unstaffedReason.trim() || undefined : undefined,
        );
      }

      for (const [staffId, p] of Object.entries(people)) {
        const reason = p.reason.trim() || undefined;
        const newlyAbsent = p.absent && !p.wasAbsent;
        // A substitute is assigned via the substitute call (which also marks
        // the person absent and carries the reason); a bare absence uses
        // markAbsent.
        if (p.absent && p.substituteId) {
          await onSubstitute(staffId, p.substituteId, instance.date, reason);
        } else if (newlyAbsent) {
          await onMarkAbsent(staffId, instance.date, reason);
        }
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
            className="flex-1 space-y-6 overflow-y-auto px-5 py-4"
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
                    const showDetails = p.absent;
                    return (
                      <li
                        key={row.staffId}
                        className={`${timetableNestedSurface} p-3`}
                      >
                        <label className="flex items-center justify-between gap-2">
                          <span className="flex min-w-0 items-center gap-2">
                            <Checkbox
                              checked={p.absent}
                              disabled={!canEdit || p.wasAbsent}
                              onChange={(e) =>
                                updatePerson(row.staffId, {
                                  absent: e.target.checked,
                                })
                              }
                            />
                            <span
                              className={`truncate text-sm font-medium ${
                                p.absent ? "text-gray-500" : "text-gray-900"
                              }`}
                            >
                              {staffLabel(staffNames, row.staffId)}
                            </span>
                          </span>
                          <span className="flex shrink-0 items-center gap-1.5">
                            {row.isPrimary && (
                              <span className="rounded-full bg-gray-100 px-2 py-0.5 text-[10px] font-semibold text-gray-600">
                                Zuständig
                              </span>
                            )}
                            {p.wasAbsent && (
                              <span className="rounded-full bg-[#FF3130]/10 px-2 py-0.5 text-[10px] font-semibold text-[#CC2626]">
                                Abwesend
                              </span>
                            )}
                          </span>
                        </label>

                        {canEdit && showDetails && (
                          <div className="mt-3 space-y-2 border-t border-gray-100 pt-3">
                            <div>
                              <label
                                htmlFor={`reason-${row.staffId}`}
                                className="mb-1 block text-[11px] font-medium text-gray-500"
                              >
                                Grund (optional)
                              </label>
                              <input
                                id={`reason-${row.staffId}`}
                                type="text"
                                value={p.reason}
                                maxLength={500}
                                disabled={p.wasAbsent}
                                onChange={(e) =>
                                  updatePerson(row.staffId, {
                                    reason: e.target.value,
                                  })
                                }
                                placeholder="z. B. krank, Fortbildung"
                                className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm text-gray-900 placeholder:text-gray-400 focus:border-gray-400 focus:outline-none disabled:bg-gray-50 disabled:text-gray-400"
                              />
                            </div>
                            <div>
                              <span className="mb-1 block text-[11px] font-medium text-gray-500">
                                Ersatzperson (optional)
                              </span>
                              <CustomSelect
                                value={p.substituteId}
                                placeholder="Ersatzperson wählen…"
                                options={substituteOptions}
                                ariaLabel={`Ersatz für ${staffLabel(staffNames, row.staffId)}`}
                                disabled={substituteOptions.length === 0}
                                onChange={(v) =>
                                  updatePerson(row.staffId, { substituteId: v })
                                }
                              />
                            </div>
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
                    Bereits als Ersatz eingetragen
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
                  Abwesenheit und Ersatz gelten für alle Termine dieser Person
                  am {formatDate(instance.date)}.
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
                      disabled={cancel}
                      onChange={(e) => setUnstaffed(e.target.checked)}
                    />
                    <span className="text-sm text-gray-800">
                      Bewusst unbesetzt
                      <span className="block text-xs text-gray-500">
                        Läuft absichtlich ohne Personal und zählt nicht als
                        offene Lücke.
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

            {!canEdit && (
              <p
                className={`${timetableMutedSurface} p-3 text-sm text-gray-500`}
              >
                Vergangene oder abgeschlossene Termine können nicht mehr
                geändert werden.
              </p>
            )}
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
