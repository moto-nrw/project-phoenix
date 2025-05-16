"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { PageHeader } from "@/components/dashboard";
import { RoomForm, RoomHistory } from "@/components/rooms";
import type { Room } from "@/lib/api";
import { roomService } from "@/lib/api";

export default function RoomDetailPage() {
  const router = useRouter();
  const params = useParams();
  // Ensure we handle both string and array ID formats from Next.js
  const roomId = Array.isArray(params.id) ? params.id[0] : (params.id ?? "");

  const [room, setRoom] = useState<Room | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isEditing, setIsEditing] = useState(false);

  useEffect(() => {
    const fetchRoom = async () => {
      if (!roomId) {
        setError("Keine Raum-ID angegeben");
        setLoading(false);
        return;
      }
      
      try {
        setLoading(true);
        const data = await roomService.getRoom(roomId);
        setRoom(data);
        setError(null);
      } catch (err) {
        console.error("Error fetching room:", err);
        setError(
          "Fehler beim Laden der Raumdaten. Bitte versuchen Sie es später erneut."
        );
        setRoom(null);
      } finally {
        setLoading(false);
      }
    };

    void fetchRoom();
  }, [roomId]);

  const handleUpdate = async (formData: Partial<Room>) => {
    if (!roomId) {
      setError("Keine Raum-ID angegeben");
      return;
    }
    
    try {
      setLoading(true);
      setError(null);

      console.log("Updating room with data:", formData);
      
      // Update room
      const updatedRoom = await roomService.updateRoom(roomId, formData);
      console.log("Room updated successfully, received:", updatedRoom);
      
      // After updating, fetch the room again to make sure we have the latest data
      try {
        console.log("Re-fetching room data to ensure we have the latest");
        const refreshedRoom = await roomService.getRoom(roomId);
        console.log("Room refreshed data:", refreshedRoom);
        setRoom(refreshedRoom);
      } catch (refreshError) {
        console.error("Error refreshing room data:", refreshError);
        // Use the updatedRoom data from the update API if refresh fails
        setRoom(updatedRoom);
      }
      
      setIsEditing(false);
    } catch (err) {
      console.error("Error updating room:", err);
      setError(
        "Fehler beim Aktualisieren des Raums. Bitte versuchen Sie es später erneut."
      );
      throw err; // Re-throw to be caught by the form component
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async () => {
    if (!roomId) {
      setError("Keine Raum-ID angegeben");
      return;
    }
    
    if (
      window.confirm("Sind Sie sicher, dass Sie diesen Raum löschen möchten?")
    ) {
      try {
        setLoading(true);
        await roomService.deleteRoom(roomId);
        router.push("/database/rooms");
      } catch (err) {
        console.error("Error deleting room:", err);
        setError(
          "Fehler beim Löschen des Raums. Bitte versuchen Sie es später erneut."
        );
        setLoading(false);
      }
    }
  };

  if (loading && !room) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-gray-50">
        <div className="flex animate-pulse flex-col items-center">
          <div className="h-12 w-12 rounded-full bg-gradient-to-r from-teal-400 to-blue-500"></div>
          <p className="mt-4 text-gray-500">Daten werden geladen...</p>
        </div>
      </div>
    );
  }

  if (error && !room) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center bg-gray-50 p-4">
        <div className="max-w-md rounded-lg bg-red-50 p-6 text-red-800 shadow-md">
          <h2 className="mb-3 text-lg font-semibold">Fehler</h2>
          <p className="mb-4">{error}</p>
          <button
            onClick={() => router.back()}
            className="rounded-lg bg-red-100 px-4 py-2 text-red-800 shadow-sm transition-colors hover:bg-red-200"
          >
            Zurück
          </button>
        </div>
      </div>
    );
  }

  if (!room) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center bg-gray-50 p-4">
        <div className="max-w-md rounded-lg bg-yellow-50 p-6 text-yellow-800 shadow-md">
          <h2 className="mb-3 text-lg font-semibold">Raum nicht gefunden</h2>
          <p className="mb-4">
            Der angeforderte Raum konnte nicht gefunden werden.
          </p>
          <button
            onClick={() => router.push("/database/rooms")}
            className="rounded-lg bg-yellow-100 px-4 py-2 text-yellow-800 shadow-sm transition-colors hover:bg-yellow-200"
          >
            Zurück zur Übersicht
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <PageHeader
        title={isEditing ? "Raum bearbeiten" : "Raumdetails"}
        backUrl="/database/rooms"
      />

      {/* Main Content */}
      <main className="mx-auto max-w-4xl p-4">
        {isEditing ? (
          <RoomForm
            initialData={room}
            onSubmitAction={handleUpdate}
            onCancelAction={() => setIsEditing(false)}
            isLoading={loading}
            formTitle="Raum bearbeiten"
            submitLabel="Speichern"
          />
        ) : (
          <div className="overflow-hidden rounded-lg bg-white shadow-md">
            {/* Room card header with color */}
            <div
              className="h-3"
              style={{ backgroundColor: room.color || "#e5e7eb" }}
            ></div>
            <div className="p-6">
              <div className="mb-6 flex items-center justify-between">
                <h2 className="text-xl font-medium text-gray-700">
                  Rauminformationen
                </h2>
                <div className="flex space-x-2">
                  <button
                    onClick={() => setIsEditing(true)}
                    className="rounded-lg bg-blue-50 px-4 py-2 text-blue-600 shadow-sm transition-colors hover:bg-blue-100"
                  >
                    Bearbeiten
                  </button>
                  <button
                    onClick={handleDelete}
                    className="rounded-lg bg-red-50 px-4 py-2 text-red-600 shadow-sm transition-colors hover:bg-red-100"
                  >
                    Löschen
                  </button>
                </div>
              </div>

              <div className="grid grid-cols-1 gap-8 md:grid-cols-2">
                {/* Room Information */}
                <div className="space-y-4">
                  <h3 className="border-b border-blue-200 pb-2 text-lg font-medium text-blue-800">
                    Basisdaten
                  </h3>

                  <div>
                    <div className="text-sm text-gray-500">Name</div>
                    <div className="text-lg font-medium">{room.name}</div>
                  </div>

                  <div>
                    <div className="text-sm text-gray-500">Kategorie</div>
                    <div className="text-base">{room.category}</div>
                  </div>

                  <div>
                    <div className="text-sm text-gray-500">Kapazität</div>
                    <div className="text-base">{room.capacity} Personen</div>
                  </div>

                  <div>
                    <div className="text-sm text-gray-500">Ort</div>
                    <div className="text-base">
                      {room.building ? `${room.building}, ` : ""}
                      Etage {room.floor}
                    </div>
                  </div>

                  {room.deviceId && (
                    <div>
                      <div className="text-sm text-gray-500">Geräte-ID</div>
                      <div className="text-base">{room.deviceId}</div>
                    </div>
                  )}
                </div>

                {/* Current Status */}
                <div className="space-y-4">
                  <h3 className="border-b border-green-200 pb-2 text-lg font-medium text-green-800">
                    Aktueller Status
                  </h3>

                  <div className="flex items-center">
                    <div
                      className={`mr-3 flex h-10 w-10 items-center justify-center rounded-full ${
                        room.isOccupied
                          ? "bg-red-100 text-red-600"
                          : "bg-green-100 text-green-600"
                      }`}
                    >
                      <svg
                        xmlns="http://www.w3.org/2000/svg"
                        className="h-6 w-6"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        {room.isOccupied ? (
                          <path
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            strokeWidth={2}
                            d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"
                          />
                        ) : (
                          <path
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            strokeWidth={2}
                            d="M8 11V7a4 4 0 118 0m-4 8v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2z"
                          />
                        )}
                      </svg>
                    </div>
                    <div>
                      <div className="text-sm text-gray-500">Status</div>
                      <div
                        className={`font-medium ${
                          room.isOccupied ? "text-red-600" : "text-green-600"
                        }`}
                      >
                        {room.isOccupied ? "Belegt" : "Frei"}
                      </div>
                    </div>
                  </div>

                  {room.isOccupied && (
                    <>
                      {room.groupName && (
                        <div>
                          <div className="text-sm text-gray-500">Gruppe</div>
                          <div className="text-base font-medium">
                            {room.groupName}
                          </div>
                        </div>
                      )}

                      {room.activityName && (
                        <div>
                          <div className="text-sm text-gray-500">Aktivität</div>
                          <div className="text-base">{room.activityName}</div>
                        </div>
                      )}

                      {room.supervisorName && (
                        <div>
                          <div className="text-sm text-gray-500">Aufsicht</div>
                          <div className="text-base">{room.supervisorName}</div>
                        </div>
                      )}

                      {room.studentCount !== undefined && (
                        <div>
                          <div className="text-sm text-gray-500">Anzahl Schüler</div>
                          <div className="text-base">
                            {room.studentCount} / {room.capacity}
                            <div className="mt-1 h-2 w-full overflow-hidden rounded-full bg-gray-200">
                              <div
                                className="h-full bg-blue-500"
                                style={{
                                  width: `${Math.min(
                                    (room.studentCount / room.capacity) * 100,
                                    100
                                  )}%`,
                                }}
                              ></div>
                            </div>
                          </div>
                        </div>
                      )}
                    </>
                  )}

                  {!room.isOccupied && (
                    <p className="text-sm text-gray-500">
                      Dieser Raum ist derzeit nicht belegt.
                    </p>
                  )}
                </div>
              </div>
            </div>
            
            {/* Room History Component */}
            <div className="mt-8">
              {roomId && <RoomHistory roomId={roomId} />}
            </div>
          </div>
        )}
      </main>
    </div>
  );
}