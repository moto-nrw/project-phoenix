"use client";

import { Suspense, useCallback, useMemo, useState } from "react";
import { useSession } from "next-auth/react";
import { redirect, useSearchParams } from "next/navigation";
import { DatabaseCreateAction } from "~/components/database/database-create-action";
import { DatabaseEmptyState } from "~/components/database/database-empty-state";
import { DatabaseGroupingToggle } from "~/components/database/database-grouping-toggle";
import { DatabasePageLayout } from "~/components/database/database-page-layout";
import { Skeleton } from "~/components/ui/skeleton";
import { formatCount } from "~/lib/format-utils";
import {
  useGroupedItems,
  type Grouper,
} from "~/components/database/use-grouped-items";
import { Alert } from "~/components/ui/alert";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { MOTO_CONCEPTS } from "~/lib/moto-concepts";
import type {
  FilterConfig,
  ActiveFilter,
} from "~/components/ui/page-header/types";
import { getDbOperationMessage } from "@/lib/use-notification";
import { createCrudService } from "@/lib/database/service-factory";
import { roomsConfig } from "@/components/database/configs/rooms.config";
import { formatFloor, type Room } from "@/lib/room-helpers";
import { DatabaseFormModal } from "~/components/ui/database/database-form-modal";
import { RoomsMasterDetail } from "@/components/rooms/rooms-master-detail";
import { ConfirmationModal } from "~/components/ui/modal";
import { useToast } from "~/contexts/ToastContext";
import { useIsMobile } from "~/components/ui/hooks/useIsMobile";
import { useDeleteConfirmation } from "~/hooks/useDeleteConfirmation";
import { useUpdateUrlParams } from "~/hooks/useUpdateUrlParams";
import { createLogger } from "~/lib/logger";
import {
  useSWRAuth,
  useTenantMutate,
  useTenantMutateMatching,
} from "~/lib/swr";
import {
  DATABASE_ROOMS_LIST_CACHE_KEY,
  ROOM_DERIVED_CACHE_KEY_FRAGMENTS,
  ROOM_LIST_CACHE_KEYS,
} from "~/lib/swr/room-derived-caches";

const logger = createLogger({ component: "DatabaseRoomsPage" });

type RoomsGroupingMode = "none" | "building" | "floor";

const ROOMS_GROUPING_DEFAULT: RoomsGroupingMode = "building";

const ROOMS_GROUPING_OPTIONS: { value: RoomsGroupingMode; label: string }[] = [
  { value: "building", label: "Gebäude" },
  { value: "floor", label: "Etage" },
  { value: "none", label: "Keine" },
];

function parseRoomsGrouping(value: string | null): RoomsGroupingMode {
  if (value === "floor" || value === "none") return value;
  return ROOMS_GROUPING_DEFAULT;
}

export default function RoomsPage() {
  return (
    <Suspense fallback={null}>
      <RoomsPageContent />
    </Suspense>
  );
}

function RoomsPageContent() {
  const searchParams = useSearchParams();
  const updateUrlParams = useUpdateUrlParams();

  const selectedId = searchParams.get("room");
  const grouping = parseRoomsGrouping(searchParams.get("groupBy"));
  const [searchTerm, setSearchTerm] = useState("");
  const [categoryFilter, setCategoryFilter] = useState<string>("all");
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

  const service = useMemo(() => createCrudService(roomsConfig), []);
  const tenantMutate = useTenantMutate();
  // Other pages stamp room data (colour, name) into their cached student/
  // visit rows, so a Room save has to invalidate them too — otherwise the
  // badge colours stay stale until the user navigates away and back. The
  // list of affected cache substrings lives in lib/swr/room-derived-caches.ts
  // as a single source of truth — keep it there, not inline here, so future
  // SWR consumers see the doc comment when they touch the file.
  const refreshRoomConsumers = useTenantMutateMatching(
    ROOM_DERIVED_CACHE_KEY_FRAGMENTS,
  );
  const refreshRoomLists = useCallback(
    () => Promise.all(ROOM_LIST_CACHE_KEYS.map((key) => tenantMutate(key))),
    [tenantMutate],
  );

  const {
    data: roomsData,
    isLoading: loading,
    error: roomsError,
  } = useSWRAuth(DATABASE_ROOMS_LIST_CACHE_KEY, async () => {
    const data = await service.getList({ page: 1, pageSize: 500 });
    return Array.isArray(data.data) ? data.data : [];
  });

  const error = roomsError
    ? "Fehler beim Laden der Räume. Bitte versuchen Sie es später erneut."
    : null;

  // Statuszeile des Seitenkopfs aus der bereits geladenen Raumliste.
  const statusLine = useMemo(() => {
    const rooms = roomsData ?? [];
    const occupied = rooms.filter((r) => r.isOccupied).length;
    return `${formatCount(rooms.length)} ${rooms.length === 1 ? "Raum" : "Räume"} · ${formatCount(occupied)} belegt`;
  }, [roomsData]);

  const uniqueCategories = useMemo(() => {
    const rooms = roomsData ?? [];
    const set = new Set<string>();
    rooms.forEach((r) => {
      if (r.category) set.add(r.category);
    });
    return Array.from(set)
      .sort((a, b) => a.localeCompare(b, "de"))
      .map((c) => ({ value: c, label: c }));
  }, [roomsData]);

  const filters: FilterConfig[] = useMemo(
    () => [
      {
        id: "category",
        label: "Kategorie",
        type: "dropdown",
        value: categoryFilter,
        onChange: (v) => setCategoryFilter(v as string),
        options: [
          { value: "all", label: "Alle Kategorien" },
          ...uniqueCategories,
        ],
      },
    ],
    [categoryFilter, uniqueCategories],
  );

  const activeFilters: ActiveFilter[] = useMemo(() => {
    const list: ActiveFilter[] = [];
    if (searchTerm)
      list.push({
        id: "search",
        label: `"${searchTerm}"`,
        onRemove: () => setSearchTerm(""),
      });
    if (categoryFilter !== "all")
      list.push({
        id: "category",
        label: categoryFilter,
        onRemove: () => setCategoryFilter("all"),
      });
    return list;
  }, [searchTerm, categoryFilter]);

  const filteredRooms = useMemo(() => {
    const rooms = roomsData ?? [];
    let arr = [...rooms];
    if (searchTerm) {
      const q = searchTerm.toLowerCase();
      arr = arr.filter(
        (r) =>
          r.name.toLowerCase().includes(q) ||
          (r.building?.toLowerCase().includes(q) ?? false) ||
          (r.category?.toLowerCase().includes(q) ?? false),
      );
    }
    if (categoryFilter !== "all") {
      arr = arr.filter((r) => r.category === categoryFilter);
    }
    arr.sort((a, b) => a.name.localeCompare(b.name, "de"));
    return arr;
  }, [roomsData, searchTerm, categoryFilter]);

  // Resolve against the unfiltered list so the detail panel survives a search
  // narrowing the visible rows.
  const selectedRoom = useMemo(
    () =>
      selectedId
        ? ((roomsData ?? []).find((room) => room.id === selectedId) ?? null)
        : null,
    [roomsData, selectedId],
  );

  const handleSelectRoom = useCallback(
    (id: string | null) => {
      updateUrlParams({ room: id });
    },
    [updateUrlParams],
  );

  const handleGroupingChange = useCallback(
    (next: RoomsGroupingMode) => {
      updateUrlParams({
        groupBy: next === ROOMS_GROUPING_DEFAULT ? null : next,
      });
    },
    [updateUrlParams],
  );

  const groupers = useMemo<Partial<Record<RoomsGroupingMode, Grouper<Room>>>>(
    () => ({
      building: (room) => {
        const id = room.building?.trim() || "__no_building__";
        const title = room.building?.trim() || "Ohne Gebäude";
        return { id, title };
      },
      floor: (room) => {
        if (room.floor === undefined || room.floor === null) {
          return { id: "__no_floor__", title: "Ohne Etage", sortKey: "zzz" };
        }
        // Offset to keep negative floors (basement) ordered before positives.
        return {
          id: `floor:${room.floor}`,
          title: formatFloor(room.floor),
          sortKey: String(room.floor + 1000).padStart(5, "0"),
        };
      },
    }),
    [],
  );

  const groupDefinitions = useGroupedItems(
    filteredRooms,
    grouping,
    groupers,
    "Räume",
  );

  const handleCreateRoom = useCallback(
    async (data: Partial<Room>) => {
      try {
        if (roomsConfig.form.transformBeforeSubmit) {
          data = roomsConfig.form.transformBeforeSubmit(data);
        }
        const created = await service.create(data);
        toastSuccess(
          getDbOperationMessage(
            "create",
            roomsConfig.name.singular,
            created.name,
          ),
        );
        setShowCreateModal(false);
        await refreshRoomLists();
      } catch (createError) {
        logger.error("failed to create room", {
          error:
            createError instanceof Error
              ? createError.message
              : String(createError),
        });
        throw createError;
      }
    },
    [service, refreshRoomLists, toastSuccess],
  );

  const handleUpdateRoom = useCallback(
    async (data: Partial<Room>) => {
      if (!selectedRoom) return;
      try {
        if (roomsConfig.form.transformBeforeSubmit) {
          data = roomsConfig.form.transformBeforeSubmit(data);
        }
        await service.update(selectedRoom.id, data);
        toastSuccess(
          getDbOperationMessage(
            "update",
            roomsConfig.name.singular,
            selectedRoom.name,
          ),
        );
        await Promise.all([
          refreshRoomLists(),
          // Refetch every consumer that holds room-stamped data so badges
          // pick up the new color without a manual reload.
          refreshRoomConsumers(),
        ]);
      } catch (updateError) {
        logger.error("failed to update room", {
          room_id: selectedRoom.id,
          error:
            updateError instanceof Error
              ? updateError.message
              : String(updateError),
        });
        throw updateError;
      }
    },
    [
      selectedRoom,
      service,
      refreshRoomLists,
      refreshRoomConsumers,
      toastSuccess,
    ],
  );

  const handleDeleteRoom = useCallback(async () => {
    if (!selectedRoom) return;
    const deleteError = await service.delete(selectedRoom.id);
    if (deleteError) {
      toastError(deleteError);
      return;
    }
    toastSuccess(
      getDbOperationMessage(
        "delete",
        roomsConfig.name.singular,
        selectedRoom.name,
      ),
    );
    handleSelectRoom(null);
    await refreshRoomLists();
  }, [
    selectedRoom,
    service,
    toastError,
    toastSuccess,
    handleSelectRoom,
    refreshRoomLists,
  ]);

  const canShowDetail =
    !loading && (filteredRooms.length > 0 || selectedRoom !== null);

  return (
    <DatabasePageLayout
      loading={loading}
      sessionLoading={status === "loading"}
      className="flex w-full flex-col"
      intro={{
        kicker: "Datenverwaltung",
        title: "Räume",
        description: loading ? <Skeleton className="h-4 w-48" /> : statusLine,
        actions: (
          <div className="flex items-center gap-2">
            {!isMobile ? (
              <DatabaseGroupingToggle
                value={grouping}
                options={ROOMS_GROUPING_OPTIONS}
                onChange={handleGroupingChange}
              />
            ) : null}
            <DatabaseCreateAction
              label="Raum"
              ariaLabel="Raum erstellen"
              onClick={() => setShowCreateModal(true)}
            />
          </div>
        ),
      }}
      search={
        <PageHeaderWithSearch
          title=""
          badge={{
            icon: (
              <MotoDuotoneIcon
                icon={MOTO_CONCEPTS.rooms.icon}
                tone={MOTO_CONCEPTS.rooms.tone}
                size={20}
              />
            ),
            count: filteredRooms.length,
            label: "Räume",
          }}
          search={{
            value: searchTerm,
            onChange: setSearchTerm,
            placeholder: "Räume suchen…",
          }}
          filters={filters}
          activeFilters={activeFilters}
          onClearAllFilters={() => {
            setSearchTerm("");
            setCategoryFilter("all");
          }}
        />
      }
    >
      {error && (
        <div className="mb-6">
          <Alert type="error" message={error} />
        </div>
      )}

      {canShowDetail ? (
        <div className="min-h-0 flex-1 pb-4">
          <RoomsMasterDetail
            groupDefinitions={groupDefinitions}
            selectedId={selectedId}
            selectedRoom={selectedRoom}
            onSelect={handleSelectRoom}
            onSaveRoom={handleUpdateRoom}
            onDeleteClick={handleDeleteClick}
          />
        </div>
      ) : !loading ? (
        <DatabaseEmptyState
          icon={
            <MotoDuotoneIcon
              icon={MOTO_CONCEPTS.rooms.icon}
              tone={MOTO_CONCEPTS.rooms.tone}
              size={48}
              className="mx-auto"
            />
          }
          title={
            searchTerm || categoryFilter !== "all"
              ? "Keine Räume gefunden"
              : "Keine Räume vorhanden"
          }
          description={
            searchTerm || categoryFilter !== "all"
              ? "Versuchen Sie andere Suchkriterien oder Filter."
              : "Es wurden noch keine Räume erstellt."
          }
        />
      ) : null}

      <DatabaseFormModal<Room>
        isOpen={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        mode="create"
        config={roomsConfig}
        onSubmit={handleCreateRoom}
      />

      {selectedRoom && (
        <ConfirmationModal
          isOpen={showDeleteConfirmModal}
          onClose={handleDeleteCancel}
          onConfirm={() => confirmDelete(() => void handleDeleteRoom())}
          title="Raum löschen?"
          confirmText="Löschen"
          cancelText="Abbrechen"
          confirmButtonClass="bg-moto-red hover:bg-moto-red-hover"
        >
          <p className="text-sm text-gray-700">
            Möchten Sie den Raum{" "}
            <span className="font-medium">{selectedRoom.name}</span> wirklich
            löschen? Diese Aktion kann nicht rückgängig gemacht werden.
          </p>
        </ConfirmationModal>
      )}
    </DatabasePageLayout>
  );
}
