"use client";

import React, { createContext, useContext, useMemo } from "react";
import { signOut, useSession } from "next-auth/react";
import { useProfile } from "~/lib/profile-context";
import { useOperatorAuth } from "~/lib/operator/auth-context";

export interface ShellUser {
  name: string;
  email: string;
  roles: string[];
}

export interface ShellProfile {
  firstName?: string;
  lastName?: string;
  avatar?: string;
}

type ShellStatus = "loading" | "authenticated" | "unauthenticated";

type ShellMode = "teacher" | "operator";

export interface ShellAuthContextType {
  user: ShellUser | null;
  profile: ShellProfile | null;
  status: ShellStatus;
  isSessionExpired: boolean;
  logout: () => Promise<void>;
  mode: ShellMode;
  homeUrl: string;
  settingsUrl: string | null;
}

const ShellAuthContext = createContext<ShellAuthContextType | undefined>(
  undefined,
);

export function useShellAuth(): ShellAuthContextType {
  const context = useContext(ShellAuthContext);
  if (context === undefined) {
    throw new Error("useShellAuth must be used within a ShellAuthProvider");
  }
  return context;
}

export function TeacherShellProvider({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  const { data: session, status: sessionStatus } = useSession();
  const { profile } = useProfile();

  const value = useMemo<ShellAuthContextType>(() => {
    const user: ShellUser | null = session?.user
      ? {
          // eslint-disable-next-line @typescript-eslint/prefer-nullish-coalescing -- intentionally treat empty string as falsy
          name: session.user.name?.trim() || "Benutzer",
          email: session.user.email ?? "",
          roles: session.user.roles ?? [],
        }
      : null;

    const shellProfile: ShellProfile | null = profile
      ? {
          firstName: profile.firstName ?? undefined,
          lastName: profile.lastName ?? undefined,
          avatar: profile.avatar ?? undefined,
        }
      : null;

    const status: ShellStatus =
      sessionStatus === "loading"
        ? "loading"
        : sessionStatus === "authenticated"
          ? "authenticated"
          : "unauthenticated";

    return {
      user,
      profile: shellProfile,
      status,
      isSessionExpired: session?.error === "RefreshTokenExpired",
      logout: async () => {
        await signOut({ callbackUrl: "/" });
      },
      mode: "teacher" as const,
      homeUrl: "/dashboard",
      settingsUrl: "/settings",
    };
  }, [session, sessionStatus, profile]);

  return (
    <ShellAuthContext.Provider value={value}>
      {children}
    </ShellAuthContext.Provider>
  );
}

export function OperatorShellProvider({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  const { operator, isLoading, isAuthenticated, logout } = useOperatorAuth();

  const value = useMemo<ShellAuthContextType>(() => {
    const user: ShellUser | null = operator
      ? {
          name: operator.displayName,
          email: operator.email,
          roles: ["operator"],
        }
      : null;

    const nameParts = operator?.displayName.split(" ") ?? [];
    const shellProfile: ShellProfile | null = operator
      ? {
          firstName: nameParts[0],
          lastName: nameParts.slice(1).join(" ") || undefined,
        }
      : null;

    const status: ShellStatus = isLoading
      ? "loading"
      : isAuthenticated
        ? "authenticated"
        : "unauthenticated";

    return {
      user,
      profile: shellProfile,
      status,
      isSessionExpired: false,
      logout,
      mode: "operator" as const,
      homeUrl: "/operator/suggestions",
      settingsUrl: null,
    };
  }, [operator, isLoading, isAuthenticated, logout]);

  return (
    <ShellAuthContext.Provider value={value}>
      {children}
    </ShellAuthContext.Provider>
  );
}
