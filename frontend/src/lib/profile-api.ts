// lib/profile-api.ts
// BetterAuth: Authentication handled via cookies, no manual token management needed
import {
  mapProfileResponse,
  mapProfileUpdateRequest,
  type Profile,
  type ProfileUpdateRequest,
  type BackendProfile,
} from "./profile-helpers";

/**
 * Fetch the current user's profile
 * BetterAuth: authentication handled via cookies
 */
export async function fetchProfile(): Promise<Profile> {
  const url = `/api/me/profile`;

  try {
    const response = await fetch(url, {
      method: "GET",
      credentials: "include",
      headers: {
        "Content-Type": "application/json",
      },
    });

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    const result = (await response.json()) as {
      success: boolean;
      message: string;
      data: BackendProfile;
    };

    if (!result.success) {
      throw new Error(result.message || "Failed to fetch profile");
    }

    return mapProfileResponse(result.data);
  } catch (error) {
    console.error("Error fetching profile:", error);
    throw new Error("Failed to fetch profile");
  }
}

/**
 * Update the current user's profile
 * BetterAuth: authentication handled via cookies
 */
export async function updateProfile(
  data: ProfileUpdateRequest,
): Promise<Profile> {
  const url = `/api/me/profile`;

  try {
    const payload = mapProfileUpdateRequest(data);
    const response = await fetch(url, {
      method: "PUT",
      credentials: "include",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(payload),
    });

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    const result = (await response.json()) as {
      success: boolean;
      message: string;
      data: BackendProfile;
    };

    if (!result.success) {
      throw new Error(result.message || "Failed to update profile");
    }

    return mapProfileResponse(result.data);
  } catch (error) {
    console.error("Error updating profile:", error);
    throw new Error("Failed to update profile");
  }
}

/**
 * Upload a new avatar image
 * BetterAuth: authentication handled via cookies
 */
export async function uploadAvatar(file: File): Promise<Profile> {
  const url = `/api/me/profile/avatar`;

  try {
    const formData = new FormData();
    formData.append("avatar", file);

    const response = await fetch(url, {
      method: "POST",
      credentials: "include",
      // Note: Don't set Content-Type for FormData - browser will set it with boundary
      body: formData,
    });

    if (!response.ok) {
      const errorData = (await response.json().catch(() => ({}))) as {
        error?: string;
      };
      throw new Error(
        errorData.error ?? `HTTP error! status: ${response.status}`,
      );
    }

    const result = (await response.json()) as {
      success: boolean;
      message: string;
      data: BackendProfile;
    };

    if (!result.success) {
      throw new Error(result.message || "Failed to upload avatar");
    }

    return mapProfileResponse(result.data);
  } catch (error) {
    console.error("Error uploading avatar:", error);
    throw new Error(
      error instanceof Error ? error.message : "Failed to upload avatar",
    );
  }
}

/**
 * Delete the user's avatar
 * BetterAuth: authentication handled via cookies
 */
export async function deleteAvatar(): Promise<Profile> {
  const url = `/api/me/profile/avatar`;

  try {
    const response = await fetch(url, {
      method: "DELETE",
      credentials: "include",
      headers: {
        "Content-Type": "application/json",
      },
    });

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    const result = (await response.json()) as {
      success: boolean;
      message: string;
      data: BackendProfile;
    };

    if (!result.success) {
      throw new Error(result.message || "Failed to delete avatar");
    }

    return mapProfileResponse(result.data);
  } catch (error) {
    console.error("Error deleting avatar:", error);
    throw new Error("Failed to delete avatar");
  }
}
