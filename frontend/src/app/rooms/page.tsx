"use client";

import { useState, useEffect, useMemo, Suspense, useCallback } from "react";
import { useSession } from "next-auth/react";
import { useRouter } from "next/navigation";
import { useSSE } from "~/lib/hooks/use-sse";
import type { SSEEvent } from "~/lib/sse-types";
import { ResponsiveLayout } from "~/components/dashboard";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import type { FilterConfig, ActiveFilter } from "~/components/ui/page-header";
import { mapRoomsResponse } from "~/lib/room-helpers";
import type { BackendRoom } from "~/lib/room-helpers";

import { Loading } from "~/components/ui/loading";

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

// Kategorie-zu-Farbe Mapping
const categoryColors: Record<string, string> = {
  "Normaler Raum": "#4F46E5",
  Gruppenraum: "#10B981",
  Themenraum: "#8B5CF6",
  Sport: "#EC4899",
};

function RoomsPageContent() {
  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      router.push("/");
    },
  });
  const router = useRouter();

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchTerm, setSearchTerm] = useState("");
  const [buildingFilter, setBuildingFilter] = useState("all");
  const [occupiedFilter, setOccupiedFilter] = useState("all");
  const [rooms, setRooms] = useState<Room[]>([]);
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

  // API Daten laden
  useEffect(() => {
    const fetchRooms = async () => {
      try {
        setLoading(true);

        const response = await fetch("/api/rooms");

        if (!response.ok) {
          throw new Error(`HTTP error! status: ${response.status}`);
        }

        const data = (await response.json()) as
          | BackendRoom[]
          | { data: BackendRoom[] };

        // Use mapping helper to transform backend data to frontend format
        let roomsData: Room[];
        if (data && Array.isArray(data)) {
          roomsData = mapRoomsResponse(data);
        } else if (data?.data && Array.isArray(data.data)) {
          roomsData = mapRoomsResponse(data.data);
        } else {
          console.error("Unerwartetes Antwortformat:", data);
          throw new Error("Unerwartetes Antwortformat");
        }

        // Apply color defaults
        roomsData = roomsData.map((room) => ({
          ...room,
          color:
            room.color ??
            (room.category ? categoryColors[room.category] : undefined) ??
            "#6B7280",
        }));

        setRooms(roomsData);
        setError(null);
      } catch (err) {
        console.error("Fehler beim Laden der Räume:", err);
        setError(
          "Fehler beim Laden der Raumdaten. Bitte versuchen Sie es später erneut.",
        );
        setRooms([]);
      } finally {
        setLoading(false);
      }
    };

    void fetchRooms();
  }, []);

  // Silent refetch for SSE updates (no loading spinner)
  const silentRefetchRooms = useCallback(async () => {
    try {
      const response = await fetch("/api/rooms");
      if (!response.ok) return;

      const data = (await response.json()) as
        | BackendRoom[]
        | { data: BackendRoom[] };

      let roomsData: Room[];
      if (data && Array.isArray(data)) {
        roomsData = mapRoomsResponse(data);
      } else if (data?.data && Array.isArray(data.data)) {
        roomsData = mapRoomsResponse(data.data);
      } else {
        return;
      }

      roomsData = roomsData.map((room) => ({
        ...room,
        color:
          room.color ??
          (room.category ? categoryColors[room.category] : undefined) ??
          "#6B7280",
      }));

      setRooms(roomsData);
    } catch {
      // Silently fail on background refresh
    }
  }, []);

  // SSE event handler - refresh when activities start/end (room occupancy changes)
  const handleSSEEvent = useCallback(
    (event: SSEEvent) => {
      if (event.type === "activity_start" || event.type === "activity_end") {
        silentRefetchRooms().catch(() => undefined);
      }
    },
    [silentRefetchRooms],
  );

  // SSE connection for real-time occupancy updates
  // Backend enforces staff-only access via person/staff record check
  useSSE("/api/sse/events", {
    onMessage: handleSSEEvent,
    enabled: !loading,
  });

  // Apply filters
  const filteredRooms = useMemo(() => {
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
  }, [rooms, searchTerm, buildingFilter, occupiedFilter]);

  // Handle room selection
  const handleSelectRoom = (room: Room) => {
    router.push(`/rooms/${room.id}`);
  };

  // Get unique values for filters
  const uniqueBuildings = useMemo(
    () =>
      Array.from(new Set(rooms.map((room) => room.building).filter(Boolean))),
    [rooms],
  );

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
        {/* PageHeaderWithSearch - Title only on mobile */}
        <PageHeaderWithSearch
          title={isMobile ? "Räume" : ""}
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
                  d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4"
                />
              </svg>
            ),
            count: filteredRooms.length,
            label: "Räume",
          }}
          search={{
            value: searchTerm,
            onChange: setSearchTerm,
            placeholder: "Raum suchen...",
          }}
          filters={filterConfigs}
          activeFilters={activeFilters}
          onClearAllFilters={() => {
            setSearchTerm("");
            setBuildingFilter("all");
            setOccupiedFilter("all");
          }}
        />

        {/* Error Display */}
        {error && (
          <div className="mb-4 rounded-lg border border-red-200 bg-red-50 p-4 text-red-800">
            {error}
          </div>
        )}

        {/* Room Cards Grid */}
        {filteredRooms.length === 0 ? (
          <div className="py-12 text-center">
            <div className="flex flex-col items-center gap-4">
              <svg
                className="h-12 w-12 text-gray-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4"
                />
              </svg>
              <div>
                <h3 className="text-lg font-medium text-gray-900">
                  Keine Räume gefunden
                </h3>
                <p className="text-gray-600">
                  Versuchen Sie Ihre Suchkriterien anzupassen.
                </p>
              </div>
            </div>
          </div>
        ) : (
          <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
            {filteredRooms.map((room) => {
              const handleClick = () => handleSelectRoom(room);
              return (
                <button
                  type="button"
                  key={room.id}
                  onClick={handleClick}
                  className="group relative w-full cursor-pointer overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 text-left shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-500 active:scale-[0.98] md:hover:scale-[1.02] md:hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)]"
                >
                  {/* Modern gradient overlay */}
                  <div className="absolute inset-0 rounded-3xl bg-gradient-to-br from-blue-50/80 to-cyan-100/80 opacity-[0.03]"></div>
                  {/* Subtle inner glow */}
                  <div className="absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
                  {/* Modern border highlight */}
                  <div className="absolute inset-0 rounded-3xl ring-1 ring-white/20 transition-all duration-300 md:group-hover:ring-blue-200/60"></div>

                  <div className="relative p-6">
                    {/* Header with room name and status */}
                    <div className="mb-3 flex items-start justify-between">
                      <div className="min-w-0 flex-1">
                        <h3 className="overflow-hidden text-lg font-bold text-ellipsis whitespace-nowrap text-gray-800 transition-colors duration-300 md:group-hover:text-blue-600">
                          {room.name}
                        </h3>
                        {(room.building !== undefined ||
                          room.floor !== undefined) && (
                          <p className="mt-0.5 text-sm text-gray-500">
                            {room.building &&
                              room.floor !== undefined &&
                              `${room.building} · Etage ${room.floor}`}
                            {room.building &&
                              room.floor === undefined &&
                              room.building}
                            {!room.building &&
                              room.floor !== undefined &&
                              `Etage ${room.floor}`}
                          </p>
                        )}
                      </div>

                      {/* Status indicator */}
                      <span
                        className={`ml-3 inline-flex items-center rounded-full px-2.5 py-1 text-xs font-bold ${
                          room.isOccupied
                            ? "bg-red-100 text-red-700"
                            : "bg-green-100 text-green-700"
                        }`}
                      >
                        <span
                          className={`mr-1.5 h-1.5 w-1.5 rounded-full ${
                            room.isOccupied
                              ? "animate-pulse bg-red-500"
                              : "bg-green-500"
                          }`}
                        ></span>
                        {room.isOccupied ? "Belegt" : "Frei"}
                      </span>
                    </div>

                    {/* Room details */}
                    <div className="space-y-2">
                      {/* Current Activity (only shown when occupied) */}
                      {room.isOccupied && room.groupName && (
                        <div className="text-sm text-gray-700">
                          <span className="font-medium">
                            Aktuelle Aktivität:
                          </span>{" "}
                          {room.groupName}
                        </div>
                      )}
                    </div>

                    {/* Decorative elements */}
                    <div className="absolute top-4 left-4 h-4 w-4 animate-ping rounded-full bg-white/20"></div>
                    <div className="absolute right-4 bottom-4 h-2.5 w-2.5 rounded-full bg-white/30"></div>
                  </div>

                  {/* Glowing border effect on hover */}
                  <div className="absolute inset-0 rounded-3xl bg-gradient-to-r from-transparent via-blue-100/30 to-transparent opacity-0 transition-opacity duration-300 md:group-hover:opacity-100"></div>
                </button>
              );
            })}
          </div>
        )}
      </div>
    </ResponsiveLayout>
  );
}

// Main component with Suspense wrapper
export default function RoomsPage() {
  return (
    <Suspense
      fallback={
        <ResponsiveLayout>
          <Loading fullPage={false} />
        </ResponsiveLayout>
      }
    >
      <RoomsPageContent />
    </Suspense>
  );
}
