import { describe, it, expect } from "vitest";
import {
  mapDeviceResponse,
  prepareDeviceForBackend,
  getDeviceTypeDisplayName,
  getDeviceStatusDisplayName,
  getDeviceStatusColor,
  getOnlineDeviceColor,
  getOfflineDeviceColor,
  formatLastSeen,
  getDeviceTypeEmoji,
  generateDefaultDeviceName,
  type BackendDevice,
  type Device,
} from "./iot-helpers";

describe("iot-helpers", () => {
  describe("mapDeviceResponse", () => {
    it("should map valid backend device to frontend device", () => {
      const backendDevice: BackendDevice = {
        id: 123,
        device_id: "RFID-001",
        device_type: "rfid_reader",
        name: "Main Reader",
        status: "active",
        last_seen: "2024-01-15T10:30:00Z",
        registered_by_id: 456,
        is_online: true,
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-15T10:30:00Z",
        api_key: "test-api-key",
      };

      const result = mapDeviceResponse(backendDevice);

      expect(result).toEqual({
        id: "123",
        device_id: "RFID-001",
        device_type: "rfid_reader",
        name: "Main Reader",
        status: "active",
        last_seen: "2024-01-15T10:30:00Z",
        registered_by_id: "456",
        is_online: true,
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-15T10:30:00Z",
        api_key: "test-api-key",
      });
    });

    it("should handle device without optional fields", () => {
      const backendDevice: BackendDevice = {
        id: 123,
        device_id: "RFID-001",
        device_type: "rfid_reader",
        status: "active",
        is_online: false,
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-15T10:30:00Z",
      };

      const result = mapDeviceResponse(backendDevice);

      expect(result.name).toBeUndefined();
      expect(result.last_seen).toBeUndefined();
      expect(result.registered_by_id).toBeUndefined();
      expect(result.api_key).toBeUndefined();
      expect(result.is_online).toBe(false);
    });

    it("should throw error for null input", () => {
      expect(() => mapDeviceResponse(null as unknown as BackendDevice)).toThrow(
        "Invalid device data provided to mapper",
      );
    });

    it("should throw error for undefined input", () => {
      expect(() =>
        mapDeviceResponse(undefined as unknown as BackendDevice),
      ).toThrow("Invalid device data provided to mapper");
    });

    it("should throw error for non-object input", () => {
      expect(() =>
        mapDeviceResponse("invalid" as unknown as BackendDevice),
      ).toThrow("Invalid device data provided to mapper");
    });

    it("should throw error when id is missing", () => {
      const backendDevice = {
        device_id: "RFID-001",
        device_type: "rfid_reader",
        status: "active",
        is_online: true,
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-15T10:30:00Z",
      } as unknown as BackendDevice;

      expect(() => mapDeviceResponse(backendDevice)).toThrow(
        "Required device fields are missing",
      );
    });

    it("should throw error when device_id is missing", () => {
      const backendDevice = {
        id: 123,
        device_type: "rfid_reader",
        status: "active",
        is_online: true,
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-15T10:30:00Z",
      } as unknown as BackendDevice;

      expect(() => mapDeviceResponse(backendDevice)).toThrow(
        "Required device fields are missing",
      );
    });

    it("should throw error when device_type is missing", () => {
      const backendDevice = {
        id: 123,
        device_id: "RFID-001",
        status: "active",
        is_online: true,
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-15T10:30:00Z",
      } as unknown as BackendDevice;

      expect(() => mapDeviceResponse(backendDevice)).toThrow(
        "Required device fields are missing",
      );
    });

    it("should handle is_online as false when undefined", () => {
      const backendDevice = {
        id: 123,
        device_id: "RFID-001",
        device_type: "rfid_reader",
        status: "active",
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-15T10:30:00Z",
      } as BackendDevice;

      const result = mapDeviceResponse(backendDevice);
      expect(result.is_online).toBe(false);
    });
  });

  describe("prepareDeviceForBackend", () => {
    it("should prepare device data with all fields", () => {
      const device: Partial<Device> = {
        device_id: "RFID-001",
        device_type: "rfid_reader",
        name: "Main Reader",
        status: "active",
        registered_by_id: "456",
      };

      const result = prepareDeviceForBackend(device);

      expect(result).toEqual({
        device_id: "RFID-001",
        device_type: "rfid_reader",
        name: "Main Reader",
        status: "active",
        registered_by_id: 456,
      });
    });

    it("should handle device without registered_by_id", () => {
      const device: Partial<Device> = {
        device_id: "RFID-001",
        device_type: "rfid_reader",
        name: "Main Reader",
        status: "active",
      };

      const result = prepareDeviceForBackend(device);

      expect(result).toEqual({
        device_id: "RFID-001",
        device_type: "rfid_reader",
        name: "Main Reader",
        status: "active",
        registered_by_id: undefined,
      });
    });

    it("should handle device with minimal fields", () => {
      const device: Partial<Device> = {
        device_id: "RFID-001",
        device_type: "rfid_reader",
      };

      const result = prepareDeviceForBackend(device);

      expect(result).toEqual({
        device_id: "RFID-001",
        device_type: "rfid_reader",
        name: undefined,
        status: undefined,
        registered_by_id: undefined,
      });
    });
  });

  describe("getDeviceTypeDisplayName", () => {
    it("should return German name for rfid_reader", () => {
      expect(getDeviceTypeDisplayName("rfid_reader")).toBe("RFID-Leser");
    });

    it("should return German name for scanner", () => {
      expect(getDeviceTypeDisplayName("scanner")).toBe("Scanner");
    });

    it("should return German name for tablet", () => {
      expect(getDeviceTypeDisplayName("tablet")).toBe("Tablet");
    });

    it("should return German name for sensor", () => {
      expect(getDeviceTypeDisplayName("sensor")).toBe("Sensor");
    });

    it("should return German name for camera", () => {
      expect(getDeviceTypeDisplayName("camera")).toBe("Kamera");
    });

    it("should return German name for beacon", () => {
      expect(getDeviceTypeDisplayName("beacon")).toBe("Beacon");
    });

    it("should return original type for unknown device type", () => {
      expect(getDeviceTypeDisplayName("unknown_type")).toBe("unknown_type");
    });
  });

  describe("getDeviceStatusDisplayName", () => {
    it("should return German name for active status", () => {
      expect(getDeviceStatusDisplayName("active")).toBe("Aktiv");
    });

    it("should return German name for inactive status", () => {
      expect(getDeviceStatusDisplayName("inactive")).toBe("Inaktiv");
    });

    it("should return German name for maintenance status", () => {
      expect(getDeviceStatusDisplayName("maintenance")).toBe("Wartung");
    });

    it("should return German name for offline status", () => {
      expect(getDeviceStatusDisplayName("offline")).toBe("Offline");
    });

    it("should return original status for unknown status", () => {
      expect(getDeviceStatusDisplayName("unknown_status")).toBe(
        "unknown_status",
      );
    });
  });

  describe("getDeviceStatusColor", () => {
    it("should return green classes for active status", () => {
      expect(getDeviceStatusColor("active")).toBe(
        "bg-green-100 text-green-800",
      );
    });

    it("should return gray classes for inactive status", () => {
      expect(getDeviceStatusColor("inactive")).toBe(
        "bg-gray-100 text-gray-800",
      );
    });

    it("should return yellow classes for maintenance status", () => {
      expect(getDeviceStatusColor("maintenance")).toBe(
        "bg-yellow-100 text-yellow-800",
      );
    });

    it("should return red classes for offline status", () => {
      expect(getDeviceStatusColor("offline")).toBe("bg-red-100 text-red-800");
    });

    it("should return default gray classes for unknown status", () => {
      expect(getDeviceStatusColor("unknown")).toBe("bg-gray-100 text-gray-800");
    });
  });

  describe("getOnlineDeviceColor", () => {
    it("should return green classes", () => {
      expect(getOnlineDeviceColor()).toBe("bg-green-100 text-green-800");
    });
  });

  describe("getOfflineDeviceColor", () => {
    it("should return red classes", () => {
      expect(getOfflineDeviceColor()).toBe("bg-red-100 text-red-800");
    });
  });

  describe("formatLastSeen", () => {
    it("should return 'Nie' for undefined", () => {
      expect(formatLastSeen(undefined)).toBe("Nie");
    });

    it("should format valid date string", () => {
      const result = formatLastSeen("2024-01-15T10:30:00Z");
      // Result format: dd.MM.yyyy, HH:mm (German locale)
      expect(result).toMatch(/\d{2}\.\d{2}\.\d{4}, \d{2}:\d{2}/);
    });

    it("should handle date at midnight", () => {
      const result = formatLastSeen("2024-01-15T00:00:00Z");
      expect(result).toMatch(/\d{2}\.\d{2}\.\d{4}, \d{2}:\d{2}/);
    });
  });

  describe("getDeviceTypeEmoji", () => {
    it("should return emoji for rfid_reader", () => {
      expect(getDeviceTypeEmoji("rfid_reader")).toBe("📡");
    });

    it("should return emoji for scanner", () => {
      expect(getDeviceTypeEmoji("scanner")).toBe("📷");
    });

    it("should return emoji for tablet", () => {
      expect(getDeviceTypeEmoji("tablet")).toBe("💻");
    });

    it("should return emoji for sensor", () => {
      expect(getDeviceTypeEmoji("sensor")).toBe("🔍");
    });

    it("should return emoji for camera", () => {
      expect(getDeviceTypeEmoji("camera")).toBe("📹");
    });

    it("should return emoji for beacon", () => {
      expect(getDeviceTypeEmoji("beacon")).toBe("📶");
    });

    it("should return wrench emoji for unknown type", () => {
      expect(getDeviceTypeEmoji("unknown")).toBe("🔧");
    });
  });

  describe("generateDefaultDeviceName", () => {
    it("should generate name for rfid_reader", () => {
      expect(generateDefaultDeviceName("rfid_reader", "001")).toBe(
        "RFID-Leser 001",
      );
    });

    it("should generate name for scanner", () => {
      expect(generateDefaultDeviceName("scanner", "002")).toBe("Scanner 002");
    });

    it("should generate name for tablet", () => {
      expect(generateDefaultDeviceName("tablet", "003")).toBe("Tablet 003");
    });

    it("should generate name for sensor", () => {
      expect(generateDefaultDeviceName("sensor", "004")).toBe("Sensor 004");
    });

    it("should generate name for camera", () => {
      expect(generateDefaultDeviceName("camera", "005")).toBe("Kamera 005");
    });

    it("should generate name for beacon", () => {
      expect(generateDefaultDeviceName("beacon", "006")).toBe("Beacon 006");
    });

    it("should generate default name for unknown type", () => {
      expect(generateDefaultDeviceName("unknown", "007")).toBe("Gerät 007");
    });
  });
});
