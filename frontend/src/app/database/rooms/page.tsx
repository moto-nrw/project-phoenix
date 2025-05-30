"use client";

import { useSession } from "next-auth/react";
import { redirect, useRouter } from "next/navigation";
import { useState, useEffect } from "react";
import type { Room } from "@/lib/api";
import { roomService } from "@/lib/api";
import { DatabaseListPage, SelectFilter } from "@/components/ui";
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
  const categoryOptions = [...new Set(rooms.map((room) => room.category))]
    .sort()
    .map(cat => ({ value: cat, label: cat }));
  
  const buildingOptions = [...new Set(rooms.map((room) => room.building).filter((b): b is string => Boolean(b)))]
    .sort()
    .map(building => ({ value: building, label: building }));
  
  const floorOptions = [...new Set(rooms.map((room) => room.floor))]
    .sort((a, b) => a - b)
    .map(floor => ({ value: floor.toString(), label: `Etage ${floor}` }));

  const statusOptions = [
    { value: "true", label: "Belegt" },
    { value: "false", label: "Frei" }
  ];

  // Render filters
  const renderFilters = () => (
    <div className="grid grid-cols-1 gap-3 md:grid-cols-2 lg:grid-cols-4">
      <SelectFilter
        id="categoryFilter"
        label="Kategorie"
        value={categoryFilter}
        onChange={setCategoryFilter}
        options={categoryOptions}
        placeholder="Alle Kategorien"
      />

      <SelectFilter
        id="buildingFilter"
        label="Gebäude"
        value={buildingFilter}
        onChange={setBuildingFilter}
        options={buildingOptions}
        placeholder="Alle Gebäude"
      />

      <SelectFilter
        id="floorFilter"
        label="Etage"
        value={floorFilter?.toString() ?? null}
        onChange={(value) => setFloorFilter(value ? parseInt(value) : null)}
        options={floorOptions}
        placeholder="Alle Etagen"
      />

      <SelectFilter
        id="occupiedFilter"
        label="Status"
        value={occupiedFilter === null ? null : occupiedFilter.toString()}
        onChange={(value) => {
          if (value === null) setOccupiedFilter(null);
          else setOccupiedFilter(value === "true");
        }}
        options={statusOptions}
        placeholder="Alle Räume"
      />
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