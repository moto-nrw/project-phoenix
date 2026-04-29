"use client";

import { useMemo, useState } from "react";
import type React from "react";
import {
  CalendarClock,
  RefreshCw,
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

interface PlanQualityPanelProps {
  instances: EnrichedInstance[];
  gaps: GapInstance[];
  conflicts: ExceptionConflict[];
  staff: Staff[];
  loading: boolean;
  onSelectInstance: (instanceId: string) => void;
  onSubstitute: (
    absentStaffId: string,
    substituteStaffId: string,
    date: string,
  ) => Promise<void>;
  onReplanWeek: () => Promise<void>;
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
  onSelectInstance,
  onSubstitute,
  onReplanWeek,
}: PlanQualityPanelProps) {
  const [replanConfirm, setReplanConfirm] = useState(false);
  const [replanning, setReplanning] = useState(false);
  const [substituteByGap, setSubstituteByGap] = useState<
    Record<string, string>
  >({});
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
  const issueCount = gaps.length + conflicts.length + conflictCount;
  const availableStaff = useMemo(
    () => staff.filter((item) => item.workStatus !== "checked_out"),
    [staff],
  );

  const handleReplan = () => {
    if (!replanConfirm) {
      setReplanConfirm(true);
      return;
    }
    setReplanning(true);
    void onReplanWeek().finally(() => {
      setReplanning(false);
      setReplanConfirm(false);
    });
  };

  const handleSubstitute = (gap: GapInstance) => {
    const substituteStaffId = substituteByGap[gap.instanceId];
    if (!substituteStaffId) return;
    const instance = instances.find((item) => item.id === gap.instanceId);
    const absentStaff = instance?.staff.find((item) => item.isAbsent);
    if (!absentStaff) return;

    setSubmittingGapId(gap.instanceId);
    void onSubstitute(absentStaff.staffId, substituteStaffId, gap.date).finally(
      () => setSubmittingGapId(null),
    );
  };

  return (
    <section className="rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="text-sm font-bold text-slate-900">Planstatus</h2>
          <p className="mt-0.5 text-xs text-slate-500">
            {loading
              ? "Prüfe Personal, Konflikte und Ausnahmen …"
              : issueCount === 0
                ? "Diese Ansicht hat keine offenen Planungsprobleme."
                : `${issueCount} Punkt(e), die vor dem Betrieb geprüft werden sollten.`}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {replanConfirm && !replanning && (
            <span className="text-xs text-slate-600">
              Ersetzt nur sichere geplante Vorlagen-Termine.
            </span>
          )}
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={handleReplan}
            isLoading={replanning}
            loadingText="Berechne …"
          >
            <span className="inline-flex items-center gap-2">
              <RefreshCw className="h-4 w-4" />
              {replanConfirm
                ? "Neu berechnen bestätigen"
                : "Woche neu berechnen"}
            </span>
          </Button>
          {replanConfirm && !replanning && (
            <button
              type="button"
              className="text-xs font-semibold text-slate-500 hover:text-slate-700"
              onClick={() => setReplanConfirm(false)}
            >
              Abbrechen
            </button>
          )}
        </div>
      </div>

      <div className="mt-4 grid gap-2 md:grid-cols-4">
        <Metric
          icon={<CalendarClock className="h-4 w-4" />}
          label="Geplant"
          value={String(plannedCount)}
          tone="neutral"
        />
        <Metric
          icon={<UsersRound className="h-4 w-4" />}
          label="Läuft"
          value={String(activeCount)}
          tone="success"
        />
        <Metric
          icon={<UserPlus className="h-4 w-4" />}
          label="Ohne Personal"
          value={String(gaps.length)}
          tone={gaps.length > 0 ? "danger" : "neutral"}
        />
        <Metric
          icon={<ShieldAlert className="h-4 w-4" />}
          label="Abwesend"
          value={String(absentStaffCount)}
          tone={absentStaffCount > 0 ? "warning" : "neutral"}
        />
      </div>

      {(gaps.length > 0 || conflicts.length > 0) && (
        <div className="mt-4 grid gap-3 lg:grid-cols-2">
          {gaps.length > 0 && (
            <div className="rounded-md border border-[#FECACA] bg-[#FEF2F2] p-3">
              <div className="flex items-center gap-2 text-xs font-bold text-[#7F1D1D]">
                <UserPlus className="h-4 w-4" />
                Personal-Lücken
              </div>
              <div className="mt-2 space-y-2">
                {gaps.map((gap) => {
                  const instance = instances.find(
                    (item) => item.id === gap.instanceId,
                  );
                  const canSubstitute = instance?.staff.some(
                    (item) => item.isAbsent,
                  );
                  return (
                    <div
                      key={gap.instanceId}
                      className="rounded-md bg-white p-2 text-xs shadow-sm"
                    >
                      <button
                        type="button"
                        onClick={() => onSelectInstance(gap.instanceId)}
                        className="text-left font-bold text-slate-900 hover:text-[#5080D8]"
                      >
                        {dateLabel(gap.date)} • {gap.startTime} {gap.title}
                      </button>
                      <div className="mt-1 text-slate-500">
                        {gap.assignedStaffCount === 0
                          ? "Kein Personal zugeordnet."
                          : `${gap.absentStaffCount} von ${gap.assignedStaffCount} zugeordneten Personen abwesend.`}
                      </div>
                      {canSubstitute && (
                        <div className="mt-2 flex flex-wrap items-center gap-2">
                          <select
                            value={substituteByGap[gap.instanceId] ?? ""}
                            onChange={(e) =>
                              setSubstituteByGap((prev) => ({
                                ...prev,
                                [gap.instanceId]: e.target.value,
                              }))
                            }
                            className="min-w-48 rounded-md border border-slate-300 bg-white px-2 py-1 text-xs"
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
                            size="sm"
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
            <div className="rounded-md border border-[#FDE68A] bg-[#FFFBEB] p-3">
              <div className="flex items-center gap-2 text-xs font-bold text-[#713F12]">
                <TriangleAlert className="h-4 w-4" />
                Ausnahmen prüfen
              </div>
              <div className="mt-2 space-y-2">
                {conflicts.slice(0, 6).map((conflict) => (
                  <button
                    key={`${conflict.kind}-${conflict.instanceId}-${conflict.studentId}`}
                    type="button"
                    onClick={() => onSelectInstance(conflict.instanceId)}
                    className="block w-full rounded-md bg-white p-2 text-left text-xs shadow-sm hover:bg-[#FFF7ED]"
                  >
                    <div className="font-bold text-slate-900">
                      {dateLabel(conflict.date)} • {conflict.activityTitle}
                    </div>
                    <div className="mt-0.5 text-slate-600">
                      {conflictText(conflict)}
                    </div>
                  </button>
                ))}
                {conflicts.length > 6 && (
                  <div className="text-xs font-semibold text-[#713F12]">
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

function Metric({
  icon,
  label,
  value,
  tone,
}: {
  icon: React.ReactNode;
  label: string;
  value: string;
  tone: "neutral" | "success" | "warning" | "danger";
}) {
  const palette = {
    neutral: "border-slate-200 bg-slate-50 text-slate-700",
    success: "border-[#BBF7D0] bg-[#F0FDF4] text-[#15803D]",
    warning: "border-[#FDE68A] bg-[#FFFBEB] text-[#A16207]",
    danger: "border-[#FECACA] bg-[#FEF2F2] text-[#B91C1C]",
  };
  return (
    <div className={`rounded-md border px-3 py-2 ${palette[tone]}`}>
      <div className="flex items-center gap-2 text-[11px] font-bold uppercase">
        {icon}
        {label}
      </div>
      <div className="mt-1 text-lg font-bold">{value}</div>
    </div>
  );
}
