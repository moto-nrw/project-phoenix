"use client";

import { Suspense, useCallback, useMemo, useState } from "react";
import { useSession } from "next-auth/react";
import Link from "next/link";
import { redirect, useSearchParams } from "next/navigation";
import { DatabaseCreateAction } from "~/components/database/database-create-action";
import { DatabaseEmptyState } from "~/components/database/database-empty-state";
import { DatabaseGroupingToggle } from "~/components/database/database-grouping-toggle";
import { DatabasePageLayout } from "~/components/database/database-page-layout";
import {
  useGroupedItems,
  type Grouper,
} from "~/components/database/use-grouped-items";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { MOTO_CONCEPTS } from "~/lib/moto-concepts";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import type { ActiveFilter } from "~/components/ui/page-header/types";
import { useToast } from "~/contexts/ToastContext";
import { useIsMobile } from "~/components/ui/hooks/useIsMobile";
import { CaregiverCapabilityModal } from "@/components/teachers/caregiver-capability-modal";
import { RoleManagementModal } from "@/components/teachers/role-management-modal";
import { StaffMasterDetail } from "@/components/teachers/staff-master-detail";
import { TeacherEditModal } from "@/components/teachers/teacher-edit-modal";
import { MFAAdminOverrideModal } from "~/components/auth/mfa-admin-override-modal";
import { InvitationForm } from "~/components/admin/invitation-form";
import { PendingInvitationsList } from "~/components/admin/pending-invitations-list";
import { RoleGuard } from "~/components/auth/role-guard";
import { hasPermission } from "~/lib/auth-utils";
import { getDbOperationMessage } from "@/lib/use-notification";
import { getRoleDisplayName } from "@/lib/auth-helpers";
import { createCrudService } from "@/lib/database/service-factory";
import { teachersConfig } from "@/components/database/configs/teachers.config";
import type { Teacher } from "@/lib/teacher-api";
import { Modal, ConfirmationModal } from "~/components/ui/modal";
import { useDeleteConfirmation } from "~/hooks/useDeleteConfirmation";
import { useUpdateUrlParams } from "~/hooks/useUpdateUrlParams";
import { createLogger } from "~/lib/logger";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";

const logger = createLogger({ component: "DatabaseTeachersPage" });

type StaffGroupingMode = "none" | "role";

const STAFF_GROUPING_DEFAULT: StaffGroupingMode = "role";

const STAFF_GROUPING_OPTIONS: { value: StaffGroupingMode; label: string }[] = [
  { value: "role", label: "Rolle" },
  { value: "none", label: "Keine" },
];

function parseStaffGrouping(value: string | null): StaffGroupingMode {
  if (value === "none") return value;
  return STAFF_GROUPING_DEFAULT;
}

// Search-match helper extracted so the page-level useMemo stays under
// the cognitive-complexity cap. Checks all teacher-display fields
// against a lowercased needle.
function matchesTeacherSearch(teacher: Teacher, searchLower: string): boolean {
  const haystacks = [
    teacher.first_name,
    teacher.last_name,
    teacher.name,
    teacher.role,
    teacher.account_role,
    teacher.specialization,
    teacher.email,
  ];
  return haystacks.some((h) => h?.toLowerCase().includes(searchLower) ?? false);
}

export default function TeachersPage() {
  return (
    <Suspense fallback={null}>
      <TeachersPageContent />
    </Suspense>
  );
}

function TeachersPageContent() {
  const searchParams = useSearchParams();
  const updateUrlParams = useUpdateUrlParams();

  const selectedId = searchParams.get("staff");
  const grouping = parseStaffGrouping(searchParams.get("groupBy"));
  const [searchTerm, setSearchTerm] = useState("");
  const isMobile = useIsMobile();

  const [showInviteModal, setShowInviteModal] = useState(false);
  const [invitationRefreshKey, setInvitationRefreshKey] = useState<number>(
    Date.now(),
  );

  const [showEditModal, setShowEditModal] = useState(false);
  const [caregiverModalOpen, setCaregiverModalOpen] = useState(false);
  const [mfaModalOpen, setMfaModalOpen] = useState(false);
  const [roleModalOpen, setRoleModalOpen] = useState(false);
  const [savingTeacher, setSavingTeacher] = useState(false);

  const {
    showConfirmModal: showDeleteConfirmModal,
    handleDeleteClick,
    handleDeleteCancel,
    confirmDelete,
  } = useDeleteConfirmation();

  const { success: toastSuccess, error: toastError } = useToast();

  const { data: sessionData, status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });
  const accessToken = sessionData?.user?.token ?? "";
  const canManageUsers = hasPermission(sessionData, "users:manage");
  // Personalnotizen am Mitarbeiter-Datensatz: staff:manage (#2906), nicht
  // mehr users:update — das hält jede Betreuungskraft für die Kinderdaten.
  const canManageStaffRecords = hasPermission(sessionData, "staff:manage");

  const service = useMemo(() => createCrudService(teachersConfig), []);
  const tenantMutate = useTenantMutate();

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

  const existingPositions = useMemo(() => {
    const teachers = teachersData ?? [];
    const positions = new Set<string>();
    for (const t of teachers) {
      if (t.role?.trim()) positions.add(t.role.trim());
    }
    return [...positions].sort((a, b) => a.localeCompare(b, "de"));
  }, [teachersData]);

  const filteredTeachers = useMemo(() => {
    const teachers = teachersData ?? [];
    let filtered = [...teachers];

    if (searchTerm) {
      const searchLower = searchTerm.toLowerCase();
      filtered = filtered.filter((teacher) =>
        matchesTeacherSearch(teacher, searchLower),
      );
    }

    filtered.sort((a, b) => {
      const nameA = a.name ?? `${a.first_name} ${a.last_name}`;
      const nameB = b.name ?? `${b.first_name} ${b.last_name}`;
      return nameA.localeCompare(nameB, "de");
    });

    return filtered;
  }, [teachersData, searchTerm]);

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

  const selectedTeacher = useMemo(() => {
    if (!selectedId) return null;
    return (
      (teachersData ?? []).find((teacher) => teacher.id === selectedId) ?? null
    );
  }, [teachersData, selectedId]);

  const handleSelectTeacher = useCallback(
    (id: string | null) => {
      updateUrlParams({ staff: id });
    },
    [updateUrlParams],
  );

  const handleGroupingChange = useCallback(
    (next: StaffGroupingMode) => {
      updateUrlParams({
        groupBy: next === STAFF_GROUPING_DEFAULT ? null : next,
      });
    },
    [updateUrlParams],
  );

  const groupers = useMemo<
    Partial<Record<StaffGroupingMode, Grouper<Teacher>>>
  >(
    () => ({
      role: (teacher) => {
        const role = teacher.account_role?.trim();
        if (!role) {
          return { id: "__no_role__", title: "Ohne Rolle", sortKey: "zzz" };
        }
        return { id: role, title: getRoleDisplayName(role) };
      },
    }),
    [],
  );

  const groupDefinitions = useGroupedItems(
    filteredTeachers,
    grouping,
    groupers,
    "Personal",
  );

  const handleCloseInviteModal = useCallback(
    () => setShowInviteModal(false),
    [],
  );
  const handleCloseEditModal = useCallback(() => setShowEditModal(false), []);
  const handleEditClick = useCallback(() => setShowEditModal(true), []);
  const handleManageCaregiverClick = useCallback(
    () => setCaregiverModalOpen(true),
    [],
  );
  const handleManageMFAClick = useCallback(() => setMfaModalOpen(true), []);
  const handleManageRoleClick = useCallback(() => setRoleModalOpen(true), []);

  const handleEditTeacher = useCallback(
    async (data: Partial<Teacher> & { password?: string }) => {
      if (!selectedTeacher) return;
      try {
        setSavingTeacher(true);
        await service.update(selectedTeacher.id, data);
        setShowEditModal(false);
        toastSuccess(
          getDbOperationMessage("update", teachersConfig.name.singular),
        );
        await tenantMutate("database-teachers-list");
      } catch (err) {
        logger.error("failed to update teacher", {
          teacher_id: selectedTeacher.id,
          error: err instanceof Error ? err.message : String(err),
        });
        throw err;
      } finally {
        setSavingTeacher(false);
      }
    },
    [selectedTeacher, service, tenantMutate, toastSuccess],
  );

  const handleDeleteTeacher = useCallback(async () => {
    if (!selectedTeacher) return;
    const deleteError = await service.delete(selectedTeacher.id);
    if (deleteError) {
      toastError(deleteError);
      return;
    }
    toastSuccess(getDbOperationMessage("delete", teachersConfig.name.singular));
    handleSelectTeacher(null);
    await tenantMutate("database-teachers-list");
  }, [
    selectedTeacher,
    service,
    toastError,
    toastSuccess,
    handleSelectTeacher,
    tenantMutate,
  ]);

  const handleUpdateNotes = useCallback(
    async (notes: string) => {
      if (!selectedTeacher) return;
      try {
        await service.update(selectedTeacher.id, { staff_notes: notes });
        await tenantMutate("database-teachers-list");
      } catch (err) {
        logger.error("failed to update teacher notes", {
          teacher_id: selectedTeacher.id,
          error: err instanceof Error ? err.message : String(err),
        });
        throw err;
      }
    },
    [selectedTeacher, service, tenantMutate],
  );

  const canShowDetail = !loading && filteredTeachers.length > 0;

  return (
    <DatabasePageLayout
      loading={loading}
      sessionLoading={status === "loading"}
      className="-mt-1.5 flex w-full flex-col"
    >
      <div className="mb-4">
        <PageHeaderWithSearch
          title={isMobile ? "Personal" : ""}
          badge={{
            icon: (
              <MotoDuotoneIcon
                icon={MOTO_CONCEPTS.staff.icon}
                tone={MOTO_CONCEPTS.staff.tone}
                size={20}
              />
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
            <div className="flex items-center gap-2">
              {!isMobile ? (
                <>
                  <DatabaseGroupingToggle
                    value={grouping}
                    options={STAFF_GROUPING_OPTIONS}
                    onChange={handleGroupingChange}
                  />
                  <Link
                    href="/database/personal/import"
                    className="flex h-10 items-center gap-2 rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 hover:bg-gray-50"
                  >
                    Importieren
                  </Link>
                </>
              ) : null}
              {/* Zweiter Import-Weg (#2132): eigener Flow mit Stichtag und
                  Begründung, deshalb im Menü statt als weiterer Button. */}
              <OverflowMenu
                ariaLabel="Weitere Import-Aktionen"
                items={[
                  {
                    label: "Eröffnungssalden importieren",
                    href: "/database/personal/opening-balances",
                    onClick: () => undefined,
                  },
                ]}
              />
              {canManageUsers ? (
                <DatabaseCreateAction
                  label="Personal"
                  ariaLabel="Personal hinzufügen"
                  onClick={() => setShowInviteModal(true)}
                />
              ) : null}
            </div>
          }
        />
      </div>

      <RoleGuard variant="adminOnly">
        <div className="mb-4">
          <PendingInvitationsList refreshKey={invitationRefreshKey} />
        </div>
      </RoleGuard>

      {error && (
        <div className="mb-6 rounded-lg border border-red-200 bg-red-50 p-4">
          <p className="text-sm text-red-800">{error}</p>
        </div>
      )}

      {canShowDetail ? (
        <div className="min-h-0 flex-1 pb-4">
          <StaffMasterDetail
            groupDefinitions={groupDefinitions}
            selectedId={selectedId}
            selectedTeacher={selectedTeacher}
            onSelect={handleSelectTeacher}
            onEditClick={canManageStaffRecords ? handleEditClick : undefined}
            onDeleteClick={handleDeleteClick}
            onUpdateNotes={
              canManageStaffRecords ? handleUpdateNotes : undefined
            }
            onManageCaregiver={
              selectedTeacher?.account_id
                ? handleManageCaregiverClick
                : undefined
            }
            onManageMFA={
              selectedTeacher?.account_id ? handleManageMFAClick : undefined
            }
            onManageRole={
              selectedTeacher?.account_id ? handleManageRoleClick : undefined
            }
          />
        </div>
      ) : !loading ? (
        <DatabaseEmptyState
          icon={
            <MotoDuotoneIcon
              icon={MOTO_CONCEPTS.staff.icon}
              tone={MOTO_CONCEPTS.staff.tone}
              size={48}
              className="mx-auto"
            />
          }
          title={
            searchTerm ? "Kein Personal gefunden" : "Kein Personal vorhanden"
          }
          description={
            searchTerm
              ? "Versuchen Sie andere Suchkriterien."
              : "Es wurde noch kein Personal erstellt."
          }
        />
      ) : null}

      {canManageUsers ? (
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
      ) : null}

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
            wirklich löschen? Der Zugang wird deaktiviert und die Person aus
            allen Listen entfernt. Vorhandene Einträge wie Anwesenheiten und
            Zeiterfassung bleiben für die Historie erhalten. Die Person kann
            jederzeit erneut eingeladen werden.
          </p>
        </ConfirmationModal>
      )}

      {selectedTeacher && (
        <TeacherEditModal
          isOpen={showEditModal}
          onClose={handleCloseEditModal}
          teacher={selectedTeacher}
          onSave={handleEditTeacher}
          loading={savingTeacher}
          existingPositions={existingPositions}
        />
      )}

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

      {selectedTeacher?.account_id && accessToken && (
        <MFAAdminOverrideModal
          isOpen={mfaModalOpen}
          onClose={() => setMfaModalOpen(false)}
          bearerToken={accessToken}
          accountId={selectedTeacher.account_id.toString()}
          accountLabel={`${selectedTeacher.first_name} ${selectedTeacher.last_name}`}
        />
      )}

      {selectedTeacher?.account_id && (
        <RoleManagementModal
          isOpen={roleModalOpen}
          onClose={() => setRoleModalOpen(false)}
          accountId={selectedTeacher.account_id.toString()}
          accountLabel={`${selectedTeacher.first_name} ${selectedTeacher.last_name}`}
          onUpdated={async () => {
            await tenantMutate("database-teachers-list");
          }}
        />
      )}
    </DatabasePageLayout>
  );
}
