// Room API client functions

import { apiGet } from './api-client';
import { type BackendRoom, mapRoomsResponse, type Room } from './rooms-helpers';

export interface RoomsApiResponse {
  status: string;
  data: BackendRoom[];
  message?: string;
}

export async function fetchRooms(token?: string): Promise<Room[]> {
  try {
    const response = await apiGet<RoomsApiResponse>('/rooms', token);
    
    if (!response || !Array.isArray(response.data)) {
      console.error('Failed to fetch rooms:', response);
      return [];
    }
    
    return mapRoomsResponse(response.data);
  } catch (error) {
    console.error('Error fetching rooms:', error);
    return [];
  }
}