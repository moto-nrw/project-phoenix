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
  room_id?: string;
  room_name?: string;
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
  room_id?: number;
  room_name?: string;
  is_online: boolean;
  created_at: string;
  updated_at: string;
  api_key?: string; // Only present during creation
}

// Device creation request
interface CreateDeviceRequest {
  device_id: string;
  device_type: string;
  name?: string;
  status?: string;
  registered_by_id?: number;
}

// Device update request
interface UpdateDeviceRequest {
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
    room_id: data.room_id?.toString(),
    room_name: data.room_name,
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
 * Single source of truth for device type values and their German display labels.
 * Used by both getDeviceTypeDisplayName and the operator create-device modal dropdown.
 */
export const DEVICE_TYPE_OPTIONS: Record<string, string> = {
  terminal: "Terminal",
  info_point: "Info-Point",
};

/**
 * Get device type display name in German
 */
export function getDeviceTypeDisplayName(deviceType: string): string {
  return DEVICE_TYPE_OPTIONS[deviceType] ?? deviceType;
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
    active: "bg-moto-green-soft text-moto-green-strong",
    inactive: "bg-gray-100 text-gray-800",
    maintenance: "bg-moto-amber-soft text-moto-amber-strong",
    offline: "bg-moto-red-soft text-moto-red-strong",
  };

  return colorMap[status] ?? "bg-gray-100 text-gray-800";
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
 * Format last seen timestamp as relative German time string
 */
export function formatRelativeLastSeen(lastSeen?: string): string {
  if (!lastSeen) return "Nie verbunden";

  const now = Date.now();
  const seen = new Date(lastSeen).getTime();
  const diffMs = now - seen;
  const diffMin = Math.floor(diffMs / 60_000);
  const diffHrs = Math.floor(diffMs / 3_600_000);
  const diffDays = Math.floor(diffMs / 86_400_000);

  if (diffMin < 1) return "Gerade eben";
  if (diffMin < 60) return `vor ${diffMin} Min.`;
  if (diffHrs < 24) return `vor ${diffHrs} Std.`;
  if (diffDays === 1) return "vor 1 Tag";
  return `vor ${diffDays} Tagen`;
}

/**
 * Get device type emoji icon
 */
export function getDeviceTypeEmoji(deviceType: string): string {
  const emojiMap: Record<string, string> = {
    terminal: "📡",
    info_point: "ℹ️",
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
  const typeName = DEVICE_TYPE_OPTIONS[deviceType] ?? "Gerät";
  return `${typeName} ${deviceId}`;
}
