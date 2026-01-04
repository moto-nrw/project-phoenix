/**
 * IoT Device Helpers
 * Type definitions and mapping functions for IoT device management
 */

// Device interface matching backend structure
export interface Device {
  id: string;
  device_id: string;
  device_type: string;
  name?: string;
  status: "active" | "inactive" | "maintenance" | "offline";
  last_seen?: string;
  registered_by_id?: string;
  is_online: boolean;
  created_at: string;
  updated_at: string;
  api_key?: string; // Only present during creation
}

// Backend device type for API responses (matches DeviceResponse from backend)
export interface BackendDevice {
  id: number;
  device_id: string;
  device_type: string;
  name?: string;
  status: string;
  last_seen?: string;
  registered_by_id?: number;
  is_online: boolean;
  created_at: string;
  updated_at: string;
  api_key?: string; // Only present during creation
}

// Device creation request
export interface CreateDeviceRequest {
  device_id: string;
  device_type: string;
  name?: string;
  status?: string;
  registered_by_id?: number;
}

// Device update request
export interface UpdateDeviceRequest {
  device_id?: string;
  device_type?: string;
  name?: string;
  status?: string;
}

/**
 * Map backend device response to frontend Device interface
 */
export function mapDeviceResponse(data: BackendDevice): Device {
  if (!data || typeof data !== "object") {
    throw new Error("Invalid device data provided to mapper");
  }

  if (!data.id || !data.device_id || !data.device_type) {
    throw new Error("Required device fields are missing");
  }

  return {
    id: data.id.toString(),
    device_id: data.device_id,
    device_type: data.device_type,
    name: data.name,
    status: data.status as Device["status"],
    last_seen: data.last_seen,
    registered_by_id: data.registered_by_id?.toString(),
    is_online: data.is_online ?? false,
    created_at: data.created_at,
    updated_at: data.updated_at,
    api_key: data.api_key,
  };
}

/**
 * Prepare device data for backend API submission
 */
export function prepareDeviceForBackend(
  data: Partial<Device>,
): CreateDeviceRequest | UpdateDeviceRequest {
  return {
    device_id: data.device_id!,
    device_type: data.device_type!,
    name: data.name,
    status: data.status,
    registered_by_id: data.registered_by_id
      ? Number.parseInt(data.registered_by_id)
      : undefined,
  };
}

/**
 * Get device type display name in German
 */
export function getDeviceTypeDisplayName(deviceType: string): string {
  const typeMap: Record<string, string> = {
    rfid_reader: "RFID-Leser",
    scanner: "Scanner",
    tablet: "Tablet",
    sensor: "Sensor",
    camera: "Kamera",
    beacon: "Beacon",
  };

  return typeMap[deviceType] ?? deviceType;
}

/**
 * Get device status display name in German
 */
export function getDeviceStatusDisplayName(status: string): string {
  const statusMap: Record<string, string> = {
    active: "Aktiv",
    inactive: "Inaktiv",
    maintenance: "Wartung",
    offline: "Offline",
  };

  return statusMap[status] ?? status;
}

/**
 * Get device status color classes
 */
export function getDeviceStatusColor(status: string): string {
  const colorMap: Record<string, string> = {
    active: "bg-green-100 text-green-800",
    inactive: "bg-gray-100 text-gray-800",
    maintenance: "bg-yellow-100 text-yellow-800",
    offline: "bg-red-100 text-red-800",
  };

  return colorMap[status] ?? "bg-gray-100 text-gray-800";
}

/**
 * Get color classes for online devices
 */
export function getOnlineDeviceColor(): string {
  return "bg-green-100 text-green-800";
}

/**
 * Get color classes for offline devices
 */
export function getOfflineDeviceColor(): string {
  return "bg-red-100 text-red-800";
}

/**
 * Format last seen timestamp for display
 */
export function formatLastSeen(lastSeen?: string): string {
  if (!lastSeen) return "Nie";

  const date = new Date(lastSeen);
  return date.toLocaleString("de-DE", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

/**
 * Get device type emoji icon
 */
export function getDeviceTypeEmoji(deviceType: string): string {
  const emojiMap: Record<string, string> = {
    rfid_reader: "📡",
    scanner: "📷",
    tablet: "💻",
    sensor: "🔍",
    camera: "📹",
    beacon: "📶",
  };

  return emojiMap[deviceType] ?? "🔧";
}

/**
 * Generate default device name based on type and ID
 */
export function generateDefaultDeviceName(
  deviceType: string,
  deviceId: string,
): string {
  const typeNames: Record<string, string> = {
    rfid_reader: "RFID-Leser",
    scanner: "Scanner",
    tablet: "Tablet",
    sensor: "Sensor",
    camera: "Kamera",
    beacon: "Beacon",
  };

  const typeName = typeNames[deviceType] ?? "Gerät";
  return `${typeName} ${deviceId}`;
}
