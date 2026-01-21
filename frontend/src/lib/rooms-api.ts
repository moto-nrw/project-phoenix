// Room API client functions

import { apiGet } from "./api-client";
import { type BackendRoom, mapRoomsResponse, type Room } from "./rooms-helpers";

export interface RoomsApiResponse {
  status: string;
  data: BackendRoom[];
  message?: string;
}

// BetterAuth: Cookies are automatically included, no token needed
export async function fetchRooms(cookieHeader?: string): Promise<Room[]> {
  try {
    // Server-side: call backend directly with cookies
    if (globalThis.window === undefined) {
      const response = await apiGet<RoomsApiResponse>(
        "/api/rooms",
        cookieHeader,
      );

      // apiGet returns AxiosResponse<RoomsApiResponse>
      // response.data is RoomsApiResponse { status, data: BackendRoom[] }
      // response.data.data is the actual BackendRoom[]
      if (!response?.data || !Array.isArray(response.data.data)) {
        console.error("Failed to fetch rooms:", response?.data);
        return [];
      }

      return mapRoomsResponse(response.data.data);
    }

    // Client-side: use Next.js API route
    // BetterAuth: Cookies are sent automatically on same-origin requests
    const response = await fetch("/api/rooms", {
      credentials: "include", // Ensure cookies are sent
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
