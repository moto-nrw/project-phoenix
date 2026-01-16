import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { fetchRooms, type RoomsApiResponse } from "./rooms-api";
import * as apiClientModule from "./api-client";
import type { BackendRoom } from "./rooms-helpers";
import type { AxiosResponse } from "axios";

// Mock the api-client module
vi.mock("./api-client", () => ({
  apiGet: vi.fn(),
}));

// Sample backend room for testing (matches rooms-helpers.ts BackendRoom)
const sampleBackendRoom: BackendRoom = {
  id: 1,
  name: "Room 101",
  building: "Building A",
  floor: 2,
  capacity: 30,
  category: "classroom",
  color: "#FF0000",
  created_at: "2024-01-15T10:00:00Z",
  updated_at: "2024-01-15T12:00:00Z",
};

// Helper to create mock AxiosResponse
function createAxiosResponse<T>(data: T): AxiosResponse<T> {
  return {
    data,
    status: 200,
    statusText: "OK",
    headers: {},
    config: {} as AxiosResponse["config"],
  };
}

// Type for mocked fetch function
type MockedFetch = ReturnType<typeof vi.fn<typeof fetch>>;

describe("fetchRooms", () => {
  let originalWindow: typeof globalThis.window;
  let consoleErrorSpy: ReturnType<typeof vi.spyOn>;
  const mockedApiGet = vi.mocked(apiClientModule.apiGet);

  beforeEach(() => {
    originalWindow = globalThis.window;
    // eslint-disable-next-line @typescript-eslint/no-empty-function
    consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    vi.clearAllMocks();
  });

  afterEach(() => {
    // Restore window
    if (originalWindow === undefined) {
      // @ts-expect-error - Intentionally deleting window to simulate server
      delete globalThis.window;
    } else {
      globalThis.window = originalWindow;
    }
    // eslint-disable-next-line @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-member-access
    consoleErrorSpy.mockRestore();
  });

  describe("server-side (SSR)", () => {
    beforeEach(() => {
      // Simulate server-side by removing window
      // @ts-expect-error - Intentionally deleting window to simulate server
      delete globalThis.window;
    });

    it("fetches rooms using apiGet when on server", async () => {
      // apiGet returns AxiosResponse<RoomsApiResponse>
      // response.data is RoomsApiResponse { status, data: BackendRoom[] }
      // response.data.data is the actual BackendRoom[]
      const roomsArray = [
        sampleBackendRoom,
        { ...sampleBackendRoom, id: 2, name: "Room 102" },
      ];
      const apiResponse: RoomsApiResponse = {
        status: "success",
        data: roomsArray,
      };
      mockedApiGet.mockResolvedValueOnce(createAxiosResponse(apiResponse));

      const result = await fetchRooms("test-token");

      expect(mockedApiGet).toHaveBeenCalledWith("/api/rooms", "test-token");
      expect(result).toHaveLength(2);
      expect(result[0]?.id).toBe("1");
      expect(result[0]?.name).toBe("Room 101");
      expect(result[1]?.id).toBe("2");
    });

    it("returns empty array when apiGet returns null", async () => {
      mockedApiGet.mockResolvedValueOnce(
        null as unknown as AxiosResponse<RoomsApiResponse>,
      );

      const result = await fetchRooms("test-token");

      expect(result).toEqual([]);
      // When response is null, response?.data is undefined
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        "Failed to fetch rooms:",
        undefined,
      );
    });

    it("returns empty array when apiGet response data.data is not array", async () => {
      // response.data is RoomsApiResponse, response.data.data should be array
      mockedApiGet.mockResolvedValueOnce(
        createAxiosResponse({
          status: "success",
          data: "not-an-array",
        } as unknown as RoomsApiResponse),
      );

      const result = await fetchRooms("test-token");

      expect(result).toEqual([]);
      expect(consoleErrorSpy).toHaveBeenCalled();
    });

    it("returns empty array when apiGet throws an error", async () => {
      mockedApiGet.mockRejectedValueOnce(new Error("Network error"));

      const result = await fetchRooms("test-token");

      expect(result).toEqual([]);
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        "Error fetching rooms:",
        expect.any(Error),
      );
    });

    it("works without token (public access)", async () => {
      // apiGet returns AxiosResponse<RoomsApiResponse>
      const apiResponse: RoomsApiResponse = {
        status: "success",
        data: [sampleBackendRoom],
      };
      mockedApiGet.mockResolvedValueOnce(createAxiosResponse(apiResponse));

      const result = await fetchRooms();

      expect(mockedApiGet).toHaveBeenCalledWith("/api/rooms", undefined);
      expect(result).toHaveLength(1);
    });
  });

  describe("client-side (browser)", () => {
    let originalFetch: typeof fetch;
    let mockFetch: MockedFetch;

    beforeEach(() => {
      // Simulate client-side by setting window
      // @ts-expect-error - Partial window for testing
      globalThis.window = { location: { href: "http://localhost:3000" } };
      originalFetch = globalThis.fetch;
      mockFetch = vi.fn();
      globalThis.fetch = mockFetch;
    });

    afterEach(() => {
      globalThis.fetch = originalFetch;
    });

    it("fetches rooms using fetch when on client", async () => {
      const mockResponse: RoomsApiResponse = {
        status: "success",
        data: [sampleBackendRoom],
      };
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      } as Response);

      const result = await fetchRooms("test-token");

      expect(mockFetch).toHaveBeenCalledWith("/api/rooms", {
        headers: {
          Authorization: "Bearer test-token",
        },
      });
      expect(result).toHaveLength(1);
      expect(result[0]?.id).toBe("1");
    });

    it("returns empty array when fetch response is not ok", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 500,
      } as Response);

      const result = await fetchRooms("test-token");

      expect(result).toEqual([]);
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        "Failed to fetch rooms:",
        500,
      );
    });

    it("returns empty array when response data is invalid", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ status: "success", data: null }),
      } as Response);

      const result = await fetchRooms("test-token");

      expect(result).toEqual([]);
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        "Invalid rooms response:",
        expect.anything(),
      );
    });

    it("sends empty Authorization header when no token", async () => {
      const mockResponse: RoomsApiResponse = {
        status: "success",
        data: [sampleBackendRoom],
      };
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      } as Response);

      await fetchRooms();

      expect(mockFetch).toHaveBeenCalledWith("/api/rooms", {
        headers: {
          Authorization: "",
        },
      });
    });

    it("returns empty array when fetch throws an error", async () => {
      mockFetch.mockRejectedValueOnce(new Error("Network error"));

      const result = await fetchRooms("test-token");

      expect(result).toEqual([]);
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        "Error fetching rooms:",
        expect.any(Error),
      );
    });

    it("correctly maps backend room data to frontend format", async () => {
      const mockResponse: RoomsApiResponse = {
        status: "success",
        data: [sampleBackendRoom],
      };
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      } as Response);

      const result = await fetchRooms("test-token");

      // Verify mapping from BackendRoom to Room
      expect(result[0]).toMatchObject({
        id: "1", // int64 → string
        name: "Room 101",
        building: "Building A",
        floor: 2,
        capacity: 30,
        category: "classroom",
        color: "#FF0000",
      });
    });
  });
});
