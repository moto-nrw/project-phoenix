import { getSession } from "next-auth/react";
import { 
  // Commented out until we use real API
  // mapProfileResponse, 
  // mapProfileUpdateRequest,
  type Profile, 
  type ProfileUpdateRequest,
  // type BackendProfile 
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

  // MOCK DATA - Remove this block when backend is ready
  const mockProfile: Profile = {
    id: "1",
    firstName: "Max",
    lastName: "Mustermann",
    email: session.user.email ?? "admin@moto.nrw",
    username: "mmustermann",
    avatar: null, // Try changing to a URL like "https://via.placeholder.com/150" to test avatar
    bio: "Ich bin der Administrator dieser Schule und kümmere mich um die technischen Belange des OGS-Systems.",
    rfidCard: "RFID-12345",
    createdAt: "2024-01-15T10:00:00Z",
    updatedAt: "2024-05-25T09:00:00Z",
    lastLogin: "2024-05-25T09:00:00Z",
    settings: {
      theme: 'light',
      language: 'de',
      notifications: {
        email: true,
        push: false,
        activities: true,
        roomChanges: true,
      },
      privacy: {
        showEmail: false,
        showProfile: true,
      }
    }
  };
  
  // Simulate API delay
  await new Promise(resolve => setTimeout(resolve, 500));
  
  return mockProfile;
  
  /* REAL API CALL - Uncomment when backend is ready
  const url = `${process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080'}/api/me/profile`;
  
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

    const data = await response.json() as BackendProfile;
    return mapProfileResponse(data);
  } catch (error) {
    console.error("Error fetching profile:", error);
    throw new Error("Failed to fetch profile");
  }
  */
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

  // MOCK UPDATE - Remove this block when backend is ready
  console.log("Mock update with data:", data);
  
  // Get current profile and merge with updates
  const currentProfile = await fetchProfile();
  const updatedProfile: Profile = {
    ...currentProfile,
    ...data,
    updatedAt: new Date().toISOString(),
  };
  
  // Simulate API delay
  await new Promise(resolve => setTimeout(resolve, 800));
  
  return updatedProfile;
  
  /* REAL API CALL - Uncomment when backend is ready
  const url = `${process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080'}/api/me/profile`;
  
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

    const responseData = await response.json() as BackendProfile;
    return mapProfileResponse(responseData);
  } catch (error) {
    console.error("Error updating profile:", error);
    throw new Error("Failed to update profile");
  }
  */
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

  const url = `${process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080'}/api/me/profile/avatar`;
  
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

  const url = `${process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080'}/api/me/profile/avatar`;
  
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