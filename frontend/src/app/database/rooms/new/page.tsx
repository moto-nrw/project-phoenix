"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { PageHeader } from "@/components/dashboard";
import { RoomForm } from "@/components/rooms";
import type { Room } from "@/lib/api";
import { roomService } from "@/lib/api";

export default function NewRoomPage() {
  const router = useRouter();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleCreateRoom = async (roomData: Partial<Room>) => {
    try {
      setLoading(true);
      setError(null);

      // Prepare room data
      const newRoom: Omit<Room, "id" | "isOccupied"> = {
        name: roomData.name ?? "",
        building: roomData.building,
        floor: roomData.floor ?? 0,
        capacity: roomData.capacity ?? 0,
        category: roomData.category ?? "Klassenzimmer",
        color: roomData.color ?? "#4F46E5",
        deviceId: roomData.deviceId,
        // These fields are not needed for creation but required by the type
        activityName: undefined,
        groupName: undefined,
        supervisorName: undefined,
        studentCount: undefined,
        createdAt: undefined,
        updatedAt: undefined,
      };

      // Create room
      await roomService.createRoom(newRoom);

      // Navigate back to rooms list on success
      router.push("/database/rooms");
    } catch (err) {
      console.error("Error creating room:", err);
      
      // Check if it's a permissions error (403)
      if (err instanceof Error && err.message.includes("403")) {
        setError(
          "Sie haben keine Berechtigung zum Erstellen von Räumen. Bitte wenden Sie sich an einen Administrator."
        );
      } else {
        setError(
          "Fehler beim Erstellen des Raums. Bitte versuchen Sie es später erneut."
        );
      }
      
      // Don't re-throw for permission errors to avoid form reset
      if (!(err instanceof Error && err.message.includes("403"))) {
        throw err; // Re-throw other errors to be caught by the form component
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <PageHeader title="Neuer Raum" backUrl="/database/rooms" />

      {/* Main Content */}
      <main className="mx-auto max-w-4xl p-4">
        {error && (
          <div className="mb-6 rounded-lg bg-red-50 p-4 text-red-800">
            <p>{error}</p>
          </div>
        )}

        <RoomForm
          initialData={{
            name: "",
            building: "",
            floor: 0,
            capacity: 30,
            category: "Klassenzimmer",
            color: "#4F46E5",
          }}
          onSubmitAction={handleCreateRoom}
          onCancelAction={() => router.back()}
          isLoading={loading}
          formTitle="Raum erstellen"
          submitLabel="Erstellen"
        />
      </main>
    </div>
  );
}