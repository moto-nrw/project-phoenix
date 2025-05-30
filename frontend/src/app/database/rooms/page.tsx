"use client";

import { useSession } from "next-auth/react";
import { redirect, useRouter } from "next/navigation";
import { useState, useEffect } from "react";
import type { Room } from "@/lib/api";
import { roomService } from "@/lib/api";
import { DatabaseListPage } from "@/components/ui";
import { RoomListItem } from "@/components/rooms";

export default function RoomsPage() {
  const router = useRouter();
  const [rooms, setRooms] = useState<Room[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchFilter, setSearchFilter] = useState("");
  const [categoryFilter, setCategoryFilter] = useState<string | null>(null);
  const [buildingFilter, setBuildingFilter] = useState<string | null>(null);
  const [floorFilter, setFloorFilter] = useState<number | null>(null);
  const [occupiedFilter, setOccupiedFilter] = useState<boolean | null>(null);

  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  // Function to fetch rooms with optional filters
  const fetchRooms = async (search?: string) => {
    try {
      setLoading(true);

      // Prepare filters for API call
      const filters = {
        search: search ?? undefined,
      };

      try {
        // Fetch from the real API using our room service
        const data = await roomService.getRooms(filters);
        
        if (data.length === 0 && !search) {
          console.log("No rooms returned from API, checking connection");
        }

        setRooms(data);
        setError(null);
      } catch (apiErr) {
        console.error("API error when fetching rooms:", apiErr);
        setError(
          "Fehler beim Laden der Raumdaten. Bitte versuchen Sie es später erneut."
        );
        setRooms([]);
      }
    } catch (err) {
      console.error("Error fetching rooms:", err);
      setError(
        "Fehler beim Laden der Raumdaten. Bitte versuchen Sie es später erneut."
      );
      setRooms([]);
    } finally {
      setLoading(false);
    }
  };

  // Initial data load
  useEffect(() => {
    void fetchRooms();
  }, []);

  // Handle search filter changes
  useEffect(() => {
    // Debounce search to avoid too many API calls
    const timer = setTimeout(() => {
      void fetchRooms(searchFilter);
    }, 300);

    return () => clearTimeout(timer);
  }, [searchFilter]);

  if (status === "loading") {
    return <div />; // Let DatabaseListPage handle the loading state
  }

  const handleSelectRoom = (room: Room) => {
    router.push(`/database/rooms/${room.id}`);
  };

  // Apply client-side filters
  const filteredRooms = rooms.filter((room) => {
    if (categoryFilter && room.category !== categoryFilter) return false;
    if (buildingFilter && room.building !== buildingFilter) return false;
    if (floorFilter !== null && room.floor !== floorFilter) return false;
    if (occupiedFilter !== null && room.isOccupied !== occupiedFilter) return false;
    return true;
  });

  // Get unique values for filters
  const categories = [...new Set(rooms.map((room) => room.category))];
  const buildings = [...new Set(rooms.map((room) => room.building).filter(Boolean))];
  const floors = [...new Set(rooms.map((room) => room.floor))];

  // Render filters
  const renderFilters = () => (
    <div className="grid grid-cols-1 gap-3 md:grid-cols-2 lg:grid-cols-4">
      <div>
        <label htmlFor="categoryFilter" className="mb-1 block text-xs md:text-sm font-medium text-gray-700">
          Kategorie
        </label>
        <select
          id="categoryFilter"
          className="w-full rounded border border-gray-300 p-2 text-xs md:text-sm"
          value={categoryFilter ?? ""}
          onChange={(e) => setCategoryFilter(e.target.value || null)}
        >
          <option value="">Alle Kategorien</option>
          {categories.map((category) => (
            <option key={category} value={category}>
              {category}
            </option>
          ))}
        </select>
      </div>

      <div>
        <label htmlFor="buildingFilter" className="mb-1 block text-xs md:text-sm font-medium text-gray-700">
          Gebäude
        </label>
        <select
          id="buildingFilter"
          className="w-full rounded border border-gray-300 p-2 text-xs md:text-sm"
          value={buildingFilter ?? ""}
          onChange={(e) => setBuildingFilter(e.target.value || null)}
        >
          <option value="">Alle Gebäude</option>
          {buildings.map((building) => (
            <option key={building} value={building}>
              {building}
            </option>
          ))}
        </select>
      </div>

      <div>
        <label htmlFor="floorFilter" className="mb-1 block text-xs md:text-sm font-medium text-gray-700">
          Etage
        </label>
        <select
          id="floorFilter"
          className="w-full rounded border border-gray-300 p-2 text-xs md:text-sm"
          value={floorFilter?.toString() ?? ""}
          onChange={(e) => setFloorFilter(e.target.value ? parseInt(e.target.value) : null)}
        >
          <option value="">Alle Etagen</option>
          {floors.map((floor) => (
            <option key={floor} value={floor}>
              {floor}
            </option>
          ))}
        </select>
      </div>

      <div>
        <label htmlFor="occupiedFilter" className="mb-1 block text-xs md:text-sm font-medium text-gray-700">
          Status
        </label>
        <select
          id="occupiedFilter"
          className="w-full rounded border border-gray-300 p-2 text-xs md:text-sm"
          value={occupiedFilter === null ? "" : occupiedFilter ? "true" : "false"}
          onChange={(e) => {
            if (e.target.value === "") setOccupiedFilter(null);
            else setOccupiedFilter(e.target.value === "true");
          }}
        >
          <option value="">Alle Räume</option>
          <option value="true">Belegt</option>
          <option value="false">Frei</option>
        </select>
      </div>
    </div>
  );

  return (
    <DatabaseListPage
      userName={session?.user?.name ?? "Root"}
      title="Räume auswählen"
      description="Verwalten Sie Räume und Belegungen"
      listTitle="Raumliste"
      searchPlaceholder="Raum suchen..."
      searchValue={searchFilter}
      onSearchChange={setSearchFilter}
      filters={renderFilters()}
      addButton={{
        label: "Neuen Raum erstellen",
        href: "/database/rooms/new"
      }}
      items={filteredRooms}
      loading={loading}
      error={error}
      onRetry={() => fetchRooms()}
      itemLabel={{ singular: "Raum", plural: "Räume" }}
      emptyIcon={
        <svg className="h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
        </svg>
      }
      renderItem={(room: Room) => (
        <RoomListItem
          key={room.id}
          room={room}
          onClick={() => handleSelectRoom(room)}
        />
      )}
    />
  );
}