import {
  sessionFetch,
  getCachedSession,
  clearSessionCache,
} from "./session-cache";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ProfileAPI" });
import {
  mapProfileResponse,
  mapProfileUpdateRequest,
  type Profile,
  type ProfileUpdateRequest,
  type BackendProfile,
} from "./profile-helpers";

interface ProfileApiResult {
  success: boolean;
  message: string;
  data: BackendProfile;
}

/**
 * Fetch the current user's profile
 */
export async function fetchProfile(): Promise<Profile> {
  try {
    const response = await sessionFetch(`/api/me/profile`, { method: "GET" });

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    const result = (await response.json()) as ProfileApiResult;

    if (!result.success) {
      throw new Error(result.message || "Failed to fetch profile");
    }

    return mapProfileResponse(result.data);
  } catch (error) {
    if (
      error instanceof Error &&
      error.message === "No authentication token available"
    ) {
      throw error;
    }
    logger.error("failed to fetch profile", { error: String(error) });
    throw new Error("Failed to fetch profile");
  }
}

/**
 * Update the current user's profile
 */
export async function updateProfile(
  data: ProfileUpdateRequest,
): Promise<Profile> {
  try {
    const payload = mapProfileUpdateRequest(data);
    const response = await sessionFetch(`/api/me/profile`, {
      method: "PUT",
      body: JSON.stringify(payload),
    });

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    const result = (await response.json()) as ProfileApiResult;

    if (!result.success) {
      throw new Error(result.message || "Failed to update profile");
    }

    return mapProfileResponse(result.data);
  } catch (error) {
    if (
      error instanceof Error &&
      error.message === "No authentication token available"
    ) {
      throw error;
    }
    logger.error("failed to update profile", { error: String(error) });
    throw new Error("Failed to update profile");
  }
}

/**
 * Upload a new avatar image
 * Note: Uses raw fetch because FormData requires browser-set Content-Type
 */
export async function uploadAvatar(file: File): Promise<Profile> {
  const session = await getCachedSession();
  const token = session?.user?.token;

  if (!token) {
    throw new Error("No authentication token available");
  }

  const url = `/api/me/profile/avatar`;

  const doUpload = async (authToken: string) => {
    const formData = new FormData();
    formData.append("avatar", file);
    return fetch(url, {
      method: "POST",
      headers: { Authorization: `Bearer ${authToken}` },
      body: formData,
    });
  };

  try {
    let response = await doUpload(token);

    if (response.status === 401) {
      clearSessionCache();
      const { handleAuthFailure } = await import("./auth-api");
      const refreshed = await handleAuthFailure();
      if (refreshed) {
        const freshSession = await getCachedSession();
        const freshToken = freshSession?.user?.token;
        if (freshToken) {
          response = await doUpload(freshToken);
        }
      }
      if (response.status === 401) {
        throw new Error("Authentication expired");
      }
    }

    if (!response.ok) {
      const errorData = (await response.json().catch(() => ({}))) as {
        error?: string;
      };
      throw new Error(
        errorData.error ?? `HTTP error! status: ${response.status}`,
      );
    }

    const result = (await response.json()) as ProfileApiResult;

    if (!result.success) {
      throw new Error(result.message || "Failed to upload avatar");
    }

    return mapProfileResponse(result.data);
  } catch (error) {
    logger.error("failed to upload avatar", { error: String(error) });
    throw new Error(
      error instanceof Error ? error.message : "Failed to upload avatar",
    );
  }
}

/**
 * Delete the user's avatar
 */
export async function deleteAvatar(): Promise<Profile> {
  try {
    const response = await sessionFetch(`/api/me/profile/avatar`, {
      method: "DELETE",
    });

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    const result = (await response.json()) as ProfileApiResult;

    if (!result.success) {
      throw new Error(result.message || "Failed to delete avatar");
    }

    return mapProfileResponse(result.data);
  } catch (error) {
    if (
      error instanceof Error &&
      error.message === "No authentication token available"
    ) {
      throw error;
    }
    logger.error("failed to delete avatar", { error: String(error) });
    throw new Error("Failed to delete avatar");
  }
}
