"use client";

import {
  Suspense,
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useSession } from "next-auth/react";
import { redirect, useSearchParams } from "next/navigation";
import { DatabaseCreateAction } from "~/components/database/database-create-action";
import { DatabasePageLayout } from "~/components/database/database-page-layout";
import { formatCount } from "~/lib/format-utils";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { MOTO_CONCEPTS } from "~/lib/moto-concepts";
import type {
  ActiveFilter,
  FilterConfig,
} from "~/components/ui/page-header/types";
import { createCrudService } from "@/lib/database/service-factory";
import { groupsConfig } from "@/components/database/configs/groups.config";
import type { Group } from "@/lib/group-helpers";
import { DatabaseFormModal } from "~/components/ui/database/database-form-modal";
import { GroupsMasterDetail } from "@/components/groups/groups-master-detail";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { useDeleteConfirmation } from "~/hooks/useDeleteConfirmation";
import { useUpdateUrlParams } from "~/hooks/useUpdateUrlParams";
import { createLogger } from "~/lib/logger";
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
  } = useDeleteConfirmation();

  const { success: toastSuccess } = useToast();
  // Schreibfehler bleiben im jeweiligen Dialog (#2517).
  const createErrors = useApiFormError();
  const deleteErrors = useApiFormError();
  const [deletePending, setDeletePending] = useState(false);
  const latestDeleteRef = useRef<() => void>(() => undefined);
  const { clear: clearCreateErrors } = createErrors;
  // Ein neu geöffnetes Anlegen beginnt ohne den Fehler des vorigen Versuchs.
  useEffect(() => {
    if (showCreateModal) clearCreateErrors();
  }, [showCreateModal, clearCreateErrors]);

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
    isLoading: swrLoading,
    error: groupsError,
    mutate: mutateGroups,
  } = useSWRAuth("database-groups-list", async () => {
    const data = await service.getList({ page: 1, pageSize: 500 });
    return Array.isArray(data.data) ? data.data : [];
  });

  const error = useSwrLoadError(groupsError, "die Liste der Gruppen", () =>
    mutateGroups(),
  );
  // Bis der Katalogtext des Ladefehlers da ist, bleibt das Skelett stehen.
  const loading = swrLoading || (Boolean(groupsError) && error === null);

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
      // Ein Fehler bleibt im Dialog: DatabaseForm zeigt ihn über errorPath.
      const payload = groupsConfig.form.transformBeforeSubmit
        ? groupsConfig.form.transformBeforeSubmit(data)
        : data;
      const created = await service.create(payload);
      toastSuccess(`Die Gruppe „${created.name}“ ist angelegt.`);
      setShowCreateModal(false);
      await tenantMutate("database-groups-list");
    },
    [service, tenantMutate, toastSuccess],
  );

  const handleUpdateGroup = useCallback(
    async (data: Partial<Group>) => {
      if (!selectedGroup) return;
      // Ein Fehler bleibt im Formular (errorPath im Stammdaten-Reiter).
      const payload = groupsConfig.form.transformBeforeSubmit
        ? groupsConfig.form.transformBeforeSubmit(data)
        : data;
      await service.update(selectedGroup.id, payload);
      toastSuccess(
        `Die Gruppe „${data.name ?? selectedGroup.name}“ ist gespeichert.`,
      );
      await tenantMutate("database-groups-list");
    },
    [selectedGroup, service, tenantMutate, toastSuccess],
  );

  const handleDeleteGroup = useCallback(async () => {
    if (!selectedGroup) return;
    setDeletePending(true);
    deleteErrors.clear();
    try {
      const deleted = await service.remove(selectedGroup.id);
      if (!deleted) return;
      toastSuccess(`Die Gruppe „${selectedGroup.name}“ ist gelöscht.`);
      handleDeleteCancel();
      handleSelectGroup(null);
      await tenantMutate("database-groups-list");
    } catch (err) {
      logger.warn("group_delete_failed", {
        group_id: selectedGroup.id,
        error: err instanceof Error ? err.message : String(err),
      });
      // Der Bestätigungsdialog bleibt offen und nennt den Grund.
      void deleteErrors.show(err, {
        object: "das Löschen der Gruppe",
        retry: () => latestDeleteRef.current(),
      });
    } finally {
      setDeletePending(false);
    }
  }, [
    selectedGroup,
    service,
    toastSuccess,
    handleDeleteCancel,
    handleSelectGroup,
    tenantMutate,
    deleteErrors,
  ]);

  useLayoutEffect(() => {
    latestDeleteRef.current = () => void handleDeleteGroup();
  });

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
                    showMobileFab={false}
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
            errorPath={createErrors}
            errorObject="die Gruppe"
          />

          {selectedGroup && (
            <ConfirmDeleteModal
              isOpen={showDeleteConfirmModal}
              onClose={() => {
                deleteErrors.clear();
                handleDeleteCancel();
              }}
              onConfirm={() => void handleDeleteGroup()}
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
              loading={deletePending}
              error={deleteErrors.error}
            />
          )}
        </>
      }
      className="flex w-full flex-col"
      intro={{
        title: "Gruppen",
        description: statusLine,
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
