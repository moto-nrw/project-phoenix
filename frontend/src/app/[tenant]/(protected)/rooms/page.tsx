"use client";

import {
  useState,
  useEffect,
  useMemo,
  Suspense,
  useCallback,
  useRef,
} from "react";
import { useSession } from "next-auth/react";
import { useSearchParams } from "next/navigation";
import { EmptyState } from "~/components/ui/empty-state";
import { useTenantRouter } from "~/lib/tenant-router";
import { useUpdateUrlParams } from "~/hooks/useUpdateUrlParams";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
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
import { RoomDetailModal } from "~/components/rooms/room-detail-modal";
import { TRANSIT_ROOM_ID } from "~/components/rooms/room-detail-modal";
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

function TransitAssignmentCard({
  count,
  onOpen,
  buttonRef,
}: {
  readonly count: number;
  readonly onOpen: () => void;
  readonly buttonRef: (node: HTMLButtonElement | null) => void;
}) {
  return (
    <button
      type="button"
      ref={buttonRef}
      onClick={onOpen}
      aria-haspopup="dialog"
      aria-controls="room-detail-panel"
      className="group moto-content-surface moto-hover-elevated mb-5 flex w-full items-center justify-between gap-4 rounded-2xl border p-4 text-left shadow-[0_1px_2px_rgba(15,23,42,0.04),0_0_0_1px_rgba(15,23,42,0.02)] focus:ring-2 focus:ring-gray-300 focus:outline-none active:shadow-[0_10px_26px_rgba(15,23,42,0.1)] sm:p-5"
    >
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
    </button>
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
  const searchParams = useSearchParams();
  const updateUrlParams = useUpdateUrlParams();

  // ?room={id} drives the detail modal so deep links work and the back
  // button closes the overlay. Same convention as /database/* pages.
  const selectedRoomId = searchParams.get("room");

  // Filters are local React state for snappy UI, but their initial
  // value comes from the URL so they SURVIVE a remount when the user
  // drills through /students/X and returns with browser back. The student page's
  // BackButton pushes back to /rooms with the params we tucked into
  // ?from= (see students-in-room-section.tsx), so on remount the URL
  // re-hydrates the React state.
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

  const [isMobile, setIsMobile] = useState(false);

  // Handle mobile detection
  useEffect(() => {
    const checkMobile = () => {
      setIsMobile(window.innerWidth < 768);
    };
    checkMobile();
    window.addEventListener("resize", checkMobile);
    return () => window.removeEventListener("resize", checkMobile);
  }, []);

  // Mirror local filter state into the URL so the current history entry
  // always reflects the user's view. Without this, typing a filter while
  // the URL is bare /rooms means closing a just-opened room modal pops
  // back to a URL that never carried those filters. They would survive
  // in React state for now, but a refresh, share, or future remount
  // would silently drop them. router.replace keeps the entry count
  // unchanged, so handleSelectRoom's push and handleCloseDetail's
  // router.back() still work as documented. The early-return guards
  // against the searchParams → updateUrlParams identity churn that
  // would otherwise re-fire this effect after each replace.
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

  // Track whether the click handler just pushed an entry. Used by the
  // effect below to stamp a marker into the resulting history entry.
  const justPushedRef = useRef(false);
  const roomCardRefs = useRef(new Map<string, HTMLButtonElement>());
  const pendingFocusRoomIdRef = useRef<string | null>(null);

  useEffect(() => {
    const roomIdToFocus = pendingFocusRoomIdRef.current;
    if (selectedRoomId || !roomIdToFocus) return;
    pendingFocusRoomIdRef.current = null;
    window.requestAnimationFrame(() => {
      roomCardRefs.current.get(roomIdToFocus)?.focus();
    });
  }, [selectedRoomId]);

  // Open the detail modal by pushing ?room={id} as a NEW history entry
  // (not replace), so the browser Back button closes the overlay
  // instead of skipping past the rooms page. Bake the current filter
  // state into the URL so it survives the round-trip through a child's
  // detail page (student-card click → /students/X → back).
  const handleSelectRoom = useCallback(
    (room: Room) => {
      const next = new URLSearchParams(searchParams.toString());
      if (searchTerm) next.set("search", searchTerm);
      else next.delete("search");
      if (buildingFilter !== "all") next.set("building", buildingFilter);
      else next.delete("building");
      if (occupiedFilter !== "all") next.set("status", occupiedFilter);
      else next.delete("status");
      next.set("room", room.id);
      justPushedRef.current = true;
      router.push(`/rooms?${next.toString()}`);
    },
    [router, searchParams, searchTerm, buildingFilter, occupiedFilter],
  );

  const handleSelectTransitRoom = useCallback(() => {
    const next = new URLSearchParams(searchParams.toString());
    if (searchTerm) next.set("search", searchTerm);
    else next.delete("search");
    if (buildingFilter !== "all") next.set("building", buildingFilter);
    else next.delete("building");
    if (occupiedFilter !== "all") next.set("status", occupiedFilter);
    else next.delete("status");
    next.set("room", TRANSIT_ROOM_ID);
    justPushedRef.current = true;
    router.push(`/rooms?${next.toString()}`);
  }, [router, searchParams, searchTerm, buildingFilter, occupiedFilter]);

  // After router.push commits, mark the now-current history entry as
  // "we pushed this". Stored in window.history.state so it survives
  // page remounts, for example when the user drills through /students/X and
  // returns via browser back, the marker is still there even though
  // the React component was unmounted in between.
  useEffect(() => {
    if (!justPushedRef.current || typeof window === "undefined") return;
    justPushedRef.current = false;
    window.history.replaceState(
      { ...(window.history.state ?? {}), roomModalPushed: true },
      "",
    );
  }, [searchParams]);

  // Close by POPPING the modal entry rather than replacing in place.
  // Replace would leave two consecutive /rooms entries in history, so
  // the first browser Back after closing would appear to do nothing.
  // Fall back to replace only when there's no in-app entry to pop
  // (e.g. user landed directly on /rooms?room=… via a deep link, or
  // got here from the student page's in-app back which pushed a fresh
  // untagged entry).
  const handleCloseDetail = useCallback(() => {
    pendingFocusRoomIdRef.current = selectedRoomId;
    const state = typeof window !== "undefined" ? window.history.state : null;
    const wasPushedByUs =
      state &&
      typeof state === "object" &&
      "roomModalPushed" in state &&
      (state as { roomModalPushed?: unknown }).roomModalPushed === true;
    if (wasPushedByUs) {
      router.back();
    } else {
      updateUrlParams({ room: null });
    }
  }, [router, selectedRoomId, updateUrlParams]);

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

  return (
    <div className="-mt-1.5 w-full">
      {/* PageHeaderWithSearch - Title only on mobile */}
      <PageHeaderWithSearch
        title={isMobile ? "Räume" : ""}
        badge={
          showSkeleton
            ? undefined
            : {
                icon: <MotoConceptIcon concept="rooms" size={20} />,
                count: filteredRooms.length,
                label: "Räume",
              }
        }
        search={{
          value: searchTerm,
          onChange: setSearchTerm,
          placeholder: "Raum suchen...",
        }}
        filters={filterConfigs}
        activeFilters={activeFilters}
        overflowMenu={overflowItems}
        onClearAllFilters={() => {
          setSearchTerm("");
          setBuildingFilter("all");
          setOccupiedFilter("all");
        }}
      />

      {error && (
        <div className="border-moto-red/30 bg-moto-red/10 text-moto-red mb-4 rounded-lg border p-4">
          {error}
        </div>
      )}

      {exportError && (
        <div className="border-moto-red/30 bg-moto-red/10 text-moto-red mb-4 rounded-lg border p-4">
          {exportError}
        </div>
      )}

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
              onOpen={handleSelectTransitRoom}
              buttonRef={(node) => {
                if (node) {
                  roomCardRefs.current.set(TRANSIT_ROOM_ID, node);
                } else {
                  roomCardRefs.current.delete(TRANSIT_ROOM_ID);
                }
              }}
            />
          ) : null}

          {filteredRooms.length === 0 && !showTransitAssignment ? (
            <EmptyState
              icon={<MotoConceptIcon concept="rooms" size={48} />}
              title="Keine Räume gefunden"
              description="Versuchen Sie Ihre Suchkriterien anzupassen."
            />
          ) : null}

          {filteredRooms.length > 0 ? (
            <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
              {filteredRooms.map((room) => {
                const handleClick = () => handleSelectRoom(room);
                return (
                  <button
                    type="button"
                    key={room.id}
                    ref={(node) => {
                      if (node) {
                        roomCardRefs.current.set(room.id, node);
                      } else {
                        roomCardRefs.current.delete(room.id);
                      }
                    }}
                    onClick={handleClick}
                    aria-haspopup="dialog"
                    aria-expanded={selectedRoomId === room.id}
                    aria-controls={
                      selectedRoomId === room.id
                        ? "room-detail-panel"
                        : undefined
                    }
                    className="group moto-content-surface moto-hover-elevated relative w-full cursor-pointer overflow-hidden rounded-2xl border text-left shadow-[0_1px_2px_rgba(15,23,42,0.04),0_0_0_1px_rgba(15,23,42,0.02)] focus-visible:ring-2 focus-visible:ring-gray-300 focus-visible:ring-offset-2 focus-visible:outline-none active:shadow-[0_10px_26px_rgba(15,23,42,0.1)]"
                  >
                    <div className="relative p-6 pb-5">
                      <div className="pointer-events-none absolute inset-0 rounded-2xl ring-1 ring-transparent transition-[box-shadow] duration-300 ease-[cubic-bezier(0.2,0.8,0.2,1)] md:group-hover:shadow-[inset_0_1px_0_rgba(255,255,255,0.9)]" />

                      <div className="relative flex min-h-[156px] flex-col">
                        <div className="mb-3 flex items-start justify-between gap-3">
                          <div className="min-w-0 flex-1">
                            <div className="flex items-center gap-2">
                              <h3 className="inline-block origin-left overflow-hidden text-lg font-bold text-ellipsis whitespace-nowrap text-gray-800 transition-[color,transform] duration-300 ease-[cubic-bezier(0.2,0.8,0.2,1)] motion-reduce:transition-none md:group-hover:scale-[1.025] md:group-hover:text-gray-950 motion-reduce:md:group-hover:scale-100">
                                {room.name}
                              </h3>
                              <ChevronRight
                                className="h-4 w-4 flex-shrink-0 translate-x-0 text-gray-300 opacity-70 transition-[color,opacity,transform] duration-300 ease-[cubic-bezier(0.2,0.8,0.2,1)] motion-reduce:transition-none md:group-hover:translate-x-0.5 md:group-hover:text-gray-600 md:group-hover:opacity-100 motion-reduce:md:group-hover:translate-x-0"
                                aria-hidden="true"
                              />
                            </div>
                            {(room.building !== undefined ||
                              room.floor !== undefined) && (
                              <p className="mt-0.5 overflow-hidden text-sm text-ellipsis whitespace-nowrap text-gray-500 transition-colors duration-300 md:group-hover:text-gray-600">
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

                        <p className="md:group-hover:text-moto-blue-light mt-2 text-xs text-gray-400 transition-colors duration-150">
                          Tippen für mehr Infos
                        </p>

                        <div className="absolute right-3 bottom-3 h-3 w-3 rounded-full bg-white/30"></div>
                      </div>
                    </div>
                  </button>
                );
              })}
            </div>
          ) : null}
        </>
      )}

      <RoomDetailModal roomId={selectedRoomId} onClose={handleCloseDetail} />
    </div>
  );
}

// Main component with Suspense wrapper + binary-mode 404 guard.
// Binary-mode tenants don't track room occupancy, so the concepts this page
// surfaces don't apply. Guard triggers Next.js notFound() for direct URL entry.
export default function RoomsPage() {
  return (
    <BinaryModeGuard>
      <Suspense fallback={<RoomsGridSkeleton />}>
        <RoomsPageContent />
      </Suspense>
    </BinaryModeGuard>
  );
}
