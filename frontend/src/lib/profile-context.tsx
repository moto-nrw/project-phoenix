"use client";

import React, {
  createContext,
  useContext,
  useEffect,
  useState,
  useCallback,
  useMemo,
} from "react";
import { useSession } from "next-auth/react";
import { fetchProfile as apiFetchProfile } from "~/lib/profile-api";
import { createLogger } from "~/lib/logger";
import { useLatest } from "~/lib/hooks/use-latest";

const logger = createLogger({ component: "ProfileContext" });
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
  initialProfile = null,
}: Readonly<{
  children: React.ReactNode;
  /** Server-preloaded profile from the tenant layout (#2973); skips the mount fetch. */
  initialProfile?: Profile | null;
}>) {
  const { data: session } = useSession();

  const [state, setState] = useState<ProfileState>(() => ({
    profile: initialProfile,
    isLoading: initialProfile === null,
  }));
  const seededRef = React.useRef(initialProfile !== null);

  // Debounce mechanism to prevent rapid successive calls
  const isRefreshingRef = React.useRef<boolean>(false);
  const lastRefreshRef = React.useRef<number>(0);

  // Store token in ref to avoid dependency loops
  const tokenRef = useLatest(session?.user?.token);

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
      logger.error("failed to load profile", { error: String(error) });
      setState((prev) => ({
        ...prev,
        profile: null,
        isLoading: false,
      }));
    }
  }, [tokenRef]);

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

  // Store the refresh function only after its render commits.
  useEffect(() => {
    refreshRef.current = refreshProfile;
  }, [refreshProfile]);

  // Reset debounce when tenant changes so profile refreshes immediately after switch
  const tenantId = session?.user?.tenantId;
  const prevTenantIdRef = React.useRef(tenantId);
  useEffect(() => {
    if (prevTenantIdRef.current !== tenantId) {
      if (tenantId !== undefined) {
        lastRefreshRef.current = 0;
      }
      prevTenantIdRef.current = tenantId;
    }
  }, [tenantId]);

  // Initial load and refresh on session changes only
  useEffect(() => {
    // Only refresh when token actually changes (not on every render)
    if (session?.user?.token) {
      // The server snapshot IS the initial load; the debounce starts now, as
      // it would after a fetched load.
      if (seededRef.current) {
        seededRef.current = false;
        lastRefreshRef.current = Date.now();
        return;
      }
      refreshRef.current?.()?.catch(() => {
        // Errors already handled in fetchProfileData
      });
    } else {
      // Clear state when no session
      seededRef.current = false;
      setState({
        profile: null,
        isLoading: false,
      });
    }
  }, [session?.user?.token]); // Only depend on token

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
