import { ApiError, apiErrorFromResponse, transportFetch } from "./api-error";
import { hasSessionAccess } from "./session-access";
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
 * A 200 answer without `success` is a server defect: the shared error path
 * shows it as general.server (#2517).
 */
function unsuccessfulResult(message: string): ApiError {
  return new ApiError(message, 500);
}

/**
 * Fetch the current user's profile
 */
export async function fetchProfile(): Promise<Profile> {
  try {
    const response = await sessionFetch(`/api/me/profile`, { method: "GET" });

    if (!response.ok) {
      throw await apiErrorFromResponse(
        response,
        `HTTP error! status: ${response.status}`,
      );
    }

    const result = (await response.json()) as ProfileApiResult;

    if (!result.success) {
      throw unsuccessfulResult(result.message || "Failed to fetch profile");
    }

    return mapProfileResponse(result.data);
  } catch (error) {
    logger.error("failed to fetch profile", { error: String(error) });
    throw error;
  }
}

/**
 * Update the current user's profile. Throws an ApiError with the backend
 * code and field errors, so the form shows the catalog text (#2517).
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
      throw await apiErrorFromResponse(
        response,
        `HTTP error! status: ${response.status}`,
      );
    }

    const result = (await response.json()) as ProfileApiResult;

    if (!result.success) {
      throw unsuccessfulResult(result.message || "Failed to update profile");
    }

    return mapProfileResponse(result.data);
  } catch (error) {
    logger.error("failed to update profile", { error: String(error) });
    throw error;
  }
}

/**
 * Upload a new avatar image. Throws an ApiError (#2517).
 * Note: Uses raw fetch because FormData requires browser-set Content-Type
 */
export async function uploadAvatar(file: File): Promise<Profile> {
  if (!hasSessionAccess(await getCachedSession())) {
    throw new ApiError("No authentication token available", 401, {
      code: "general.permission",
    });
  }

  const url = `/api/me/profile/avatar`;

  // The route reads the session cookie; the browser holds no token (#5005).
  const doUpload = async () => {
    const formData = new FormData();
    formData.append("avatar", file);
    return transportFetch(url, {
      method: "POST",
      body: formData,
    });
  };

  try {
    let response = await doUpload();

    if (response.status === 401) {
      clearSessionCache();
      const { handleAuthFailure } = await import("./auth-api");
      const refreshed = await handleAuthFailure();
      // The session lookup persists the rotated cookie before the retry.
      if (refreshed && hasSessionAccess(await getCachedSession())) {
        response = await doUpload();
      }
    }

    if (!response.ok) {
      throw await apiErrorFromResponse(
        response,
        `HTTP error! status: ${response.status}`,
      );
    }

    const result = (await response.json()) as ProfileApiResult;

    if (!result.success) {
      throw unsuccessfulResult(result.message || "Failed to upload avatar");
    }

    return mapProfileResponse(result.data);
  } catch (error) {
    logger.error("failed to upload avatar", { error: String(error) });
    throw error;
  }
}
