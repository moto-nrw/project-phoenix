import { sessionFetch } from "./session-cache";
import { createLogger } from "./logger";

const logger = createLogger({ component: "SettingsApi" });

// --- Types ---

export interface SettingsSchema {
  tabs: SchemaTab[];
}

export interface SchemaTab {
  key: string;
  label: string;
  categories: SchemaCategory[];
}

export interface SchemaCategory {
  key: string;
  label: string;
  items: ResolvedSetting[];
}

export interface ResolvedSetting {
  key: string;
  label: string;
  description: string;
  type: "boolean" | "number" | "time" | "text" | "password" | "select";
  default: unknown;
  value: unknown;
  is_default: boolean;
  writable: boolean;
  visible: boolean;
  sort_order: number;
  validation?: {
    required?: boolean;
    min?: number;
    max?: number;
  } | null;
  depends_on?: {
    key: string;
    condition: string;
    value: unknown;
  } | null;
  options?: {
    static?: { label: string; value: unknown }[];
  } | null;
}

interface ApiResponse<T> {
  status: string;
  data: T;
  message?: string;
}

// --- API Functions ---

export async function fetchSettingsSchema(): Promise<SettingsSchema | null> {
  try {
    const response = await sessionFetch("/api/settings/schema", {
      method: "GET",
    });

    // 403 = user lacks config:read permission — expected, not an error
    if (response.status === 403) {
      return null;
    }

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    const result = (await response.json()) as ApiResponse<SettingsSchema>;
    return result.data;
  } catch (error) {
    logger.error("fetch_settings_schema_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    throw new Error("Failed to fetch settings schema");
  }
}

export async function setSettingValue(
  key: string,
  value: unknown,
): Promise<void> {
  try {
    const response = await sessionFetch(`/api/settings/values/${key}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ value }),
    });

    if (!response.ok) {
      const result = (await response.json()) as { error?: string };
      throw new Error(result.error ?? `HTTP error! status: ${response.status}`);
    }
  } catch (error) {
    logger.error("set_setting_value_failed", {
      key,
      error: error instanceof Error ? error.message : String(error),
    });
    throw error;
  }
}

export async function resetSettingValue(key: string): Promise<void> {
  try {
    const response = await sessionFetch(`/api/settings/values/${key}`, {
      method: "DELETE",
    });

    if (!response.ok && response.status !== 204) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }
  } catch (error) {
    logger.error("reset_setting_value_failed", {
      key,
      error: error instanceof Error ? error.message : String(error),
    });
    throw error;
  }
}
