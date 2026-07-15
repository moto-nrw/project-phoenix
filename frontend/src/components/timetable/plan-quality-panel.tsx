"use client";

import { useMemo, useState } from "react";
import {
  CalendarClock,
  ShieldAlert,
  TriangleAlert,
  UserPlus,
  UsersRound,
} from "lucide-react";

import { Button } from "~/components/ui/button";
import type { Staff } from "~/lib/staff-api";
import type {
  ExceptionConflict,
  GapInstance,
  EnrichedInstance,
} from "~/lib/timetable-types";
import {
  timetableDangerPanel,
  timetableNestedSurface,
  timetableSelectClass,
  timetableSurfacePadded,
  timetableWarningPanel,
} from "./timetable-style";
import { TimetableStatCard } from "./timetable-stat-card";

interface PlanQualityPanelProps {
  instances: EnrichedInstance[];
  gaps: GapInstance[];
  conflicts: ExceptionConflict[];
  staff: Staff[];
  loading: boolean;
  error?: string | null;
  onSelectInstance: (instanceId: string) => void;
  onEditInstance: (instanceId: string) => void;
  onSubstitute: (
    instanceId: string,
    absentStaffId: string,
    substituteStaffId: string,
    date: string,
  ) => Promise<void>;
}

function dateLabel(iso: string): string {
  const d = new Date(`${iso}T00:00:00`);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleDateString("de-DE", {
    weekday: "short",
    day: "2-digit",
    month: "2-digit",
  });
}

function conflictText(conflict: ExceptionConflict): string {
  if (conflict.kind === "cancelled_instance_with_scheduled_arrivals") {
    return "Abgesagter Termin hat weiterhin erwartete Kinder.";
  }
  return "Geänderte Terminzeit passt nicht zur erwarteten Ankunft.";
}

export function PlanQualityPanel({
  instances,
  gaps,
  conflicts,
  staff,
  loading,
  error,
  onSelectInstance,
  onEditInstance,
  onSubstitute,
}: PlanQualityPanelProps) {
  const [substituteByGap, setSubstituteByGap] = useState<
    Record<string, string>
  >({});
  const [absentByGap, setAbsentByGap] = useState<Record<string, string>>({});
  const [submittingGapId, setSubmittingGapId] = useState<string | null>(null);

  const conflictCount = instances.reduce(
    (sum, inst) => sum + inst.conflictWarnings.length,
    0,
  );
  const activeCount = instances.filter(
    (inst) => inst.status === "active",
  ).length;
  const plannedCount = instances.filter(
    (inst) => inst.status === "planned",
  ).length;
  const absentStaffCount = instances.reduce(
    (sum, inst) => sum + inst.absentStaffCount,
    0,
  );
  const hasError = !!error;
  const issueCount = gaps.length + conflicts.length + conflictCount;
  const availableStaff = useMemo(
    () => staff.filter((item) => item.workStatus !== "checked_out"),
    [staff],
  );
  const staffNameById = useMemo(
    () => new Map(staff.map((item) => [item.id, item.name])),
    [staff],
  );

  const handleSubstitute = (gap: GapInstance) => {
    const substituteStaffId = substituteByGap[gap.instanceId];
    if (!substituteStaffId) return;
    const instance = instances.find((item) => item.id === gap.instanceId);
    const absentRows = instance?.staff.filter((item) => item.isAbsent) ?? [];
    const absentStaffId =
      absentByGap[gap.instanceId] ?? absentRows[0]?.staffId ?? null;
    const absentStaff = absentRows.find(
      (item) => item.staffId === absentStaffId,
    );
    if (!absentStaff) return;

    setSubmittingGapId(gap.instanceId);
    void onSubstitute(
      gap.instanceId,
      absentStaff.staffId,
      substituteStaffId,
      gap.date,
    ).finally(() => setSubmittingGapId(null));
  };

  return (
    <section className={timetableSurfacePadded}>
      <div>
        <h2 className="text-sm font-bold text-gray-900">Planstatus</h2>
        <p className="mt-0.5 text-xs text-gray-500">
          {hasError
            ? "Planstatus konnte nicht vollständig geprüft werden."
            : loading
              ? "Prüfe Personal, Konflikte und Ausnahmen …"
              : issueCount === 0
                ? "Diese Ansicht hat keine offenen Planungsprobleme."
                : `${issueCount} Punkt(e), die vor dem Betrieb geprüft werden sollten.`}
        </p>
      </div>

      <div className="mt-4 grid gap-2 md:grid-cols-4">
        <TimetableStatCard
          icon={<CalendarClock className="h-4 w-4" />}
          label="Geplant"
          value={String(plannedCount)}
          tone="neutral"
        />
        <TimetableStatCard
          icon={<UsersRound className="h-4 w-4" />}
          label="Läuft"
          value={String(activeCount)}
          tone="success"
        />
        <TimetableStatCard
          icon={<UserPlus className="h-4 w-4" />}
          label="Ohne Personal"
          value={String(gaps.length)}
          tone={gaps.length > 0 ? "danger" : "neutral"}
        />
        <TimetableStatCard
          icon={<ShieldAlert className="h-4 w-4" />}
          label="Abwesend"
          value={String(absentStaffCount)}
          tone={absentStaffCount > 0 ? "warning" : "neutral"}
        />
      </div>

      {hasError && (
        <div className={`${timetableWarningPanel} mt-4 text-xs text-[#92400E]`}>
          <div className="flex items-center gap-2 font-bold text-[#92400E]">
            <TriangleAlert className="h-4 w-4" />
            Prüfung unvollständig
          </div>
          <p className="mt-1">
            {error ??
              "Mindestens ein Planstatus-Check konnte nicht geladen werden."}
          </p>
        </div>
      )}

      {(gaps.length > 0 || conflicts.length > 0) && (
        <div className="mt-4 grid gap-3 lg:grid-cols-2">
          {gaps.length > 0 && (
            <div className={timetableDangerPanel}>
              <div className="flex items-center gap-2 text-xs font-bold text-[#CC2626]">
                <UserPlus className="h-4 w-4" />
                Personal-Lücken
              </div>
              <div className="mt-2 space-y-2">
                {gaps.map((gap) => {
                  const instance = instances.find(
                    (item) => item.id === gap.instanceId,
                  );
                  const absentRows =
                    instance?.staff.filter((item) => item.isAbsent) ?? [];
                  const canSubstitute = absentRows.length > 0;
                  return (
                    <div
                      key={gap.instanceId}
                      className={`${timetableNestedSurface} p-2 text-xs`}
                    >
                      <button
                        type="button"
                        onClick={() => onSelectInstance(gap.instanceId)}
                        className="text-left font-bold text-gray-900 hover:text-gray-900"
                      >
                        {dateLabel(gap.date)} • {gap.startTime} {gap.title}
                      </button>
                      <div className="mt-1 text-gray-500">
                        {gap.assignedStaffCount === 0
                          ? "Kein Personal zugeordnet."
                          : `${gap.absentStaffCount} von ${gap.assignedStaffCount} zugeordneten Personen abwesend.`}
                      </div>
                      {gap.assignedStaffCount === 0 && (
                        <div className="mt-2">
                          <Button
                            type="button"
                            size="compact"
                            variant="outline"
                            onClick={() => onEditInstance(gap.instanceId)}
                          >
                            Personal zuordnen
                          </Button>
                        </div>
                      )}
                      {canSubstitute && (
                        <div className="mt-2 flex flex-wrap items-center gap-2">
                          {absentRows.length === 1 ? (
                            <span className="text-xs font-semibold text-[#CC2626]">
                              Abwesend:{" "}
                              {staffNameById.get(absentRows[0]!.staffId) ??
                                `Personal #${absentRows[0]!.staffId}`}
                            </span>
                          ) : (
                            <select
                              value={
                                absentByGap[gap.instanceId] ??
                                absentRows[0]?.staffId ??
                                ""
                              }
                              onChange={(e) =>
                                setAbsentByGap((prev) => ({
                                  ...prev,
                                  [gap.instanceId]: e.target.value,
                                }))
                              }
                              className={`${timetableSelectClass} min-w-48`}
                            >
                              {absentRows.map((item) => (
                                <option key={item.staffId} value={item.staffId}>
                                  {staffNameById.get(item.staffId) ??
                                    `Personal #${item.staffId}`}
                                </option>
                              ))}
                            </select>
                          )}
                          <select
                            value={substituteByGap[gap.instanceId] ?? ""}
                            onChange={(e) =>
                              setSubstituteByGap((prev) => ({
                                ...prev,
                                [gap.instanceId]: e.target.value,
                              }))
                            }
                            className={`${timetableSelectClass} min-w-48`}
                          >
                            <option value="">Ersatz auswählen …</option>
                            {availableStaff.map((item) => (
                              <option key={item.id} value={item.id}>
                                {item.name}
                              </option>
                            ))}
                          </select>
                          <Button
                            type="button"
                            size="compact"
                            variant="outline"
                            onClick={() => handleSubstitute(gap)}
                            isLoading={submittingGapId === gap.instanceId}
                            disabled={!substituteByGap[gap.instanceId]}
                          >
                            Anwenden
                          </Button>
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            </div>
          )}

          {conflicts.length > 0 && (
            <div className={timetableWarningPanel}>
              <div className="flex items-center gap-2 text-xs font-bold text-[#92400E]">
                <TriangleAlert className="h-4 w-4" />
                Ausnahmen prüfen
              </div>
              <div className="mt-2 space-y-2">
                {conflicts.slice(0, 6).map((conflict) => (
                  <button
                    key={`${conflict.kind}-${conflict.instanceId}-${conflict.studentId}`}
                    type="button"
                    onClick={() => onSelectInstance(conflict.instanceId)}
                    className={`${timetableNestedSurface} block w-full p-2 text-left text-xs hover:bg-[#EAB308]/5 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none`}
                  >
                    <div className="font-bold text-gray-900">
                      {dateLabel(conflict.date)} • {conflict.activityTitle}
                    </div>
                    <div className="mt-0.5 text-gray-600">
                      {conflictText(conflict)}
                    </div>
                  </button>
                ))}
                {conflicts.length > 6 && (
                  <div className="text-xs font-semibold text-[#92400E]">
                    + {conflicts.length - 6} weitere Konflikte
                  </div>
                )}
              </div>
            </div>
          )}
        </div>
      )}
    </section>
  );
}
