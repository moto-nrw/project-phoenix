"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import type { Room } from "@/lib/api";
import { roomService } from "@/lib/api";

interface RoomIntegrationsProps {
  roomId?: string;
  categoryFilter?: string;
  showUtilization?: boolean;
  onRoomSelect?: (room: Room) => void;
}

interface RoomFilters {
  category?: string;
}

export function RoomIntegrations({
  roomId,
  categoryFilter,
  showUtilization = true,
  onRoomSelect,
}: RoomIntegrationsProps) {
  const router = useRouter();
  const [rooms, setRooms] = useState<Room[]>([]);
  const [selectedRoom, setSelectedRoom] = useState<Room | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Fetch room data
  useEffect(() => {
    void (async function fetchRooms() {
      try {
        setLoading(true);
        setError(null);

        if (roomId) {
          // Fetch single room if roomId is provided
          const room = await roomService.getRoom(roomId);
          setSelectedRoom(room);
          setRooms([room]);
        } else {
          // Fetch rooms with optional filters
          const filters: RoomFilters = {};

          if (categoryFilter) {
            filters.category = categoryFilter;
          }

          const roomsData = await roomService.getRooms(filters);
          setRooms(roomsData);
        }
      } catch (err) {
        console.error("Error fetching room data:", err);
        setError(
          "Fehler beim Laden der Raumdaten. Bitte versuchen Sie es später erneut.",
        );
      } finally {
        setLoading(false);
      }
    })();
  }, [roomId, categoryFilter]);

  const handleRoomClick = (room: Room) => {
    setSelectedRoom(room);
    if (onRoomSelect) {
      onRoomSelect(room);
    }
  };

  // Calculate usage percentage
  const getUtilizationPercentage = (room: Room): number => {
    if (room.capacity <= 0) return 0;
    return Math.min(((room.studentCount ?? 0) / room.capacity) * 100, 100);
  };

  // Get utilization class based on percentage
  const getUtilizationClass = (percentage: number): string => {
    if (percentage < 50) return "bg-green-500";
    if (percentage < 80) return "bg-yellow-500";
    return "bg-red-500";
  };

  if (loading) {
    return (
      <div className="flex h-40 items-center justify-center rounded-lg bg-gray-50">
        <div className="flex animate-pulse flex-col items-center">
          <div className="h-8 w-8 rounded-full bg-gradient-to-r from-teal-400 to-blue-500"></div>
          <p className="mt-3 text-sm text-gray-500">Daten werden geladen...</p>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="rounded-lg bg-red-50 p-4 text-red-800">
        <p>{error}</p>
      </div>
    );
  }

  if (rooms.length === 0) {
    return (
      <div className="rounded-lg bg-yellow-50 p-4 text-yellow-800">
        <p>Keine Räume gefunden.</p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Room grid */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {rooms.map((room) => {
          const utilizationPercentage = getUtilizationPercentage(room);
          const utilizationClass = getUtilizationClass(utilizationPercentage);

          return (
            <div
              key={room.id}
              className={`cursor-pointer rounded-lg border-2 bg-white p-4 shadow-sm transition-all duration-200 hover:shadow-md ${
                selectedRoom?.id === room.id
                  ? "border-blue-500"
                  : "border-gray-100"
              }`}
              onClick={() => handleRoomClick(room)}
            >
              <div className="flex items-center justify-between">
                <div className="flex items-center space-x-2">
                  <div
                    className="h-4 w-4 rounded-full"
                    style={{ backgroundColor: room.color }}
                  ></div>
                  <h3 className="font-medium text-gray-900">{room.name}</h3>
                </div>
                <span
                  className={`rounded-full px-2 py-1 text-xs ${
                    room.isOccupied
                      ? "bg-red-100 text-red-800"
                      : "bg-green-100 text-green-800"
                  }`}
                >
                  {room.isOccupied ? "Belegt" : "Frei"}
                </span>
              </div>

              <div className="mt-2 text-sm text-gray-600">
                <p>Kategorie: {room.category}</p>
                <p>
                  Ort: {room.building ? `${room.building}, ` : ""}
                  Etage {room.floor}
                </p>
                <p>
                  Kapazität: {room.studentCount ?? 0}/{room.capacity}
                </p>
              </div>

              {showUtilization && (
                <div className="mt-3">
                  <div className="flex items-center justify-between text-xs">
                    <span>Auslastung</span>
                    <span>{Math.round(utilizationPercentage)}%</span>
                  </div>
                  <div className="mt-1 h-2 w-full overflow-hidden rounded-full bg-gray-200">
                    <div
                      className={`h-full ${utilizationClass}`}
                      style={{ width: `${utilizationPercentage}%` }}
                    ></div>
                  </div>
                </div>
              )}

              {room.isOccupied && room.groupName && (
                <div className="mt-3 rounded-lg bg-blue-50 p-2 text-xs text-blue-800">
                  <p>
                    <span className="font-medium">Gruppe:</span>{" "}
                    {room.groupName}
                  </p>
                  {room.activityName && (
                    <p>
                      <span className="font-medium">Aktivität:</span>{" "}
                      {room.activityName}
                    </p>
                  )}
                  {room.supervisorName && (
                    <p>
                      <span className="font-medium">Aufsicht:</span>{" "}
                      {room.supervisorName}
                    </p>
                  )}
                </div>
              )}
            </div>
          );
        })}
      </div>

      {/* Selected room details (when in multi-room view) */}
      {selectedRoom && rooms.length > 1 && (
        <div className="mt-6 rounded-lg border border-blue-200 bg-blue-50 p-4">
          <h3 className="mb-3 text-lg font-medium text-blue-800">
            Ausgewählter Raum: {selectedRoom.name}
          </h3>

          <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
            <div>
              <h4 className="mb-2 font-medium text-blue-700">Details</h4>
              <ul className="space-y-1 text-sm">
                <li>
                  <span className="font-medium">Kategorie:</span>{" "}
                  {selectedRoom.category}
                </li>
                <li>
                  <span className="font-medium">Ort:</span>{" "}
                  {selectedRoom.building ? `${selectedRoom.building}, ` : ""}
                  Etage {selectedRoom.floor}
                </li>
                <li>
                  <span className="font-medium">Kapazität:</span>{" "}
                  {selectedRoom.capacity} Personen
                </li>
                {selectedRoom.deviceId && (
                  <li>
                    <span className="font-medium">Geräte-ID:</span>{" "}
                    {selectedRoom.deviceId}
                  </li>
                )}
              </ul>
            </div>

            <div>
              <h4 className="mb-2 font-medium text-blue-700">Status</h4>
              <div
                className={`inline-block rounded-full px-3 py-1 text-sm ${
                  selectedRoom.isOccupied
                    ? "bg-red-100 text-red-800"
                    : "bg-green-100 text-green-800"
                }`}
              >
                {selectedRoom.isOccupied ? "Belegt" : "Frei"}
              </div>

              {selectedRoom.isOccupied && (
                <div className="mt-2 space-y-1 text-sm">
                  {selectedRoom.groupName && (
                    <p>
                      <span className="font-medium">Gruppe:</span>{" "}
                      {selectedRoom.groupName}
                    </p>
                  )}
                  {selectedRoom.activityName && (
                    <p>
                      <span className="font-medium">Aktivität:</span>{" "}
                      {selectedRoom.activityName}
                    </p>
                  )}
                  {selectedRoom.supervisorName && (
                    <p>
                      <span className="font-medium">Aufsicht:</span>{" "}
                      {selectedRoom.supervisorName}
                    </p>
                  )}
                  {selectedRoom.studentCount !== undefined && (
                    <p>
                      <span className="font-medium">Schüleranzahl:</span>{" "}
                      {selectedRoom.studentCount} / {selectedRoom.capacity}
                    </p>
                  )}
                </div>
              )}
            </div>
          </div>

          <div className="mt-4 flex justify-end">
            <button
              onClick={() => {
                void router.push(`/database/rooms/${selectedRoom.id}`);
              }}
              className="rounded bg-blue-600 px-3 py-1 text-sm text-white transition hover:bg-blue-700"
            >
              Details anzeigen
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
