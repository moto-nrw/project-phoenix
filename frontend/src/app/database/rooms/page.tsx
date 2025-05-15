"use client";

import { useSession } from "next-auth/react";
import { redirect, useRouter } from "next/navigation";
import { useState, useEffect } from "react";
import { PageHeader, SectionTitle } from "@/components/dashboard";
import { RoomList } from "@/components/rooms";
import type { Room } from "@/lib/api";
import { roomService } from "@/lib/api";
import Link from "next/link";

export default function RoomsPage() {
  const router = useRouter();
  const [rooms, setRooms] = useState<Room[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchFilter, setSearchFilter] = useState("");

  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/login");
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
        
        // Log the data received from the API
        console.log("Rooms data received:", data);

        if (data.length === 0 && !search) {
          console.log("No rooms returned from API, checking connection");
        }

        // Set the rooms data in state
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

  if (status === "loading" || loading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p>Loading...</p>
      </div>
    );
  }

  const handleSelectRoom = (room: Room) => {
    router.push(`/database/rooms/${room.id}`);
  };

  // Show error if loading failed
  if (error) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center p-4">
        <div className="max-w-md rounded-lg bg-red-50 p-4 text-red-800">
          <h2 className="mb-2 font-semibold">Fehler</h2>
          <p>{error}</p>
          <button
            onClick={() => fetchRooms()}
            className="mt-4 rounded bg-red-100 px-4 py-2 text-red-800 transition-colors hover:bg-red-200"
          >
            Erneut versuchen
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen">
      <PageHeader title="Raumübersicht" backUrl="/database" />

      <main className="mx-auto max-w-6xl p-4">
        <div className="mb-8">
          <SectionTitle title="Räume anzeigen" />
        </div>

        {/* Search and Add Section */}
        <div className="mb-8">
          <div className="mb-4 flex flex-col items-center justify-between gap-4 sm:flex-row">
            <div className="relative w-full sm:max-w-md">
              <input
                type="text"
                placeholder="Suchen..."
                value={searchFilter}
                onChange={(e) => setSearchFilter(e.target.value)}
                className="w-full rounded-lg border border-gray-300 px-4 py-3 pl-10 transition-all duration-200 hover:border-gray-400 focus:shadow-md focus:ring-2 focus:ring-blue-500 focus:outline-none"
              />
              <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3">
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  className="h-5 w-5 text-gray-400"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
                  />
                </svg>
              </div>
            </div>

            <Link href="/database/rooms/new" className="w-full sm:w-auto">
              <button 
                className="group flex w-full items-center justify-center gap-2 rounded-lg bg-gradient-to-r from-teal-500 to-blue-600 px-4 py-3 text-white transition-all duration-200 hover:scale-[1.02] hover:from-teal-600 hover:to-blue-700 hover:shadow-lg sm:w-auto sm:justify-start"
                title="Sie benötigen die 'rooms:create' Berechtigung, um Räume zu erstellen"
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  className="h-5 w-5 transition-transform duration-200 group-hover:rotate-90"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M12 4v16m8-8H4"
                  />
                </svg>
                <span>Neuen Raum erstellen</span>
              </button>
            </Link>
          </div>
        </div>

        {/* Room List */}
        <RoomList rooms={rooms} onSelectRoom={handleSelectRoom} searchTerm={searchFilter} />
      </main>
    </div>
  );
}