"use client";

import { Suspense, useCallback, useMemo, useState } from "react";
import { useSession } from "next-auth/react";
import { redirect, useSearchParams } from "next/navigation";
import { DatabaseCreateAction } from "~/components/database/database-create-action";
import { DatabasePageLayout } from "~/components/database/database-page-layout";
import { Skeleton } from "~/components/ui/skeleton";
import { formatCount } from "~/lib/format-utils";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { MOTO_CONCEPTS } from "~/lib/moto-concepts";
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
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { useToast } from "~/contexts/ToastContext";
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

  // Statuszeile des Seitenkopfs aus der bereits geladenen Gruppenliste.
  const statusLine = useMemo(() => {
    const groups = groupsData ?? [];
    return `${formatCount(groups.length)} ${groups.length === 1 ? "Gruppe" : "Gruppen"}`;
  }, [groupsData]);

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
      error={error}
      empty={
        filteredGroups.length === 0 && selectedGroup === null
          ? {
              title:
                searchTerm || roomFilter !== "all"
                  ? "Keine Gruppen gefunden"
                  : "Keine Gruppen vorhanden",
              description:
                searchTerm || roomFilter !== "all"
                  ? "Versuchen Sie andere Suchkriterien oder Filter."
                  : "Legen Sie die erste Gruppe an, um Kinder zuzuordnen.",
              icon: (
                <MotoDuotoneIcon
                  icon={MOTO_CONCEPTS.groups.icon}
                  tone={MOTO_CONCEPTS.groups.tone}
                  size={48}
                />
              ),
              action:
                searchTerm || roomFilter !== "all" ? undefined : (
                  <DatabaseCreateAction
                    label="Gruppe"
                    ariaLabel="Gruppe erstellen"
                    onClick={() => setShowCreateModal(true)}
                  />
                ),
            }
          : null
      }
      overlays={
        <>
          <DatabaseFormModal<Group>
            isOpen={showCreateModal}
            onClose={() => setShowCreateModal(false)}
            mode="create"
            config={groupsConfig}
            onSubmit={handleCreateGroup}
          />

          {selectedGroup && (
            <ConfirmDeleteModal
              isOpen={showDeleteConfirmModal}
              onClose={handleDeleteCancel}
              onConfirm={() => confirmDelete(() => void handleDeleteGroup())}
              title="Gruppe löschen?"
              description={
                <>
                  Möchten Sie die Gruppe{" "}
                  <span className="font-medium">{selectedGroup.name}</span>{" "}
                  wirklich löschen? Diese Aktion kann nicht rückgängig gemacht
                  werden.
                </>
              }
              gate={{ mode: "twoStep" }}
              loading={false}
              error=""
            />
          )}
        </>
      }
      className="flex w-full flex-col"
      intro={{
        title: "Gruppen",
        description: loading ? <Skeleton className="h-4 w-40" /> : statusLine,
        actions: (
          <DatabaseCreateAction
            label="Gruppe"
            ariaLabel="Gruppe erstellen"
            onClick={() => setShowCreateModal(true)}
          />
        ),
      }}
      search={
        <PageHeaderWithSearch
          embedded
          title=""
          badge={{
            icon: (
              <MotoDuotoneIcon
                icon={MOTO_CONCEPTS.groups.icon}
                tone={MOTO_CONCEPTS.groups.tone}
                size={20}
              />
            ),
            count: filteredGroups.length,
            label: "Gruppen",
          }}
          search={{
            value: searchTerm,
            onChange: setSearchTerm,
            placeholder: "Gruppen suchen…",
          }}
          filters={filters}
          activeFilters={activeFilters}
          onClearAllFilters={() => {
            setSearchTerm("");
            setRoomFilter("all");
          }}
        />
      }
    >
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
      ) : null}
    </DatabasePageLayout>
  );
}
