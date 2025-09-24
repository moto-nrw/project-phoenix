"use client";

import React, { createContext, useContext, useState, useEffect, useCallback, type ReactNode } from "react";
import { useSession } from "next-auth/react";
import { usePathname } from "next/navigation";
import { userContextService } from "./usercontext-api";
import type { EducationalGroup } from "./usercontext-helpers";

interface UserContextState {
    educationalGroups: EducationalGroup[];
    hasEducationalGroups: boolean;
    isLoading: boolean;
    error: string | null;
    refetch: () => Promise<void>;
}

const UserContextContext = createContext<UserContextState | undefined>(undefined);

interface UserContextProviderProps {
    children: ReactNode;
}

export function UserContextProvider({ children }: UserContextProviderProps) {
    const { data: session, status } = useSession();
    const pathname = usePathname();
    const [educationalGroups, setEducationalGroups] = useState<EducationalGroup[]>([]);
    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const fetchUserData = useCallback(async () => {
        // Only fetch if we have an authenticated session
        if (!session?.user?.token) {
            setEducationalGroups([]);
            setIsLoading(false);
            return;
        }

        try {
            setIsLoading(true);
            setError(null);
            
            // Fetch educational groups
            const groups = await userContextService.getMyEducationalGroups();
            
            setEducationalGroups(groups);
        } catch (err) {
            console.error("Failed to fetch user data:", err);
            setError(err instanceof Error ? err.message : "Failed to fetch user data");
            // Set empty data on error
            setEducationalGroups([]);
        } finally {
            setIsLoading(false);
        }
    }, [session?.user?.token]);

    useEffect(() => {
        // Skip API calls on login/register pages
        const isAuthPage = pathname === "/" || pathname === "/register";

        // Only fetch when session status is "authenticated" and we have a token and not on auth pages
        if (status === "authenticated" && session?.user?.token && !isAuthPage) {
            void fetchUserData();
        } else if (status === "unauthenticated" || status === "loading" || isAuthPage) {
            // Clear data when unauthenticated, loading, or on auth pages
            setEducationalGroups([]);
            setIsLoading(false);
            setError(null);
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [status, session?.user?.token, fetchUserData]); // Removed pathname dependency to prevent refetch on navigation

    const value: UserContextState = {
        educationalGroups,
        hasEducationalGroups: educationalGroups.length > 0,
        isLoading,
        error,
        refetch: fetchUserData,
    };

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