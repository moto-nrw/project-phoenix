"use client";

import { Suspense, useCallback, useMemo, useState } from "react";
import { useSession } from "next-auth/react";
import { redirect, useSearchParams } from "next/navigation";
import { DatabaseCreateAction } from "~/components/database/database-create-action";
import { DatabaseGroupingToggle } from "~/components/database/database-grouping-toggle";
import { DatabasePageLayout } from "~/components/database/database-page-layout";
import { Skeleton } from "~/components/ui/skeleton";
import { formatCount } from "~/lib/format-utils";
import {
  useGroupedItems,
  type Grouper,
} from "~/components/database/use-grouped-items";
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
import { RoomsList } from "@/components/rooms/rooms-list";
import { useToast } from "~/contexts/ToastContext";
import { useIsMobile } from "~/components/ui/hooks/useIsMobile";
import { useUpdateUrlParams } from "~/hooks/useUpdateUrlParams";
import { createLogger } from "~/lib/logger";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";
import {
  DATABASE_ROOMS_LIST_CACHE_KEY,
  ROOM_LIST_CACHE_KEYS,
} from "~/lib/swr/room-derived-caches";
import { useTenantAwarePath } from "~/lib/tenant-path";

const logger = createLogger({ component: "DatabaseRoomsPage" });

type RoomsGroupingMode = "none" | "building" | "floor";

const ROOMS_GROUPING_DEFAULT: RoomsGroupingMode = "building";

const ROOMS_GROUPING_OPTIONS: { value: RoomsGroupingMode; label: string }[] = [
  { value: "building", label: "Gebäude" },
  { value: "floor", label: "Etage" },
  { value: "none", label: "Keine" },
];

/** Die Sammlung dieser Seite, für den Rückweg aus der Raumseite (`?from=`). */
const COLLECTION_PATH = "/database/rooms";

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

/**
 * Räume (BAUARTEN-SPEC Bauart 1): die Sammlung mit Anlegen, Gruppierung,
 * Suche und Filter. Jede Zeile führt auf die Raumseite `/rooms/[id]`, die
 * einzige Objektansicht eines Raums (#3115); Stammdaten und Löschen liegen
 * dort.
 */
function RoomsPageContent() {
  const tenantPath = useTenantAwarePath();
  const searchParams = useSearchParams();
  const updateUrlParams = useUpdateUrlParams();

  const grouping = parseRoomsGrouping(searchParams.get("groupBy"));
  const [searchTerm, setSearchTerm] = useState("");
  const [categoryFilter, setCategoryFilter] = useState<string>("all");
  const isMobile = useIsMobile();

  const [showCreateModal, setShowCreateModal] = useState(false);

  const { success: toastSuccess } = useToast();

  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  const service = useMemo(() => createCrudService(roomsConfig), []);
  const tenantMutate = useTenantMutate();
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

  // Der Rückweg trägt die Gruppierung mit, damit „Zurück" aus der Raumseite
  // dieselbe Liste zeigt, die man verlassen hat.
  const objectHref = useCallback(
    (room: Room) => {
      const query = searchParams.toString();
      const from = query ? `${COLLECTION_PATH}?${query}` : COLLECTION_PATH;
      return tenantPath(`/rooms/${room.id}?from=${encodeURIComponent(from)}`);
    },
    [searchParams, tenantPath],
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

  const canShowList = !loading && filteredRooms.length > 0;

  return (
    <DatabasePageLayout
      loading={loading}
      sessionLoading={status === "loading"}
      error={error}
      empty={
        filteredRooms.length === 0
          ? {
              title:
                searchTerm || categoryFilter !== "all"
                  ? "Keine Räume gefunden"
                  : "Keine Räume vorhanden",
              description:
                searchTerm || categoryFilter !== "all"
                  ? "Versuchen Sie andere Suchkriterien oder Filter."
                  : "Legen Sie den ersten Raum an, damit Gruppen einen Ort haben.",
              icon: (
                <MotoDuotoneIcon
                  icon={MOTO_CONCEPTS.rooms.icon}
                  tone={MOTO_CONCEPTS.rooms.tone}
                  size={48}
                />
              ),
              action:
                searchTerm || categoryFilter !== "all" ? undefined : (
                  <DatabaseCreateAction
                    label="Raum"
                    ariaLabel="Raum erstellen"
                    onClick={() => setShowCreateModal(true)}
                  />
                ),
            }
          : null
      }
      overlays={
        <DatabaseFormModal<Room>
          isOpen={showCreateModal}
          onClose={() => setShowCreateModal(false)}
          mode="create"
          config={roomsConfig}
          onSubmit={handleCreateRoom}
        />
      }
      className="flex w-full flex-col"
      intro={{
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
          embedded
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
      {canShowList ? (
        <div className="min-h-0 flex-1 pb-4">
          <RoomsList
            groupDefinitions={groupDefinitions}
            objectHref={objectHref}
          />
        </div>
      ) : null}
    </DatabasePageLayout>
  );
}
