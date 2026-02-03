// Settings API client functions

import { apiGet, apiPost, apiPut } from "./api-helpers";
import type {
  BackendSettingTab,
  BackendTabSettingsResponse,
  BackendSettingDefinition,
  BackendResolvedSetting,
  BackendObjectRefOption,
  BackendSettingAuditEntry,
  SettingTab,
  TabSettingsResponse,
  SettingDefinition,
  ObjectRefOption,
  SettingAuditEntry,
  SetValueRequest,
  DeleteValueRequest,
  RestoreValueRequest,
  PurgeRequest,
  ResolvedSetting,
} from "./settings-helpers";
import {
  mapSettingTab,
  mapTabSettingsResponse,
  mapSettingDefinition,
  mapResolvedSetting,
  mapObjectRefOption,
  mapSettingAuditEntry,
} from "./settings-helpers";

// API response types (match backend wrapper)
interface ApiWrapper<T> {
  status: string;
  data: T;
  message?: string;
}

/**
 * Fetches available settings tabs for the current user
 */
export async function fetchSettingsTabs(token: string): Promise<SettingTab[]> {
  try {
    const response = await apiGet<ApiWrapper<BackendSettingTab[]>>(
      "/api/settings/tabs",
      token,
    );

    if (!response?.data || !Array.isArray(response.data)) {
      console.error("Failed to fetch settings tabs:", response);
      return [];
    }

    return response.data.map(mapSettingTab);
  } catch (error) {
    console.error("Error fetching settings tabs:", error);
    return [];
  }
}

/**
 * Fetches settings for a specific tab
 */
export async function fetchTabSettings(
  tab: string,
  token: string,
  deviceId?: string,
): Promise<TabSettingsResponse | null> {
  try {
    const params = new URLSearchParams();
    if (deviceId) {
      params.set("device_id", deviceId);
    }

    const endpoint = `/api/settings/tabs/${tab}${params.toString() ? `?${params.toString()}` : ""}`;
    const response = await apiGet<ApiWrapper<BackendTabSettingsResponse>>(
      endpoint,
      token,
    );

    if (!response?.data) {
      console.error("Failed to fetch tab settings:", response);
      return null;
    }

    return mapTabSettingsResponse(response.data);
  } catch (error) {
    console.error("Error fetching tab settings:", error);
    return null;
  }
}

/**
 * Fetches all setting definitions (admin only)
 */
export async function fetchSettingDefinitions(
  token: string,
): Promise<SettingDefinition[]> {
  try {
    const response = await apiGet<ApiWrapper<BackendSettingDefinition[]>>(
      "/api/settings/definitions",
      token,
    );

    if (!response?.data || !Array.isArray(response.data)) {
      console.error("Failed to fetch setting definitions:", response);
      return [];
    }

    return response.data.map(mapSettingDefinition);
  } catch (error) {
    console.error("Error fetching setting definitions:", error);
    return [];
  }
}

/**
 * Fetches the effective value for a specific setting
 */
export async function fetchSettingValue(
  key: string,
  token: string,
  deviceId?: string,
): Promise<ResolvedSetting | null> {
  try {
    const params = new URLSearchParams();
    if (deviceId) {
      params.set("device_id", deviceId);
    }

    const endpoint = `/api/settings/values/${encodeURIComponent(key)}${params.toString() ? `?${params.toString()}` : ""}`;
    const response = await apiGet<ApiWrapper<BackendResolvedSetting>>(
      endpoint,
      token,
    );

    if (!response?.data) {
      console.error("Failed to fetch setting value:", response);
      return null;
    }

    return mapResolvedSetting(response.data);
  } catch (error) {
    console.error("Error fetching setting value:", error);
    return null;
  }
}

/**
 * Sets a setting value at a specific scope
 */
export async function setSettingValue(
  key: string,
  request: SetValueRequest,
  token: string,
): Promise<boolean> {
  try {
    const body = {
      value: request.value,
      scope: request.scope,
      scope_id: request.scopeId ? parseInt(request.scopeId, 10) : undefined,
    };

    await apiPut(
      `/api/settings/values/${encodeURIComponent(key)}`,
      token,
      body,
    );

    return true;
  } catch (error) {
    console.error("Error setting setting value:", error);
    return false;
  }
}

/**
 * Deletes a setting override at a specific scope
 * Note: Uses POST to /delete endpoint since apiDelete doesn't support body
 */
export async function deleteSettingValue(
  key: string,
  request: DeleteValueRequest,
  token: string,
): Promise<boolean> {
  try {
    const body = {
      scope: request.scope,
      scope_id: request.scopeId ? parseInt(request.scopeId, 10) : undefined,
    };

    // Since the backend expects DELETE with a body, we need to use fetch directly
    // or modify the backend. For now, use fetch on client side.
    const { env } = await import("~/env");
    const response = await fetch(
      `${env.NEXT_PUBLIC_API_URL}/api/settings/values/${encodeURIComponent(key)}`,
      {
        method: "DELETE",
        headers: {
          Authorization: `Bearer ${token}`,
          "Content-Type": "application/json",
        },
        body: JSON.stringify(body),
      },
    );

    return response.ok;
  } catch (error) {
    console.error("Error deleting setting value:", error);
    return false;
  }
}

/**
 * Fetches object reference options for a setting
 */
export async function fetchObjectRefOptions(
  key: string,
  token: string,
  deviceId?: string,
): Promise<ObjectRefOption[]> {
  try {
    const params = new URLSearchParams();
    if (deviceId) {
      params.set("device_id", deviceId);
    }

    const endpoint = `/api/settings/values/${encodeURIComponent(key)}/options${params.toString() ? `?${params.toString()}` : ""}`;
    const response = await apiGet<ApiWrapper<BackendObjectRefOption[]>>(
      endpoint,
      token,
    );

    if (!response?.data || !Array.isArray(response.data)) {
      console.error("Failed to fetch object ref options:", response);
      return [];
    }

    return response.data.map(mapObjectRefOption);
  } catch (error) {
    console.error("Error fetching object ref options:", error);
    return [];
  }
}

/**
 * Fetches the change history for a setting
 */
export async function fetchSettingHistory(
  key: string,
  token: string,
  limit?: number,
): Promise<SettingAuditEntry[]> {
  try {
    const params = new URLSearchParams();
    if (limit) {
      params.set("limit", limit.toString());
    }

    const endpoint = `/api/settings/values/${encodeURIComponent(key)}/history${params.toString() ? `?${params.toString()}` : ""}`;
    const response = await apiGet<
      ApiWrapper<{ key: string; entries: BackendSettingAuditEntry[] }>
    >(endpoint, token);

    if (!response?.data?.entries) {
      console.error("Failed to fetch setting history:", response);
      return [];
    }

    return response.data.entries.map(mapSettingAuditEntry);
  } catch (error) {
    console.error("Error fetching setting history:", error);
    return [];
  }
}

/**
 * Fetches recent changes across all settings (admin only)
 */
export async function fetchRecentChanges(
  token: string,
  limit?: number,
): Promise<SettingAuditEntry[]> {
  try {
    const params = new URLSearchParams();
    if (limit) {
      params.set("limit", limit.toString());
    }

    const endpoint = `/api/settings/audit/recent${params.toString() ? `?${params.toString()}` : ""}`;
    const response = await apiGet<ApiWrapper<BackendSettingAuditEntry[]>>(
      endpoint,
      token,
    );

    if (!response?.data || !Array.isArray(response.data)) {
      console.error("Failed to fetch recent changes:", response);
      return [];
    }

    return response.data.map(mapSettingAuditEntry);
  } catch (error) {
    console.error("Error fetching recent changes:", error);
    return [];
  }
}

/**
 * Syncs setting definitions from code to database (admin only)
 */
export async function syncSettingDefinitions(
  token: string,
): Promise<number | null> {
  try {
    const response = await apiPost<ApiWrapper<{ synced: number }>>(
      "/api/settings/sync",
      token,
      {},
    );

    if (!response?.data) {
      console.error("Failed to sync definitions:", response);
      return null;
    }

    return response.data.synced;
  } catch (error) {
    console.error("Error syncing definitions:", error);
    return null;
  }
}

/**
 * Restores a soft-deleted setting value (admin only)
 */
export async function restoreSettingValue(
  key: string,
  request: RestoreValueRequest,
  token: string,
): Promise<boolean> {
  try {
    const body = {
      scope: request.scope,
      scope_id: request.scopeId ? parseInt(request.scopeId, 10) : undefined,
    };

    await apiPost(
      `/api/settings/values/${encodeURIComponent(key)}/restore`,
      token,
      body,
    );

    return true;
  } catch (error) {
    console.error("Error restoring setting value:", error);
    return false;
  }
}

/**
 * Purges old deleted records (admin only)
 */
export async function purgeDeletedRecords(
  request: PurgeRequest,
  token: string,
): Promise<{ purged: number; days: number } | null> {
  try {
    const response = await apiPost<
      ApiWrapper<{ purged: number; days: number }>
    >("/api/settings/purge", token, request);

    if (!response?.data) {
      console.error("Failed to purge records:", response);
      return null;
    }

    return response.data;
  } catch (error) {
    console.error("Error purging records:", error);
    return null;
  }
}

// ========== Client-side API functions (for use in components) ==========

/**
 * Client-side: Fetches settings tabs
 */
export async function clientFetchSettingsTabs(): Promise<SettingTab[]> {
  try {
    const response = await fetch("/api/settings/tabs");
    if (!response.ok) {
      console.error("Failed to fetch settings tabs:", response.status);
      return [];
    }

    const data = (await response.json()) as ApiWrapper<BackendSettingTab[]>;
    if (!data?.data || !Array.isArray(data.data)) {
      return [];
    }

    return data.data.map(mapSettingTab);
  } catch (error) {
    console.error("Error fetching settings tabs:", error);
    return [];
  }
}

/**
 * Client-side: Fetches tab settings
 */
export async function clientFetchTabSettings(
  tab: string,
  deviceId?: string,
): Promise<TabSettingsResponse | null> {
  try {
    const params = new URLSearchParams();
    if (deviceId) {
      params.set("device_id", deviceId);
    }

    const url = `/api/settings/tabs/${tab}${params.toString() ? `?${params.toString()}` : ""}`;
    const response = await fetch(url);

    if (!response.ok) {
      console.error("Failed to fetch tab settings:", response.status);
      return null;
    }

    const data = (await response.json()) as ApiWrapper<BackendTabSettingsResponse>;
    if (!data?.data) {
      return null;
    }

    return mapTabSettingsResponse(data.data);
  } catch (error) {
    console.error("Error fetching tab settings:", error);
    return null;
  }
}

/**
 * Client-side: Sets a setting value
 */
export async function clientSetSettingValue(
  key: string,
  request: SetValueRequest,
): Promise<boolean> {
  try {
    const body = {
      value: request.value,
      scope: request.scope,
      scope_id: request.scopeId ? parseInt(request.scopeId, 10) : undefined,
    };

    const response = await fetch(
      `/api/settings/values/${encodeURIComponent(key)}`,
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      },
    );

    return response.ok;
  } catch (error) {
    console.error("Error setting setting value:", error);
    return false;
  }
}

/**
 * Client-side: Fetches object reference options
 */
export async function clientFetchObjectRefOptions(
  key: string,
  deviceId?: string,
): Promise<ObjectRefOption[]> {
  try {
    const params = new URLSearchParams();
    if (deviceId) {
      params.set("device_id", deviceId);
    }

    const url = `/api/settings/values/${encodeURIComponent(key)}/options${params.toString() ? `?${params.toString()}` : ""}`;
    const response = await fetch(url);

    if (!response.ok) {
      console.error("Failed to fetch object ref options:", response.status);
      return [];
    }

    const data = (await response.json()) as ApiWrapper<BackendObjectRefOption[]>;
    if (!data?.data || !Array.isArray(data.data)) {
      return [];
    }

    return data.data.map(mapObjectRefOption);
  } catch (error) {
    console.error("Error fetching object ref options:", error);
    return [];
  }
}

/**
 * Client-side: Fetches a specific setting value
 */
export async function clientFetchSettingValue(
  key: string,
): Promise<ResolvedSetting | null> {
  try {
    const url = `/api/settings/values/${encodeURIComponent(key)}`;
    const response = await fetch(url);

    if (!response.ok) {
      console.error("Failed to fetch setting value:", response.status);
      return null;
    }

    // Response is double-wrapped:
    // { success: true, data: { status: "success", data: BackendResolvedSetting } }
    const outerResponse = (await response.json()) as {
      success?: boolean;
      data?: ApiWrapper<BackendResolvedSetting>;
    };

    // Extract the actual setting data from the nested structure
    const backendData = outerResponse?.data?.data;
    if (!backendData) {
      console.error("Failed to extract setting data from response:", outerResponse);
      return null;
    }

    const mapped = mapResolvedSetting(backendData);
    return mapped;
  } catch (error) {
    console.error("Error fetching setting value:", error);
    return null;
  }
}
