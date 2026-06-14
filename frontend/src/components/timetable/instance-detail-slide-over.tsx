"use client";

/**
 * InstanceDetailSlideOver — right-side slide-over showing the full state
 * of a clicked instance plus lifecycle action buttons.
 *
 * Shows the operational state of one timetable instance: lifecycle,
 * assigned staff, children, attendance state, and admin corrections.
 */

import { useEffect, useMemo, useState } from "react";
import type React from "react";
import {
  Check,
  CheckCircle2,
  CircleX,
  DoorOpen,
  Pencil,
  Repeat,
  RotateCcw,
  StickyNote,
  Timer,
  Trash2,
  TriangleAlert,
  UserCheck,
  Users,
  X,
} from "lucide-react";

import { Button } from "~/components/ui/button";
import { useModal } from "~/components/dashboard/modal-context";
import { ConfirmationModal } from "~/components/ui/modal";
import { LOCATION_COLORS } from "~/lib/location-helper";
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
  getActivityTypeBadge,
  getGermanWeekdayLong,
  getStatusLabel,
} from "~/lib/timetable-helpers";
import {
  timetableDangerPanel,
  timetableMutedSurface,
  timetableNestedSurface,
} from "./timetable-style";
import type {
  AttendancePatchBody,
  EnrichedInstance,
  InstanceStudentSummary,
  InstanceStatus,
} from "~/lib/timetable-types";

export type LifecycleAction = "start" | "complete" | "cancel";

type PendingConfirmAction = "complete" | "cancel" | "delete";

const CONFIRM_DIALOGS: Record<
  PendingConfirmAction,
  {
    title: string;
    body: string;
    confirmText: string;
    confirmButtonClass?: string;
  }
> = {
  complete: {
    title: "Termin beenden?",
    body: "Der laufende Termin wird beendet und kann nicht erneut gestartet werden.",
    confirmText: "Beenden",
  },
  cancel: {
    title: "Termin absagen?",
    body: "Der Termin wird im Plan als abgesagt markiert. Das kann nicht rückgängig gemacht werden.",
    confirmText: "Absagen",
  },
  delete: {
    title: "Abgesagten Termin löschen?",
    body: "Der abgesagte Termin wird dauerhaft entfernt.",
    confirmText: "Löschen",
    confirmButtonClass: "bg-red-600 hover:bg-red-700",
  },
};

interface InstanceDetailSlideOverProps {
  instance: EnrichedInstance | null;
  onClose: () => void;
  onLifecycleAction: (action: LifecycleAction) => Promise<void>;
  onDeleteCancelled?: (instance: EnrichedInstance) => Promise<void>;
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

const EMPTY_STAFF_NAMES = new Map<string, string>();
const EMPTY_STUDENT_NAMES = new Map<string, string>();

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
    active: { bg: LOCATION_COLORS.GROUP_ROOM, text: "#FFFFFF" },
    completed: { bg: "#E5E7EB", text: "#6B7280" },
    cancelled: { bg: LOCATION_COLORS.HOME, text: "#FFFFFF" },
  };
  const { bg, text } = palette[status];
  return (
    <span
      className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-bold tracking-wide uppercase"
      style={{ backgroundColor: bg, color: text }}
    >
      {status === "active" && (
        <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-white" />
      )}
      {getStatusLabel(status)}
    </span>
  );
}

function attendanceLabel(
  status: InstanceStudentSummary["status"],
  plannedContext = false,
): string {
  switch (status) {
    case "expected":
      return "Erwartet";
    case "present":
      return "Anwesend";
    case "absent":
      return plannedContext ? "Abgemeldet" : "Fehlt";
  }
}

function attendanceSubstatusLabel(
  substatus: NonNullable<InstanceStudentSummary["substatus"]>,
): string {
  switch (substatus) {
    case "late":
      return "Verspätet";
    case "excused":
      return "Entschuldigt";
    case "sick":
      return "Krank";
    case "field_trip":
      return "Ausflug";
    case "other":
      return "Sonstiges";
  }
}

function attendanceTone(status: InstanceStudentSummary["status"]): string {
  switch (status) {
    case "present":
      return "border-[#83CD2D]/20 bg-[#83CD2D]/10 text-[#6BA023]";
    case "absent":
      return "border-[#FF3130]/20 bg-[#FF3130]/10 text-[#CC2626]";
    case "expected":
      return "border-gray-200 bg-gray-50 text-gray-600";
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
  onDeleteCancelled,
  onEdit,
  onRepeat,
  staffNames = EMPTY_STAFF_NAMES,
  studentNames = EMPTY_STUDENT_NAMES,
  onAttendancePatch,
  editDeferred = true,
}: InstanceDetailSlideOverProps) {
  const { isModalOpen } = useModal();
  const [pendingAction, setPendingAction] = useState<LifecycleAction | null>(
    null,
  );
  const [pendingConfirm, setPendingConfirm] =
    useState<PendingConfirmAction | null>(null);
  const [pendingDelete, setPendingDelete] = useState(false);
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

  useEffect(() => {
    setPendingConfirm(null);
  }, [instance?.id]);

  const handleLifecycle = async (action: LifecycleAction) => {
    setPendingAction(action);
    try {
      await onLifecycleAction(action);
    } finally {
      setPendingAction(null);
    }
  };

  const handleDeleteCancelled = async () => {
    if (!instance || !onDeleteCancelled) return;
    setPendingDelete(true);
    try {
      await onDeleteCancelled(instance);
    } finally {
      setPendingDelete(false);
    }
  };

  const handleConfirm = () => {
    const action = pendingConfirm;
    setPendingConfirm(null);
    if (action === "delete") {
      void handleDeleteCancelled();
    } else if (action) {
      void handleLifecycle(action);
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
        <SlideOverContent
          // The confirmation modal portals to document.body and therefore
          // lives outside the drawer's DOM. Without these guards Vaul's
          // DismissableLayer treats every click inside the open modal as an
          // outside-click and closes the slide-over, unmounting the modal
          // before its buttons can fire. See issue #1358.
          onInteractOutside={(event) => {
            if (isModalOpen || pendingConfirm !== null) event.preventDefault();
          }}
          onEscapeKeyDown={(event) => {
            if (isModalOpen || pendingConfirm !== null) event.preventDefault();
          }}
        >
          <SlideOverHeader>
            <div className="flex items-start justify-between gap-2">
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <SlideOverTitle>{instance.title}</SlideOverTitle>
                  <StatusBadge status={instance.status} />
                  {instance.isSpontaneous && (
                    <span
                      className="rounded-full bg-gray-100 px-1.5 py-0.5 text-[9px] font-bold tracking-wide text-gray-600 uppercase"
                      title="Dieser Termin wurde spontan gestartet und war nicht geplant."
                    >
                      Spontan gestartet
                    </span>
                  )}
                  {(() => {
                    const tb = getActivityTypeBadge(instance.activityType);
                    return tb ? (
                      <span
                        className="rounded-full px-1.5 py-0.5 text-[9px] font-bold tracking-wide text-white uppercase"
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
              <SlideOverCloseButton />
            </div>
          </SlideOverHeader>

          <div className="flex-1 space-y-5 overflow-y-auto px-5 py-4">
            {instance.conflictWarnings.length > 0 && (
              <div className={timetableDangerPanel}>
                <div className="flex items-center gap-2 text-xs font-bold text-[#CC2626]">
                  <TriangleAlert className="h-4 w-4" />
                  {instance.conflictWarnings.length} Konflikt(e)
                </div>
                <ul className="mt-1 space-y-0.5 text-xs text-[#CC2626]">
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
              <Row icon={<Users className="h-4 w-4" />} label="Kinder">
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
                        item.isPrimary ? "Zuständig" : null,
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
                      onAttendancePatch={
                        instance.status === "planned" ||
                        instance.status === "active" ||
                        instance.status === "completed"
                          ? onAttendancePatch
                          : undefined
                      }
                      instanceStatus={instance.status}
                      handleAttendancePatch={handleAttendancePatch}
                    />
                  ))}
                </div>
              )}
            </Section>
          </div>

          <SlideOverFooter>
            <div className="flex flex-wrap items-center gap-2">
              {instance.status === "planned" && !editDeferred && onEdit && (
                <Button
                  variant="outline"
                  size="md"
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
                    size="md"
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
                  size="md"
                  type="button"
                  onClick={() => setPendingConfirm("complete")}
                  isLoading={pendingAction === "complete"}
                  loadingText="Beende …"
                  disabled={pendingAction !== null}
                >
                  <span className="inline-flex items-center gap-2">
                    <CheckCircle2 className="h-4 w-4" />
                    Beenden
                  </span>
                </Button>
              )}
              {(instance.status === "planned" ||
                instance.status === "active") && (
                <Button
                  variant="outline_danger"
                  size="md"
                  type="button"
                  onClick={() => setPendingConfirm("cancel")}
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
                <span className="inline-flex items-center gap-2 text-xs text-gray-500">
                  <CheckCircle2 className="h-4 w-4" />
                  Diese Aktivität ist bereits abgeschlossen.
                </span>
              )}
              {instance.status === "cancelled" && (
                <>
                  <span className="inline-flex items-center gap-2 text-xs text-gray-500">
                    <CircleX className="h-4 w-4" />
                    Diese Aktivität wurde abgesagt.
                  </span>
                  {onDeleteCancelled && (
                    <Button
                      variant="outline_danger"
                      size="md"
                      type="button"
                      onClick={() => setPendingConfirm("delete")}
                      isLoading={pendingDelete}
                      loadingText="Lösche …"
                      disabled={pendingAction !== null || pendingDelete}
                    >
                      <span className="inline-flex items-center gap-2">
                        <Trash2 className="h-4 w-4" />
                        Löschen
                      </span>
                    </Button>
                  )}
                </>
              )}
            </div>
            {editDeferred && (
              <div className="flex items-center justify-end gap-2 text-xs text-gray-400">
                <Pencil className="h-3.5 w-3.5" />
                <span>Bearbeiten kommt im nächsten Update</span>
              </div>
            )}
          </SlideOverFooter>
        </SlideOverContent>
      )}
      {pendingConfirm && (
        <ConfirmationModal
          isOpen
          onClose={() => setPendingConfirm(null)}
          onConfirm={handleConfirm}
          title={CONFIRM_DIALOGS[pendingConfirm].title}
          confirmText={CONFIRM_DIALOGS[pendingConfirm].confirmText}
          cancelText="Abbrechen"
          confirmButtonClass={
            CONFIRM_DIALOGS[pendingConfirm].confirmButtonClass
          }
        >
          <p className="text-sm leading-relaxed text-gray-600">
            {pendingConfirm === "cancel" && instance?.status === "active"
              ? "Die laufende Betreuung wird gestoppt und der Termin als abgesagt markiert. Das kann nicht rückgängig gemacht werden."
              : CONFIRM_DIALOGS[pendingConfirm].body}
          </p>
        </ConfirmationModal>
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
  instanceStatus,
  handleAttendancePatch,
}: {
  status: InstanceStudentSummary["status"];
  students: InstanceStudentSummary[];
  studentNames: Map<string, string>;
  pendingStudentId: string | null;
  onAttendancePatch?: InstanceDetailSlideOverProps["onAttendancePatch"];
  instanceStatus: InstanceStatus;
  handleAttendancePatch: (
    studentId: string,
    body: AttendancePatchBody,
  ) => Promise<void>;
}) {
  if (students.length === 0) return null;
  const isPlanned = instanceStatus === "planned";
  return (
    <div className="space-y-1.5">
      <div className="flex items-center justify-between text-[11px] font-bold tracking-wide text-gray-400 uppercase">
        <span>{attendanceLabel(status, isPlanned)}</span>
        <span>{students.length}</span>
      </div>
      {students.map((student) => (
        <div
          key={student.studentId}
          className={`flex flex-wrap items-center justify-between gap-2 rounded-xl border px-3 py-2 shadow-sm ${attendanceTone(
            student.status,
          )}`}
        >
          <div className="min-w-0">
            <div className="truncate text-sm font-semibold text-gray-900">
              {studentNames.get(student.studentId) ??
                `Kind #${student.studentId}`}
            </div>
            <div className="text-[11px] text-gray-500">
              {attendanceLabel(student.status, isPlanned)}
              {student.substatus
                ? ` • ${attendanceSubstatusLabel(student.substatus)}`
                : ""}
              {student.note ? ` • ${student.note}` : ""}
            </div>
          </div>
          {onAttendancePatch && (
            <div className="flex shrink-0 items-center gap-1">
              {!isPlanned && student.status !== "present" && (
                <IconActionButton
                  icon={<Check className="h-3.5 w-3.5" />}
                  label="Als anwesend markieren"
                  tone="green"
                  isLoading={pendingStudentId === student.studentId}
                  disabled={pendingStudentId !== null}
                  onClick={() =>
                    void handleAttendancePatch(student.studentId, {
                      status: "present",
                      substatus: null,
                    })
                  }
                />
              )}
              {student.status !== "absent" && (
                <IconActionButton
                  icon={<X className="h-3.5 w-3.5" />}
                  label={isPlanned ? "Kind abmelden" : "Als fehlend markieren"}
                  tone="red"
                  isLoading={pendingStudentId === student.studentId}
                  disabled={pendingStudentId !== null}
                  onClick={() =>
                    void handleAttendancePatch(student.studentId, {
                      status: "absent",
                      substatus: isPlanned ? "excused" : null,
                    })
                  }
                />
              )}
              {student.status !== "expected" && (
                <IconActionButton
                  icon={<RotateCcw className="h-3.5 w-3.5" />}
                  label="Status zurücksetzen"
                  tone="slate"
                  isLoading={pendingStudentId === student.studentId}
                  disabled={pendingStudentId !== null}
                  onClick={() =>
                    void handleAttendancePatch(student.studentId, {
                      status: "expected",
                      substatus: null,
                      note: null,
                    })
                  }
                />
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
    <div
      className={`${timetableMutedSurface} border-dashed px-3 py-2 text-xs text-gray-500`}
    >
      {children}
    </div>
  );
}

type IconActionTone = "green" | "red" | "slate";

interface IconActionButtonProps {
  icon: React.ReactNode;
  label: string;
  tone: IconActionTone;
  onClick: () => void;
  isLoading?: boolean;
  disabled?: boolean;
}

const ICON_ACTION_PALETTE: Record<IconActionTone, string> = {
  green:
    "border-[#83CD2D]/20 bg-white text-[#6BA023] hover:border-[#83CD2D]/40 hover:bg-[#83CD2D]/10",
  red: "border-[#FF3130]/20 bg-white text-[#CC2626] hover:border-[#FF3130]/40 hover:bg-[#FF3130]/10",
  slate:
    "border-gray-200 bg-white text-gray-600 hover:border-gray-300 hover:bg-gray-50",
};

function IconActionButton({
  icon,
  label,
  tone,
  onClick,
  isLoading,
  disabled,
}: IconActionButtonProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled || isLoading}
      title={label}
      aria-label={label}
      className={`inline-flex h-7 w-7 items-center justify-center rounded-full border transition-colors disabled:cursor-not-allowed disabled:opacity-40 ${ICON_ACTION_PALETTE[tone]}`}
    >
      {isLoading ? (
        <span className="h-3 w-3 animate-spin rounded-full border-2 border-current border-t-transparent" />
      ) : (
        icon
      )}
    </button>
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
      className={`rounded-xl border px-3 py-2 shadow-sm ${
        danger
          ? "border-[#FF3130]/20 bg-[#FF3130]/10"
          : "border-gray-200 bg-white"
      }`}
    >
      <div className="text-sm font-semibold text-gray-900">{name}</div>
      {labels.length > 0 && (
        <div
          className={`mt-0.5 text-[11px] ${
            danger ? "text-[#CC2626]" : "text-gray-500"
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
  const totalStudents = expected + present;
  const activeStaff = instance.staffCount - instance.absentStaffCount;

  const studentTone: StatPillTone =
    totalStudents === 0 ? "slate" : present > 0 ? "green" : "slate";
  const staffTone: StatPillTone =
    instance.staffCount === 0
      ? "slate"
      : instance.absentStaffCount > 0
        ? "amber"
        : "blue";

  return (
    <div className="flex flex-wrap items-center gap-1.5">
      <StatPill
        icon={<Users className="h-3.5 w-3.5" />}
        label="Anwesend"
        value={totalStudents === 0 ? "—" : `${present} / ${totalStudents}`}
        tone={studentTone}
      />
      <StatPill
        icon={<UserCheck className="h-3.5 w-3.5" />}
        label="Personal"
        value={
          instance.staffCount === 0
            ? "—"
            : `${activeStaff} / ${instance.staffCount}`
        }
        tone={staffTone}
      />
    </div>
  );
}

type StatPillTone = "green" | "blue" | "amber" | "slate";

interface StatPillProps {
  icon: React.ReactNode;
  label: string;
  value: string;
  tone: StatPillTone;
}

const STAT_PILL_PALETTE: Record<
  StatPillTone,
  { bg: string; border: string; accent: string; muted: string }
> = {
  // Brand-aligned pill tints: green=GROUP_ROOM #83CD2D, blue=OTHER_ROOM
  // #5080D8, amber=SICK #EAB308, slate=neutral gray. Solid hex (not /10
  // classes) because these feed inline styles.
  green: {
    bg: "#F1F9E6",
    border: "#D7EDB8",
    accent: "#5A8E1F",
    muted: "#4A7A15CC",
  },
  blue: {
    bg: "#EDF3FC",
    border: "#C9DBF6",
    accent: "#3F66C0",
    muted: "#2F4F9ECC",
  },
  amber: {
    bg: "#FBF3D6",
    border: "#F2E2A0",
    accent: "#8A6D00",
    muted: "#6E5700CC",
  },
  slate: {
    bg: "#F9FAFB",
    border: "#E5E7EB",
    accent: "#374151",
    muted: "#4B5563CC",
  },
};

function StatPill({ icon, label, value, tone }: StatPillProps) {
  const { bg, border, accent, muted } = STAT_PILL_PALETTE[tone];
  return (
    <span
      className="inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs"
      style={{ backgroundColor: bg, borderColor: border, color: accent }}
    >
      <span aria-hidden className="flex shrink-0 items-center">
        {icon}
      </span>
      <span className="font-medium" style={{ color: muted }}>
        {label}
      </span>
      <span className="font-bold tabular-nums">{value}</span>
    </span>
  );
}

interface SectionProps {
  title: string;
  children: React.ReactNode;
}

function Section({ title, children }: SectionProps) {
  return (
    <div className="space-y-2">
      <h4 className="text-[10px] font-bold tracking-wider text-gray-400 uppercase">
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
    <div
      className={`${timetableNestedSurface} flex items-start gap-3 p-3 text-sm`}
    >
      <span className="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center text-gray-400">
        {icon}
      </span>
      <div className="flex min-w-0 flex-1 flex-col">
        <span className="text-[11px] font-medium text-gray-500">{label}</span>
        <span className="text-sm text-gray-900">{children}</span>
      </div>
    </div>
  );
}
