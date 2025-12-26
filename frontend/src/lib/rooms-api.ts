// Room API client functions

import { apiGet } from "./api-client";
import { type BackendRoom, mapRoomsResponse, type Room } from "./rooms-helpers";

export interface RoomsApiResponse {
  status: string;
  data: BackendRoom[];
  message?: string;
}

export async function fetchRooms(token?: string): Promise<Room[]> {
  try {
    // If we're on the client side, use the Next.js API route
    if (typeof globalThis.window !== "undefined") {
      const response = await fetch("/api/rooms", {
        headers: {
          Authorization: token ? `Bearer ${token}` : "",
        },
      });

      if (!response.ok) {
        console.error("Failed to fetch rooms:", response.status);
        return [];
      }

      const data = (await response.json()) as RoomsApiResponse;

      if (!data || !Array.isArray(data.data)) {
        console.error("Invalid rooms response:", data);
        return [];
      }

      return mapRoomsResponse(data.data);
    } else {
      // Server-side: call backend directly
      const response = await apiGet<RoomsApiResponse>("/api/rooms", token);

      if (!response || !Array.isArray(response.data)) {
        console.error("Failed to fetch rooms:", response);
        return [];
      }

      return mapRoomsResponse(response.data);
    }
  } catch (error) {
    console.error("Error fetching rooms:", error);
    return [];
  }
}
