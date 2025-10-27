/**
 * Profile Update Events
 *
 * Event system for notifying components about profile changes.
 * Used to keep Header in sync with Settings page updates.
 */

export const PROFILE_EVENTS = {
  PROFILE_UPDATED: 'profile-updated',
} as const;

export interface ProfileUpdatedEvent {
  firstName: string;
  lastName: string;
  email: string;
  avatar?: string | null;
}

/**
 * Dispatch a profile update event to notify all listening components
 * @param profile - Updated profile data
 */
export function dispatchProfileUpdate(profile: ProfileUpdatedEvent): void {
  window.dispatchEvent(
    new CustomEvent(PROFILE_EVENTS.PROFILE_UPDATED, {
      detail: profile,
    })
  );
}
