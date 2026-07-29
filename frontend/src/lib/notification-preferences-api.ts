/**
 * Client for the per-account notification consent API.
 *
 * Mirrors push-api.ts: it throws on failure and the card renders an Alert,
 * rather than the settings-page convention of returning a German error string.
 * The two cards sit next to each other, so they behave the same way.
 */

export type PreferencePortal = "tenant" | "parent";

export interface NotificationPreferenceType {
  key: string;
  label: string;
  description: string;
  group: string;
  enabled: boolean;
  /** False when the school currently blocks this type for everyone. */
  available: boolean;
}

export interface NotificationPreferences {
  /** Mirrors the school-wide switch. False means nothing is delivered at all. */
  tenant_enabled: boolean;
  types: NotificationPreferenceType[];
}

export const NOTIFICATION_PREFERENCES_SWR_KEY = "notification-preferences";

class PreferencesApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
    this.name = "PreferencesApiError";
  }
}

function basePath(portal: PreferencePortal): string {
  return portal === "parent"
    ? "/api/parent/me/notification-preferences"
    : "/api/notifications/preferences";
}

async function requestJson<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(url, {
    credentials: "include",
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...init?.headers,
    },
  });
  if (response.status === 204) return undefined as T;

  const text = await response.text();
  let data: unknown = null;
  try {
    data = text ? (JSON.parse(text) as unknown) : null;
  } catch {
    data = null;
  }

  if (!response.ok) {
    const message =
      data && typeof data === "object" && "error" in data
        ? String((data as { error: unknown }).error)
        : `Einstellung konnte nicht geladen werden (${response.status}).`;
    throw new PreferencesApiError(response.status, message);
  }
  return data as T;
}

export async function fetchNotificationPreferences(
  portal: PreferencePortal = "tenant",
): Promise<NotificationPreferences> {
  const response = await requestJson<
    { data?: NotificationPreferences } & Partial<NotificationPreferences>
  >(basePath(portal));

  // The tenant proxy unwraps the envelope, the parent proxy may not — accept
  // either shape rather than depending on which layer stripped it.
  const payload = response.data ?? (response as NotificationPreferences);
  return {
    tenant_enabled: payload.tenant_enabled ?? false,
    types: payload.types ?? [],
  };
}

export async function setNotificationPreference(
  type: string,
  enabled: boolean,
  portal: PreferencePortal = "tenant",
): Promise<void> {
  await requestJson<void>(`${basePath(portal)}/${encodeURIComponent(type)}`, {
    method: "PUT",
    body: JSON.stringify({ enabled }),
  });
}

export async function disableAllNotificationPreferences(
  portal: PreferencePortal = "tenant",
): Promise<void> {
  await requestJson<void>(basePath(portal), { method: "DELETE" });
}
