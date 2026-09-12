"use client";

import { useState, useEffect, useMemo, Suspense, useCallback } from "react";
import { useSession } from "next-auth/react";
import { useSearchParams } from "next/navigation";
import { CollectionGrid } from "~/components/ui/collection-grid";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { useTenantRouter } from "~/lib/tenant-router";
import { useTenantAwarePath } from "~/lib/tenant-path";
import { useUpdateUrlParams } from "~/hooks/useUpdateUrlParams";
import { TenantPage } from "~/components/ui/tenant-page";
import { TileCard } from "~/components/ui/tile-card";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import type {
  FilterConfig,
  ActiveFilter,
  OverflowMenuItem,
} from "~/components/ui/page-header/types";
import {
  formatFloor,
  getRoomCategoryColor,
  mapRoomsResponse,
} from "~/lib/room-helpers";
import type { BackendRoom } from "~/lib/room-helpers";
import { useSWRAuth } from "~/lib/swr";
import {
  ArrowRight,
  ChevronRight,
  FileSpreadsheet,
  FileText,
} from "lucide-react";
import { IdentificationCardIcon, UsersIcon } from "@phosphor-icons/react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { SectionHeader } from "~/components/ui/concept-section-header";
import { RoomStatusBadge } from "~/components/rooms/room-status-badge";

import { BinaryModeGuard } from "~/components/tenant/binary-mode-guard";
import { fetchDashboardAnalyticsClient } from "~/lib/dashboard-api";
import type { DashboardAnalytics } from "~/lib/dashboard-helpers";
import {
  exportRoomSnapshot,
  type RoomSnapshotExportFormat,
} from "~/lib/room-export-api";
import { RoomsGridSkeleton } from "./page-skeleton";

// Room interface - entspricht der BackendRoom-Struktur aus den API-Dateien
interface Room {
  id: string;
  name: string;
  building?: string;
  floor?: number; // Optional (nullable in DB)
  capacity?: number; // Optional (nullable in DB)
  category?: string; // Optional (nullable in DB)
  color?: string; // Optional (nullable in DB)
  isOccupied: boolean;
  groupName?: string;
  activityName?: string;
  supervisorName?: string;
  deviceId?: string;
  studentCount?: number;
}

/** Die Sammlung dieser Seite, für den Rückweg aus der Raumseite (`?from=`). */
const COLLECTION_PATH = "/rooms";
const TRANSIT_PATH = "/rooms/unterwegs";

function TransitAssignmentCard({
  count,
  href,
}: {
  readonly count: number;
  readonly href: string;
}) {
  return (
    <TileCard href={href} className="flex items-center justify-between gap-4">
      <SectionHeader
        className="min-w-0 flex-1"
        title="Unterwegs"
        icon={<MotoConceptIcon concept="transit" size={22} />}
        subtitle={
          <>
            <span className="font-medium text-gray-900">{count}</span>{" "}
            {count === 1 ? "Kind" : "Kinder"} ohne Raumzuweisung
          </>
        }
        actions={
          <span className="moto-content-surface inline-flex shrink-0 items-center gap-2 rounded-full border px-3 py-2 text-sm font-medium text-gray-700 transition-colors group-hover:border-gray-300 group-hover:bg-gray-50">
            Zuweisen
            <ArrowRight className="h-4 w-4 text-gray-500" aria-hidden="true" />
          </span>
        }
      />
    </TileCard>
  );
}

function RoomsPageContent() {
  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      router.push("/");
    },
  });
  const router = useTenantRouter();
  const tenantPath = useTenantAwarePath();
  const searchParams = useSearchParams();
  const updateUrlParams = useUpdateUrlParams();

  // Filters are local React state for snappy UI, but their initial
  // value comes from the URL so they SURVIVE a remount when the user
  // drills through /rooms/X or /students/X and returns with browser back.
  // The object pages push back to /rooms with the params we tucked into
  // ?from=, so on remount the URL re-hydrates the React state.
  const [searchTerm, setSearchTerm] = useState(
    () => searchParams.get("search") ?? "",
  );
  const [buildingFilter, setBuildingFilter] = useState(
    () => searchParams.get("building") ?? "all",
  );
  const [occupiedFilter, setOccupiedFilter] = useState(
    () => searchParams.get("status") ?? "all",
  );
  const [isExporting, setIsExporting] = useState(false);
  const [exportError, setExportError] = useState<string | null>(null);

  // Mirror local filter state into the URL so the current history entry
  // always reflects the user's view: a refresh, a shared link or the way
  // back from a room page lands on the same narrowed grid. router.replace
  // keeps the entry count unchanged. The early-return guards against the
  // searchParams → updateUrlParams identity churn that would otherwise
  // re-fire this effect after each replace.
  useEffect(() => {
    const currentSearch = searchParams.get("search") ?? "";
    const currentBuilding = searchParams.get("building") ?? "all";
    const currentStatus = searchParams.get("status") ?? "all";
    if (
      currentSearch === searchTerm &&
      currentBuilding === buildingFilter &&
      currentStatus === occupiedFilter
    ) {
      return;
    }
    updateUrlParams({
      search: searchTerm || null,
      building: buildingFilter !== "all" ? buildingFilter : null,
      status: occupiedFilter !== "all" ? occupiedFilter : null,
    });
  }, [
    searchTerm,
    buildingFilter,
    occupiedFilter,
    searchParams,
    updateUrlParams,
  ]);

  // Fetch rooms with SWR (automatic caching, deduplication, revalidation)
  // Global SSE in TenantAuthWrapper handles cache invalidation automatically
  const {
    data: roomsData,
    isLoading: loading,
    error: roomsError,
  } = useSWRAuth<Room[]>(
    "rooms-list",
    async () => {
      // include_system: this page is the live occupancy view — system rooms
      // (Schulhof, WC) must stay visible here and in the Wer-ist-wo export.
      const response = await fetch("/api/rooms?include_system=true");
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }

      const data = (await response.json()) as
        BackendRoom[] | { data: BackendRoom[] };

      // Use mapping helper to transform backend data to frontend format
      let roomsData: Room[];
      if (data && Array.isArray(data)) {
        roomsData = mapRoomsResponse(data);
      } else if (data?.data && Array.isArray(data.data)) {
        roomsData = mapRoomsResponse(data.data);
      } else {
        throw new Error("Unerwartetes Antwortformat");
      }

      // Apply color defaults
      return roomsData.map((room) => ({
        ...room,
        color: room.color ?? getRoomCategoryColor(room.category),
      }));
    },
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
    },
  );

  const { data: dashboardData } = useSWRAuth<DashboardAnalytics>(
    "dashboard-analytics",
    fetchDashboardAnalyticsClient,
    { refreshInterval: 5 * 60 * 1000 },
  );

  const error = roomsError
    ? "Fehler beim Laden der Raumdaten. Bitte versuchen Sie es später erneut."
    : null;

  // Apply filters
  const filteredRooms = useMemo(() => {
    const rooms = roomsData ?? [];
    let filtered = [...rooms];

    // Search filter
    if (searchTerm) {
      const searchLower = searchTerm.toLowerCase();
      filtered = filtered.filter((room) => {
        const checks = [
          room.name?.toLowerCase().includes(searchLower),
          room.groupName?.toLowerCase().includes(searchLower),
          room.activityName?.toLowerCase().includes(searchLower),
        ];
        return checks.some(Boolean);
      });
    }

    // Building filter
    if (buildingFilter !== "all") {
      filtered = filtered.filter((room) => room.building === buildingFilter);
    }

    // Occupied filter
    if (occupiedFilter !== "all") {
      const isOccupied = occupiedFilter === "occupied";
      filtered = filtered.filter((room) => room.isOccupied === isOccupied);
    }

    // Sort by name
    filtered.sort((a, b) => a.name.localeCompare(b.name, "de"));

    return filtered;
  }, [roomsData, searchTerm, buildingFilter, occupiedFilter]);

  const exportRoomIds = useMemo(() => {
    return filteredRooms
      .map((room) => Number.parseInt(room.id, 10))
      .filter((id) => Number.isFinite(id));
  }, [filteredRooms]);

  const handleExport = useCallback(
    async (format: RoomSnapshotExportFormat) => {
      setIsExporting(true);
      setExportError(null);
      try {
        await exportRoomSnapshot({
          format,
          title: "Wer ist wo",
          room_ids: exportRoomIds,
          include_transit: true,
        });
      } catch {
        setExportError(
          "Der Raum-Snapshot konnte nicht exportiert werden. Bitte versuchen Sie es erneut.",
        );
      } finally {
        setIsExporting(false);
      }
    },
    [exportRoomIds],
  );

  // Der Rückweg trägt die Filter mit, damit „Zurück" aus der Raumseite
  // dieselbe eingegrenzte Übersicht zeigt, die man verlassen hat.
  const fromReferrer = useMemo(() => {
    const next = new URLSearchParams();
    if (searchTerm) next.set("search", searchTerm);
    if (buildingFilter !== "all") next.set("building", buildingFilter);
    if (occupiedFilter !== "all") next.set("status", occupiedFilter);
    const query = next.toString();
    return query ? `${COLLECTION_PATH}?${query}` : COLLECTION_PATH;
  }, [buildingFilter, occupiedFilter, searchTerm]);
  // Jede Kachel führt auf die Raumseite: eine Objektansicht je Typ, immer auf
  // demselben Weg (BAUARTEN-SPEC Bauart 1 Regel 2, #3115).
  const roomHref = useCallback(
    (roomId: string) =>
      tenantPath(`/rooms/${roomId}?from=${encodeURIComponent(fromReferrer)}`),
    [fromReferrer, tenantPath],
  );

  // Get unique values for filters
  const uniqueBuildings = useMemo(() => {
    const rooms = roomsData ?? [];
    return Array.from(
      new Set(rooms.map((room) => room.building).filter(Boolean)),
    );
  }, [roomsData]);

  // Prepare filter configurations
  const filterConfigs: FilterConfig[] = useMemo(
    () => [
      {
        id: "building",
        label: "Gebäude",
        type: "dropdown",
        value: buildingFilter,
        onChange: (value) => setBuildingFilter(value as string),
        options: [
          { value: "all", label: "Alle Gebäude" },
          ...uniqueBuildings.map((building) => ({
            value: building!,
            label: building!,
          })),
        ],
      },
      {
        id: "occupied",
        label: "Status",
        type: "buttons",
        value: occupiedFilter,
        onChange: (value) => setOccupiedFilter(value as string),
        options: [
          { value: "all", label: "Alle" },
          { value: "occupied", label: "Belegt" },
          { value: "free", label: "Frei" },
        ],
      },
    ],
    [buildingFilter, occupiedFilter, uniqueBuildings],
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

    if (buildingFilter !== "all") {
      filters.push({
        id: "building",
        label: buildingFilter,
        onRemove: () => setBuildingFilter("all"),
      });
    }

    if (occupiedFilter !== "all") {
      const statusLabels = {
        occupied: "Belegt",
        free: "Frei",
      };
      filters.push({
        id: "occupied",
        label:
          statusLabels[occupiedFilter as keyof typeof statusLabels] ??
          occupiedFilter,
        onRemove: () => setOccupiedFilter("all"),
      });
    }

    return filters;
  }, [searchTerm, buildingFilter, occupiedFilter]);

  const transitCount = dashboardData?.studentsInTransit ?? 0;
  const normalizedSearchTerm = searchTerm.trim().toLowerCase();
  const showTransitAssignment =
    transitCount > 0 ||
    (normalizedSearchTerm.length > 0 &&
      "unterwegs".includes(normalizedSearchTerm));
  const exportTargetCount = filteredRooms.length + 1;
  const overflowItems = useMemo<OverflowMenuItem[]>(
    () => [
      {
        label: "Wer ist wo als PDF",
        icon: <FileText className="size-4" aria-hidden />,
        badge: exportTargetCount,
        disabled: loading || isExporting,
        onClick: () => {
          handleExport("pdf").catch(() => undefined);
        },
      },
      {
        label: "Wer ist wo als Word",
        icon: <FileText className="size-4" aria-hidden />,
        badge: exportTargetCount,
        disabled: loading || isExporting,
        onClick: () => {
          handleExport("docx").catch(() => undefined);
        },
      },
      {
        label: "Wer ist wo als Excel",
        icon: <FileSpreadsheet className="size-4" aria-hidden />,
        badge: exportTargetCount,
        disabled: loading || isExporting,
        onClick: () => {
          handleExport("xlsx").catch(() => undefined);
        },
      },
    ],
    [exportTargetCount, handleExport, isExporting, loading],
  );

  // Auth-loading joins the data-loading condition below instead of an early
  // return before the header, so the real PageHeaderWithSearch (title,
  // search field, static tabs) renders immediately and only the room-card
  // grid skeletonizes. The `useSession({ required: true })` callback
  // redirects on unauthenticated.
  const showSkeleton = status === "loading" || loading;

  // Leerzustand kommt aus dem Gerüst (`empty`), nicht als handgebauter
  // Block im Inhalt. Er bleibt aus, solange die Übergangsliste steht: dann
  // ist die Seite nicht leer.
  const hasActiveFilters =
    searchTerm !== "" || buildingFilter !== "all" || occupiedFilter !== "all";
  const resetFilters = useCallback(() => {
    setSearchTerm("");
    setBuildingFilter("all");
    setOccupiedFilter("all");
  }, []);
  const emptyState =
    !showSkeleton &&
    !showTransitAssignment &&
    !exportError &&
    filteredRooms.length === 0
      ? hasActiveFilters
        ? {
            icon: <MotoConceptIcon concept="rooms" size={48} />,
            title: "Keine Räume gefunden",
            description:
              "Zu Suche und Filtern passt kein Raum. Setzen Sie die Filter zurück, um alle Räume zu sehen.",
            action: (
              <Button type="button" size="md" onClick={resetFilters}>
                Filter zurücksetzen
              </Button>
            ),
          }
        : {
            icon: <MotoConceptIcon concept="rooms" size={48} />,
            title: "Keine Räume gefunden",
            description:
              "Für diese Schule ist noch kein Raum angelegt. Räume legen Sie in der Datenverwaltung an.",
          }
      : null;

  // Statuszeile unter dem Seitentitel, allein aus der geladenen Raumliste.
  const roomSummary = (() => {
    const rooms = roomsData ?? [];
    const occupied = rooms.filter((room) => room.isOccupied).length;
    return `${rooms.length} ${rooms.length === 1 ? "Raum" : "Räume"} · ${occupied} belegt`;
  })();

  return (
    <TenantPage
      title="Räume"
      stats={roomSummary}
      statsLoading={showSkeleton}
      // Das Exportmenü ist eine Aktion der Seite und sitzt deshalb im Kopf,
      // damit es auch mobil erreichbar bleibt.
      actions={<OverflowMenu items={overflowItems} />}
      search={{
        value: searchTerm,
        onChange: setSearchTerm,
        placeholder: "Raum suchen…",
      }}
      filters={filterConfigs}
      activeFilters={activeFilters}
      onClearAllFilters={resetFilters}
      error={error}
      empty={emptyState}
    >
      {exportError && <Alert type="error" message={exportError} />}

      {/* Room Cards Grid, skeleton mirrors the populated grid's column
          breakpoints and per-card shape (rounded-2xl, min-h-[180px],
          header row + meta line + status pill, middle content rows,
          footer hint) so the grid area doesn't visibly resize when real
          data arrives. Review feedback (#1323): a generic spinner
          collapsed the header row into a tiny payload, then the layout
          jumped open when rooms loaded. */}
      {showSkeleton ? (
        <RoomsGridSkeleton />
      ) : (
        <>
          {showTransitAssignment ? (
            <TransitAssignmentCard
              count={transitCount}
              href={tenantPath(TRANSIT_PATH)}
            />
          ) : null}

          {filteredRooms.length > 0 ? (
            <CollectionGrid>
              {filteredRooms.map((room) => (
                <TileCard
                  key={room.id}
                  href={roomHref(room.id)}
                  ariaLabel={room.name}
                  padding="none"
                >
                  <div className="relative p-4 sm:p-5">
                    <div className="relative flex min-h-[120px] flex-col">
                      <div className="mb-3 flex items-start justify-between gap-3">
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center gap-2">
                            <h3 className="truncate text-base font-bold text-gray-900">
                              {room.name}
                            </h3>
                            <ChevronRight
                              className="h-4 w-4 flex-shrink-0 text-gray-300 transition-colors duration-200 md:group-hover:text-gray-500"
                              aria-hidden="true"
                            />
                          </div>
                          {(room.building !== undefined ||
                            room.floor !== undefined) && (
                            <p className="mt-0.5 truncate text-xs text-gray-500">
                              {room.building &&
                                room.floor !== undefined &&
                                `${room.building} · ${formatFloor(room.floor)}`}
                              {room.building &&
                                room.floor === undefined &&
                                room.building}
                              {!room.building &&
                                room.floor !== undefined &&
                                formatFloor(room.floor)}
                            </p>
                          )}
                        </div>

                        <RoomStatusBadge
                          isOccupied={room.isOccupied}
                          size="sm"
                          className="font-bold"
                        />
                      </div>

                      <div className="flex-1 space-y-2">
                        {room.isOccupied && room.groupName && (
                          <div className="text-sm text-gray-700">
                            <span className="font-medium">
                              Aktuelle Aktivität:
                            </span>{" "}
                            {room.groupName}
                          </div>
                        )}
                        {room.isOccupied &&
                          ((room.studentCount !== undefined &&
                            room.studentCount > 0) ||
                            room.supervisorName) && (
                            <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-sm text-gray-600">
                              {room.studentCount !== undefined &&
                                room.studentCount > 0 && (
                                  <span className="flex items-center gap-1">
                                    <MotoDuotoneIcon
                                      icon={UsersIcon}
                                      tone="neutral"
                                      size={16}
                                    />
                                    {room.studentCount}{" "}
                                    {room.studentCount === 1
                                      ? "Kind"
                                      : "Kinder"}
                                  </span>
                                )}
                              {room.supervisorName && (
                                <span className="flex items-center gap-1">
                                  <MotoDuotoneIcon
                                    icon={IdentificationCardIcon}
                                    tone="neutral"
                                    size={16}
                                  />
                                  {room.supervisorName}
                                </span>
                              )}
                            </div>
                          )}

                        {!room.isOccupied && (
                          <>
                            <div className="text-sm text-gray-600">
                              Für Aktivitäten buchbar
                            </div>
                            {room.capacity !== undefined &&
                              room.capacity > 0 && (
                                <div className="text-sm text-gray-600">
                                  Kapazität: {room.capacity} Plätze
                                </div>
                              )}
                          </>
                        )}
                      </div>

                      <div className="absolute right-3 bottom-3 h-3 w-3 rounded-full bg-white/30"></div>
                    </div>
                  </div>
                </TileCard>
              ))}
            </CollectionGrid>
          ) : null}
        </>
      )}
    </TenantPage>
  );
}

// Main component with Suspense wrapper + binary-mode 404 guard.
// Binary-mode tenants don't track room occupancy, so the concepts this page
// surfaces don't apply. Guard triggers Next.js notFound() for direct URL entry.
export default function RoomsPage() {
  return (
    <BinaryModeGuard title="Räume">
      <Suspense fallback={<RoomsGridSkeleton />}>
        <RoomsPageContent />
      </Suspense>
    </BinaryModeGuard>
  );
}
