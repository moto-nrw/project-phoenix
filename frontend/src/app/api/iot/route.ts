import type { NextRequest } from "next/server";
import { createGetHandler, createPostHandler } from '@/lib/route-wrapper';
import { apiGet, apiPost } from '@/lib/api-client';
import type { BackendDevice, Device } from '@/lib/iot-helpers';
import { mapDeviceResponse } from '@/lib/iot-helpers';

/**
 * Type definition for paginated response that the database page expects
 */
interface PaginatedDevicesResponse {
  data: Device[];
  pagination: {
    current_page: number;
    page_size: number;
    total_pages: number;
    total_records: number;
  };
}

export const GET = createGetHandler(async (request: NextRequest, token: string): Promise<PaginatedDevicesResponse> => {
  const response = await apiGet('/api/iot', token);
  
  // Handle null or undefined response
  if (!response) {
    return {
      data: [],
      pagination: {
        current_page: 1,
        page_size: 50,
        total_pages: 0,
        total_records: 0
      }
    };
  }
  
  // apiGet returns AxiosResponse, so we need response.data to get the actual data
  const actualData = response.data;
  
  // Check for the backend response structure { status: "success", data: [...] }
  if (actualData && 'data' in actualData && Array.isArray(actualData.data)) {
    // Map the backend response format to the frontend format
    const mappedDevices = actualData.data.map((device: BackendDevice) => {
      const mapped = mapDeviceResponse(device);
      return mapped;
    });
    
    return {
      data: mappedDevices,
      pagination: {
        current_page: 1,
        page_size: mappedDevices.length,
        total_pages: 1,
        total_records: mappedDevices.length
      }
    };
  }
  
  // If the response doesn't have the expected structure, return empty
  return {
    data: [],
    pagination: {
      current_page: 1,
      page_size: 50,
      total_pages: 0,
      total_records: 0
    }
  };
});

export const POST = createPostHandler(async (request, body, token) => {
  const response = await apiPost('/api/iot', body, token);
  
  // apiPost returns AxiosResponse, so we need response.data to get the actual data
  const actualData = response.data;
  
  // Check for the backend response structure { status: "success", data: {...} }
  if (actualData && 'data' in actualData) {
    return mapDeviceResponse(actualData.data as BackendDevice);
  }
  
  throw new Error('Invalid response format');
});