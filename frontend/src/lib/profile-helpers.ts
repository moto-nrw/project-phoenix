export interface Profile {
  id: string;
  firstName: string;
  lastName: string;
  email: string;
  username?: string;
  avatar?: string | null;
  bio?: string | null;
  rfidCard?: string | null;
  createdAt: string;
  updatedAt: string;
  lastLogin?: string | null;
  settings?: ProfileSettings;
}

export interface ProfileSettings {
  theme?: 'light' | 'dark' | 'system';
  language?: string;
  notifications?: NotificationSettings;
  privacy?: PrivacySettings;
}

export interface NotificationSettings {
  email?: boolean;
  push?: boolean;
  activities?: boolean;
  roomChanges?: boolean;
}

export interface PrivacySettings {
  showEmail?: boolean;
  showProfile?: boolean;
}

export interface ProfileUpdateRequest {
  firstName?: string;
  lastName?: string;
  username?: string;
  bio?: string;
  avatar?: string;
  settings?: ProfileSettings;
}

// Backend response mapping
export interface BackendProfile {
  id: number;
  first_name: string;
  last_name: string;
  email: string;
  username?: string;
  avatar?: string;
  bio?: string;
  rfid_card?: string;
  settings?: string | Record<string, unknown>;
  created_at: string;
  updated_at: string;
  last_login?: string;
}

export function mapProfileResponse(data: BackendProfile): Profile {
  // Parse settings if it's a string
  let settings: ProfileSettings | undefined;
  if (data.settings) {
    if (typeof data.settings === 'string') {
      try {
        settings = JSON.parse(data.settings) as ProfileSettings;
      } catch (error) {
        if (process.env.NODE_ENV === 'development') {
          console.warn('Failed to parse profile settings JSON:', data.settings, error);
        }
        settings = undefined;
      }
    } else {
      settings = data.settings as ProfileSettings;
    }
  }

  // Convert avatar path to authenticated API URL
  let avatarUrl = data.avatar;
  if (avatarUrl?.startsWith('/uploads/')) {
    // Extract filename from path
    const filename = avatarUrl.split('/').pop();
    if (filename) {
      // Use authenticated API route that goes through Next.js
      avatarUrl = `/api/me/profile/avatar/${filename}`;
    }
  }

  return {
    id: data.id.toString(),
    firstName: data.first_name ?? '',
    lastName: data.last_name ?? '',
    email: data.email,
    username: data.username,
    avatar: avatarUrl,
    bio: data.bio,
    rfidCard: data.rfid_card,
    createdAt: data.created_at,
    updatedAt: data.updated_at,
    lastLogin: data.last_login,
    settings,
  };
}

export function mapProfileUpdateRequest(request: ProfileUpdateRequest): Record<string, unknown> {
  const mapped: Record<string, unknown> = {};
  
  if (request.firstName !== undefined) {
    mapped.first_name = request.firstName;
  }
  if (request.lastName !== undefined) {
    mapped.last_name = request.lastName;
  }
  if (request.username !== undefined) {
    mapped.username = request.username;
  }
  if (request.bio !== undefined) {
    mapped.bio = request.bio;
  }
  if (request.avatar !== undefined) {
    mapped.avatar = request.avatar;
  }
  if (request.settings !== undefined) {
    // Convert settings to JSON string if needed
    mapped.settings = typeof request.settings === 'object' 
      ? JSON.stringify(request.settings) 
      : request.settings;
  }
  
  return mapped;
}