"use client";

import { useState, useMemo, useRef, useEffect } from "react";
import { useIsMobile } from "~/hooks/useIsMobile";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import Link from "next/link";
import { DatabasePageLayout } from "~/components/database/database-page-layout";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import type {
  FilterConfig,
  ActiveFilter,
} from "~/components/ui/page-header/types";
import { useToast } from "~/contexts/ToastContext";
import { StudentDetailModal } from "@/components/students/student-detail-modal";
import { StudentEditModal } from "@/components/students/student-edit-modal";
import { StudentCreateModal } from "@/components/students/student-create-modal";
import { ConfirmationModal } from "~/components/ui/modal";
import { getDbOperationMessage } from "@/lib/use-notification";
import { createCrudService } from "@/lib/database/service-factory";
import { studentsConfig } from "@/lib/database/configs/students.config";
import { useDeleteConfirmation } from "~/hooks/useDeleteConfirmation";
import type { Student } from "@/lib/api";
import { useSWRAuth, mutate } from "~/lib/swr";

export default function StudentsPage() {
  const [searchTerm, setSearchTerm] = useState("");
  const [groupFilter, setGroupFilter] = useState("all");
  const isMobile = useIsMobile();

  // Modal states
  const [showCreateModal, setShowCreateModal] = useState(false);

  const [showDetailModal, setShowDetailModal] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
  const [selectedStudent, setSelectedStudent] = useState<Student | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState<string | null>(null);

  // Delete confirmation modal management
  const {
    showConfirmModal: showDeleteConfirmModal,
    handleDeleteClick,
    handleDeleteCancel,
    confirmDelete,
  } = useDeleteConfirmation(setShowDetailModal);

  const { success: toastSuccess, error: toastError } = useToast();

  // Track mounted state to prevent race conditions
  const isMountedRef = useRef(true);

  useEffect(() => {
    isMountedRef.current = true;
    return () => {
      isMountedRef.current = false;
    };
  }, []);

  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  // Create service instance
  const service = useMemo(() => createCrudService(studentsConfig), []);

  // Fetch students with SWR (automatic caching, deduplication, revalidation)
  const {
    data: studentsData,
    isLoading: loading,
    error: studentsError,
  } = useSWRAuth("database-students-list", async () => {
    const data = await service.getList({ page: 1, pageSize: 1000 });
    return Array.isArray(data.data) ? data.data : [];
  });

  const error = studentsError
    ? "Fehler beim Laden der Schüler. Bitte versuchen Sie es später erneut."
    : null;

  // Fetch all groups for the dropdown with SWR
  const { data: allGroups = [] } = useSWRAuth<
    Array<{ value: string; label: string }>
  >("database-groups-dropdown", async () => {
    const response = await fetch("/api/groups");
    if (!response.ok) {
      console.error("Failed to fetch groups:", response.status);
      return [];
    }
    const data: unknown = await response.json();

    // Handle the response - it might be wrapped or an array
    let groups: Array<{ id: number; name: string }> = [];
    if (Array.isArray(data)) {
      groups = data as Array<{ id: number; name: string }>;
    } else if (data && typeof data === "object" && "data" in data) {
      const wrappedData = data as { data: unknown };
      if (Array.isArray(wrappedData.data)) {
        groups = wrappedData.data as Array<{ id: number; name: string }>;
      }
    } else {
      console.error("Unexpected groups response format:", data);
      return [];
    }

    // Map to the format needed by the dropdown
    return groups
      .map((group) => ({
        value: String(group.id),
        label: group.name,
      }))
      .sort((a, b) => a.label.localeCompare(b.label));
  });

  // Apply filters (use studentsData directly to avoid dependency issues)
  const filteredStudents = useMemo(() => {
    const students = studentsData ?? [];
    let filtered = [...students];

    // Search filter
    if (searchTerm) {
      const searchLower = searchTerm.toLowerCase();
      filtered = filtered.filter(
        (student) =>
          (student.first_name?.toLowerCase().includes(searchLower) ?? false) ||
          (student.second_name?.toLowerCase().includes(searchLower) ?? false) ||
          (student.school_class?.toLowerCase().includes(searchLower) ??
            false) ||
          (student.group_name?.toLowerCase().includes(searchLower) ?? false) ||
          (student.name_lg?.toLowerCase().includes(searchLower) ?? false),
      );
    }

    // Group filter
    if (groupFilter !== "all") {
      filtered = filtered.filter((student) => student.group_id === groupFilter);
    }

    // Sort alphabetically by name
    filtered.sort((a, b) => {
      const nameA = `${a.first_name} ${a.second_name}`;
      const nameB = `${b.first_name} ${b.second_name}`;
      return nameA.localeCompare(nameB, "de");
    });

    return filtered;
  }, [studentsData, searchTerm, groupFilter]);

  // Use all groups fetched from API (not just groups with students)
  const uniqueGroups = allGroups;

  // Prepare filters for PageHeaderWithSearch
  const filters: FilterConfig[] = useMemo(
    () => [
      {
        id: "group",
        label: "Gruppe",
        type: "dropdown",
        value: groupFilter,
        onChange: (value) => setGroupFilter(value as string),
        options: [{ value: "all", label: "Alle Gruppen" }, ...uniqueGroups],
      },
    ],
    [groupFilter, uniqueGroups],
  );

  // Prepare active filters
  const activeFilters: ActiveFilter[] = useMemo(() => {
    const filters: ActiveFilter[] = [];

    if (searchTerm) {
      filters.push({
        id: "search",
        label: `"${searchTerm}"`,
        onRemove: () => setSearchTerm(""),
      });
    }

    if (groupFilter !== "all") {
      const group = uniqueGroups.find((g) => g.value === groupFilter);
      filters.push({
        id: "group",
        label: group?.label ?? "Gruppe",
        onRemove: () => setGroupFilter("all"),
      });
    }

    return filters;
  }, [searchTerm, groupFilter, uniqueGroups]);

  // Handle student selection
  const handleSelectStudent = async (student: Student) => {
    setSelectedStudent(student);
    setShowDetailModal(true);
    setDetailError(null);

    try {
      setDetailLoading(true);
      const freshData = await service.getOne(student.id);

      // Only update state if still mounted
      if (!isMountedRef.current) return;

      setSelectedStudent(freshData);
    } catch (err) {
      // Only update state if still mounted
      if (!isMountedRef.current) return;

      const errorMessage =
        err instanceof Error
          ? err.message
          : "Fehler beim Laden der Schülerdaten.";
      setDetailError(errorMessage);
      toastError(errorMessage);
    } finally {
      if (isMountedRef.current) {
        setDetailLoading(false);
      }
    }
  };

  // Handle create student
  const handleCreateStudent = async (studentData: Partial<Student>) => {
    if (studentsConfig.form.transformBeforeSubmit) {
      studentData = studentsConfig.form.transformBeforeSubmit(studentData);
    }

    const newStudent = await service.create(studentData);

    // Only update state if still mounted
    if (!isMountedRef.current) return;

    const displayName = studentsConfig.list.item.title(newStudent);
    toastSuccess(
      getDbOperationMessage(
        "create",
        studentsConfig.name.singular,
        displayName,
      ),
    );

    setShowCreateModal(false);
    await mutate("database-students-list");
  };

  // Handle update student
  const handleUpdateStudent = async (studentData: Partial<Student>) => {
    if (!selectedStudent) return;

    try {
      setDetailLoading(true);

      if (studentsConfig.form.transformBeforeSubmit) {
        studentData = studentsConfig.form.transformBeforeSubmit(studentData);
      }

      await service.update(selectedStudent.id, studentData);

      // Only update state if still mounted
      if (!isMountedRef.current) return;

      const displayName = studentsConfig.list.item.title(selectedStudent);
      toastSuccess(
        getDbOperationMessage(
          "update",
          studentsConfig.name.singular,
          displayName,
        ),
      );

      // Refresh student data
      const refreshedStudent = await service.getOne(selectedStudent.id);

      // Check again before updating state after second async operation
      if (!isMountedRef.current) return;

      setSelectedStudent(refreshedStudent);

      // Close edit modal and show updated detail modal
      setShowEditModal(false);
      setShowDetailModal(true);

      await mutate("database-students-list");
    } catch (err) {
      console.error("Error updating student:", err);
      throw err;
    } finally {
      if (isMountedRef.current) {
        setDetailLoading(false);
      }
    }
  };

  // Handle delete student
  const handleDeleteStudent = async () => {
    if (!selectedStudent) return;

    try {
      setDetailLoading(true);
      await service.delete(selectedStudent.id);

      // Only update state if still mounted
      if (!isMountedRef.current) return;

      const displayName = studentsConfig.list.item.title(selectedStudent);
      toastSuccess(
        getDbOperationMessage(
          "delete",
          studentsConfig.name.singular,
          displayName,
        ),
      );

      setShowDetailModal(false);
      setSelectedStudent(null);
      await mutate("database-students-list");
    } catch (err) {
      console.error("Error deleting student:", err);
    } finally {
      if (isMountedRef.current) {
        setDetailLoading(false);
      }
    }
  };

  // Handle edit button click
  const handleEditClick = () => {
    setShowDetailModal(false);
    setShowEditModal(true);
  };

  return (
    <DatabasePageLayout
      loading={loading}
      sessionLoading={status === "loading"}
      className="-mt-1.5 w-full"
    >
      <div className="mb-4">
        <PageHeaderWithSearch
          title={isMobile ? "Schüler" : ""}
          badge={{
            icon: (
              <svg
                className="h-5 w-5 text-gray-600"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z"
                />
              </svg>
            ),
            count: filteredStudents.length,
            label: "Schüler",
          }}
          search={{
            value: searchTerm,
            onChange: setSearchTerm,
            placeholder: "Schüler suchen...",
          }}
          filters={filters}
          activeFilters={activeFilters}
          onClearAllFilters={() => {
            setSearchTerm("");
            setGroupFilter("all");
          }}
          actionButton={
            !isMobile && (
              <div className="flex items-center gap-3">
                <Link
                  href="/database/students/csv-import"
                  className="group relative flex h-10 items-center gap-2 rounded-full bg-gradient-to-br from-purple-500 to-purple-600 px-4 text-white shadow-lg transition-all duration-300 hover:scale-105 hover:shadow-xl active:scale-95"
                  aria-label="CSV Import"
                >
                  <div className="pointer-events-none absolute inset-[2px] rounded-full bg-gradient-to-br from-white/20 to-white/0 opacity-0 transition-opacity duration-150 group-hover:opacity-100"></div>
                  <svg
                    className="relative h-5 w-5 transition-transform duration-300"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    strokeWidth={2.5}
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12"
                    />
                  </svg>
                  <span className="relative text-sm font-semibold">
                    CSV Import
                  </span>
                  <div className="pointer-events-none absolute inset-0 scale-0 rounded-full bg-white/20 opacity-0 transition-transform duration-200 group-hover:scale-100 group-hover:opacity-100"></div>
                </Link>
                <button
                  onClick={() => setShowCreateModal(true)}
                  className="group relative flex h-10 w-10 items-center justify-center rounded-full bg-gradient-to-br from-[#5080D8] to-[#4070c8] text-white shadow-lg transition-all duration-150 hover:scale-105 hover:shadow-xl active:scale-95"
                  style={{
                    background:
                      "linear-gradient(135deg, rgb(80, 128, 216) 0%, rgb(64, 112, 200) 100%)",
                    willChange: "transform, opacity",
                    WebkitTransform: "translateZ(0)",
                    transform: "translateZ(0)",
                  }}
                  aria-label="Schüler erstellen"
                >
                  <div className="pointer-events-none absolute inset-[2px] rounded-full bg-gradient-to-br from-white/20 to-white/0 opacity-0 transition-opacity duration-150 group-hover:opacity-100"></div>
                  <svg
                    className="relative h-5 w-5 transition-transform duration-150 group-active:rotate-90"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    strokeWidth={2.5}
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      d="M12 4.5v15m7.5-7.5h-15"
                    />
                  </svg>
                  <div className="pointer-events-none absolute inset-0 scale-0 rounded-full bg-white/20 opacity-0 transition-transform duration-200 group-hover:scale-100 group-hover:opacity-100"></div>
                </button>
              </div>
            )
          }
        />
      </div>

      {/* Mobile FAB Create Button */}
      <button
        onClick={() => setShowCreateModal(true)}
        className="group pointer-events-auto fixed right-4 bottom-24 z-40 flex h-14 w-14 translate-y-0 items-center justify-center rounded-full bg-gradient-to-br from-[#5080D8] to-[#4070c8] text-white opacity-100 shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all duration-300 ease-out hover:shadow-[0_8px_40px_rgb(80,128,216,0.3)] active:scale-95 md:hidden"
        style={{
          background:
            "linear-gradient(135deg, rgb(80, 128, 216) 0%, rgb(64, 112, 200) 100%)",
          willChange: "transform, opacity",
          WebkitTransform: "translateZ(0)",
          transform: "translateZ(0)",
        }}
        aria-label="Schüler erstellen"
      >
        <div className="pointer-events-none absolute inset-[2px] rounded-full bg-gradient-to-br from-white/20 to-white/0 opacity-0 transition-opacity duration-150 group-hover:opacity-100"></div>
        <svg
          className="pointer-events-none relative h-6 w-6 transition-transform duration-150 group-active:rotate-90"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={2.5}
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            d="M12 4.5v15m7.5-7.5h-15"
          />
        </svg>
        <div className="pointer-events-none absolute inset-0 scale-0 rounded-full bg-white/20 opacity-0 transition-transform duration-200 group-hover:scale-100 group-hover:opacity-100"></div>
      </button>

      {/* Error Display */}
      {error && (
        <div className="mb-6 rounded-lg border border-red-200 bg-red-50 p-4">
          <p className="text-sm text-red-800">{error}</p>
        </div>
      )}

      {/* Student List */}
      {filteredStudents.length === 0 ? (
        <div className="flex min-h-[300px] items-center justify-center">
          <div className="text-center">
            <svg
              className="mx-auto h-12 w-12 text-gray-400"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={1.5}
                d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z"
              />
            </svg>
            <h3 className="mt-4 text-lg font-medium text-gray-900">
              {searchTerm || groupFilter !== "all"
                ? "Keine Schüler gefunden"
                : "Keine Schüler vorhanden"}
            </h3>
            <p className="mt-2 text-sm text-gray-600">
              {searchTerm || groupFilter !== "all"
                ? "Versuchen Sie andere Suchkriterien oder Filter."
                : "Es wurden noch keine Schüler erstellt."}
            </p>
          </div>
        </div>
      ) : (
        <div className="space-y-3">
          {filteredStudents.map((student, index) => {
            const initials = `${student.first_name?.[0] ?? ""}${student.second_name?.[0] ?? ""}`;
            const handleClick = () => handleSelectStudent(student);

            return (
              <button
                type="button"
                key={student.id}
                onClick={handleClick}
                className="group relative w-full cursor-pointer overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 text-left shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-150 active:scale-[0.98] md:hover:-translate-y-0.5 md:hover:border-blue-300/50 md:hover:bg-white md:hover:shadow-[0_12px_40px_rgb(0,0,0,0.18)]"
                style={{
                  animationName: "fadeInUp",
                  animationDuration: "0.5s",
                  animationTimingFunction: "ease-out",
                  animationFillMode: "forwards",
                  animationDelay: `${index * 0.03}s`,
                  opacity: 0,
                }}
              >
                {/* Modern gradient overlay */}
                <div className="absolute inset-0 rounded-3xl bg-gradient-to-br from-blue-50/80 to-cyan-100/80 opacity-[0.03]"></div>
                {/* Subtle inner glow */}
                <div className="absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
                {/* Modern border highlight */}
                <div className="absolute inset-0 rounded-3xl ring-1 ring-white/20 transition-all duration-300 md:group-hover:ring-blue-200/60"></div>

                <div className="relative flex items-center gap-4 p-5">
                  {/* Avatar */}
                  <div className="flex-shrink-0">
                    <div className="flex h-12 w-12 items-center justify-center rounded-full bg-gradient-to-br from-[#5080D8] to-[#4070c8] font-semibold text-white shadow-md transition-transform duration-150 md:group-hover:scale-105">
                      {initials}
                    </div>
                  </div>

                  {/* Student Info */}
                  <div className="min-w-0 flex-1">
                    <h3 className="text-lg font-semibold text-gray-900 transition-colors duration-300 md:group-hover:text-blue-600">
                      {student.first_name} {student.second_name}
                    </h3>
                    <div className="mt-1 flex flex-wrap items-center gap-2">
                      {/* Group Badge */}
                      {student.group_name && (
                        <span className="inline-flex items-center rounded-full bg-[#83CD2D]/10 px-2 py-1 text-xs font-medium text-[#5A8B1F]">
                          {student.group_name}
                        </span>
                      )}
                    </div>
                    {/* Guardian info */}
                    {student.name_lg && (
                      <p className="mt-1 text-sm text-gray-500">
                        <span className="text-gray-400">
                          Erziehungsberechtigter:
                        </span>{" "}
                        {student.name_lg}
                      </p>
                    )}
                  </div>

                  {/* Arrow Icon */}
                  <div className="flex-shrink-0">
                    <svg
                      className="h-6 w-6 text-gray-400 transition-all duration-300 md:group-hover:translate-x-1 md:group-hover:text-blue-600"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M9 5l7 7-7 7"
                      />
                    </svg>
                  </div>
                </div>

                {/* Glowing border effect on hover */}
                <div className="absolute inset-0 rounded-3xl bg-gradient-to-r from-transparent via-blue-100/30 to-transparent opacity-0 transition-opacity duration-300 md:group-hover:opacity-100"></div>
              </button>
            );
          })}
        </div>
      )}

      {/* Create Modal */}
      <StudentCreateModal
        isOpen={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        onCreate={handleCreateStudent}
        groups={uniqueGroups}
      />

      {/* Detail Modal */}
      <StudentDetailModal
        isOpen={showDetailModal}
        onClose={() => {
          setShowDetailModal(false);
          setSelectedStudent(null);
          setDetailError(null);
        }}
        student={selectedStudent}
        onEdit={handleEditClick}
        onDelete={() => void handleDeleteStudent()}
        loading={detailLoading}
        error={detailError}
        onDeleteClick={handleDeleteClick}
      />

      {/* Delete Confirmation Modal */}
      {selectedStudent && (
        <ConfirmationModal
          isOpen={showDeleteConfirmModal}
          onClose={handleDeleteCancel}
          onConfirm={() => confirmDelete(() => void handleDeleteStudent())}
          title="Schüler löschen?"
          confirmText="Löschen"
          cancelText="Abbrechen"
          confirmButtonClass="bg-red-600 hover:bg-red-700"
        >
          <p className="text-sm text-gray-700">
            Möchten Sie den Schüler{" "}
            <span className="font-medium">
              {selectedStudent.first_name} {selectedStudent.second_name}
            </span>{" "}
            wirklich löschen? Diese Aktion kann nicht rückgängig gemacht werden.
          </p>
        </ConfirmationModal>
      )}

      {/* Edit Modal */}
      <StudentEditModal
        isOpen={showEditModal}
        onClose={() => {
          setShowEditModal(false);
        }}
        student={selectedStudent}
        onSave={handleUpdateStudent}
        loading={detailLoading}
        groups={uniqueGroups}
      />

      {/* Success toasts handled globally */}
    </DatabasePageLayout>
  );
}
