"use client";

import { useState } from "react";
import type { Room } from "@/lib/api";

interface RoomListProps {
  rooms: Room[];
  onSelectRoom: (room: Room) => void;
  searchTerm?: string;
}

export function RoomList({ rooms, onSelectRoom, searchTerm }: RoomListProps) {
  const [categoryFilter, setCategoryFilter] = useState<string | null>(null);
  const [buildingFilter, setBuildingFilter] = useState<string | null>(null);
  const [floorFilter, setFloorFilter] = useState<number | null>(null);
  const [occupiedFilter, setOccupiedFilter] = useState<boolean | null>(null);

  // Get unique categories, buildings, and floors for filters
  const categories = [...new Set(rooms.map((room) => room.category))];
  const buildings = [
    ...new Set(rooms.map((room) => room.building).filter(Boolean)),
  ];
  const floors = [...new Set(rooms.map((room) => room.floor))];

  // Apply filters
  const filteredRooms = rooms.filter((room) => {
    // Apply search term filter if provided
    if (
      searchTerm &&
      !room.name.toLowerCase().includes(searchTerm.toLowerCase())
    ) {
      return false;
    }

    // Apply other filters
    if (categoryFilter && room.category !== categoryFilter) return false;
    if (buildingFilter && room.building !== buildingFilter) return false;
    if (floorFilter !== null && room.floor !== floorFilter) return false;
    if (occupiedFilter !== null && room.isOccupied !== occupiedFilter)
      return false;
    return true;
  });

  // Render a single room item
  const renderRoom = (room: Room) => (
    <div className="flex w-full items-center justify-between">
      <div className="flex flex-col transition-transform duration-200 group-hover:translate-x-1">
        <div className="flex items-center">
          <div
            className="mr-2 h-3 w-3 rounded-full"
            style={{ backgroundColor: room.color ?? "#e5e7eb" }}
          ></div>
          <span className="font-semibold text-gray-900 transition-colors duration-200 group-hover:text-blue-600">
            {room.name}
            {room.isOccupied ? (
              <span className="ml-2 rounded-full bg-red-100 px-2 py-0.5 text-xs text-red-800">
                Belegt
              </span>
            ) : (
              <span className="ml-2 rounded-full bg-green-100 px-2 py-0.5 text-xs text-green-800">
                Frei
              </span>
            )}
          </span>
        </div>
        <span className="text-sm text-gray-500">
          Kategorie: {room.category}
          {room.building &&
            ` | Gebäude: ${room.building}, Etage: ${room.floor}`}
          {` | Kapazität: ${room.capacity}`}
        </span>
        {room.isOccupied && room.groupName && (
          <span className="mt-1 text-xs text-blue-600">
            {room.groupName}
            {room.activityName && ` - ${room.activityName}`}
          </span>
        )}
      </div>
      <svg
        xmlns="http://www.w3.org/2000/svg"
        className="h-5 w-5 text-gray-400 transition-all duration-200 group-hover:translate-x-1 group-hover:transform group-hover:text-blue-500"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth={2}
          d="M9 5l7 7-7 7"
        />
      </svg>
    </div>
  );

  return (
    <div className="w-full">
      {/* Filter Controls */}
      <div className="mb-6 grid grid-cols-1 gap-4 rounded-lg bg-gray-50 p-4 md:grid-cols-4">
        <div>
          <label
            htmlFor="categoryFilter"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            Kategorie
          </label>
          <select
            id="categoryFilter"
            className="w-full rounded border border-gray-300 p-2 text-sm"
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
          <label
            htmlFor="buildingFilter"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            Gebäude
          </label>
          <select
            id="buildingFilter"
            className="w-full rounded border border-gray-300 p-2 text-sm"
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
          <label
            htmlFor="floorFilter"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            Etage
          </label>
          <select
            id="floorFilter"
            className="w-full rounded border border-gray-300 p-2 text-sm"
            value={floorFilter?.toString() ?? ""}
            onChange={(e) =>
              setFloorFilter(e.target.value ? parseInt(e.target.value) : null)
            }
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
          <label
            htmlFor="occupiedFilter"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            Status
          </label>
          <select
            id="occupiedFilter"
            className="w-full rounded border border-gray-300 p-2 text-sm"
            value={
              occupiedFilter === null ? "" : occupiedFilter ? "true" : "false"
            }
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

      {/* Room List */}
      <div className="w-full space-y-3">
        {filteredRooms.length > 0 ? (
          filteredRooms.map((room) => (
            <div
              key={room.id}
              className="group flex cursor-pointer items-center justify-between rounded-lg border border-gray-100 bg-white p-4 shadow-sm transition-all duration-200 hover:translate-y-[-1px] hover:border-blue-200 hover:shadow-md"
              onClick={() => onSelectRoom(room)}
            >
              {renderRoom(room)}
            </div>
          ))
        ) : (
          <div className="py-8 text-center">
            <p className="text-gray-500">
              {searchTerm ||
              categoryFilter ||
              buildingFilter ||
              floorFilter !== null ||
              occupiedFilter !== null
                ? "Keine Ergebnisse gefunden."
                : "Keine Einträge vorhanden."}
            </p>
          </div>
        )}
      </div>
    </div>
  );
}
