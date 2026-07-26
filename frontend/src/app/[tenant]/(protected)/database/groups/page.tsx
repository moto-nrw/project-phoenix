"use client";

import { Suspense, useCallback, useMemo, useState } from "react";
import { useSession } from "next-auth/react";
import { redirect, useSearchParams } from "next/navigation";
import { DatabaseCreateAction } from "~/components/database/database-create-action";
import { DatabaseEmptyState } from "~/components/database/database-empty-state";
import { DatabasePageLayout } from "~/components/database/database-page-layout";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import type {
  ActiveFilter,
  FilterConfig,
} from "~/components/ui/page-header/types";
import { getDbOperationMessage } from "@/lib/use-notification";
import { createCrudService } from "@/lib/database/service-factory";
import { groupsConfig } from "@/components/database/configs/groups.config";
import type { Group } from "@/lib/group-helpers";
import { DatabaseFormModal } from "~/components/ui/database/database-form-modal";
import { GroupsMasterDetail } from "@/components/groups/groups-master-detail";
import { ConfirmationModal } from "~/components/ui/modal";
import { useToast } from "~/contexts/ToastContext";
import { useIsMobile } from "~/components/ui/hooks/useIsMobile";
import { useDeleteConfirmation } from "~/hooks/useDeleteConfirmation";
import { useUpdateUrlParams } from "~/hooks/useUpdateUrlParams";
import { createLogger } from "~/lib/logger";
import { trackEvent } from "~/lib/analytics";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";

const logger = createLogger({ component: "DatabaseGroupsPage" });

export default function GroupsPage() {
  return (
    <Suspense fallback={null}>
      <GroupsPageContent />
    </Suspense>
  );
}

function GroupsPageContent() {
  const searchParams = useSearchParams();
  const updateUrlParams = useUpdateUrlParams();

  const selectedId = searchParams.get("group");
  const [searchTerm, setSearchTerm] = useState("");
  const [roomFilter, setRoomFilter] = useState<string>("all");
  const isMobile = useIsMobile();

  const [showCreateModal, setShowCreateModal] = useState(false);

  const {
    showConfirmModal: showDeleteConfirmModal,
    handleDeleteClick,
    handleDeleteCancel,
    confirmDelete,
  } = useDeleteConfirmation();

  const { success: toastSuccess, error: toastError } = useToast();

  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  const service = useMemo(() => createCrudService(groupsConfig), []);
  const tenantMutate = useTenantMutate();

  const {
    data: groupsData,
    isLoading: loading,
    error: groupsError,
  } = useSWRAuth("database-groups-list", async () => {
    const data = await service.getList({ page: 1, pageSize: 500 });
    return Array.isArray(data.data) ? data.data : [];
  });

  const error = groupsError
    ? "Fehler beim Laden der Gruppen. Bitte versuchen Sie es später erneut."
    : null;

  const uniqueRooms = useMemo(() => {
    const groups = groupsData ?? [];
    const set = new Set<string>();
    groups.forEach((g) => {
      if (g.room_name) set.add(g.room_name);
    });
    return Array.from(set)
      .sort((a, b) => a.localeCompare(b, "de"))
      .map((r) => ({ value: r, label: r }));
  }, [groupsData]);

  const filters: FilterConfig[] = useMemo(
    () => [
      {
        id: "room",
        label: "Raum",
        type: "dropdown",
        value: roomFilter,
        onChange: (v) => setRoomFilter(v as string),
        options: [{ value: "all", label: "Alle Räume" }, ...uniqueRooms],
      },
    ],
    [roomFilter, uniqueRooms],
  );

  const activeFilters: ActiveFilter[] = useMemo(() => {
    const list: ActiveFilter[] = [];
    if (searchTerm)
      list.push({
        id: "search",
        label: `"${searchTerm}"`,
        onRemove: () => setSearchTerm(""),
      });
    if (roomFilter !== "all")
      list.push({
        id: "room",
        label: roomFilter,
        onRemove: () => setRoomFilter("all"),
      });
    return list;
  }, [searchTerm, roomFilter]);

  const filteredGroups = useMemo(() => {
    const groups = groupsData ?? [];
    let arr = [...groups];
    if (searchTerm) {
      const q = searchTerm.toLowerCase();
      arr = arr.filter(
        (g) =>
          g.name.toLowerCase().includes(q) ||
          (g.room_name?.toLowerCase().includes(q) ?? false) ||
          (g.representative_name?.toLowerCase().includes(q) ?? false),
      );
    }
    if (roomFilter !== "all") {
      arr = arr.filter((g) => g.room_name === roomFilter);
    }
    arr.sort((a, b) => a.name.localeCompare(b.name, "de"));
    return arr;
  }, [groupsData, searchTerm, roomFilter]);

  // Resolve against the unfiltered list so the detail panel survives a search
  // narrowing the visible rows.
  const selectedGroup = useMemo(
    () =>
      selectedId
        ? ((groupsData ?? []).find((group) => group.id === selectedId) ?? null)
        : null,
    [groupsData, selectedId],
  );

  const handleSelectGroup = useCallback(
    (id: string | null) => {
      updateUrlParams({ group: id });
    },
    [updateUrlParams],
  );

  const handleCreateGroup = useCallback(
    async (data: Partial<Group>) => {
      try {
        const payload = groupsConfig.form.transformBeforeSubmit
          ? groupsConfig.form.transformBeforeSubmit(data)
          : data;
        const created = await service.create(payload);
        trackEvent("group_created");
        toastSuccess(
          getDbOperationMessage(
            "create",
            groupsConfig.name.singular,
            created.name,
          ),
        );
        setShowCreateModal(false);
        await tenantMutate("database-groups-list");
      } catch (createError) {
        logger.error("failed to create group", {
          error:
            createError instanceof Error
              ? createError.message
              : String(createError),
        });
        throw createError;
      }
    },
    [service, tenantMutate, toastSuccess],
  );

  const handleUpdateGroup = useCallback(
    async (data: Partial<Group>) => {
      if (!selectedGroup) return;
      try {
        const payload = groupsConfig.form.transformBeforeSubmit
          ? groupsConfig.form.transformBeforeSubmit(data)
          : data;
        await service.update(selectedGroup.id, payload);
        trackEvent("group_updated");
        toastSuccess(
          getDbOperationMessage(
            "update",
            groupsConfig.name.singular,
            selectedGroup.name,
          ),
        );
        await tenantMutate("database-groups-list");
      } catch (updateError) {
        logger.error("failed to update group", {
          group_id: selectedGroup.id,
          error:
            updateError instanceof Error
              ? updateError.message
              : String(updateError),
        });
        throw updateError;
      }
    },
    [selectedGroup, service, tenantMutate, toastSuccess],
  );

  const handleDeleteGroup = useCallback(async () => {
    if (!selectedGroup) return;
    const deleteError = await service.delete(selectedGroup.id);
    if (deleteError) {
      toastError(deleteError);
      return;
    }
    toastSuccess(
      getDbOperationMessage(
        "delete",
        groupsConfig.name.singular,
        selectedGroup.name,
      ),
    );
    handleSelectGroup(null);
    await tenantMutate("database-groups-list");
  }, [
    selectedGroup,
    service,
    toastError,
    toastSuccess,
    handleSelectGroup,
    tenantMutate,
  ]);

  const canShowDetail =
    !loading && (filteredGroups.length > 0 || selectedGroup !== null);

  return (
    <DatabasePageLayout
      loading={loading}
      sessionLoading={status === "loading"}
      className="-mt-1.5 flex w-full flex-col"
    >
      <div className="mb-4">
        <PageHeaderWithSearch
          title={isMobile ? "Gruppen" : ""}
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
                  d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0z"
                />
              </svg>
            ),
            count: filteredGroups.length,
            label: "Gruppen",
          }}
          search={{
            value: searchTerm,
            onChange: setSearchTerm,
            placeholder: "Gruppen suchen...",
          }}
          filters={filters}
          activeFilters={activeFilters}
          onClearAllFilters={() => {
            setSearchTerm("");
            setRoomFilter("all");
          }}
          actionButton={
            <DatabaseCreateAction
              label="Gruppe"
              ariaLabel="Gruppe erstellen"
              onClick={() => setShowCreateModal(true)}
            />
          }
        />
      </div>

      {error && (
        <div className="mb-6 rounded-lg border border-red-200 bg-red-50 p-4">
          <p className="text-sm text-red-800">{error}</p>
        </div>
      )}

      {canShowDetail ? (
        <div className="min-h-0 flex-1 pb-4">
          <GroupsMasterDetail
            groups={filteredGroups}
            selectedId={selectedId}
            selectedGroup={selectedGroup}
            onSelect={handleSelectGroup}
            onSaveGroup={handleUpdateGroup}
            onDeleteClick={handleDeleteClick}
          />
        </div>
      ) : !loading ? (
        <DatabaseEmptyState
          icon={
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
                d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0z"
              />
            </svg>
          }
          title={
            searchTerm || roomFilter !== "all"
              ? "Keine Gruppen gefunden"
              : "Keine Gruppen vorhanden"
          }
          description={
            searchTerm || roomFilter !== "all"
              ? "Versuchen Sie andere Suchkriterien oder Filter."
              : "Es wurden noch keine Gruppen erstellt."
          }
        />
      ) : null}

      <DatabaseFormModal<Group>
        isOpen={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        mode="create"
        config={groupsConfig}
        onSubmit={handleCreateGroup}
      />

      {selectedGroup && (
        <ConfirmationModal
          isOpen={showDeleteConfirmModal}
          onClose={handleDeleteCancel}
          onConfirm={() => confirmDelete(() => void handleDeleteGroup())}
          title="Gruppe löschen?"
          confirmText="Löschen"
          cancelText="Abbrechen"
          confirmButtonClass="bg-red-600 hover:bg-red-700"
        >
          <p className="text-sm text-gray-700">
            Möchten Sie die Gruppe{" "}
            <span className="font-medium">{selectedGroup.name}</span> wirklich
            löschen? Diese Aktion kann nicht rückgängig gemacht werden.
          </p>
        </ConfirmationModal>
      )}
    </DatabasePageLayout>
  );
}
