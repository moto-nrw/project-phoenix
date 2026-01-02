"use client";

import { useState, useEffect, useMemo, useCallback } from "react";
import { useSession } from "next-auth/react";
import { redirect, useRouter } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import type { ActiveFilter } from "~/components/ui/page-header/types";
import { useToast } from "~/contexts/ToastContext";
import {
  TeacherRoleManagementModal,
  TeacherPermissionManagementModal,
} from "@/components/teachers";
import { TeacherDetailModal } from "@/components/teachers/teacher-detail-modal";
import { TeacherEditModal } from "@/components/teachers/teacher-edit-modal";
import { TeacherCreateModal } from "@/components/teachers/teacher-create-modal";
import { getDbOperationMessage } from "@/lib/use-notification";
import { createCrudService } from "@/lib/database/service-factory";
import { teachersConfig } from "@/lib/database/configs/teachers.config";
import type { Teacher } from "@/lib/teacher-api";
import { Modal } from "~/components/ui/modal";

import { Loading } from "~/components/ui/loading";

// Helper function to get teacher initials without nested ternary
function getTeacherInitials(
  firstName: string | undefined,
  lastName: string | undefined,
  fullName: string | undefined,
): string {
  if (firstName && lastName) {
    return `${firstName[0]}${lastName[0]}`;
  }
  if (fullName) {
    return fullName
      .split(" ")
      .map((n) => n[0])
      .join("")
      .substring(0, 2);
  }
  return "XX";
}

export default function TeachersPage() {
  const router = useRouter();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [teachers, setTeachers] = useState<Teacher[]>([]);
  const [searchTerm, setSearchTerm] = useState("");
  const [isMobile, setIsMobile] = useState(false);

  // Modal states
  const [showChoiceModal, setShowChoiceModal] = useState(false);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [createLoading, setCreateLoading] = useState(false);

  const [showDetailModal, setShowDetailModal] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
  const [selectedTeacher, setSelectedTeacher] = useState<Teacher | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  // Role and permission modals
  const [roleModalOpen, setRoleModalOpen] = useState(false);
  const [permissionModalOpen, setPermissionModalOpen] = useState(false);

  const { success: toastSuccess } = useToast();

  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  // Create service instance
  const service = useMemo(() => createCrudService(teachersConfig), []);

  // Handle mobile detection
  useEffect(() => {
    const checkMobile = () => {
      setIsMobile(window.innerWidth < 768);
    };
    checkMobile();
    window.addEventListener("resize", checkMobile);
    return () => window.removeEventListener("resize", checkMobile);
  }, []);

  // Fetch teachers
  const fetchTeachers = useCallback(async () => {
    try {
      setLoading(true);
      const data = await service.getList({ page: 1, pageSize: 1000 });
      const teachersArray = Array.isArray(data.data) ? data.data : [];
      setTeachers(teachersArray);
      setError(null);
    } catch (err) {
      console.error("Error fetching teachers:", err);
      setError(
        "Fehler beim Laden der Betreuer. Bitte versuchen Sie es später erneut.",
      );
      setTeachers([]);
    } finally {
      setLoading(false);
    }
  }, [service]);

  // Load teachers on mount
  useEffect(() => {
    void fetchTeachers();
  }, [fetchTeachers]);

  // Apply filters
  const filteredTeachers = useMemo(() => {
    let filtered = [...teachers];

    // Search filter
    if (searchTerm) {
      const searchLower = searchTerm.toLowerCase();
      filtered = filtered.filter(
        (teacher) =>
          (teacher.first_name?.toLowerCase().includes(searchLower) ?? false) ||
          (teacher.last_name?.toLowerCase().includes(searchLower) ?? false) ||
          (teacher.name?.toLowerCase().includes(searchLower) ?? false) ||
          (teacher.role?.toLowerCase().includes(searchLower) ?? false) ||
          (teacher.specialization?.toLowerCase().includes(searchLower) ??
            false) ||
          (teacher.email?.toLowerCase().includes(searchLower) ?? false),
      );
    }

    // Sort alphabetically by name
    filtered.sort((a, b) => {
      const nameA = a.name ?? `${a.first_name} ${a.last_name}`;
      const nameB = b.name ?? `${b.first_name} ${b.last_name}`;
      return nameA.localeCompare(nameB, "de");
    });

    return filtered;
  }, [teachers, searchTerm]);

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

    return filters;
  }, [searchTerm]);

  // Handle teacher selection
  const handleSelectTeacher = async (teacher: Teacher) => {
    setSelectedTeacher(teacher);
    setShowDetailModal(true);

    try {
      setDetailLoading(true);
      // Use staff_id if available, otherwise fall back to id
      const idToFetch = teacher.staff_id ?? teacher.id;
      const freshData = await service.getOne(idToFetch);
      setSelectedTeacher(freshData);
    } catch (err) {
      console.error("Error fetching teacher details:", err);
    } finally {
      setDetailLoading(false);
    }
  };

  // Handle create teacher
  const handleCreateTeacher = async (
    data: Partial<Teacher> & { password?: string },
  ) => {
    try {
      setCreateLoading(true);
      await service.create(data);
      setShowCreateModal(false);
      toastSuccess(
        getDbOperationMessage("create", teachersConfig.name.singular),
      );
      await fetchTeachers();
    } catch (err) {
      console.error("Error creating teacher:", err);
      throw err;
    } finally {
      setCreateLoading(false);
    }
  };

  // Handle edit teacher
  const handleEditTeacher = async (
    data: Partial<Teacher> & { password?: string },
  ) => {
    if (!selectedTeacher) return;

    try {
      setDetailLoading(true);
      await service.update(selectedTeacher.id, data);
      setShowEditModal(false);
      setShowDetailModal(false);
      toastSuccess(
        getDbOperationMessage("update", teachersConfig.name.singular),
      );
      await fetchTeachers();
      setSelectedTeacher(null);
    } catch (err) {
      console.error("Error updating teacher:", err);
      throw err;
    } finally {
      setDetailLoading(false);
    }
  };

  // Handle delete teacher
  const handleDeleteTeacher = async () => {
    if (!selectedTeacher) return;

    try {
      setDetailLoading(true);
      await service.delete(selectedTeacher.id);
      setShowDetailModal(false);
      toastSuccess(
        getDbOperationMessage("delete", teachersConfig.name.singular),
      );
      await fetchTeachers();
      setSelectedTeacher(null);
    } catch (err) {
      console.error("Error deleting teacher:", err);
      throw err;
    } finally {
      setDetailLoading(false);
    }
  };

  // Handle edit click from detail modal
  const handleEditClick = () => {
    setShowDetailModal(false);
    setShowEditModal(true);
  };

  if (status === "loading" || loading) {
    return (
      <ResponsiveLayout>
        <Loading fullPage={false} />
      </ResponsiveLayout>
    );
  }

  return (
    <ResponsiveLayout>
      <div className="-mt-1.5 w-full">
        {/* Mobile Back Button */}
        {isMobile && (
          <button
            onClick={() => (window.location.href = "/database")}
            className="relative z-10 mb-3 flex items-center gap-2 text-gray-600 transition-colors duration-200 hover:text-gray-900"
            aria-label="Zurück zur Datenverwaltung"
          >
            <svg
              className="h-5 w-5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M15 19l-7-7 7-7"
              />
            </svg>
            <span className="text-sm font-medium">Zurück</span>
          </button>
        )}

        {/* PageHeaderWithSearch - Title only on mobile */}
        <div className="mb-4">
          <PageHeaderWithSearch
            title={isMobile ? "Betreuer" : ""}
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
                    d="M12 14l9-5-9-5-9 5 9 5z M12 14l6.16-3.422a12.083 12.083 0 01.665 6.479A11.952 11.952 0 0012 20.055a11.952 11.952 0 00-6.824-2.998 12.078 12.078 0 01.665-6.479L12 14z"
                  />
                </svg>
              ),
              count: filteredTeachers.length,
              label: "Betreuer",
            }}
            search={{
              value: searchTerm,
              onChange: setSearchTerm,
              placeholder: "Betreuer suchen...",
            }}
            filters={[]}
            activeFilters={activeFilters}
            onClearAllFilters={() => {
              setSearchTerm("");
            }}
            actionButton={
              !isMobile && (
                <button
                  onClick={() => setShowChoiceModal(true)}
                  className="group relative flex h-10 w-10 items-center justify-center rounded-full bg-gradient-to-br from-[#F78C10] to-[#e57a00] text-white shadow-lg transition-all duration-300 hover:scale-110 hover:shadow-xl active:scale-95"
                  style={{
                    background:
                      "linear-gradient(135deg, rgb(247, 140, 16) 0%, rgb(229, 122, 0) 100%)",
                    willChange: "transform, opacity",
                    WebkitTransform: "translateZ(0)",
                    transform: "translateZ(0)",
                  }}
                  aria-label="Betreuer hinzufügen"
                >
                  <div className="pointer-events-none absolute inset-[2px] rounded-full bg-gradient-to-br from-white/20 to-white/0 opacity-0 transition-opacity duration-300 group-hover:opacity-100"></div>
                  <svg
                    className="relative h-5 w-5 transition-transform duration-300 group-active:rotate-90"
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
                  <div className="pointer-events-none absolute inset-0 scale-0 rounded-full bg-white/20 opacity-0 transition-transform duration-500 group-hover:scale-100 group-hover:opacity-100"></div>
                </button>
              )
            }
          />
        </div>

        {/* Mobile FAB Create Button */}
        <button
          onClick={() => setShowChoiceModal(true)}
          className="group pointer-events-auto fixed right-4 bottom-24 z-40 flex h-14 w-14 translate-y-0 items-center justify-center rounded-full bg-gradient-to-br from-[#F78C10] to-[#e57a00] text-white opacity-100 shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all duration-300 ease-out hover:shadow-[0_8px_40px_rgb(247,140,16,0.3)] active:scale-95 md:hidden"
          style={{
            background:
              "linear-gradient(135deg, rgb(247, 140, 16) 0%, rgb(229, 122, 0) 100%)",
            willChange: "transform, opacity",
            WebkitTransform: "translateZ(0)",
            transform: "translateZ(0)",
          }}
          aria-label="Betreuer hinzufügen"
        >
          <div className="pointer-events-none absolute inset-[2px] rounded-full bg-gradient-to-br from-white/20 to-white/0 opacity-0 transition-opacity duration-300 group-hover:opacity-100"></div>
          <svg
            className="pointer-events-none relative h-6 w-6 transition-transform duration-300 group-active:rotate-90"
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
          <div className="pointer-events-none absolute inset-0 scale-0 rounded-full bg-white/20 opacity-0 transition-transform duration-500 group-hover:scale-100 group-hover:opacity-100"></div>
        </button>

        {/* Error Display */}
        {error && (
          <div className="mb-6 rounded-lg border border-red-200 bg-red-50 p-4">
            <p className="text-sm text-red-800">{error}</p>
          </div>
        )}

        {/* Teacher List */}
        {filteredTeachers.length === 0 ? (
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
                  d="M12 14l9-5-9-5-9 5 9 5z M12 14l6.16-3.422a12.083 12.083 0 01.665 6.479A11.952 11.952 0 0012 20.055a11.952 11.952 0 00-6.824-2.998 12.078 12.078 0 01.665-6.479L12 14z"
                />
              </svg>
              <h3 className="mt-4 text-lg font-medium text-gray-900">
                {searchTerm
                  ? "Keine Betreuer gefunden"
                  : "Keine Betreuer vorhanden"}
              </h3>
              <p className="mt-2 text-sm text-gray-600">
                {searchTerm
                  ? "Versuchen Sie andere Suchkriterien."
                  : "Es wurden noch keine Betreuer erstellt."}
              </p>
            </div>
          </div>
        ) : (
          <div className="space-y-3">
            {filteredTeachers.map((teacher, index) => {
              const initials = getTeacherInitials(
                teacher.first_name,
                teacher.last_name,
                teacher.name,
              );
              const displayName =
                teacher.name || `${teacher.first_name} ${teacher.last_name}`;

              const handleClick = () => handleSelectTeacher(teacher);
              return (
                <button
                  type="button"
                  key={teacher.id}
                  onClick={handleClick}
                  className="group relative w-full cursor-pointer overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 text-left shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-500 active:scale-[0.99] md:hover:-translate-y-1 md:hover:scale-[1.01] md:hover:border-orange-200/50 md:hover:bg-white md:hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)]"
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
                  <div className="absolute inset-0 rounded-3xl bg-gradient-to-br from-orange-50/80 to-amber-100/80 opacity-[0.03]"></div>
                  {/* Subtle inner glow */}
                  <div className="absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
                  {/* Modern border highlight */}
                  <div className="absolute inset-0 rounded-3xl ring-1 ring-white/20 transition-all duration-300 md:group-hover:ring-orange-200/60"></div>

                  <div className="relative flex items-center gap-4 p-5">
                    {/* Avatar */}
                    <div className="flex-shrink-0">
                      <div className="flex h-12 w-12 items-center justify-center rounded-full bg-gradient-to-br from-[#F78C10] to-[#e57a00] text-sm font-semibold text-white shadow-md transition-transform duration-300 md:group-hover:scale-110">
                        {initials.toUpperCase()}
                      </div>
                    </div>

                    {/* Teacher Info */}
                    <div className="min-w-0 flex-1">
                      <h3 className="text-lg font-semibold text-gray-900 transition-colors duration-300 md:group-hover:text-orange-600">
                        {displayName}
                      </h3>
                      <div className="mt-1 flex flex-wrap items-center gap-2">
                        {/* Specialization Badge */}
                        {teacher.role && (
                          <span className="inline-flex items-center rounded-full bg-orange-100 px-2 py-1 text-xs font-medium text-orange-800">
                            {teacher.role}
                          </span>
                        )}
                      </div>
                      {/* Email info */}
                      {teacher.email && (
                        <p className="mt-1 text-sm text-gray-500">
                          <span className="text-gray-400">E-Mail:</span>{" "}
                          {teacher.email}
                        </p>
                      )}
                    </div>

                    {/* Arrow Icon */}
                    <div className="flex-shrink-0">
                      <svg
                        className="h-6 w-6 text-gray-400 transition-all duration-300 md:group-hover:translate-x-1 md:group-hover:text-orange-600"
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
                  <div className="absolute inset-0 rounded-3xl bg-gradient-to-r from-transparent via-orange-100/30 to-transparent opacity-0 transition-opacity duration-300 md:group-hover:opacity-100"></div>
                </button>
              );
            })}
          </div>
        )}
      </div>

      {/* Choice Modal - Create or Invite */}
      <Modal
        isOpen={showChoiceModal}
        onClose={() => setShowChoiceModal(false)}
        title="Betreuer hinzufügen"
      >
        <div className="space-y-4">
          <p className="text-sm text-gray-600">
            Wählen Sie, wie Sie einen neuen Betreuer hinzufügen möchten:
          </p>

          <div className="grid grid-cols-1 gap-3">
            {/* Manual Create Option */}
            <button
              onClick={() => {
                setShowChoiceModal(false);
                setShowCreateModal(true);
              }}
              className="group relative overflow-hidden rounded-xl border-2 border-gray-200 bg-white p-4 text-left transition-all duration-300 hover:border-gray-300 hover:bg-gray-50 active:scale-98"
            >
              <div className="flex items-start gap-3">
                <div className="rounded-lg bg-gray-100 p-2.5 transition-all duration-300 group-hover:bg-gray-200">
                  <svg
                    className="h-5 w-5 text-gray-600 transition-colors duration-300"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    strokeWidth={2}
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
                    />
                  </svg>
                </div>
                <div className="flex-1">
                  <h3 className="font-semibold text-gray-900">
                    Manuell erstellen
                  </h3>
                  <p className="mt-1 text-sm text-gray-600">
                    Account direkt als Admin anlegen und Daten eingeben
                  </p>
                </div>
              </div>
            </button>

            {/* Email Invite Option */}
            <button
              onClick={() => {
                setShowChoiceModal(false);
                router.push("/invitations");
              }}
              className="group relative overflow-hidden rounded-xl border-2 border-gray-200 bg-white p-4 text-left transition-all duration-300 hover:border-gray-300 hover:bg-gray-50 active:scale-98"
            >
              <div className="flex items-start gap-3">
                <div className="rounded-lg bg-gray-100 p-2.5 transition-all duration-300 group-hover:bg-gray-200">
                  <svg
                    className="h-5 w-5 text-gray-600 transition-colors duration-300"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    strokeWidth={2}
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
                    />
                  </svg>
                </div>
                <div className="flex-1">
                  <h3 className="font-semibold text-gray-900">
                    Per E-Mail einladen
                  </h3>
                  <p className="mt-1 text-sm text-gray-600">
                    Einladungslink per E-Mail senden - Betreuer erstellt eigenen
                    Account
                  </p>
                </div>
              </div>
            </button>
          </div>
        </div>
      </Modal>

      {/* Create Teacher Modal */}
      <TeacherCreateModal
        isOpen={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        onCreate={handleCreateTeacher}
        loading={createLoading}
      />

      {/* Teacher Detail Modal */}
      {selectedTeacher && (
        <TeacherDetailModal
          isOpen={showDetailModal}
          onClose={() => {
            setShowDetailModal(false);
            setSelectedTeacher(null);
          }}
          teacher={selectedTeacher}
          onEdit={handleEditClick}
          onDelete={handleDeleteTeacher}
          loading={detailLoading}
        />
      )}

      {/* Teacher Edit Modal */}
      {selectedTeacher && (
        <TeacherEditModal
          isOpen={showEditModal}
          onClose={() => {
            setShowEditModal(false);
          }}
          teacher={selectedTeacher}
          onSave={handleEditTeacher}
          loading={detailLoading}
        />
      )}

      {/* Role Management Modal */}
      {selectedTeacher && (
        <TeacherRoleManagementModal
          isOpen={roleModalOpen}
          onClose={() => {
            setRoleModalOpen(false);
            setSelectedTeacher(null);
          }}
          teacher={selectedTeacher}
          onUpdate={() => void fetchTeachers()}
        />
      )}

      {/* Permission Management Modal */}
      {selectedTeacher && (
        <TeacherPermissionManagementModal
          isOpen={permissionModalOpen}
          onClose={() => {
            setPermissionModalOpen(false);
            setSelectedTeacher(null);
          }}
          teacher={selectedTeacher}
          onUpdate={() => void fetchTeachers()}
        />
      )}

      {/* Success toasts handled globally */}
    </ResponsiveLayout>
  );
}
