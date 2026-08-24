"use client";

import { useEffect, useMemo, useState } from "react";
import { Check, ChevronDown, ExternalLink } from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";
import type { Session } from "next-auth";
import {
  useSWRAuth,
  useTenantMutate,
  useTenantMutateMatching,
} from "~/lib/swr";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { DatabaseSelect } from "~/components/ui/database/database-select";
import { useToast } from "~/contexts/ToastContext";
import { activeService } from "~/lib/active-service";
import type { ActiveGroup, Supervisor } from "~/lib/active-helpers";
import { studentService } from "~/lib/api";
import type { Student } from "~/lib/api";
import { userContextService } from "~/lib/usercontext-api";
import type { Staff } from "~/lib/usercontext-helpers";
import { CompactStudentCard } from "~/components/students/compact-student-card";
import { useTenantRouter } from "~/lib/tenant-router";
import { useAttendanceWebEnabled } from "~/lib/tenant-context";
import { useOptionalSupervision } from "~/lib/supervision-context";

const DETAIL_CARD_CLASS =
  "rounded-3xl moto-content-surface border p-5 shadow-sm sm:p-6";
const EMPTY_STUDENTS: Student[] = [];

function buildSessionLabel(group: ActiveGroup): string {
  const roomName = group.room?.name ?? `Raum ${group.roomId}`;
  const activityName = group.actualGroup?.name;
  return activityName ? `${roomName} · ${activityName}` : roomName;
}

function canUseAllMoveTargets(session: Session | null): boolean {
  const permissions = session?.user?.permissions ?? [];
  return (
    session?.user?.isAdmin === true ||
    session?.user?.roles?.includes("admin") === true ||
    permissions.includes("admin:*") ||
    permissions.includes("*:*")
  );
}

interface TransitStudentsSectionProps {
  readonly onSelectionActiveChange?: (active: boolean) => void;
  readonly fromReferrer?: string;
  readonly collapsible?: boolean;
}

export function TransitStudentsSection({
  onSelectionActiveChange,
  fromReferrer: fromReferrerOverride,
  collapsible = false,
}: TransitStudentsSectionProps) {
  const router = useTenantRouter();
  const { data: session } = useSession();
  const attendanceWebEnabled = useAttendanceWebEnabled();
  // Mirrors the backend's move authorization exactly (#2380): callers the
  // school-wide overview covers may move children into any running module,
  // everyone else only into a module they supervise themselves. Gating on
  // the organisational group mode here would offer targets the server then
  // rejects with 403.
  const { overviewEnabled } = useOptionalSupervision();
  const showAllTargets = canUseAllMoveTargets(session) || overviewEnabled;
  const sectionSearchParams = useSearchParams();
  const [selectedIds, setSelectedIds] = useState<Set<string>>(() => new Set());
  const [collapsibleExpanded, setCollapsibleExpanded] = useState(false);
  const isExpanded = !collapsible || collapsibleExpanded;
  const [targetGroupId, setTargetGroupId] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const { success: toastSuccess } = useToast();
  const mutateKey = useTenantMutate();
  // "dashboard" (matching the aggregated active-supervision-dashboard key,
  // which carries the room's visits since #2096) and "timetable-roster-"
  // keep the /active-supervisions room grid and roster in sync when this
  // section is embedded there — without them the assigned children only
  // appear once an SSE event happens to arrive.
  const mutateMatching = useTenantMutateMatching([
    "room-students-",
    "room-detail-",
    "rooms-list",
    "dashboard",
    "timetable-roster-",
  ]);

  const {
    data: studentsData,
    error: studentsError,
    isLoading: studentsLoading,
    mutate: mutateStudents,
  } = useSWRAuth<{
    students: Student[];
    pagination?: { total_records: number };
  }>("transit-students", () =>
    studentService.getStudents({ locationState: "transit", pageSize: 200 }),
  );

  const {
    data: activeGroups = [],
    error: groupsError,
    isLoading: groupsLoading,
  } = useSWRAuth<ActiveGroup[]>("active-groups-for-transit", () =>
    activeService.getActiveGroups({ active: true }),
  );
  const {
    data: currentStaff,
    error: staffError,
    isLoading: staffLoading,
  } = useSWRAuth<Staff>(
    showAllTargets ? null : "transit-target-current-staff",
    () => userContextService.getCurrentStaff(),
  );
  const {
    data: activeSupervisions = [],
    error: supervisionsError,
    isLoading: supervisionsLoading,
  } = useSWRAuth<Supervisor[]>(
    showAllTargets || !currentStaff?.id
      ? null
      : `transit-target-supervisions-${currentStaff.id}`,
    () => activeService.getStaffActiveSupervisions(currentStaff?.id ?? ""),
  );

  const supervisedTargetGroupIds = useMemo(
    () =>
      new Set(
        activeSupervisions
          .filter((supervision) => supervision.isActive)
          .map((supervision) => supervision.activeGroupId),
      ),
    [activeSupervisions],
  );

  const targetOptions = useMemo(
    () =>
      [...activeGroups]
        .filter(
          (group) =>
            group.isActive &&
            (showAllTargets || supervisedTargetGroupIds.has(group.id)),
        )
        .sort((a, b) =>
          buildSessionLabel(a).localeCompare(buildSessionLabel(b), "de"),
        ),
    [activeGroups, showAllTargets, supervisedTargetGroupIds],
  );

  const students = studentsData?.students ?? EMPTY_STUDENTS;
  const visibleStudentIds = useMemo(
    () => new Set(students.map((student) => String(student.id))),
    [students],
  );
  const selectedVisibleCount = [...selectedIds].filter((studentId) =>
    visibleStudentIds.has(studentId),
  ).length;
  const totalCount = studentsData?.pagination?.total_records ?? students.length;
  const allSelected =
    students.length > 0 && selectedVisibleCount === students.length;
  const defaultFromReferrer = (() => {
    const qs = sectionSearchParams?.toString() ?? "";
    return qs ? `/rooms?${qs}` : "/rooms?room=__transit__";
  })();
  const fromReferrer = fromReferrerOverride ?? defaultFromReferrer;

  useEffect(() => {
    onSelectionActiveChange?.(selectedVisibleCount > 0);
  }, [onSelectionActiveChange, selectedVisibleCount]);

  useEffect(() => {
    if (
      targetGroupId &&
      !targetOptions.some((group) => group.id === targetGroupId)
    ) {
      setTargetGroupId("");
    }
  }, [targetGroupId, targetOptions]);

  const toggleSelected = (studentId: string) => {
    setSelectedIds((current) => {
      const next = new Set(current);
      if (next.has(studentId)) next.delete(studentId);
      else next.add(studentId);
      return next;
    });
    setSubmitError(null);
  };

  const selectAllVisible = () => {
    setSelectedIds(new Set(students.map((student) => student.id.toString())));
    setSubmitError(null);
  };

  const clearSelection = () => {
    setSelectedIds(new Set());
    setSubmitError(null);
  };

  const assignSelected = async () => {
    const studentIds = [...selectedIds].filter((studentId) =>
      visibleStudentIds.has(studentId),
    );
    if (!targetGroupId || studentIds.length === 0) return;
    setSubmitting(true);
    setSubmitError(null);
    try {
      const result = await activeService.assignTransitStudents(
        studentIds,
        targetGroupId,
      );
      setSelectedIds(new Set());
      await mutateStudents();
      await mutateKey("rooms-list");
      await mutateMatching();
      const skipped = result.skipped.length;
      toastSuccess(
        skipped > 0
          ? `${result.assigned.length} zugewiesen, ${skipped} übersprungen.`
          : `${result.assigned.length} ${
              result.assigned.length === 1 ? "Kind" : "Kinder"
            } zugewiesen.`,
      );
    } catch {
      setSubmitError(
        "Die ausgewählten Kinder konnten nicht zugewiesen werden.",
      );
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <section aria-label="Kinder unterwegs" className={DETAIL_CARD_CLASS}>
      {collapsible ? (
        <button
          type="button"
          onClick={() => setCollapsibleExpanded((current) => !current)}
          aria-expanded={isExpanded}
          className="group flex w-full min-w-0 items-center gap-3 text-left focus-visible:ring-2 focus-visible:ring-gray-300 focus-visible:outline-none"
        >
          <span
            className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-gray-100"
            aria-hidden="true"
          >
            <MotoConceptIcon concept="transit" size={18} />
          </span>
          <span className="min-w-0 flex-1">
            <span className="block text-xs font-semibold tracking-wider text-gray-500 uppercase">
              Kinder unterwegs
            </span>
            <span className="block truncate text-sm text-gray-600">
              <span className="font-medium text-gray-900">{totalCount}</span>{" "}
              {totalCount === 1 ? "Kind" : "Kinder"} ohne Raum
            </span>
          </span>
          <ChevronDown
            className={`h-4 w-4 shrink-0 text-gray-400 transition-transform group-hover:text-gray-600 ${
              isExpanded ? "rotate-180" : ""
            }`}
            aria-hidden="true"
          />
        </button>
      ) : (
        <>
          <h2 className="mb-3 text-xs font-semibold tracking-wider text-gray-500 uppercase">
            Kinder unterwegs
          </h2>
          <div className="flex flex-wrap items-end justify-between gap-3">
            <p className="text-sm text-gray-600">
              <span className="font-medium text-gray-900">{totalCount}</span>{" "}
              {totalCount === 1 ? "Kind" : "Kinder"} ohne Raum
            </p>
          </div>
        </>
      )}

      {isExpanded &&
      (studentsError || groupsError || staffError || supervisionsError) ? (
        <div className="mt-4">
          <Alert
            type="error"
            message="Die Unterwegs-Daten konnten nicht geladen werden."
          />
        </div>
      ) : null}
      {isExpanded && submitError ? (
        <div className="mt-4">
          <Alert type="error" message={submitError} />
        </div>
      ) : null}

      {isExpanded ? (
        <>
          {attendanceWebEnabled ? (
            <div
              className={`mt-4 mb-4 rounded-2xl border p-3 transition-shadow ${
                selectedVisibleCount > 0
                  ? "sticky bottom-3 z-20 border-gray-200 bg-white/95 shadow-sm backdrop-blur"
                  : "border-transparent bg-gray-50/80 shadow-none"
              }`}
            >
              <div className="flex items-start justify-between gap-3">
                <p className="min-w-0 text-sm font-semibold text-gray-900">
                  <span className="block truncate">
                    {selectedVisibleCount > 0
                      ? `${selectedVisibleCount} von ${students.length} ausgewählt`
                      : "Kinder auswählen"}
                  </span>
                  <span className="block text-xs font-medium text-gray-500">
                    Zielraum wählen und gemeinsam zuweisen
                  </span>
                </p>
                {students.length > 0 ? (
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={allSelected ? clearSelection : selectAllVisible}
                    className="h-8 shrink-0 rounded-full px-3 py-0 text-xs shadow-none"
                  >
                    {allSelected ? "Aufheben" : "Alle auswählen"}
                  </Button>
                ) : null}
              </div>

              <div className="mt-3 grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end">
                <DatabaseSelect
                  id="transit-target-room"
                  name="transit-target-room"
                  label="Zielraum"
                  value={targetGroupId}
                  onChange={(value) => {
                    setTargetGroupId(value);
                    setSubmitError(null);
                  }}
                  disabled={
                    groupsLoading ||
                    staffLoading ||
                    supervisionsLoading ||
                    targetOptions.length === 0 ||
                    submitting
                  }
                  placeholder={
                    groupsLoading || staffLoading || supervisionsLoading
                      ? "Aktive Räume werden geladen..."
                      : targetOptions.length === 0
                        ? "Keine aktiven Räume"
                        : "Zielraum wählen"
                  }
                  options={targetOptions.map((group) => ({
                    value: group.id,
                    label: buildSessionLabel(group),
                  }))}
                  className="bg-white text-sm md:text-sm"
                />
                <Button
                  type="button"
                  variant="primary"
                  size="sm"
                  isLoading={submitting}
                  loadingText="Weise zu..."
                  onClick={() => void assignSelected()}
                  disabled={
                    !targetGroupId || selectedVisibleCount === 0 || submitting
                  }
                  className="h-9 w-full px-3 py-2 text-xs shadow-sm sm:w-auto"
                >
                  In Raum setzen
                </Button>
              </div>
            </div>
          ) : null}

          <div className="mt-4 flex flex-col gap-2">
            {studentsLoading && students.length === 0 ? (
              <div className="moto-content-surface rounded-xl border px-4 py-6 text-center text-sm text-gray-500">
                Kinder werden geladen...
              </div>
            ) : students.length === 0 ? (
              <div className="rounded-xl border border-dashed border-gray-200 bg-white px-4 py-6 text-center text-sm text-gray-500">
                Aktuell keine Kinder unterwegs.
              </div>
            ) : (
              students.map((student) => {
                const studentId = student.id.toString();
                const checked = selectedIds.has(studentId);
                const fullName =
                  `${student.first_name ?? ""} ${student.second_name ?? ""}`.trim() ||
                  "Kind";
                return (
                  <div
                    key={student.id}
                    className={`flex items-center gap-2 rounded-2xl border px-3 py-2.5 transition-colors ${
                      checked
                        ? "border-gray-300 bg-gray-50"
                        : "border-gray-100 bg-white hover:border-gray-200 hover:bg-gray-50"
                    }`}
                  >
                    {attendanceWebEnabled ? (
                      <button
                        type="button"
                        role="checkbox"
                        aria-checked={checked}
                        aria-label={`${fullName} auswählen`}
                        onClick={() => toggleSelected(studentId)}
                        className="flex min-w-0 flex-1 items-center gap-3 rounded-xl text-left focus:outline-none focus-visible:ring-2 focus-visible:ring-gray-300"
                      >
                        <span
                          className={`flex h-5 w-5 shrink-0 items-center justify-center rounded-md border shadow-sm transition-all ${
                            checked
                              ? "border-gray-900 bg-gray-900"
                              : "border-gray-300 bg-white"
                          }`}
                          aria-hidden="true"
                        >
                          <Check
                            className={`h-3.5 w-3.5 text-white transition-opacity ${
                              checked ? "opacity-100" : "opacity-0"
                            }`}
                          />
                        </span>
                        <CompactStudentCard
                          studentId={student.id}
                          firstName={student.first_name}
                          lastName={student.second_name}
                          schoolClass={student.school_class}
                          groupName={student.group_name ?? undefined}
                          photoUrl={student.photo_url ?? null}
                          chrome="plain"
                        />
                      </button>
                    ) : (
                      <div className="min-w-0 flex-1">
                        <CompactStudentCard
                          studentId={student.id}
                          firstName={student.first_name}
                          lastName={student.second_name}
                          schoolClass={student.school_class}
                          groupName={student.group_name ?? undefined}
                          photoUrl={student.photo_url ?? null}
                          chrome="plain"
                        />
                      </div>
                    )}
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() =>
                        router.push(
                          `/students/${student.id}?from=${encodeURIComponent(
                            fromReferrer,
                          )}`,
                        )
                      }
                      aria-label={`${fullName} Profil öffnen`}
                      title="Profil öffnen"
                      className="h-8 shrink-0 gap-1.5 rounded-full px-2.5 py-0 text-xs font-medium shadow-none"
                    >
                      Profil
                      <ExternalLink
                        className="h-3.5 w-3.5"
                        aria-hidden="true"
                      />
                    </Button>
                  </div>
                );
              })
            )}
          </div>
        </>
      ) : null}
    </section>
  );
}
