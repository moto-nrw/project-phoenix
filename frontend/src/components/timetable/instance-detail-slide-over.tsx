"use client";

/**
 * InstanceDetailSlideOver — right-side slide-over showing the full state
 * of a clicked instance plus lifecycle action buttons.
 *
 * Shows the operational state of one timetable instance: lifecycle,
 * assigned staff, children, attendance state, and admin corrections.
 */

import { useMemo, useState } from "react";
import type React from "react";
import {
  CheckCircle2,
  CircleX,
  DoorOpen,
  Pencil,
  Play,
  Repeat,
  Square,
  StickyNote,
  Timer,
  TriangleAlert,
  UserCheck,
  Users,
  X,
} from "lucide-react";

import { Button } from "~/components/ui/button";
import {
  SlideOver,
  SlideOverClose,
  SlideOverContent,
  SlideOverDescription,
  SlideOverFooter,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";
import {
  getActivityTypeBadge,
  getGermanWeekdayLong,
  getStatusLabel,
} from "~/lib/timetable-helpers";
import type {
  AttendancePatchBody,
  EnrichedInstance,
  InstanceStudentSummary,
  InstanceStatus,
} from "~/lib/timetable-types";

export type LifecycleAction = "start" | "complete" | "cancel";

interface InstanceDetailSlideOverProps {
  instance: EnrichedInstance | null;
  onClose: () => void;
  onLifecycleAction: (action: LifecycleAction) => Promise<void>;
  onEdit?: (instance: EnrichedInstance) => void;
  onRepeat?: (instance: EnrichedInstance) => void;
  staffNames?: Map<string, string>;
  studentNames?: Map<string, string>;
  onAttendancePatch?: (
    instanceId: string,
    studentId: string,
    body: AttendancePatchBody,
  ) => Promise<void>;
  /**
   * When true, edit + spontaneous-create UI surfaces are visible but
   * disabled with a tooltip. Default true until backend PUT/POST land.
   */
  editDeferred?: boolean;
}

function germanFullDate(iso: string): string {
  const d = new Date(`${iso}T00:00:00`);
  if (Number.isNaN(d.getTime())) return iso;
  const day = String(d.getDate()).padStart(2, "0");
  const month = String(d.getMonth() + 1).padStart(2, "0");
  return `${getGermanWeekdayLong(d)}, ${day}.${month}.${d.getFullYear()}`;
}

interface StatusBadgeProps {
  status: InstanceStatus;
}

function StatusBadge({ status }: StatusBadgeProps) {
  const palette: Record<InstanceStatus, { bg: string; text: string }> = {
    planned: { bg: "#F3F4F6", text: "#374151" },
    active: { bg: "#83CD2D", text: "#FFFFFF" },
    completed: { bg: "#E5E7EB", text: "#6B7280" },
    cancelled: { bg: "#FF3130", text: "#FFFFFF" },
  };
  const { bg, text } = palette[status];
  return (
    <span
      className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-bold tracking-wide uppercase"
      style={{ backgroundColor: bg, color: text }}
    >
      {status === "active" && (
        <span className="h-1.5 w-1.5 rounded-full bg-white" />
      )}
      {getStatusLabel(status)}
    </span>
  );
}

function attendanceLabel(status: InstanceStudentSummary["status"]): string {
  switch (status) {
    case "expected":
      return "Erwartet";
    case "present":
      return "Anwesend";
    case "absent":
      return "Fehlt";
  }
}

function attendanceTone(status: InstanceStudentSummary["status"]): string {
  switch (status) {
    case "present":
      return "border-[#BBF7D0] bg-[#F0FDF4] text-[#15803D]";
    case "absent":
      return "border-[#FECACA] bg-[#FEF2F2] text-[#B91C1C]";
    case "expected":
      return "border-slate-200 bg-slate-50 text-slate-600";
  }
}

function fallbackStudentRows(studentIds: string[]): InstanceStudentSummary[] {
  return studentIds.map((studentId) => ({
    studentId,
    status: "expected",
  }));
}

export function InstanceDetailSlideOver({
  instance,
  onClose,
  onLifecycleAction,
  onEdit,
  onRepeat,
  staffNames = new Map(),
  studentNames = new Map(),
  onAttendancePatch,
  editDeferred = true,
}: InstanceDetailSlideOverProps) {
  const [pendingAction, setPendingAction] = useState<LifecycleAction | null>(
    null,
  );
  const [pendingStudentId, setPendingStudentId] = useState<string | null>(null);
  const students = useMemo(
    () =>
      instance
        ? instance.students.length > 0
          ? instance.students
          : fallbackStudentRows(instance.studentIds)
        : [],
    [instance],
  );
  const groupedStudents = useMemo(
    () => ({
      expected: students.filter((student) => student.status === "expected"),
      present: students.filter((student) => student.status === "present"),
      absent: students.filter((student) => student.status === "absent"),
    }),
    [students],
  );

  const handleLifecycle = async (action: LifecycleAction) => {
    setPendingAction(action);
    try {
      await onLifecycleAction(action);
    } finally {
      setPendingAction(null);
    }
  };

  const handleAttendancePatch = async (
    studentId: string,
    body: AttendancePatchBody,
  ) => {
    if (!instance || !onAttendancePatch) return;
    setPendingStudentId(studentId);
    try {
      await onAttendancePatch(instance.id, studentId, body);
    } finally {
      setPendingStudentId(null);
    }
  };

  const open = instance !== null;

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
                <div className="flex items-center gap-2">
                  <SlideOverTitle>{instance.title}</SlideOverTitle>
                  <StatusBadge status={instance.status} />
                  {(() => {
                    const tb = getActivityTypeBadge(instance.activityType);
                    return tb ? (
                      <span
                        className="rounded px-1.5 py-0.5 text-[9px] font-bold tracking-wide text-white uppercase"
                        style={{ backgroundColor: tb.bg }}
                      >
                        {tb.label}
                      </span>
                    ) : null;
                  })()}
                </div>
                <SlideOverDescription>
                  {germanFullDate(instance.date)} • {instance.startTime} –{" "}
                  {instance.endTime}
                </SlideOverDescription>
              </div>
              <SlideOverClose asChild>
                <button
                  type="button"
                  className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-slate-200 text-slate-500 hover:bg-slate-100"
                  aria-label="Schließen"
                >
                  <X className="h-4 w-4" />
                </button>
              </SlideOverClose>
            </div>
          </SlideOverHeader>

          <div className="flex-1 space-y-5 overflow-y-auto px-5 py-4">
            {instance.conflictWarnings.length > 0 && (
              <div className="rounded-md border border-[#FECACA] bg-[#FEF2F2] p-3">
                <div className="flex items-center gap-2 text-xs font-bold text-[#7F1D1D]">
                  <TriangleAlert className="h-4 w-4" />
                  {instance.conflictWarnings.length} Konflikt(e)
                </div>
                <ul className="mt-1 space-y-0.5 text-xs text-[#991B1B]">
                  {instance.conflictWarnings.map((w, i) => (
                    <li key={i}>• {w.message}</li>
                  ))}
                </ul>
              </div>
            )}

            <StatsRow instance={instance} />

            <Section title="Details">
              <Row icon={<Timer className="h-4 w-4" />} label="Zeit">
                {instance.startTime} – {instance.endTime}
              </Row>
              <Row icon={<DoorOpen className="h-4 w-4" />} label="Raum">
                {instance.roomName || `Raum #${instance.roomId}`}
              </Row>
              <Row
                icon={<UserCheck className="h-4 w-4" />}
                label={`Personal (${instance.staffCount})`}
              >
                {instance.staffCount === 0
                  ? "Niemand zugeordnet"
                  : `${instance.staffCount - instance.absentStaffCount} aktiv${
                      instance.absentStaffCount > 0
                        ? `, ${instance.absentStaffCount} abwesend`
                        : ""
                    }`}
              </Row>
              <Row icon={<Users className="h-4 w-4" />} label="Schüler">
                {instance.expectedStudentsCount + instance.presentStudentsCount}{" "}
                eingetragen
                {instance.presentStudentsCount > 0
                  ? ` • ${instance.presentStudentsCount} anwesend`
                  : ""}
              </Row>
              {instance.notes && (
                <Row icon={<StickyNote className="h-4 w-4" />} label="Notiz">
                  <span className="whitespace-pre-line">{instance.notes}</span>
                </Row>
              )}
            </Section>

            <Section title="Personal">
              {instance.staff.length === 0 ? (
                <EmptyLine>Kein Personal zugeordnet.</EmptyLine>
              ) : (
                <div className="space-y-1.5">
                  {instance.staff.map((item) => (
                    <PersonLine
                      key={item.staffId}
                      name={
                        staffNames.get(item.staffId) ??
                        `Personal #${item.staffId}`
                      }
                      meta={[
                        item.isPrimary ? "Primär" : null,
                        item.isAbsent ? "Abwesend" : null,
                        item.isSubstitute ? "Ersatz" : null,
                      ]}
                      danger={item.isAbsent}
                    />
                  ))}
                </div>
              )}
            </Section>

            <Section title="Kinder">
              {students.length === 0 ? (
                <EmptyLine>Keine Kinder geplant.</EmptyLine>
              ) : (
                <div className="space-y-3">
                  {(
                    [
                      ["expected", groupedStudents.expected],
                      ["present", groupedStudents.present],
                      ["absent", groupedStudents.absent],
                    ] as const
                  ).map(([status, rows]) => (
                    <StudentGroup
                      key={status}
                      status={status}
                      students={rows}
                      studentNames={studentNames}
                      pendingStudentId={pendingStudentId}
                      onAttendancePatch={onAttendancePatch}
                      handleAttendancePatch={handleAttendancePatch}
                    />
                  ))}
                </div>
              )}
            </Section>
          </div>

          <SlideOverFooter>
            <div className="flex flex-wrap items-center gap-2">
              {instance.status === "planned" && (
                <Button
                  variant="success"
                  size="sm"
                  type="button"
                  onClick={() => void handleLifecycle("start")}
                  isLoading={pendingAction === "start"}
                  loadingText="Starte …"
                  disabled={pendingAction !== null}
                >
                  <span className="inline-flex items-center gap-2">
                    <Play className="h-4 w-4" />
                    Starten
                  </span>
                </Button>
              )}
              {instance.status === "planned" && !editDeferred && onEdit && (
                <Button
                  variant="outline"
                  size="sm"
                  type="button"
                  onClick={() => onEdit(instance)}
                  disabled={pendingAction !== null}
                >
                  <span className="inline-flex items-center gap-2">
                    <Pencil className="h-4 w-4" />
                    Bearbeiten
                  </span>
                </Button>
              )}
              {instance.status === "planned" &&
                !instance.activityGroupId &&
                onRepeat && (
                  <Button
                    variant="outline"
                    size="sm"
                    type="button"
                    onClick={() => onRepeat(instance)}
                    disabled={pendingAction !== null}
                  >
                    <span className="inline-flex items-center gap-2">
                      <Repeat className="h-4 w-4" />
                      Wiederholen
                    </span>
                  </Button>
                )}
              {instance.status === "active" && (
                <Button
                  variant="primary"
                  size="sm"
                  type="button"
                  onClick={() => void handleLifecycle("complete")}
                  isLoading={pendingAction === "complete"}
                  loadingText="Beende …"
                  disabled={pendingAction !== null}
                >
                  <span className="inline-flex items-center gap-2">
                    <Square className="h-4 w-4" />
                    Beenden
                  </span>
                </Button>
              )}
              {(instance.status === "planned" ||
                instance.status === "active") && (
                <Button
                  variant="outline_danger"
                  size="sm"
                  type="button"
                  onClick={() => void handleLifecycle("cancel")}
                  isLoading={pendingAction === "cancel"}
                  loadingText="Sage ab …"
                  disabled={pendingAction !== null}
                >
                  <span className="inline-flex items-center gap-2">
                    <CircleX className="h-4 w-4" />
                    Absagen
                  </span>
                </Button>
              )}
              {instance.status === "completed" && (
                <span className="inline-flex items-center gap-2 text-xs text-slate-500">
                  <CheckCircle2 className="h-4 w-4" />
                  Diese Aktivität ist bereits abgeschlossen.
                </span>
              )}
              {instance.status === "cancelled" && (
                <span className="inline-flex items-center gap-2 text-xs text-slate-500">
                  <CircleX className="h-4 w-4" />
                  Diese Aktivität wurde abgesagt.
                </span>
              )}
            </div>
            {editDeferred && (
              <div className="flex items-center justify-end gap-2 text-xs text-slate-400">
                <Pencil className="h-3.5 w-3.5" />
                <span>Bearbeiten kommt im nächsten Update</span>
              </div>
            )}
          </SlideOverFooter>
        </SlideOverContent>
      )}
    </SlideOver>
  );
}

function StudentGroup({
  status,
  students,
  studentNames,
  pendingStudentId,
  onAttendancePatch,
  handleAttendancePatch,
}: {
  status: InstanceStudentSummary["status"];
  students: InstanceStudentSummary[];
  studentNames: Map<string, string>;
  pendingStudentId: string | null;
  onAttendancePatch?: InstanceDetailSlideOverProps["onAttendancePatch"];
  handleAttendancePatch: (
    studentId: string,
    body: AttendancePatchBody,
  ) => Promise<void>;
}) {
  if (students.length === 0) return null;
  return (
    <div className="space-y-1.5">
      <div className="flex items-center justify-between text-[11px] font-bold tracking-wide text-slate-400 uppercase">
        <span>{attendanceLabel(status)}</span>
        <span>{students.length}</span>
      </div>
      {students.map((student) => (
        <div
          key={student.studentId}
          className={`flex flex-wrap items-center justify-between gap-2 rounded-md border px-3 py-2 ${attendanceTone(
            student.status,
          )}`}
        >
          <div className="min-w-0">
            <div className="truncate text-sm font-semibold text-slate-900">
              {studentNames.get(student.studentId) ??
                `Kind #${student.studentId}`}
            </div>
            <div className="text-[11px] text-slate-500">
              {attendanceLabel(student.status)}
              {student.substatus ? ` • ${student.substatus}` : ""}
              {student.note ? ` • ${student.note}` : ""}
            </div>
          </div>
          {onAttendancePatch && (
            <div className="flex items-center gap-1">
              {student.status !== "present" && (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  isLoading={pendingStudentId === student.studentId}
                  disabled={pendingStudentId !== null}
                  onClick={() =>
                    void handleAttendancePatch(student.studentId, {
                      status: "present",
                      substatus: null,
                    })
                  }
                >
                  Anwesend
                </Button>
              )}
              {student.status !== "absent" && (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  isLoading={pendingStudentId === student.studentId}
                  disabled={pendingStudentId !== null}
                  onClick={() =>
                    void handleAttendancePatch(student.studentId, {
                      status: "absent",
                      substatus: null,
                    })
                  }
                >
                  Fehlt
                </Button>
              )}
              {student.status !== "expected" && (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  isLoading={pendingStudentId === student.studentId}
                  disabled={pendingStudentId !== null}
                  onClick={() =>
                    void handleAttendancePatch(student.studentId, {
                      status: "expected",
                      substatus: null,
                      note: null,
                    })
                  }
                >
                  Zurücksetzen
                </Button>
              )}
            </div>
          )}
        </div>
      ))}
    </div>
  );
}

function EmptyLine({ children }: { children: React.ReactNode }) {
  return (
    <div className="rounded-md border border-dashed border-slate-300 bg-slate-50 px-3 py-2 text-xs text-slate-500">
      {children}
    </div>
  );
}

function PersonLine({
  name,
  meta,
  danger,
}: {
  name: string;
  meta: Array<string | null>;
  danger?: boolean;
}) {
  const labels = meta.filter(Boolean);
  return (
    <div
      className={`rounded-md border px-3 py-2 ${
        danger
          ? "border-[#FECACA] bg-[#FEF2F2]"
          : "border-slate-200 bg-slate-50"
      }`}
    >
      <div className="text-sm font-semibold text-slate-900">{name}</div>
      {labels.length > 0 && (
        <div
          className={`mt-0.5 text-[11px] ${
            danger ? "text-[#B91C1C]" : "text-slate-500"
          }`}
        >
          {labels.join(" • ")}
        </div>
      )}
    </div>
  );
}

interface StatsRowProps {
  instance: EnrichedInstance;
}

function StatsRow({ instance }: StatsRowProps) {
  const expected = instance.expectedStudentsCount;
  const present = instance.presentStudentsCount;
  return (
    <div className="grid grid-cols-3 gap-2">
      <StatBox
        label="Anwesend"
        value={present > 0 ? `${present} / ${expected + present}` : "—"}
        color="#15803D"
        bg="#F0FDF4"
        border="#BBF7D0"
      />
      <StatBox
        label="Personal"
        value={`${instance.staffCount - instance.absentStaffCount} / ${instance.staffCount}`}
        color="#1E3A8A"
        bg="#EFF6FF"
        border="#BFDBFE"
      />
      <StatBox
        label="Status"
        value={getStatusLabel(instance.status)}
        color={instance.status === "cancelled" ? "#7F1D1D" : "#1F2937"}
        bg={instance.status === "cancelled" ? "#FEF2F2" : "#F8FAFC"}
        border={instance.status === "cancelled" ? "#FECACA" : "#E2E8F0"}
      />
    </div>
  );
}

interface StatBoxProps {
  label: string;
  value: string;
  color: string;
  bg: string;
  border: string;
}

function StatBox({ label, value, color, bg, border }: StatBoxProps) {
  return (
    <div
      className="rounded-md border px-3 py-2"
      style={{ backgroundColor: bg, borderColor: border }}
    >
      <div
        className="text-[10px] font-semibold tracking-wide uppercase"
        style={{ color }}
      >
        {label}
      </div>
      <div className="text-base font-bold" style={{ color }}>
        {value}
      </div>
    </div>
  );
}

interface SectionProps {
  title: string;
  children: React.ReactNode;
}

function Section({ title, children }: SectionProps) {
  return (
    <div className="space-y-2">
      <h4 className="text-[10px] font-bold tracking-wider text-slate-400 uppercase">
        {title}
      </h4>
      <div className="space-y-1.5">{children}</div>
    </div>
  );
}

interface RowProps {
  icon: React.ReactNode;
  label: string;
  children: React.ReactNode;
}

function Row({ icon, label, children }: RowProps) {
  return (
    <div className="flex items-start gap-3 text-sm">
      <span className="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center text-slate-400">
        {icon}
      </span>
      <div className="flex min-w-0 flex-1 flex-col">
        <span className="text-[11px] font-medium text-slate-500">{label}</span>
        <span className="text-sm text-slate-900">{children}</span>
      </div>
    </div>
  );
}
