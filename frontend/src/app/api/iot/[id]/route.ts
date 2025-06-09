import { createGetHandler, createPutHandler, createDeleteHandler } from '@/lib/route-wrapper';
import { apiGet, apiPut, apiDelete } from '@/lib/api-client';
import type { BackendDevice } from '@/lib/iot-helpers';
import { mapDeviceResponse } from '@/lib/iot-helpers';

export const GET = createGetHandler(async (request, token, params) => {
  const { id } = await params;
  const response = await apiGet(`/api/iot/${id}`, token);
  
  // Handle null or undefined response
  if (!response) {
    throw new Error('Device not found');
  }
  
  // apiGet returns AxiosResponse, so we need response.data to get the actual data
  const actualData = response.data;
  
  // Check for the backend response structure { status: "success", data: {...} }
  if (actualData && 'data' in actualData) {
    return mapDeviceResponse(actualData.data as BackendDevice);
  }
  
  throw new Error('Invalid response format');
});

export const PUT = createPutHandler(async (request, body, token, params) => {
  const { id } = params;
  const response = await apiPut(`/api/iot/${id}`, body, token);
  
  // Handle null or undefined response
  if (!response) {
    throw new Error('Failed to update device');
  }
  
  // apiPut returns AxiosResponse, so we need response.data to get the actual data
  const actualData = response.data;
  
  // Check for the backend response structure { status: "success", data: {...} }
  if (actualData && 'data' in actualData) {
    return mapDeviceResponse(actualData.data as BackendDevice);
  }
  
  throw new Error('Invalid response format');
});

export const DELETE = createDeleteHandler(async (request, token, params) => {
  const { id } = params;
  const response = await apiDelete(`/api/iot/${id}`, token);
  
  // Return success response for delete operations
  return { success: true, message: 'Device deleted successfully' };
});