"use client";

import React, {
  createContext,
  useContext,
  useCallback,
  useMemo,
  type ReactNode,
} from "react";
import { useSession } from "next-auth/react";
import { usePathname } from "next/navigation";
import { useSupervision } from "./supervision-context";
import {
  mapEducationalGroupResponse,
  type EducationalGroup,
} from "./usercontext-helpers";

interface UserContextState {
  educationalGroups: EducationalGroup[];
  hasEducationalGroups: boolean;
  isLoading: boolean;
  error: string | null;
  refetch: () => Promise<void>;
}

const UserContextContext = createContext<UserContextState | undefined>(
  undefined,
);

interface UserContextProviderProps {
  readonly children: ReactNode;
}

export function UserContextProvider({ children }: UserContextProviderProps) {
  const { data: session, status } = useSession();
  const pathname = usePathname();
  const {
    groups: supervisionGroups,
    isLoadingGroups,
    refresh,
  } = useSupervision();

  // Calculate isAuthPage outside the effect to avoid dependency issues
  const isAuthPage = useMemo(() => {
    return pathname === "/" || pathname === "/register";
  }, [pathname]);

  const shouldProvideData =
    status === "authenticated" && !!session?.user?.token && !isAuthPage;

  const mappedGroups = useMemo<EducationalGroup[]>(() => {
    if (!shouldProvideData) {
      return [];
    }
    return supervisionGroups.map(mapEducationalGroupResponse);
  }, [shouldProvideData, supervisionGroups]);

  const isLoading =
    status === "loading" || (shouldProvideData && isLoadingGroups);

  const refetch = useCallback(async () => {
    try {
      await refresh();
    } catch (err) {
      console.error("Failed to refresh supervision context:", err);
    }
  }, [refresh]);

  const value = useMemo<UserContextState>(
    () => ({
      educationalGroups: mappedGroups,
      hasEducationalGroups: mappedGroups.length > 0,
      isLoading,
      error: null,
      refetch,
    }),
    [mappedGroups, isLoading, refetch],
  );

  return (
    <UserContextContext.Provider value={value}>
      {children}
    </UserContextContext.Provider>
  );
}

export function useUserContext() {
  const context = useContext(UserContextContext);
  if (context === undefined) {
    throw new Error("useUserContext must be used within a UserContextProvider");
  }
  return context;
}

// Hook specifically for checking if user has educational groups
export function useHasEducationalGroups() {
  const { hasEducationalGroups, isLoading, error } = useUserContext();
  return { hasEducationalGroups, isLoading, error };
}
