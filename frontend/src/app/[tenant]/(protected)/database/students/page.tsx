"use client";

import { Suspense, useCallback, useMemo, useState } from "react";
import { redirect, useSearchParams } from "next/navigation";
import Link from "next/link";
import { useSession } from "next-auth/react";
import { Trash2 } from "lucide-react";
import { DatabaseCreateAction } from "~/components/database/database-create-action";
import { DatabaseEmptyState } from "~/components/database/database-empty-state";
import { DatabaseGroupingToggle } from "~/components/database/database-grouping-toggle";
import { DatabasePageLayout } from "~/components/database/database-page-layout";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
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
import { ConfirmationModal } from "~/components/ui/modal";
import { getDbOperationMessage } from "@/lib/use-notification";
import { createCrudService } from "@/lib/database/service-factory";
import { studentsConfig } from "@/components/database/configs/students.config";
import { useDeleteConfirmation } from "~/hooks/useDeleteConfirmation";
import type { Student } from "@/lib/api";
import type { StudentGuardianPayload } from "@/lib/guardian-helpers";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";
import { createLogger } from "~/lib/logger";
import { hasPermission } from "~/lib/auth-utils";

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
    <Suspense fallback={null}>
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
  const [arrivalRevision, setArrivalRevision] = useState(0);
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

  const handleDeleteStudent = useCallback(async () => {
    if (!selectedStudent) return;
    const deleteError = await service.delete(selectedStudent.id);
    if (deleteError) {
      toastError(deleteError);
      return;
    }
    const displayName = studentsConfig.list.item.title(selectedStudent);
    toastSuccess(
      getDbOperationMessage(
        "delete",
        studentsConfig.name.singular,
        displayName,
      ),
    );
    handleSelect(null);
    await tenantMutate("database-students-list");
  }, [
    selectedStudent,
    service,
    tenantMutate,
    toastSuccess,
    toastError,
    handleSelect,
  ]);

  const {
    showConfirmModal: showDeleteConfirmModal,
    handleDeleteClick,
    handleDeleteCancel,
    confirmDelete,
  } = useDeleteConfirmation();

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

  const detailActions = selectedStudent ? (
    <button
      type="button"
      onClick={handleDeleteClick}
      className="flex items-center gap-1.5 rounded-md border border-red-200 bg-red-50 px-3 py-1.5 text-sm font-medium text-red-700 hover:bg-red-100"
    >
      <Trash2 className="h-3.5 w-3.5" aria-hidden />
      Löschen
    </button>
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
          actionButton={
            <div className="flex items-center gap-2">
              {!isMobile ? (
                <>
                  <DatabaseGroupingToggle
                    value={grouping}
                    options={STUDENTS_GROUPING_OPTIONS}
                    onChange={handleGroupingChange}
                  />
                  {hasPermission(session, "grade_transitions:read") && (
                    <Link
                      href="/database/grade-transitions"
                      className="flex h-10 items-center gap-2 rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 hover:bg-gray-50"
                    >
                      Jahrgangswechsel
                    </Link>
                  )}
                  <Link
                    href="/database/students/import"
                    className="flex h-10 items-center gap-2 rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 hover:bg-gray-50"
                  >
                    Importieren
                  </Link>
                </>
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
          />
        </div>
      ) : !loading ? (
        <DatabaseEmptyState
          icon={null}
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
        groups={allGroups}
      />

      {selectedStudent ? (
        <ConfirmationModal
          isOpen={showDeleteConfirmModal}
          onClose={handleDeleteCancel}
          onConfirm={() => confirmDelete(() => void handleDeleteStudent())}
          title="Kind löschen?"
          confirmText="Löschen"
          cancelText="Abbrechen"
          confirmButtonClass="bg-red-600 hover:bg-red-700"
        >
          <p className="text-sm text-gray-700">
            Möchten Sie das Kind{" "}
            <span className="font-medium">
              {selectedStudent.first_name} {selectedStudent.second_name}
            </span>{" "}
            wirklich löschen? Diese Aktion kann nicht rückgängig gemacht werden.
          </p>
        </ConfirmationModal>
      ) : null}
    </DatabasePageLayout>
  );
}
