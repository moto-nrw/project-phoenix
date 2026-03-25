"use client";

import React, { createContext, useContext, useMemo } from "react";
import { signOut, useSession } from "next-auth/react";
import { useProfile } from "~/lib/profile-context";
import { operatorAbsoluteUrl, operatorPath } from "~/lib/operator-url";
import { clearSessionCache, DELIBERATE_LOGOUT_KEY } from "~/lib/session-cache";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ShellAuthContext" });

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
        // Mark as deliberate logout so the login page suppresses the
        // "session expired" banner that NextAuth's required-session
        // redirect would otherwise trigger (race with useSession).
        try {
          sessionStorage.setItem(DELIBERATE_LOGOUT_KEY, "1");
        } catch {
          // sessionStorage unavailable (e.g. private browsing quota)
        }
        // Delete backend refresh tokens. Fire-and-forget: the cookie is
        // included in the request headers at send time, so signOut() clearing
        // the cookie afterward doesn't affect the in-flight request.
        fetch("/api/auth/logout", {
          method: "POST",
          signal: AbortSignal.timeout(5000),
        }).catch((err: unknown) => {
          logger.warn("backend_logout_failed", {
            error: err instanceof Error ? err.message : String(err),
          });
        });
        clearSessionCache();
        await signOut({ redirect: false });
        window.location.href = "/";
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
        // Note: operator backend has no logout endpoint — tokens expire naturally.
        // The tenant /api/auth/logout route uses tenant auth cookies and would
        // return 401 for operator sessions. Operator accounts don't do tenant
        // switching, so the stale-session issue from #1067 doesn't apply here.
        clearSessionCache();
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
