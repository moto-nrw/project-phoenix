import {
  createGetHandler,
  createPutHandler,
  createDeleteHandler,
} from "@/lib/route-wrapper";
import { apiGet, apiPut, apiDelete } from "~/lib/api-helpers";
import type { BackendDevice } from "@/lib/iot-helpers";
import { mapDeviceResponse } from "@/lib/iot-helpers";

export const GET = createGetHandler(async (request, token, params) => {
  const id = params.id as string;
  const response = await apiGet<{ data: BackendDevice }>(`/api/iot/${id}`, token);

  if (!response?.data) {
    throw new Error("Device not found");
  }

  return mapDeviceResponse(response.data);
});

export const PUT = createPutHandler(async (request, body, token, params) => {
  const id = params.id as string;
  const response = await apiPut<{ data: BackendDevice }>(`/api/iot/${id}`, token, body);

  if (!response?.data) {
    throw new Error("Failed to update device");
  }

  return mapDeviceResponse(response.data);
});

export const DELETE = createDeleteHandler(async (request, token, params) => {
  const id = params.id as string;
  await apiDelete(`/api/iot/${id}`, token);

  // Return success response for delete operations
  return { success: true, message: "Device deleted successfully" };
});
