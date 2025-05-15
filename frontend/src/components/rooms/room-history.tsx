"use client";

import { useState, useEffect } from "react";
import { getSession } from "next-auth/react";
import type { Room } from "@/lib/api";
import type { RoomHistoryEntry, BackendRoomHistoryEntry } from "@/lib/room-history-helpers";
import { formatDate, formatDuration } from "@/lib/room-history-helpers";

interface RoomHistoryProps {
  roomId: string;
  dateRange?: {
    start: Date;
    end: Date;
  };
}

interface BackendRoomResponse {
  id: number;
  name: string;
  room_name?: string;
  building?: string;
  floor: number;
  capacity: number;
  category: string;
  color: string;
  device_id?: string;
  is_occupied: boolean;
  activity_name?: string;
  group_name?: string;
  supervisor_name?: string;
  student_count?: number;
  created_at: string;
  updated_at: string;
}

export function RoomHistory({ roomId, dateRange }: RoomHistoryProps) {
  const [historyData, setHistoryData] = useState<RoomHistoryEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [room, setRoom] = useState<Room | null>(null);

  // Fetch history data
  useEffect(() => {
    void (async function fetchRoomHistory() {
      try {
        setLoading(true);
        setError(null);
        
        // Build query parameters
        let url = `/api/rooms/${roomId}/history`;
        const params = new URLSearchParams();
        
        if (dateRange?.start) {
          params.append('start_date', dateRange.start.toISOString());
        }
        
        if (dateRange?.end) {
          params.append('end_date', dateRange.end.toISOString());
        }
        
        if (params.toString()) {
          url += `?${params.toString()}`;
        }
        
        // Fetch room history data
        const session = await getSession();
        const historyResponse = await fetch(url, {
          credentials: "include",
          headers: session?.user?.token
            ? {
                Authorization: `Bearer ${session.user.token}`,
                "Content-Type": "application/json",
              }
            : undefined,
        });

        if (!historyResponse.ok) {
          throw new Error(`Failed to fetch room history: ${historyResponse.status}`);
        }

        // Safely parse the response
        const historyResponseData = await historyResponse.json() as unknown;
        
        // Check if response follows ApiResponse format or is a direct array
        // Type-safe approach to handle different response formats
        const backendEntries = Array.isArray(historyResponseData) 
          ? historyResponseData as BackendRoomHistoryEntry[] 
          : (
              typeof historyResponseData === 'object' && 
              historyResponseData !== null && 
              'data' in historyResponseData && 
              Array.isArray((historyResponseData as Record<string, unknown>).data)
            )
            ? (historyResponseData as { data: BackendRoomHistoryEntry[] }).data
            : null;
        
        if (!backendEntries) {
          throw new Error("Invalid response format from room history API");
        }
        
        // Map backend types to frontend types using our helper function
        const frontendHistoryData = backendEntries.map((entry: BackendRoomHistoryEntry): RoomHistoryEntry => {
          if (typeof entry.id !== 'number' || typeof entry.room_id !== 'number' || 
              typeof entry.date !== 'string' || typeof entry.group_name !== 'string' ||
              typeof entry.student_count !== 'number' || typeof entry.duration !== 'number') {
            throw new Error("Invalid entry data in room history response");
          }
          
          return {
            id: String(entry.id),
            roomId: String(entry.room_id),
            date: entry.date,
            groupName: entry.group_name,
            activityName: entry.activity_name,
            supervisorName: entry.supervisor_name,
            studentCount: entry.student_count,
            duration: entry.duration
          };
        });
        
        setHistoryData(frontendHistoryData);
        
        // Also fetch the room details if not already available
        if (!room) {
          const roomResponse = await fetch(`/api/rooms/${roomId}`, {
            credentials: "include",
            headers: session?.user?.token
              ? {
                  Authorization: `Bearer ${session.user.token}`,
                  "Content-Type": "application/json",
                }
              : undefined,
          });
          
          if (roomResponse.ok) {
            const roomData = await roomResponse.json() as BackendRoomResponse;
            // Map from backend properties to frontend properties
            setRoom({
              id: String(roomData.id),
              name: roomData.name ?? roomData.room_name ?? "", // Support both name formats
              building: roomData.building,
              floor: roomData.floor,
              capacity: roomData.capacity,
              category: roomData.category,
              color: roomData.color,
              deviceId: roomData.device_id,
              isOccupied: roomData.is_occupied ?? false,
              activityName: roomData.activity_name,
              groupName: roomData.group_name,
              supervisorName: roomData.supervisor_name,
              studentCount: roomData.student_count,
              createdAt: roomData.created_at,
              updatedAt: roomData.updated_at,
            });
          }
        }
      } catch (err) {
        console.error("Error fetching room history:", err);
        setError("Fehler beim Laden der Raumhistorie. Bitte versuchen Sie es später erneut.");
      } finally {
        setLoading(false);
      }
    })();
  }, [roomId, dateRange, room]);
  
  if (loading) {
    return (
      <div className="flex h-40 items-center justify-center rounded-lg bg-gray-50">
        <div className="flex animate-pulse flex-col items-center">
          <div className="h-8 w-8 rounded-full bg-gradient-to-r from-teal-400 to-blue-500"></div>
          <p className="mt-3 text-sm text-gray-500">Raumverlauf wird geladen...</p>
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
  
  if (historyData.length === 0) {
    return (
      <div className="rounded-lg bg-yellow-50 p-4 text-yellow-800">
        <p>Keine historischen Daten für diesen Raum verfügbar.</p>
      </div>
    );
  }

  return (
    <div className="mt-6 space-y-6">
      <div className="border-b border-gray-200 pb-3">
        <h3 className="text-lg font-medium text-gray-800">
          Raumnutzung: {room?.name}
        </h3>
        <p className="text-sm text-gray-500">
          Verlauf der letzten Nutzungen
        </p>
      </div>
      
      {/* Timeline view */}
      <div className="relative space-y-6 border-l-2 border-gray-200 pl-6">
        {historyData.map((entry, index) => (
          <div key={entry.id || index} className="relative">
            {/* Timeline dot */}
            <div className="absolute -left-[31px] top-1.5 h-4 w-4 rounded-full border-2 border-white bg-blue-500"></div>
            
            {/* Entry card */}
            <div className="rounded-lg border border-gray-100 bg-white p-4 shadow-sm">
              <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
                <span className="text-sm font-medium text-gray-600">
                  {formatDate(entry.date)}
                </span>
                <span className="rounded-full bg-blue-100 px-3 py-1 text-xs font-medium text-blue-800">
                  {formatDuration(entry.duration)}
                </span>
              </div>
              
              <div className="space-y-2">
                <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
                  <div className="flex items-center space-x-1 text-sm">
                    <svg xmlns="http://www.w3.org/2000/svg" className="h-4 w-4 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
                    </svg>
                    <span className="font-medium">{entry.groupName}</span>
                  </div>
                  
                  <div className="flex items-center space-x-1 text-sm">
                    <svg xmlns="http://www.w3.org/2000/svg" className="h-4 w-4 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
                    </svg>
                    <span>{entry.studentCount} Schüler</span>
                  </div>
                </div>
                
                {entry.activityName && (
                  <div className="flex items-center space-x-1 text-sm">
                    <svg xmlns="http://www.w3.org/2000/svg" className="h-4 w-4 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
                    </svg>
                    <span className="font-medium">Aktivität:</span>
                    <span>{entry.activityName}</span>
                  </div>
                )}
                
                {entry.supervisorName && (
                  <div className="flex items-center space-x-1 text-sm">
                    <svg xmlns="http://www.w3.org/2000/svg" className="h-4 w-4 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                    </svg>
                    <span className="font-medium">Aufsicht:</span>
                    <span>{entry.supervisorName}</span>
                  </div>
                )}
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}