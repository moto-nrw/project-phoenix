"use client";

/**
 * SubstitutionSlideOver, the Vertretungsplan (#1840) block editor.
 *
 * Deliberately distinct from InstanceDetailSlideOver: that panel is the full
 * lifecycle + attendance editor for the base planner. This one is a focused
 * staffing-deviation surface, mark a person absent, assign a substitute,
 * cancel the block, or accept it running unstaffed, and it shows the base
 * plan vs the substitution side by side so the difference is traceable
 * (issue #1840 AC7). Built from the shared UI kit + timetable style tokens.
 */

import { TriangleAlert, UserMinus, UserX } from "lucide-react";
import { useState } from "react";

import { formatDate } from "~/lib/date-helpers";
import { getActivityTypeBadge, getStatusLabel } from "~/lib/timetable-helpers";
import type { EnrichedInstance } from "~/lib/timetable-types";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import {
  SlideOver,
  SlideOverCloseButton,
  SlideOverContent,
  SlideOverDescription,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";

import {
  timetableMutedSurface,
  timetableNestedSurface,
  timetableWarningPanel,
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
  onMarkAbsent: (absentStaffId: string, date: string) => Promise<void>;
  onSubstitute: (
    absentStaffId: string,
    substituteStaffId: string,
    date: string,
  ) => Promise<void>;
  onCancelBlock: (instance: EnrichedInstance) => Promise<void>;
  onAcknowledge: (
    instance: EnrichedInstance,
    ack: boolean,
    note?: string,
  ) => Promise<void>;
}

function staffLabel(staffNames: Map<string, string>, id: string): string {
  return staffNames.get(id) ?? `Personal #${id}`;
}

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
  const [pending, setPending] = useState(false);
  const [note, setNote] = useState("");

  const open = instance !== null;
  const canEdit =
    instance?.status === "planned" || instance?.status === "active";

  async function run(fn: () => Promise<void>) {
    setPending(true);
    try {
      await fn();
    } finally {
      setPending(false);
    }
  }

  // Planned staff = the base plan (non-substitute rows). Substitutes are the
  // deviation. Splitting them is exactly the base-vs-Vertretung diff (AC7).
  const plannedStaff = (instance?.staff ?? []).filter((s) => !s.isSubstitute);
  const substitutes = (instance?.staff ?? []).filter((s) => s.isSubstitute);
  const assignedIds = new Set((instance?.staff ?? []).map((s) => s.staffId));

  const substituteOptions = staffOptions
    .filter((s) => !assignedIds.has(s.id))
    .map((s) => ({ value: s.id, label: s.name }));

  const typeBadge = instance
    ? getActivityTypeBadge(instance.activityType)
    : null;
  const isUnstaffedAck = instance?.understaffedAck === true;

  return (
    <SlideOver
      open={open}
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

          <div className="flex-1 space-y-5 overflow-y-auto px-5 py-4">
            {/* Base plan vs substitution, the traceable diff. */}
            <section className="space-y-2">
              <h3 className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                Geplantes Personal
              </h3>
              {plannedStaff.length === 0 ? (
                <p
                  className={`${timetableMutedSurface} p-3 text-sm text-gray-500`}
                >
                  Für diesen Block war niemand geplant.
                </p>
              ) : (
                <ul className="space-y-2">
                  {plannedStaff.map((row) => (
                    <li
                      key={row.staffId}
                      className={`${timetableNestedSurface} p-3`}
                    >
                      <div className="flex items-center justify-between gap-2">
                        <span
                          className={`truncate text-sm font-medium ${
                            row.isAbsent
                              ? "text-gray-400 line-through"
                              : "text-gray-900"
                          }`}
                        >
                          {staffLabel(staffNames, row.staffId)}
                        </span>
                        <div className="flex shrink-0 items-center gap-1.5">
                          {row.isPrimary && (
                            <span className="rounded-full bg-gray-100 px-2 py-0.5 text-[10px] font-semibold text-gray-600">
                              Zuständig
                            </span>
                          )}
                          {row.isAbsent && (
                            <span className="rounded-full bg-[#FF3130]/10 px-2 py-0.5 text-[10px] font-semibold text-[#CC2626]">
                              Abwesend
                            </span>
                          )}
                        </div>
                      </div>

                      {canEdit && !row.isAbsent && (
                        <div className="mt-2">
                          <Button
                            type="button"
                            variant="outline"
                            size="md"
                            disabled={pending}
                            onClick={() =>
                              run(() =>
                                onMarkAbsent(row.staffId, instance.date),
                              )
                            }
                          >
                            <UserMinus className="mr-1.5 h-4 w-4" />
                            Abwesend melden
                          </Button>
                        </div>
                      )}

                      {canEdit && row.isAbsent && (
                        <div className="mt-2">
                          <span className="mb-1 block text-[11px] font-medium text-gray-500">
                            Ersatz eintragen
                          </span>
                          <CustomSelect
                            value=""
                            placeholder="Ersatzperson wählen…"
                            options={substituteOptions}
                            ariaLabel={`Ersatz für ${staffLabel(staffNames, row.staffId)}`}
                            disabled={pending || substituteOptions.length === 0}
                            onChange={(sub) => {
                              if (!sub) return;
                              void run(() =>
                                onSubstitute(row.staffId, sub, instance.date),
                              );
                            }}
                          />
                        </div>
                      )}
                    </li>
                  ))}
                </ul>
              )}

              {substitutes.length > 0 && (
                <div className="space-y-1 pt-1">
                  <h4 className="text-[11px] font-semibold tracking-wide text-gray-500 uppercase">
                    Vertretung
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

            {/* Deliberately unstaffed (AC5). */}
            {canEdit && (
              <section className="space-y-2">
                <h3 className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                  Bewusst unbesetzt
                </h3>
                {isUnstaffedAck ? (
                  <div className={timetableWarningPanel}>
                    <div className="flex items-center gap-2 text-xs font-bold text-[#8A6D00]">
                      <TriangleAlert className="h-4 w-4" />
                      Dieser Block läuft bewusst ohne Personal.
                    </div>
                    {instance.understaffedNote && (
                      <p className="mt-1 text-xs text-[#8A6D00]">
                        {instance.understaffedNote}
                      </p>
                    )}
                    <div className="mt-2">
                      <Button
                        type="button"
                        variant="outline"
                        size="md"
                        disabled={pending}
                        onClick={() =>
                          run(() => onAcknowledge(instance, false))
                        }
                      >
                        Markierung aufheben
                      </Button>
                    </div>
                  </div>
                ) : (
                  <div className={`${timetableNestedSurface} space-y-2 p-3`}>
                    <p className="text-sm text-gray-600">
                      Markiere den Block, wenn er absichtlich ohne Personal
                      läuft. Er zählt dann nicht mehr als offene Lücke.
                    </p>
                    <textarea
                      value={note}
                      onChange={(e) => setNote(e.target.value)}
                      rows={2}
                      maxLength={500}
                      placeholder="Grund (optional)"
                      className="w-full resize-none rounded-lg border border-gray-200 px-3 py-2 text-sm text-gray-900 placeholder:text-gray-400 focus:border-gray-400 focus:outline-none"
                    />
                    <Button
                      type="button"
                      variant="outline"
                      size="md"
                      disabled={pending}
                      onClick={() =>
                        run(() =>
                          onAcknowledge(
                            instance,
                            true,
                            note.trim() || undefined,
                          ),
                        )
                      }
                    >
                      Als bewusst unbesetzt markieren
                    </Button>
                  </div>
                )}
              </section>
            )}

            {/* Cancel the block (AC4). */}
            {canEdit && (
              <section className="space-y-2">
                <h3 className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                  Block absagen
                </h3>
                <div className={`${timetableNestedSurface} space-y-2 p-3`}>
                  <p className="text-sm text-gray-600">
                    Sagt diesen Termin ab. Die Halbjahresvorlage bleibt
                    unverändert.
                  </p>
                  <Button
                    type="button"
                    variant="outline_danger"
                    size="md"
                    disabled={pending}
                    onClick={() => run(() => onCancelBlock(instance))}
                  >
                    <UserX className="mr-1.5 h-4 w-4" />
                    Block absagen
                  </Button>
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
          </div>
        </SlideOverContent>
      )}
    </SlideOver>
  );
}
