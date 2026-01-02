"use client";

import React, {
  createContext,
  useContext,
  useEffect,
  useState,
  useCallback,
} from "react";
import { useSession } from "next-auth/react";
import { fetchProfile as apiFetchProfile } from "~/lib/profile-api";
import type { Profile } from "~/lib/profile-helpers";

interface ProfileState {
  profile: Profile | null;
  isLoading: boolean;
}

interface ProfileContextType extends ProfileState {
  refreshProfile: (silent?: boolean) => Promise<void>;
  updateProfileData: (data: Partial<Profile>) => void;
}

const ProfileContext = createContext<ProfileContextType | undefined>(undefined);

/**
 * Provider that manages user profile data with caching
 * Fetches profile once on mount and caches it across navigations
 *
 * Pattern inspired by SupervisionProvider for consistency
 */
export function ProfileProvider({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  const { data: session } = useSession();

  const [state, setState] = useState<ProfileState>({
    profile: null,
    isLoading: true,
  });

  // Debounce mechanism to prevent rapid successive calls
  const isRefreshingRef = React.useRef<boolean>(false);
  const lastRefreshRef = React.useRef<number>(0);

  // Store token in ref to avoid dependency loops
  const tokenRef = React.useRef<string | undefined>(session?.user?.token);
  tokenRef.current = session?.user?.token;

  // Use a ref for the refresh function to break dependency cycles
  const refreshRef = React.useRef<((silent?: boolean) => Promise<void>) | null>(
    null,
  );

  // Fetch profile data from API
  const fetchProfileData = useCallback(async () => {
    const token = tokenRef.current;
    if (!token) {
      setState((prev) => ({
        ...prev,
        profile: null,
        isLoading: false,
      }));
      return;
    }

    try {
      const profileData = await apiFetchProfile();
      setState((prev) => {
        // Only update if data actually changed
        if (
          prev.profile?.id === profileData.id &&
          prev.profile?.avatar === profileData.avatar &&
          prev.profile?.firstName === profileData.firstName &&
          prev.profile?.lastName === profileData.lastName &&
          !prev.isLoading
        ) {
          return prev;
        }
        return {
          profile: profileData,
          isLoading: false,
        };
      });
    } catch (error) {
      console.error("Failed to load profile:", error);
      setState((prev) => ({
        ...prev,
        profile: null,
        isLoading: false,
      }));
    }
  }, []); // No dependencies - uses ref

  // Refresh profile with debouncing
  const refreshProfile = useCallback(
    async (silent = false) => {
      // Prevent rapid successive refreshes (min 5 seconds between refreshes)
      const now = Date.now();
      if (now - lastRefreshRef.current < 5000) {
        return;
      }

      // Already refreshing, don't start another
      if (isRefreshingRef.current) {
        return;
      }

      lastRefreshRef.current = now;
      isRefreshingRef.current = true;

      // Only show loading state if not a silent refresh
      if (!silent) {
        setState((s) => ({
          ...s,
          isLoading: true,
        }));
      }

      await fetchProfileData();
      isRefreshingRef.current = false;
    },
    [fetchProfileData],
  );

  // Manual update function for optimistic updates
  const updateProfileData = useCallback((data: Partial<Profile>) => {
    setState((prev) => {
      if (!prev.profile) return prev;

      return {
        ...prev,
        profile: {
          ...prev.profile,
          ...data,
        },
      };
    });
  }, []);

  // Store the refresh function in ref
  refreshRef.current = refreshProfile;

  // Initial load and refresh on session changes only
  useEffect(() => {
    // Only refresh when token actually changes (not on every render)
    if (session?.user?.token) {
      refreshRef.current?.()?.catch(() => {
        // Errors already handled in fetchProfileData
      });
    } else {
      // Clear state when no session
      setState({
        profile: null,
        isLoading: false,
      });
    }
  }, [session?.user?.token]); // Only depend on token

  return (
    <ProfileContext.Provider
      value={{ ...state, refreshProfile, updateProfileData }}
    >
      {children}
    </ProfileContext.Provider>
  );
}

/**
 * Hook to access profile context
 * @throws Error if used outside ProfileProvider
 */
export function useProfile() {
  const context = useContext(ProfileContext);
  if (context === undefined) {
    throw new Error("useProfile must be used within a ProfileProvider");
  }
  return context;
}
