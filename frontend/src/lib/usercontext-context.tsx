"use client";

import React, { createContext, useContext, useState, useEffect, type ReactNode } from "react";
import { useSession } from "next-auth/react";
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
    const [educationalGroups, setEducationalGroups] = useState<EducationalGroup[]>([]);
    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const fetchEducationalGroups = async () => {
        // Only fetch if we have an authenticated session
        if (!session?.user?.token) {
            setEducationalGroups([]);
            setIsLoading(false);
            return;
        }

        try {
            setIsLoading(true);
            setError(null);
            const groups = await userContextService.getMyEducationalGroups();
            setEducationalGroups(groups);
        } catch (err) {
            console.error("Failed to fetch educational groups:", err);
            setError(err instanceof Error ? err.message : "Failed to fetch educational groups");
            // Set empty array on error so UI doesn't show OGS groups
            setEducationalGroups([]);
        } finally {
            setIsLoading(false);
        }
    };

    useEffect(() => {
        // Only fetch when session status is "authenticated" and we have a token
        if (status === "authenticated" && session?.user?.token) {
            void fetchEducationalGroups();
        } else if (status === "unauthenticated") {
            // Clear data when unauthenticated
            setEducationalGroups([]);
            setIsLoading(false);
            setError(null);
        }
        // Set loading state when session is loading
        else if (status === "loading") {
            setIsLoading(true);
        }
    }, [status, session?.user?.token]);

    const value: UserContextState = {
        educationalGroups,
        hasEducationalGroups: educationalGroups.length > 0,
        isLoading,
        error,
        refetch: fetchEducationalGroups,
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