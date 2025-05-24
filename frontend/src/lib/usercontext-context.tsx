"use client";

import React, { createContext, useContext, useState, useEffect, useCallback, type ReactNode } from "react";
import { useSession } from "next-auth/react";
import { userContextService } from "./usercontext-api";
import type { EducationalGroup, Person } from "./usercontext-helpers";

interface UserContextState {
    educationalGroups: EducationalGroup[];
    hasEducationalGroups: boolean;
    person: Person | null;
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
    const [person, setPerson] = useState<Person | null>(null);
    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const fetchUserData = useCallback(async () => {
        // Only fetch if we have an authenticated session
        if (!session?.user?.token) {
            setEducationalGroups([]);
            setPerson(null);
            setIsLoading(false);
            return;
        }

        try {
            setIsLoading(true);
            setError(null);
            
            // Fetch both educational groups and person data in parallel
            const [groups, personData] = await Promise.all([
                userContextService.getMyEducationalGroups(),
                userContextService.getCurrentPerson().catch((err) => {
                    console.error("Error fetching current person:", err);
                    if (err?.response?.status === 404) {
                        // Return null if person not found (404)
                        return null;
                    }
                    // Rethrow other errors to be handled by the outer try-catch
                    throw err;
                })
            ]);
            
            setEducationalGroups(groups);
            setPerson(personData);
        } catch (err) {
            console.error("Failed to fetch user data:", err);
            setError(err instanceof Error ? err.message : "Failed to fetch user data");
            // Set empty data on error
            setEducationalGroups([]);
            setPerson(null);
        } finally {
            setIsLoading(false);
        }
    }, [session?.user?.token]);

    useEffect(() => {
        // Only fetch when session status is "authenticated" and we have a token
        if (status === "authenticated" && session?.user?.token) {
            void fetchUserData();
        } else if (status === "unauthenticated") {
            // Clear data when unauthenticated
            setEducationalGroups([]);
            setPerson(null);
            setIsLoading(false);
            setError(null);
        }
        // Set loading state when session is loading
        else if (status === "loading") {
            setIsLoading(true);
        }
    }, [status, session?.user?.token, fetchUserData]);

    const value: UserContextState = {
        educationalGroups,
        hasEducationalGroups: educationalGroups.length > 0,
        person,
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

// Hook for accessing current user's person data
export function useCurrentPerson() {
    const { person, isLoading, error } = useUserContext();
    return { person, isLoading, error };
}

// Safe hook for accessing current user's person data (returns null when not in provider)
export function useCurrentPersonSafe() {
    const context = useContext(UserContextContext);
    if (context === undefined) {
        return { person: null, isLoading: false, error: null };
    }
    return { person: context.person, isLoading: context.isLoading, error: context.error };
}