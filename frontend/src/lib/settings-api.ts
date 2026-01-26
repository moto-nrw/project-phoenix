/**
 * API client for scoped settings
 */

import { getSession } from "next-auth/react";
import type {
  BackendResolvedSetting,
  BackendSettingChange,
  BackendSettingDefinition,
  ResolvedSetting,
  SettingChange,
  SettingDefinition,
  UpdateSettingRequest,
} from "./settings-helpers";
import {
  mapDefinitionResponse,
  mapResolvedSettingResponse,
  mapSettingChangeResponse,
} from "./settings-helpers";

const API_BASE = "/api/settings";

async function getAuthHeaders(): Promise<HeadersInit> {
  const session = await getSession();
  const token = session?.user?.token;

  if (!token) {
    throw new Error("Not authenticated");
  }

  return {
    Authorization: `Bearer ${token}`,
    "Content-Type": "application/json",
  };
}

async function handleResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    const error = (await response.json().catch(() => ({}))) as {
      message?: string;
    };
    throw new Error(
      error.message ?? `Request failed with status ${response.status}`,
    );
  }

  const json = (await response.json()) as { data: T };
  return json.data;
}

// === Definitions ===

export async function fetchDefinitions(
  filters?: Record<string, string>,
): Promise<SettingDefinition[]> {
  const params = new URLSearchParams(filters);
  const query = params.toString();
  const suffix = query ? `?${query}` : "";
  const url = `${API_BASE}/definitions${suffix}`;

  const response = await fetch(url, {
    headers: await getAuthHeaders(),
  });

  const data = await handleResponse<BackendSettingDefinition[]>(response);
  return data.map(mapDefinitionResponse);
}

export async function fetchDefinition(
  key: string,
): Promise<SettingDefinition | null> {
  const response = await fetch(`${API_BASE}/definitions/${key}`, {
    headers: await getAuthHeaders(),
  });

  if (response.status === 404) {
    return null;
  }

  const data = await handleResponse<BackendSettingDefinition>(response);
  return mapDefinitionResponse(data);
}

// === User Settings (Personal Preferences) ===

export async function fetchUserSettings(): Promise<ResolvedSetting[]> {
  const response = await fetch(`${API_BASE}/user/me`, {
    headers: await getAuthHeaders(),
  });

  const data = await handleResponse<BackendResolvedSetting[]>(response);
  return data.map(mapResolvedSettingResponse);
}

export async function updateUserSetting(
  key: string,
  value: unknown,
  reason?: string,
): Promise<void> {
  const body: UpdateSettingRequest = { value, reason };

  const response = await fetch(`${API_BASE}/user/me/${key}`, {
    method: "PUT",
    headers: await getAuthHeaders(),
    body: JSON.stringify(body),
  });

  if (!response.ok) {
    const error = (await response.json().catch(() => ({}))) as {
      message?: string;
    };
    throw new Error(error.message ?? "Failed to update setting");
  }
}

// === System Settings (Admin) ===

export async function fetchSystemSettings(): Promise<ResolvedSetting[]> {
  const response = await fetch(`${API_BASE}/system`, {
    headers: await getAuthHeaders(),
  });

  const data = await handleResponse<BackendResolvedSetting[]>(response);
  return data.map(mapResolvedSettingResponse);
}

export async function updateSystemSetting(
  key: string,
  value: unknown,
  reason?: string,
): Promise<void> {
  const body: UpdateSettingRequest = { value, reason };

  const response = await fetch(`${API_BASE}/system/${key}`, {
    method: "PUT",
    headers: await getAuthHeaders(),
    body: JSON.stringify(body),
  });

  if (!response.ok) {
    const error = (await response.json().catch(() => ({}))) as {
      message?: string;
    };
    throw new Error(error.message ?? "Failed to update setting");
  }
}

export async function resetSystemSetting(key: string): Promise<void> {
  const response = await fetch(`${API_BASE}/system/${key}`, {
    method: "DELETE",
    headers: await getAuthHeaders(),
  });

  if (!response.ok) {
    const error = (await response.json().catch(() => ({}))) as {
      message?: string;
    };
    throw new Error(error.message ?? "Failed to reset setting");
  }
}

// === OG Settings ===

export async function fetchOGSettings(
  ogId: string,
): Promise<ResolvedSetting[]> {
  const response = await fetch(`${API_BASE}/og/${ogId}`, {
    headers: await getAuthHeaders(),
  });

  const data = await handleResponse<BackendResolvedSetting[]>(response);
  return data.map(mapResolvedSettingResponse);
}

export async function updateOGSetting(
  ogId: string,
  key: string,
  value: unknown,
  reason?: string,
): Promise<void> {
  const body: UpdateSettingRequest = { value, reason };

  const response = await fetch(`${API_BASE}/og/${ogId}/${key}`, {
    method: "PUT",
    headers: await getAuthHeaders(),
    body: JSON.stringify(body),
  });

  if (!response.ok) {
    const error = (await response.json().catch(() => ({}))) as {
      message?: string;
    };
    throw new Error(error.message ?? "Failed to update setting");
  }
}

export async function resetOGSetting(ogId: string, key: string): Promise<void> {
  const response = await fetch(`${API_BASE}/og/${ogId}/${key}`, {
    method: "DELETE",
    headers: await getAuthHeaders(),
  });

  if (!response.ok) {
    const error = (await response.json().catch(() => ({}))) as {
      message?: string;
    };
    throw new Error(error.message ?? "Failed to reset setting");
  }
}

// === Initialization (Admin) ===

export async function initializeDefinitions(): Promise<void> {
  const response = await fetch(`${API_BASE}/initialize`, {
    method: "POST",
    headers: await getAuthHeaders(),
  });

  if (!response.ok) {
    const error = (await response.json().catch(() => ({}))) as {
      message?: string;
    };
    throw new Error(error.message ?? "Failed to initialize definitions");
  }
}

// === History ===

export interface HistoryFilters {
  scopeType?: string;
  scopeId?: string;
  limit?: number;
}

export async function fetchSettingHistory(
  filters?: HistoryFilters,
): Promise<SettingChange[]> {
  const params = new URLSearchParams();
  if (filters?.scopeType) params.set("scope_type", filters.scopeType);
  if (filters?.scopeId) params.set("scope_id", filters.scopeId);
  if (filters?.limit) params.set("limit", filters.limit.toString());

  const query = params.toString();
  const suffix = query ? `?${query}` : "";
  const url = `${API_BASE}/history${suffix}`;

  const response = await fetch(url, {
    headers: await getAuthHeaders(),
  });

  const data = await handleResponse<BackendSettingChange[]>(response);
  return data.map(mapSettingChangeResponse);
}

export async function fetchOGSettingHistory(
  ogId: string,
  limit?: number,
): Promise<SettingChange[]> {
  const params = limit ? `?limit=${limit}` : "";
  const url = `${API_BASE}/og/${ogId}/history${params}`;

  const response = await fetch(url, {
    headers: await getAuthHeaders(),
  });

  const data = await handleResponse<BackendSettingChange[]>(response);
  return data.map(mapSettingChangeResponse);
}

export async function fetchOGKeyHistory(
  ogId: string,
  key: string,
  limit?: number,
): Promise<SettingChange[]> {
  const params = limit ? `?limit=${limit}` : "";
  const url = `${API_BASE}/og/${ogId}/${key}/history${params}`;

  const response = await fetch(url, {
    headers: await getAuthHeaders(),
  });

  const data = await handleResponse<BackendSettingChange[]>(response);
  return data.map(mapSettingChangeResponse);
}
