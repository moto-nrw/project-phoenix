"use client";

import { createLogger } from "~/lib/logger";
import Image from "next/image";
import { useState, useMemo, useCallback } from "react";
import { getRoleDisplayName } from "~/lib/auth-helpers";

const logger = createLogger({ component: "DatabaseTeachersPage" });
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";

import { DatabasePageLayout } from "~/components/database/database-page-layout";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import type { ActiveFilter } from "~/components/ui/page-header/types";
import { useToast } from "~/contexts/ToastContext";
import { useIsMobile } from "~/hooks/useIsMobile";
import {
  CaregiverCapabilityModal,
  TeacherRoleManagementModal,
  TeacherPermissionManagementModal,
} from "@/components/teachers";
import { TeacherDetailModal } from "@/components/teachers/teacher-detail-modal";
import { TeacherEditModal } from "@/components/teachers/teacher-edit-modal";
import { InvitationForm } from "~/components/admin/invitation-form";
import { PendingInvitationsList } from "~/components/admin/pending-invitations-list";
import { RoleGuard } from "~/components/auth/role-guard";
import { getDbOperationMessage } from "@/lib/use-notification";
import { createCrudService } from "@/lib/database/service-factory";
import { teachersConfig } from "@/lib/database/configs/teachers.config";
import type { Teacher } from "@/lib/teacher-api";
import { Modal, ConfirmationModal } from "~/components/ui/modal";
import { useDeleteConfirmation } from "~/hooks/useDeleteConfirmation";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";

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
  const [searchTerm, setSearchTerm] = useState("");
  const isMobile = useIsMobile();

  // Modal states
  const [showInviteModal, setShowInviteModal] = useState(false);
  const [invitationRefreshKey, setInvitationRefreshKey] = useState<number>(
    Date.now(),
  );

  const [showDetailModal, setShowDetailModal] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);

  // Stable onClose handlers to prevent Modal animation-reset flicker
  const handleCloseInviteModal = useCallback(
    () => setShowInviteModal(false),
    [],
  );
  const handleCloseDetailModal = useCallback(() => {
    setShowDetailModal(false);
    setSelectedTeacher(null);
  }, []);
  const handleCloseEditModal = useCallback(() => setShowEditModal(false), []);
  const [selectedTeacher, setSelectedTeacher] = useState<Teacher | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  // Delete confirmation modal management
  const {
    showConfirmModal: showDeleteConfirmModal,
    handleDeleteClick,
    handleDeleteCancel,
    confirmDelete,
  } = useDeleteConfirmation(setShowDetailModal);

  // Role and permission modals
  const [roleModalOpen, setRoleModalOpen] = useState(false);
  const [permissionModalOpen, setPermissionModalOpen] = useState(false);
  const [caregiverModalOpen, setCaregiverModalOpen] = useState(false);

  const { success: toastSuccess, error: toastError } = useToast();

  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  // Create service instance
  const service = useMemo(() => createCrudService(teachersConfig), []);
  const tenantMutate = useTenantMutate();

  // Fetch teachers with SWR (automatic caching, deduplication, revalidation)
  const {
    data: teachersData,
    isLoading: loading,
    error: teachersError,
  } = useSWRAuth("database-teachers-list", async () => {
    const data = await service.getList({ page: 1, pageSize: 1000 });
    return Array.isArray(data.data) ? data.data : [];
  });

  const error = teachersError
    ? "Fehler beim Laden des Personals. Bitte versuchen Sie es später erneut."
    : null;

  // Extract unique positions for autocomplete suggestions
  const existingPositions = useMemo(() => {
    const teachers = teachersData ?? [];
    const positions = new Set<string>();
    for (const t of teachers) {
      if (t.role?.trim()) positions.add(t.role.trim());
    }
    return [...positions].sort((a, b) => a.localeCompare(b, "de"));
  }, [teachersData]);

  // Apply filters (use teachersData directly to avoid dependency issues)
  const filteredTeachers = useMemo(() => {
    const teachers = teachersData ?? [];
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
          (teacher.account_role?.toLowerCase().includes(searchLower) ??
            false) ||
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
  }, [teachersData, searchTerm]);

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
      logger.error("failed to fetch teacher details", {
        error: err instanceof Error ? err.message : String(err),
      });
    } finally {
      setDetailLoading(false);
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
      await tenantMutate("database-teachers-list");
      setSelectedTeacher(null);
    } catch (err) {
      logger.error("failed to update teacher", {
        error: err instanceof Error ? err.message : String(err),
      });
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
      const deleteError = await service.delete(selectedTeacher.id);
      if (deleteError) {
        toastError(deleteError);
        return;
      }
      setShowDetailModal(false);
      toastSuccess(
        getDbOperationMessage("delete", teachersConfig.name.singular),
      );
      await tenantMutate("database-teachers-list");
      setSelectedTeacher(null);
    } finally {
      setDetailLoading(false);
    }
  };

  // Handle inline notes update from detail modal
  const handleUpdateNotes = async (notes: string) => {
    if (!selectedTeacher) return;

    try {
      await service.update(selectedTeacher.id, { staff_notes: notes });
      setSelectedTeacher({ ...selectedTeacher, staff_notes: notes });
      await tenantMutate("database-teachers-list");
    } catch (err) {
      logger.error("failed to update teacher notes", {
        error: err instanceof Error ? err.message : String(err),
      });
      throw err;
    }
  };

  // Handle edit click from detail modal
  const handleEditClick = () => {
    setShowDetailModal(false);
    setShowEditModal(true);
  };

  const handleManageCaregiverClick = () => {
    setShowDetailModal(false);
    setCaregiverModalOpen(true);
  };

  return (
    <DatabasePageLayout
      loading={loading}
      sessionLoading={status === "loading"}
      className="-mt-1.5 w-full"
    >
      <div className="mb-4">
        <PageHeaderWithSearch
          title={isMobile ? "Personal" : ""}
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
            label: "Personal",
          }}
          search={{
            value: searchTerm,
            onChange: setSearchTerm,
            placeholder: "Personal suchen...",
          }}
          filters={[]}
          activeFilters={activeFilters}
          onClearAllFilters={() => {
            setSearchTerm("");
          }}
          actionButton={
            !isMobile && (
              <button
                onClick={() => setShowInviteModal(true)}
                className="group relative flex h-10 w-10 items-center justify-center rounded-full bg-gradient-to-br from-[#F78C10] to-[#e57a00] text-white shadow-lg transition-all duration-150 hover:scale-105 hover:shadow-xl active:scale-95"
                style={{
                  background:
                    "linear-gradient(135deg, rgb(247, 140, 16) 0%, rgb(229, 122, 0) 100%)",
                  willChange: "transform, opacity",
                  WebkitTransform: "translateZ(0)",
                  transform: "translateZ(0)",
                }}
                aria-label="Personal hinzufügen"
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
            )
          }
        />
      </div>

      {/* Mobile FAB Create Button */}
      <button
        onClick={() => setShowInviteModal(true)}
        className="group pointer-events-auto fixed right-4 bottom-24 z-40 flex h-14 w-14 translate-y-0 items-center justify-center rounded-full bg-gradient-to-br from-[#F78C10] to-[#e57a00] text-white opacity-100 shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all duration-300 ease-out hover:shadow-[0_8px_40px_rgb(247,140,16,0.3)] active:scale-95 md:hidden"
        style={{
          background:
            "linear-gradient(135deg, rgb(247, 140, 16) 0%, rgb(229, 122, 0) 100%)",
          willChange: "transform, opacity",
          WebkitTransform: "translateZ(0)",
          transform: "translateZ(0)",
        }}
        aria-label="Personal hinzufügen"
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

      {/* Pending Invitations (above staff list) */}
      <RoleGuard variant="adminOnly">
        <div className="mb-4">
          <PendingInvitationsList refreshKey={invitationRefreshKey} />
        </div>
      </RoleGuard>

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
                ? "Kein Personal gefunden"
                : "Kein Personal vorhanden"}
            </h3>
            <p className="mt-2 text-sm text-gray-600">
              {searchTerm
                ? "Versuchen Sie andere Suchkriterien."
                : "Es wurde noch kein Personal erstellt."}
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
            const hasAvatar = !!teacher.avatar;
            const avatarUrl =
              hasAvatar && teacher.staff_id
                ? `/api/staff/${teacher.staff_id}/avatar`
                : null;

            const handleClick = () => handleSelectTeacher(teacher);
            return (
              <div
                role="button"
                tabIndex={0}
                key={teacher.id}
                onClick={handleClick}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    handleClick();
                  }
                }}
                className="group relative w-full cursor-pointer overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 text-left shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-150 active:scale-[0.98] md:hover:-translate-y-0.5 md:hover:border-orange-300/50 md:hover:bg-white md:hover:shadow-[0_12px_40px_rgb(0,0,0,0.18)]"
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
                    <div className="relative flex h-12 w-12 items-center justify-center overflow-hidden rounded-full bg-gradient-to-br from-[#F78C10] to-[#e57a00] text-sm font-semibold text-white shadow-md transition-transform duration-150 md:group-hover:scale-105">
                      {avatarUrl ? (
                        <Image
                          src={avatarUrl}
                          alt={displayName}
                          fill
                          className="object-cover"
                          sizes="48px"
                          unoptimized
                        />
                      ) : (
                        initials.toUpperCase()
                      )}
                    </div>
                  </div>

                  {/* Teacher Info */}
                  <div className="min-w-0 flex-1">
                    <h3 className="text-lg font-semibold text-gray-900 transition-colors duration-300 md:group-hover:text-orange-600">
                      {displayName}
                    </h3>
                    {(teacher.account_role || teacher.role) && (
                      <p className="mt-0.5 text-sm text-gray-500">
                        {(() => {
                          const displayRole = teacher.account_role
                            ? getRoleDisplayName(teacher.account_role)
                            : null;
                          const position =
                            teacher.role &&
                            teacher.role.toLowerCase() !==
                              displayRole?.toLowerCase() &&
                            teacher.role.toLowerCase() !==
                              teacher.account_role?.toLowerCase()
                              ? teacher.role
                              : null;
                          return [displayRole, position]
                            .filter(Boolean)
                            .join(" · ");
                        })()}
                      </p>
                    )}
                    {teacher.email && (
                      <div className="mt-1 flex items-center gap-1.5">
                        <span className="truncate text-xs text-gray-400">
                          {teacher.email}
                        </span>
                        <button
                          type="button"
                          onClick={(e) => {
                            e.stopPropagation();
                            navigator.clipboard
                              .writeText(teacher.email!)
                              .then(() => toastSuccess("E-Mail kopiert"))
                              .catch(() =>
                                toastError("Kopieren fehlgeschlagen"),
                              );
                          }}
                          className="flex-shrink-0 rounded-lg p-2 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600"
                          aria-label="E-Mail kopieren"
                          title="E-Mail kopieren"
                        >
                          <svg
                            className="h-4 w-4"
                            fill="none"
                            viewBox="0 0 24 24"
                            stroke="currentColor"
                            strokeWidth={2}
                          >
                            <path
                              strokeLinecap="round"
                              strokeLinejoin="round"
                              d="M15.666 3.888A2.25 2.25 0 0013.5 2.25h-3c-1.03 0-1.9.693-2.166 1.638m7.332 0c.055.194.084.4.084.612v0a.75.75 0 01-.75.75H9.75a.75.75 0 01-.75-.75v0c0-.212.03-.418.084-.612m7.332 0c.646.049 1.288.11 1.927.184 1.1.128 1.907 1.077 1.907 2.185V19.5a2.25 2.25 0 01-2.25 2.25H6.75A2.25 2.25 0 014.5 19.5V6.257c0-1.108.806-2.057 1.907-2.185a48.208 48.208 0 011.927-.184"
                            />
                          </svg>
                        </button>
                        <a
                          href={`mailto:${teacher.email}`}
                          onClick={(e) => e.stopPropagation()}
                          className="flex-shrink-0 rounded-lg p-2 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600"
                          aria-label="E-Mail senden"
                          title="E-Mail senden"
                        >
                          <svg
                            className="h-4 w-4"
                            fill="none"
                            viewBox="0 0 24 24"
                            stroke="currentColor"
                            strokeWidth={2}
                          >
                            <path
                              strokeLinecap="round"
                              strokeLinejoin="round"
                              d="M21.75 6.75v10.5a2.25 2.25 0 01-2.25 2.25h-15a2.25 2.25 0 01-2.25-2.25V6.75m19.5 0A2.25 2.25 0 0019.5 4.5h-15a2.25 2.25 0 00-2.25 2.25m19.5 0v.243a2.25 2.25 0 01-1.07 1.916l-7.5 4.615a2.25 2.25 0 01-2.36 0L3.32 8.91a2.25 2.25 0 01-1.07-1.916V6.75"
                            />
                          </svg>
                        </a>
                      </div>
                    )}
                  </div>

                  {/* Arrow Icon */}
                  <div className="flex-shrink-0">
                    <svg
                      className="h-5 w-5 text-gray-300 transition-all duration-300 md:group-hover:translate-x-1 md:group-hover:text-orange-500"
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
                <div className="pointer-events-none absolute inset-0 rounded-3xl bg-gradient-to-r from-transparent via-orange-100/30 to-transparent opacity-0 transition-opacity duration-300 md:group-hover:opacity-100"></div>
              </div>
            );
          })}
        </div>
      )}

      {/* Invite Modal */}
      <Modal
        isOpen={showInviteModal}
        onClose={handleCloseInviteModal}
        title="Personal einladen"
      >
        <InvitationForm
          existingPositions={existingPositions}
          onCreated={() => {
            setInvitationRefreshKey(Date.now());
            setShowInviteModal(false);
          }}
        />
      </Modal>

      {/* Teacher Detail Modal */}
      {selectedTeacher && (
        <TeacherDetailModal
          isOpen={showDetailModal}
          onClose={handleCloseDetailModal}
          teacher={selectedTeacher}
          onEdit={handleEditClick}
          onDelete={handleDeleteTeacher}
          onManageCaregiver={
            selectedTeacher.account_id ? handleManageCaregiverClick : undefined
          }
          onUpdateNotes={handleUpdateNotes}
          loading={detailLoading}
          onDeleteClick={handleDeleteClick}
        />
      )}

      {/* Delete Confirmation Modal */}
      {selectedTeacher && (
        <ConfirmationModal
          isOpen={showDeleteConfirmModal}
          onClose={handleDeleteCancel}
          onConfirm={() => confirmDelete(() => void handleDeleteTeacher())}
          title="Personal löschen?"
          confirmText="Löschen"
          cancelText="Abbrechen"
          confirmButtonClass="bg-red-600 hover:bg-red-700"
        >
          <p className="text-sm text-gray-700">
            Möchten Sie das Personal{" "}
            <span className="font-medium">
              {selectedTeacher.first_name} {selectedTeacher.last_name}
            </span>{" "}
            wirklich löschen? Diese Aktion kann nicht rückgängig gemacht werden.
          </p>
        </ConfirmationModal>
      )}

      {/* Teacher Edit Modal */}
      {selectedTeacher && (
        <TeacherEditModal
          isOpen={showEditModal}
          onClose={handleCloseEditModal}
          teacher={selectedTeacher}
          onSave={handleEditTeacher}
          loading={detailLoading}
          existingPositions={existingPositions}
        />
      )}

      {/* Role Management Modal */}
      {selectedTeacher && (
        <CaregiverCapabilityModal
          isOpen={caregiverModalOpen}
          onClose={() => setCaregiverModalOpen(false)}
          scope="tenant"
          accountId={selectedTeacher.account_id?.toString() ?? ""}
          accountLabel={`${selectedTeacher.first_name} ${selectedTeacher.last_name}`}
          onUpdated={async () => {
            await tenantMutate("database-teachers-list");
          }}
        />
      )}

      {selectedTeacher && (
        <TeacherRoleManagementModal
          isOpen={roleModalOpen}
          onClose={() => {
            setRoleModalOpen(false);
            setSelectedTeacher(null);
          }}
          teacher={selectedTeacher}
          onUpdate={() => {
            tenantMutate("database-teachers-list").catch(() => {
              // Ignore revalidation errors
            });
          }}
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
          onUpdate={() => {
            tenantMutate("database-teachers-list").catch(() => {
              // Ignore revalidation errors
            });
          }}
        />
      )}

      {/* Success toasts handled globally */}
    </DatabasePageLayout>
  );
}
