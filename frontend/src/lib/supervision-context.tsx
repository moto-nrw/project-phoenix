"use client";

import React, { createContext, useContext, useEffect, useState, useCallback } from "react";
import { useSession } from "next-auth/react";

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
  
  const [state, setState] = useState<SupervisionState>({
    hasGroups: false,
    isLoadingGroups: true,
    isSupervising: false,
    supervisedRoomId: undefined,
    supervisedRoomName: undefined,
    isLoadingSupervision: true,
  });
  
  // Debounce mechanism to prevent rapid successive calls
  const [, setIsRefreshing] = useState(false);
  
  // Store token in ref to avoid dependency loops
  const tokenRef = React.useRef<string | undefined>(session?.user?.token);
  tokenRef.current = session?.user?.token;
  
  // Use a ref for the refresh function to break dependency cycles
  const refreshRef = React.useRef<(() => Promise<void>) | null>(null);

  // Check if user has any groups (as teacher or representative)
  const checkGroups = useCallback(async () => {
    const token = tokenRef.current;
    if (!token) {
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
        // Add cache control to reduce redundant requests
        cache: "no-store",
      });

      if (response.ok) {
        const data = await response.json() as { data?: { groups?: unknown[] } };
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
    } catch {
      setState(prev => ({
        ...prev,
        hasGroups: false,
        isLoadingGroups: false,
      }));
    }
  }, []); // No dependencies - uses ref

  // Check if user is supervising an active room
  const checkSupervision = useCallback(async () => {
    const token = tokenRef.current;
    console.log("[SupervisionContext] checkSupervision called, token available:", !!token);
    if (!token) {
      console.log("[SupervisionContext] No token, skipping supervision check");
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
      console.log("[SupervisionContext] Calling /api/me/groups/supervised directly");
      const response = await fetch("/api/me/groups/supervised", {
        headers: {
          "Content-Type": "application/json",
        },
        // Add cache control to reduce redundant requests
        cache: "no-store",
      });
      
      console.log("[SupervisionContext] API response status:", response.status);

      if (response.ok) {
        const response_data = await response.json() as {
          success: boolean;
          message: string;
          data: Array<{
            id: number;
            room_id?: number;
            group_id: number;
            room?: {
              id: number;
              name: string;
            };
            actual_group?: {
              id: number;
              name: string;
            };
          }>;
        };
        
        console.log("[SupervisionContext] Backend response:", response_data);
        
        // Check if user has any supervised groups (indicating room supervision)
        const supervisedGroups = response_data.data ?? [];
        const hasSupervision = supervisedGroups.length > 0;
        
        console.log("[SupervisionContext] Supervised groups count:", supervisedGroups.length);
        console.log("[SupervisionContext] Has supervision:", hasSupervision);
        
        if (hasSupervision) {
          const firstGroup = supervisedGroups[0];
          console.log("[SupervisionContext] First supervised group:", firstGroup);
          
          setState(prev => ({
            ...prev,
            isSupervising: true,
            supervisedRoomId: firstGroup.room_id?.toString(),
            supervisedRoomName: firstGroup.room?.name ?? `Room ${firstGroup.room_id}`,
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
      } else {
        setState(prev => ({
          ...prev,
          isSupervising: false,
          supervisedRoomId: undefined,
          supervisedRoomName: undefined,
          isLoadingSupervision: false,
        }));
      }
    } catch {
      setState(prev => ({
        ...prev,
        isSupervising: false,
        supervisedRoomId: undefined,
        supervisedRoomName: undefined,
        isLoadingSupervision: false,
      }));
    }
  }, []); // No dependencies - uses ref

  // Refresh all supervision states with debouncing
  const refresh = useCallback(async () => {
    setIsRefreshing(prev => {
      if (prev) return prev; // Already refreshing, don't start another
      
      // Start the refresh process
      setState(s => ({
        ...s,
        isLoadingGroups: true,
        isLoadingSupervision: true,
      }));
      
      void Promise.all([checkGroups(), checkSupervision()])
        .finally(() => setIsRefreshing(false));
      
      return true;
    });
  }, [checkGroups, checkSupervision]);
  
  // Store the refresh function in ref
  refreshRef.current = refresh;

  // Initial load and refresh on session changes only
  useEffect(() => {
    // Only refresh when session actually changes (not on every render)
    if (session?.user?.token) {
      void refreshRef.current?.();
    } else {
      // Clear state when no session
      setState({
        hasGroups: false,
        isLoadingGroups: false,
        isSupervising: false,
        supervisedRoomId: undefined,
        supervisedRoomName: undefined,
        isLoadingSupervision: false,
      });
    }
  }, [session?.user?.token]); // Only depend on token

  // Periodic refresh every minute for timely supervision updates
  useEffect(() => {
    if (!session?.user?.token) return;
    
    const interval = setInterval(() => {
      void refreshRef.current?.();
    }, 60000); // 1 minute - ensures supervision changes are reflected quickly

    return () => clearInterval(interval);
  }, [session?.user?.token]); // Only depend on token

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