import type { NextRequest } from "next/server";
import { createGetHandler, createPostHandler } from "@/lib/route-wrapper";
import { apiGet, apiPost } from "~/lib/api-helpers";
import type { BackendDevice, Device } from "@/lib/iot-helpers";
import { mapDeviceResponse } from "@/lib/iot-helpers";

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

export const GET = createGetHandler(
  async (
    request: NextRequest,
    token: string,
  ): Promise<PaginatedDevicesResponse> => {
    const response = await apiGet<{ data: BackendDevice[] }>("/api/iot", token);

    if (!response?.data || !Array.isArray(response.data)) {
      return {
        data: [],
        pagination: {
          current_page: 1,
          page_size: 50,
          total_pages: 0,
          total_records: 0,
        },
      };
    }

    const mappedDevices = response.data.map((device: BackendDevice) =>
      mapDeviceResponse(device),
    );

    return {
      data: mappedDevices,
      pagination: {
        current_page: 1,
        page_size: mappedDevices.length,
        total_pages: 1,
        total_records: mappedDevices.length,
      },
    };
  },
);

export const POST = createPostHandler(async (request, body, token) => {
  const response = await apiPost<{ data: BackendDevice }>("/api/iot", token, body);

  if (!response?.data) {
    throw new Error("Invalid response format");
  }

  return mapDeviceResponse(response.data);
});
