import { getSession } from "next-auth/react";
import { 
  mapProfileResponse, 
  mapProfileUpdateRequest,
  type Profile, 
  type ProfileUpdateRequest,
  type BackendProfile 
} from "./profile-helpers";

/**
 * Fetch the current user's profile
 */
export async function fetchProfile(): Promise<Profile> {
  const session = await getSession();
  const token = session?.user?.token;

  if (!token) {
    throw new Error("No authentication token available");
  }

  const url = `/api/me/profile`;
  
  try {
    const response = await fetch(url, {
      method: 'GET',
      headers: {
        'Authorization': `Bearer ${token}`,
        'Content-Type': 'application/json',
      },
    });

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    const result = await response.json() as { data?: BackendProfile } | BackendProfile;
    
    // Check if response has data wrapper (common API pattern)
    const data = 'data' in result && result.data ? result.data : result as BackendProfile;
    
    return mapProfileResponse(data);
  } catch (error) {
    console.error("Error fetching profile:", error);
    throw new Error("Failed to fetch profile");
  }
}

/**
 * Update the current user's profile
 */
export async function updateProfile(data: ProfileUpdateRequest): Promise<Profile> {
  const session = await getSession();
  const token = session?.user?.token;

  if (!token) {
    throw new Error("No authentication token available");
  }

  const url = `/api/me/profile`;
  
  try {
    const payload = mapProfileUpdateRequest(data);
    const response = await fetch(url, {
      method: 'PUT',
      headers: {
        'Authorization': `Bearer ${token}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(payload),
    });

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    const result = await response.json() as { data?: BackendProfile } | BackendProfile;
    
    // Check if response has data wrapper (common API pattern)
    const responseData = 'data' in result && result.data ? result.data : result as BackendProfile;
    
    return mapProfileResponse(responseData);
  } catch (error) {
    console.error("Error updating profile:", error);
    throw new Error("Failed to update profile");
  }
}

/**
 * Upload a new avatar image
 */
export async function uploadAvatar(file: File): Promise<string> {
  const session = await getSession();
  const token = session?.user?.token;

  if (!token) {
    throw new Error("No authentication token available");
  }

  const url = `/api/me/profile/avatar`;
  
  try {
    const formData = new FormData();
    formData.append('avatar', file);

    const response = await fetch(url, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${token}`,
      },
      body: formData,
    });

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    const data = await response.json() as { url: string };
    return data.url;
  } catch (error) {
    console.error("Error uploading avatar:", error);
    throw new Error("Failed to upload avatar");
  }
}

/**
 * Delete the user's avatar
 */
export async function deleteAvatar(): Promise<void> {
  const session = await getSession();
  const token = session?.user?.token;

  if (!token) {
    throw new Error("No authentication token available");
  }

  const url = `/api/me/profile/avatar`;
  
  try {
    const response = await fetch(url, {
      method: 'DELETE',
      headers: {
        'Authorization': `Bearer ${token}`,
        'Content-Type': 'application/json',
      },
    });

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }
  } catch (error) {
    console.error("Error deleting avatar:", error);
    throw new Error("Failed to delete avatar");
  }
}