"use client";

import React, { createContext, useContext, useMemo } from "react";
import { signOut, useSession } from "next-auth/react";
import { useProfile } from "~/lib/profile-context";
import { operatorAbsoluteUrl, operatorPath } from "~/lib/operator-url";

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
  const { data: session, status: sessionStatus } = useSession();

  const value = useMemo<ShellAuthContextType>(() => {
    const user: ShellUser | null = session?.user
      ? {
          // eslint-disable-next-line @typescript-eslint/prefer-nullish-coalescing -- intentionally treat empty string as falsy
          name: session.user.name?.trim() || "Operator",
          email: session.user.email ?? "",
          roles: session.user.roles ?? ["operator"],
        }
      : null;

    const displayName = session?.user?.name ?? "";
    const nameParts = displayName.split(" ");
    const shellProfile: ShellProfile | null = session?.user
      ? {
          firstName: nameParts[0],
          lastName: nameParts.slice(1).join(" ") || undefined,
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
        await signOut({ callbackUrl: operatorAbsoluteUrl("/operator/login") });
      },
      mode: "operator" as const,
      homeUrl: operatorPath("/operator/suggestions"),
      settingsUrl: operatorPath("/operator/settings"),
    };
  }, [session, sessionStatus]);

  return (
    <ShellAuthContext.Provider value={value}>
      {children}
    </ShellAuthContext.Provider>
  );
}
