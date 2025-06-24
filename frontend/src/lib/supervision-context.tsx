"use client";

import React, { createContext, useContext, useEffect, useState, useCallback } from "react";
import { useSession } from "next-auth/react";
import { usePathname } from "next/navigation";

interface SupervisionState {
  // Group supervision
  hasGroups: boolean;
  isLoadingGroups: boolean;
  
  // Room supervision (for active sessions)
  isSupervising: boolean;
  supervisedRoomId?: string;
  supervisedRoomName?: string;
  isLoadingSupervision: boolean;
}

interface SupervisionContextType extends SupervisionState {
  refresh: () => Promise<void>;
}

const SupervisionContext = createContext<SupervisionContextType | undefined>(undefined);

/**
 * Provider that manages dynamic supervision states
 * Checks for group assignments and active room supervision
 */
export function SupervisionProvider({ children }: { children: React.ReactNode }) {
  const { data: session } = useSession();
  const pathname = usePathname();
  
  const [state, setState] = useState<SupervisionState>({
    hasGroups: false,
    isLoadingGroups: true,
    isSupervising: false,
    supervisedRoomId: undefined,
    supervisedRoomName: undefined,
    isLoadingSupervision: true,
  });

  // Check if user has any groups (as teacher or representative)
  const checkGroups = useCallback(async () => {
    if (!session?.user?.token) {
      setState(prev => ({
        ...prev,
        hasGroups: false,
        isLoadingGroups: false,
      }));
      return;
    }

    try {
      const response = await fetch("/api/groups/context", {
        headers: {
          "Content-Type": "application/json",
        },
      });

      if (response.ok) {
        const data = await response.json() as { data?: { groups?: unknown[] } };
        console.log("DEBUG: Supervision context groups data:", data);
        console.log("DEBUG: Setting hasGroups to:", data?.data?.groups ? data.data.groups.length > 0 : false);
        setState(prev => ({
          ...prev,
          hasGroups: data?.data?.groups ? data.data.groups.length > 0 : false,
          isLoadingGroups: false,
        }));
      } else {
        setState(prev => ({
          ...prev,
          hasGroups: false,
          isLoadingGroups: false,
        }));
      }
    } catch (error) {
      console.error("Error checking groups:", error);
      setState(prev => ({
        ...prev,
        hasGroups: false,
        isLoadingGroups: false,
      }));
    }
  }, [session?.user?.token]);

  // Check if user is supervising an active room
  const checkSupervision = useCallback(async () => {
    if (!session?.user?.token) {
      setState(prev => ({
        ...prev,
        isSupervising: false,
        supervisedRoomId: undefined,
        supervisedRoomName: undefined,
        isLoadingSupervision: false,
      }));
      return;
    }

    try {
      const response = await fetch("/api/active/supervision", {
        headers: {
          "Content-Type": "application/json",
        },
      });

      if (response.ok) {
        const data = await response.json() as {
          isSupervising: boolean;
          roomId?: string;
          roomName?: string;
        };
        
        setState(prev => ({
          ...prev,
          isSupervising: data.isSupervising,
          supervisedRoomId: data.roomId,
          supervisedRoomName: data.roomName,
          isLoadingSupervision: false,
        }));
      } else {
        setState(prev => ({
          ...prev,
          isSupervising: false,
          supervisedRoomId: undefined,
          supervisedRoomName: undefined,
          isLoadingSupervision: false,
        }));
      }
    } catch (error) {
      console.error("Error checking supervision:", error);
      setState(prev => ({
        ...prev,
        isSupervising: false,
        supervisedRoomId: undefined,
        supervisedRoomName: undefined,
        isLoadingSupervision: false,
      }));
    }
  }, [session?.user?.token]);

  // Refresh all supervision states
  const refresh = useCallback(async () => {
    setState(prev => ({
      ...prev,
      isLoadingGroups: true,
      isLoadingSupervision: true,
    }));
    
    await Promise.all([checkGroups(), checkSupervision()]);
  }, [checkGroups, checkSupervision]);

  // Initial load and refresh on session/route changes
  useEffect(() => {
    void refresh();
  }, [session, pathname, refresh]);

  // Periodic refresh every 60 seconds
  useEffect(() => {
    const interval = setInterval(() => {
      void refresh();
    }, 60000);

    return () => clearInterval(interval);
  }, [refresh]);

  return (
    <SupervisionContext.Provider value={{ ...state, refresh }}>
      {children}
    </SupervisionContext.Provider>
  );
}

/**
 * Hook to access supervision context
 */
export function useSupervision() {
  const context = useContext(SupervisionContext);
  if (context === undefined) {
    throw new Error("useSupervision must be used within a SupervisionProvider");
  }
  return context;
}

/**
 * Hook to check if user has groups (convenience wrapper)
 */
export function useHasGroups(): boolean {
  const { hasGroups, isLoadingGroups } = useSupervision();
  return !isLoadingGroups && hasGroups;
}

/**
 * Hook to check if user is supervising a room (convenience wrapper)
 */
export function useIsSupervising(): boolean {
  const { isSupervising, isLoadingSupervision } = useSupervision();
  return !isLoadingSupervision && isSupervising;
}