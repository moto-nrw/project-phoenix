"use client";

import { Suspense, useCallback, useEffect, useMemo, useState } from "react";
import { redirect, useSearchParams } from "next/navigation";
import Link from "next/link";
import { useSession } from "next-auth/react";
import {
  ClipboardList,
  GraduationCap,
  ListChecks,
  LogOut,
  Trash2,
  Undo2,
  UserMinus,
} from "lucide-react";
import { DatabaseCreateAction } from "~/components/database/database-create-action";
import { DatabaseEmptyState } from "~/components/database/database-empty-state";
import { DatabaseGroupingToggle } from "~/components/database/database-grouping-toggle";
import { DatabasePageLayout } from "~/components/database/database-page-layout";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { MOTO_CONCEPTS } from "~/lib/moto-concepts";
import type {
  ActiveFilter,
  FilterConfig,
} from "~/components/ui/page-header/types";
import { useIsMobile } from "~/components/ui/hooks/useIsMobile";
import { useUpdateUrlParams } from "~/hooks/useUpdateUrlParams";
import { useToast } from "~/contexts/ToastContext";
import {
  StudentCreateModal,
  type CreateStudentSchedules,
} from "@/components/students/student-create-modal";
import {
  StudentsMasterDetail,
  type GroupingMode,
} from "@/components/students/students-master-detail";
import { StudentDeletionModal } from "~/components/students/student-deletion-modal";
import { CareExitModal } from "~/components/students/care-exit-modal";
import { CareResumeModal } from "~/components/students/care-resume-modal";
import { cancelCareExit } from "~/lib/care-exit-api";
import { getDbOperationMessage } from "@/lib/use-notification";
import { createCrudService } from "@/lib/database/service-factory";
import { studentsConfig } from "@/components/database/configs/students.config";
import type { Student } from "@/lib/api";
import type { StudentGuardianPayload } from "@/lib/guardian-helpers";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";
import { createLogger } from "~/lib/logger";
import { hasPermission } from "~/lib/auth-utils";
import { createClassListEntry } from "~/lib/class-list-entries-api";
import { Button } from "~/components/ui/button";
import { cn } from "~/lib/utils";
import { MasterDetailSkeleton } from "~/components/database/master-detail-skeleton";

const logger = createLogger({ component: "DatabaseStudentsPage" });

const STUDENTS_GROUPING_DEFAULT: GroupingMode = "class";

const STUDENTS_GROUPING_OPTIONS: { value: GroupingMode; label: string }[] = [
  { value: "class", label: "Klasse" },
  { value: "group", label: "Gruppe" },
  { value: "none", label: "Keine" },
];

function parseGrouping(value: string | null): GroupingMode {
  if (value === "group" || value === "none") return value;
  return STUDENTS_GROUPING_DEFAULT;
}

export default function StudentsPage() {
  return (
    <Suspense fallback={<MasterDetailSkeleton />}>
      <StudentsPageContent />
    </Suspense>
  );
}

function StudentsPageContent() {
  const searchParams = useSearchParams();
  const updateUrlParams = useUpdateUrlParams();

  const selectedId = searchParams.get("student");
  const grouping = parseGrouping(searchParams.get("groupBy"));

  const handleSelect = useCallback(
    (id: string | null) => {
      updateUrlParams({ student: id });
    },
    [updateUrlParams],
  );

  const handleGroupingChange = useCallback(
    (next: GroupingMode) => {
      updateUrlParams({
        groupBy: next === STUDENTS_GROUPING_DEFAULT ? null : next,
      });
    },
    [updateUrlParams],
  );

  const [searchTerm, setSearchTerm] = useState("");
  const [groupFilter, setGroupFilter] = useState("all");
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Student | null>(null);
  // "Betreuung beenden" (#2487): entweder für das ausgewählte Kind im Detail
  // oder für die Mehrfachauswahl. Beide Wege führen durch dieselbe Vorschau.
  const [careExitIds, setCareExitIds] = useState<string[] | null>(null);
  const [resumeTarget, setResumeTarget] = useState<Student | null>(null);
  const [cancellingExit, setCancellingExit] = useState(false);
  const [arrivalRevision, setArrivalRevision] = useState(0);
  const [selectionMode, setSelectionMode] = useState(false);
  const [selectedStudentIds, setSelectedStudentIds] = useState<Set<string>>(
    () => new Set(),
  );
  const isMobile = useIsMobile();

  const { success: toastSuccess, error: toastError } = useToast();

  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  const service = useMemo(() => createCrudService(studentsConfig), []);
  const tenantMutate = useTenantMutate();

  const {
    data: studentsData,
    isLoading: loading,
    error: studentsError,
  } = useSWRAuth("database-students-list", async () => {
    const data = await service.getList({
      page: 1,
      pageSize: 1000,
      include_arrival_times: true,
    });
    return Array.isArray(data.data) ? data.data : [];
  });

  const errorMessage = studentsError
    ? "Fehler beim Laden der Kinder. Bitte versuchen Sie es später erneut."
    : null;

  useEffect(() => {
    if (!studentsData) return;
    const currentIds = new Set(
      studentsData.map((student) => String(student.id)),
    );
    setSelectedStudentIds((previous) => {
      const next = new Set([...previous].filter((id) => currentIds.has(id)));
      return next.size === previous.size ? previous : next;
    });
  }, [studentsData]);

  const toggleStudentSelection = useCallback(
    (studentId: string) => {
      setSelectedStudentIds((previous) => {
        const next = new Set(previous);
        if (next.has(studentId)) {
          next.delete(studentId);
        } else if (next.size >= 500) {
          toastError("Maximal 500 Kinder können ausgewählt werden");
          return previous;
        } else {
          next.add(studentId);
        }
        return next;
      });
    },
    [toastError],
  );

  const finishSelection = useCallback(() => {
    setSelectionMode(false);
    setSelectedStudentIds(new Set());
  }, []);

  // "Alle angezeigten auswählen" (#2487): Suche und Filter bestimmen, was
  // angezeigt wird — die Auswahl folgt genau dem, nicht der ganzen Kartei.
  // Storniert ein noch nicht wirksames Betreuungsende (#2487). Ein bereits
  // wirksamer Austritt kann nur über "Wieder aufnehmen" zurückgenommen werden
  // — mit neuem Beginn und ausdrücklicher Prüfung.
  const cancelPlannedExit = useCallback(
    async (student: Student) => {
      setCancellingExit(true);
      try {
        await cancelCareExit([String(student.id)]);
        toastSuccess(
          `Das geplante Betreuungsende von ${studentsConfig.list.item.title(student)} wurde storniert`,
        );
        await tenantMutate("database-students-list");
      } catch (cancelError) {
        const message =
          cancelError instanceof Error
            ? cancelError.message
            : "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.";
        logger.error("care_exit_cancel_failed", {
          student_id: String(student.id),
          error: message,
        });
        toastError(message);
      } finally {
        setCancellingExit(false);
      }
    },
    [tenantMutate, toastError, toastSuccess],
  );

  const selectAllVisible = useCallback(
    (studentIds: string[]) => {
      if (studentIds.length > 500) {
        toastError("Maximal 500 Kinder können ausgewählt werden");
        setSelectedStudentIds(new Set(studentIds.slice(0, 500)));
        return;
      }
      setSelectedStudentIds(new Set(studentIds));
    },
    [toastError],
  );

  const { data: allGroups = [] } = useSWRAuth<
    Array<{ value: string; label: string }>
  >("database-groups-dropdown", async () => {
    const response = await fetch("/api/groups");
    if (!response.ok) {
      logger.error("failed to fetch groups", { status: response.status });
      return [];
    }
    const data: unknown = await response.json();

    let groups: Array<{ id: number; name: string }> = [];
    if (Array.isArray(data)) {
      groups = data as Array<{ id: number; name: string }>;
    } else if (data && typeof data === "object" && "data" in data) {
      const wrappedData = data as { data: unknown };
      if (Array.isArray(wrappedData.data)) {
        groups = wrappedData.data as Array<{ id: number; name: string }>;
      }
    } else {
      logger.error("unexpected groups response format", {
        data_type: typeof data,
      });
      return [];
    }

    return groups
      .map((group) => ({
        value: String(group.id),
        label: group.name,
      }))
      .sort((a, b) => a.label.localeCompare(b.label));
  });

  const filteredStudents = useMemo(() => {
    const students = studentsData ?? [];
    let filtered = [...students];

    if (searchTerm) {
      const searchLower = searchTerm.trim().toLowerCase();
      filtered = filtered.filter((student) => {
        const fullName = [student.first_name, student.second_name]
          .filter(Boolean)
          .join(" ")
          .toLowerCase();
        const lastNameFirst = [student.second_name, student.first_name]
          .filter(Boolean)
          .join(" ")
          .toLowerCase();

        return (
          (student.first_name?.toLowerCase().includes(searchLower) ?? false) ||
          (student.second_name?.toLowerCase().includes(searchLower) ?? false) ||
          fullName.includes(searchLower) ||
          lastNameFirst.includes(searchLower) ||
          (student.school_class?.toLowerCase().includes(searchLower) ??
            false) ||
          (student.group_name?.toLowerCase().includes(searchLower) ?? false) ||
          (student.name_lg?.toLowerCase().includes(searchLower) ?? false)
        );
      });
    }

    if (groupFilter !== "all") {
      filtered = filtered.filter((student) => student.group_id === groupFilter);
    }

    filtered.sort((a, b) => {
      const nameA = `${a.second_name ?? ""} ${a.first_name ?? ""}`;
      const nameB = `${b.second_name ?? ""} ${b.first_name ?? ""}`;
      return nameA.localeCompare(nameB, "de");
    });

    return filtered;
  }, [studentsData, searchTerm, groupFilter]);

  const selectedStudent = useMemo(
    () =>
      selectedId
        ? (filteredStudents.find((s) => String(s.id) === selectedId) ?? null)
        : null,
    [selectedId, filteredStudents],
  );

  const filters: FilterConfig[] = useMemo(
    () => [
      {
        id: "group",
        label: "Gruppe",
        type: "dropdown",
        value: groupFilter,
        onChange: (value) => setGroupFilter(value as string),
        options: [{ value: "all", label: "Alle Gruppen" }, ...allGroups],
      },
    ],
    [groupFilter, allGroups],
  );

  const activeFilters: ActiveFilter[] = useMemo(() => {
    const chips: ActiveFilter[] = [];
    if (searchTerm) {
      chips.push({
        id: "search",
        label: `"${searchTerm}"`,
        onRemove: () => setSearchTerm(""),
      });
    }
    if (groupFilter !== "all") {
      const group = allGroups.find((g) => g.value === groupFilter);
      chips.push({
        id: "group",
        label: group?.label ?? "Gruppe",
        onRemove: () => setGroupFilter("all"),
      });
    }
    return chips;
  }, [searchTerm, groupFilter, allGroups]);

  // Class-list-only entry (#2382): created straight from the "+ Kinder"
  // modal's "Nur Klassenliste" mode. The entry does NOT appear in this list
  // (it is no student) — the toast says where it lives instead.
  const handleCreateListEntry = useCallback(
    async (input: {
      firstName: string;
      lastName: string;
      schoolClass: string;
    }) => {
      await createClassListEntry(input);
      toastSuccess(
        "Klassenlisteneintrag angelegt — zu finden im Menü oben rechts unter Klassenliste",
      );
    },
    [toastSuccess],
  );

  const handleCreateStudent = useCallback(
    async (
      studentData: Partial<Student> & {
        guardians?: StudentGuardianPayload[];
      } & CreateStudentSchedules,
    ) => {
      // Run the config transform, then re-attach guardians and weekly schedules
      // from the original input. transformBeforeSubmit is typed Partial<Student>
      // and drops these extra fields from the static type; re-attaching here
      // makes the contract explicit so a future transform change can't silently
      // strip them (see issue #1500 atomic create flow, #1502 schedules).
      let payload: Partial<Student> & {
        guardians?: StudentGuardianPayload[];
      } & CreateStudentSchedules = studentsConfig.form.transformBeforeSubmit
        ? studentsConfig.form.transformBeforeSubmit(studentData)
        : studentData;
      if (studentData.guardians && studentData.guardians.length > 0) {
        payload = { ...payload, guardians: studentData.guardians };
      }
      if (
        studentData.arrival_schedules &&
        studentData.arrival_schedules.length > 0
      ) {
        payload = {
          ...payload,
          arrival_schedules: studentData.arrival_schedules,
        };
      }
      if (
        studentData.pickup_schedules &&
        studentData.pickup_schedules.length > 0
      ) {
        payload = {
          ...payload,
          pickup_schedules: studentData.pickup_schedules,
        };
      }
      const newStudent = await service.create(payload);
      const displayName = studentsConfig.list.item.title(newStudent);
      toastSuccess(
        getDbOperationMessage(
          "create",
          studentsConfig.name.singular,
          displayName,
        ),
      );
      setShowCreateModal(false);
      await tenantMutate("database-students-list");
    },
    [service, tenantMutate, toastSuccess],
  );

  const handleUpdateStudent = useCallback(
    async (studentId: string, studentData: Partial<Student>) => {
      try {
        if (studentsConfig.form.transformBeforeSubmit) {
          studentData = studentsConfig.form.transformBeforeSubmit(studentData);
        }
        await service.update(studentId, studentData);
        toastSuccess(
          getDbOperationMessage(
            "update",
            studentsConfig.name.singular,
            studentData.first_name && studentData.second_name
              ? `${studentData.first_name} ${studentData.second_name}`
              : studentsConfig.name.singular,
          ),
        );
        await tenantMutate("database-students-list");
      } catch (err) {
        logger.error("failed to update student", {
          error: err instanceof Error ? err.message : String(err),
        });
        throw err;
      }
    },
    [service, tenantMutate, toastSuccess],
  );

  const handleStudentDeleted = useCallback(async () => {
    if (!deleteTarget) return;
    const displayName = studentsConfig.list.item.title(deleteTarget);
    toastSuccess(
      getDbOperationMessage(
        "delete",
        studentsConfig.name.singular,
        displayName,
      ),
    );
    setDeleteTarget(null);
    handleSelect(null);
    await tenantMutate("database-students-list");
  }, [deleteTarget, tenantMutate, toastSuccess, handleSelect]);

  const handleArrivalChanged = useCallback(() => {
    setArrivalRevision((prev) => prev + 1);
    void tenantMutate("database-students-list");
  }, [tenantMutate]);

  const studentsWithArrival = useMemo(
    () => new Set(filteredStudents.map((s) => String(s.id))),
    // NOTE: v1 defaults every student to "has arrival" so no false-positive
    // warnings are shown in the list. A bulk arrival-status endpoint (arrival-times/bulk)
    // can later drive real warnings — reference arrivalRevision below to refetch on save.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [filteredStudents, arrivalRevision],
  );

  const arrivalSummaryById = useMemo(() => {
    const map = new Map<string, string>();
    for (const student of filteredStudents) {
      if (student.arrival_is_exception && !student.arrival_time) {
        map.set(String(student.id), "Kommt heute nicht");
      } else if (student.arrival_time) {
        map.set(String(student.id), `Ankunft ${student.arrival_time} Uhr`);
      }
    }
    return map;
  }, [filteredStudents]);

  const canShowDetail = !loading && filteredStudents.length > 0;
  const canViewEnrollments = hasPermission(session, "config:manage");
  const canCreateStudents = hasPermission(session, "users:create");
  const canDeleteStudents = hasPermission(session, "users:delete");
  const canUpdateStudents = hasPermission(session, "users:update");

  // Kopfzeilen-Aktionen des Detailbereichs. "Betreuung beenden" steht neben
  // "Löschen", weil beides dieselbe Berechtigung braucht — aber ein regulärer
  // Austritt löscht nichts, deshalb ist er kein roter Knopf (#2487).
  const detailActions =
    selectedStudent && canDeleteStudents ? (
      <div className="flex flex-wrap items-center gap-1.5">
        {selectedStudent.care_ended ? (
          <Button
            type="button"
            variant="outline"
            size="compact"
            onClick={() => setResumeTarget(selectedStudent)}
          >
            <Undo2 className="mr-1.5 h-3.5 w-3.5" aria-hidden />
            Wieder aufnehmen
          </Button>
        ) : (
          <>
            <Button
              type="button"
              variant="outline"
              size="compact"
              onClick={() => setCareExitIds([String(selectedStudent.id)])}
            >
              <LogOut className="mr-1.5 h-3.5 w-3.5" aria-hidden />
              {selectedStudent.care_ends_on
                ? "Ende ändern"
                : "Betreuung beenden"}
            </Button>
            {selectedStudent.care_ends_on ? (
              <Button
                type="button"
                variant="ghost"
                size="compact"
                isLoading={cancellingExit}
                loadingText="Wird storniert…"
                onClick={() => void cancelPlannedExit(selectedStudent)}
              >
                Ende stornieren
              </Button>
            ) : null}
          </>
        )}
        <button
          type="button"
          onClick={() => setDeleteTarget(selectedStudent)}
          className="border-moto-red/20 bg-moto-red-soft text-moto-red-strong hover:bg-moto-red/10 flex items-center gap-1.5 rounded-md border px-3 py-1.5 text-sm font-medium"
        >
          <Trash2 className="h-3.5 w-3.5" aria-hidden />
          Löschen
        </button>
      </div>
    ) : null;

  return (
    <DatabasePageLayout
      loading={loading}
      sessionLoading={status === "loading"}
      className="-mt-1.5 flex w-full flex-col"
    >
      <div className="mb-4">
        <PageHeaderWithSearch
          title={isMobile ? "Kinder" : ""}
          badge={{
            icon: (
              <MotoDuotoneIcon
                icon={MOTO_CONCEPTS.children.icon}
                tone={MOTO_CONCEPTS.children.tone}
                size={20}
              />
            ),
            count: filteredStudents.length,
            label: "Kinder",
          }}
          search={{
            value: searchTerm,
            onChange: setSearchTerm,
            placeholder: "Kinder suchen...",
          }}
          filters={filters}
          activeFilters={activeFilters}
          onClearAllFilters={() => {
            setSearchTerm("");
            setGroupFilter("all");
          }}
          // Sekundäre Navigationsziele (Jahrgangswechsel, Klassenliste) liegen
          // im Kebab-Menü: als vierter und fünfter Textbutton sprengten sie
          // die Aktionszeile auf üblichen Laptop-Breiten (#2382 Review).
          overflowMenu={[
            ...(hasPermission(session, "grade_transitions:read")
              ? [
                  {
                    label: "Jahrgangswechsel",
                    icon: <GraduationCap className="h-4 w-4" aria-hidden />,
                    href: "/database/grade-transitions",
                    // Navigation only — OverflowMenu verlangt onClick auch bei href.
                    onClick: () => undefined,
                  },
                ]
              : []),
            {
              label: "Klassenliste",
              icon: <ClipboardList className="h-4 w-4" aria-hidden />,
              href: "/database/students/class-list",
              onClick: () => undefined,
            },
            ...(canDeleteStudents
              ? [
                  {
                    label: "Beendete Betreuungen",
                    icon: <UserMinus className="h-4 w-4" aria-hidden />,
                    href: "/database/students/ended-care",
                    onClick: () => undefined,
                  },
                ]
              : []),
          ]}
          actionButton={
            <div className="flex items-center gap-2">
              {!isMobile ? (
                <>
                  <DatabaseGroupingToggle
                    value={grouping}
                    options={STUDENTS_GROUPING_OPTIONS}
                    onChange={handleGroupingChange}
                  />
                  <Link
                    href="/database/students/import"
                    className="flex h-10 items-center gap-2 rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 hover:bg-gray-50"
                  >
                    Importieren
                  </Link>
                </>
              ) : null}
              {canUpdateStudents ? (
                <Button
                  type="button"
                  variant="outline"
                  size="md"
                  aria-pressed={selectionMode}
                  className={cn(
                    "h-10 gap-2 px-3 shadow-none hover:ring-gray-300",
                    selectionMode && "ring-gray-900 hover:ring-gray-900",
                  )}
                  onClick={() => {
                    if (selectionMode) {
                      finishSelection();
                      return;
                    }
                    handleSelect(null);
                    setSelectionMode(true);
                  }}
                >
                  <ListChecks className="h-4 w-4" aria-hidden />
                  Auswählen
                </Button>
              ) : null}
              <DatabaseCreateAction
                label="Kinder"
                ariaLabel="Kind erstellen"
                onClick={() => setShowCreateModal(true)}
              />
            </div>
          }
        />
      </div>

      {errorMessage ? (
        <div className="mb-4 rounded-lg border border-red-200 bg-red-50 p-4">
          <p className="text-sm text-red-800">{errorMessage}</p>
        </div>
      ) : null}

      {canShowDetail ? (
        <div className="min-h-0 flex-1 pb-4">
          <StudentsMasterDetail
            students={filteredStudents}
            bulkStudents={studentsData ?? []}
            selectedId={selectedId}
            onSelect={handleSelect}
            grouping={grouping}
            studentsWithArrival={studentsWithArrival}
            arrivalSummaryById={arrivalSummaryById}
            onArrivalDataChanged={handleArrivalChanged}
            groups={allGroups}
            onUpdateStudent={handleUpdateStudent}
            canViewEnrollments={canViewEnrollments}
            detailActions={detailActions}
            selectionMode={selectionMode}
            selectedStudentIds={selectedStudentIds}
            onToggleStudentSelection={toggleStudentSelection}
            onClearSelection={() => setSelectedStudentIds(new Set())}
            onFinishSelection={finishSelection}
            onSelectAllVisible={selectAllVisible}
            onEndCare={
              canDeleteStudents
                ? () => setCareExitIds([...selectedStudentIds])
                : undefined
            }
          />
        </div>
      ) : !loading ? (
        <DatabaseEmptyState
          icon={
            <MotoDuotoneIcon
              icon={MOTO_CONCEPTS.children.icon}
              tone={MOTO_CONCEPTS.children.tone}
              size={48}
              className="mx-auto"
            />
          }
          title={
            searchTerm || groupFilter !== "all"
              ? "Keine Kinder gefunden"
              : "Keine Kinder vorhanden"
          }
          description={
            searchTerm || groupFilter !== "all"
              ? "Versuchen Sie andere Suchkriterien oder Filter."
              : "Es wurden noch keine Kinder erstellt."
          }
        />
      ) : null}

      <StudentCreateModal
        isOpen={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        onCreate={handleCreateStudent}
        // Same gate as POST /api/class-list-entries (users:create) — without
        // the permission the modal must not offer the "Nur Klassenliste" mode.
        onCreateListEntry={
          canCreateStudents ? handleCreateListEntry : undefined
        }
        groups={allGroups}
      />

      {careExitIds ? (
        <CareExitModal
          isOpen
          studentIds={careExitIds}
          onClose={() => setCareExitIds(null)}
          onFinished={async () => {
            setCareExitIds(null);
            finishSelection();
            handleSelect(null);
            await tenantMutate("database-students-list");
          }}
        />
      ) : null}

      {resumeTarget ? (
        <CareResumeModal
          isOpen
          studentId={String(resumeTarget.id)}
          displayName={studentsConfig.list.item.title(resumeTarget)}
          onClose={() => setResumeTarget(null)}
          onResumed={async () => {
            setResumeTarget(null);
            await tenantMutate("database-students-list");
          }}
        />
      ) : null}

      {deleteTarget ? (
        <StudentDeletionModal
          isOpen
          studentId={String(deleteTarget.id)}
          displayName={studentsConfig.list.item.title(deleteTarget)}
          onClose={() => setDeleteTarget(null)}
          onDeleted={handleStudentDeleted}
        />
      ) : null}
    </DatabasePageLayout>
  );
}
