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
  account_id: number;
  first_name: string;
  last_name: string;
  email: string;
  username?: string;
  avatar?: string;
  bio?: string;
  tag_id?: string;
  settings?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
  last_login?: string;
}

export function mapProfileResponse(data: BackendProfile): Profile {
  return {
    id: data.id.toString(),
    firstName: data.first_name,
    lastName: data.last_name,
    email: data.email,
    username: data.username,
    avatar: data.avatar,
    bio: data.bio,
    rfidCard: data.tag_id,
    createdAt: data.created_at,
    updatedAt: data.updated_at,
    lastLogin: data.last_login,
    settings: data.settings as ProfileSettings,
  };
}

export function mapProfileUpdateRequest(request: ProfileUpdateRequest): Record<string, unknown> {
  return {
    first_name: request.firstName,
    last_name: request.lastName,
    username: request.username,
    bio: request.bio,
    avatar: request.avatar,
    settings: request.settings,
  };
}