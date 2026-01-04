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
    // Server-side: call backend directly
    if (globalThis.window === undefined) {
      const response = await apiGet<RoomsApiResponse>("/api/rooms", token);

      if (!response || !Array.isArray(response.data)) {
        console.error("Failed to fetch rooms:", response);
        return [];
      }

      return mapRoomsResponse(response.data);
    }

    // Client-side: use Next.js API route
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
  } catch (error) {
    console.error("Error fetching rooms:", error);
    return [];
  }
}
