"use client";

import React, {
  createContext,
  useContext,
  useEffect,
  useState,
  useCallback,
  useMemo,
} from "react";
import { useSession } from "~/lib/auth-client";
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
  const { data: session, isPending } = useSession();

  const [state, setState] = useState<ProfileState>({
    profile: null,
    isLoading: true,
  });

  // Debounce mechanism to prevent rapid successive calls
  const isRefreshingRef = React.useRef<boolean>(false);
  const lastRefreshRef = React.useRef<number>(0);

  // Store authentication status in ref to avoid dependency loops
  // BetterAuth uses cookies for auth, so we just check if user exists
  const isAuthenticatedRef = React.useRef<boolean>(
    !isPending && !!session?.user,
  );
  isAuthenticatedRef.current = !isPending && !!session?.user;

  // Use a ref for the refresh function to break dependency cycles
  const refreshRef = React.useRef<((silent?: boolean) => Promise<void>) | null>(
    null,
  );

  // Fetch profile data from API
  const fetchProfileData = useCallback(async () => {
    // BetterAuth: check if user is authenticated (cookies handle auth)
    if (!isAuthenticatedRef.current) {
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
    // Only refresh when authentication status changes
    // BetterAuth: session.user indicates authenticated (cookies handle auth)
    if (!isPending && session?.user) {
      refreshRef.current?.()?.catch(() => {
        // Errors already handled in fetchProfileData
      });
    } else if (!isPending) {
      // Clear state when no session
      setState({
        profile: null,
        isLoading: false,
      });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- Intentionally depend on user ID only, not session object reference
  }, [isPending, session?.user?.id]);

  const contextValue = useMemo(
    () => ({ ...state, refreshProfile, updateProfileData }),
    [state, refreshProfile, updateProfileData],
  );

  return (
    <ProfileContext.Provider value={contextValue}>
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
