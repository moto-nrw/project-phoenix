"use client";

import React, { createContext, useContext, useMemo } from "react";
import { signOut, useSession } from "next-auth/react";
import { useProfile } from "~/lib/profile-context";
import { operatorAbsoluteUrl, operatorPath } from "~/lib/operator-url";
import { parentAbsoluteUrl, parentPath } from "~/lib/parent-url";
import { clearSessionCache, DELIBERATE_LOGOUT_KEY } from "~/lib/session-cache";
import { createLogger } from "~/lib/logger";
import { unsubscribePushSilently } from "~/lib/push-api";

const logger = createLogger({ component: "ShellAuthContext" });

interface ShellUser {
  name: string;
  email: string;
  roles: string[];
}

interface ShellProfile {
  firstName?: string;
  lastName?: string;
  avatar?: string;
}

type ShellStatus = "loading" | "authenticated" | "unauthenticated";

type ShellMode = "teacher" | "operator" | "parent";

interface ShellAuthContextType {
  user: ShellUser | null;
  profile: ShellProfile | null;
  status: ShellStatus;
  isSessionExpired: boolean;
  logout: () => Promise<void>;
  mode: ShellMode;
  homeUrl: string;
  profileUrl: string | null;
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
        // Best-effort: drop this device's Web Push registration while the
        // session is still valid. Must never block logout.
        await unsubscribePushSilently("tenant");
        // Finish backend revocation before clearing the Auth.js cookie. The
        // response-aware route may persist a just-refreshed session cookie;
        // awaiting it guarantees signOut remains the final cookie mutation.
        try {
          await fetch("/api/auth/logout", {
            method: "POST",
            signal: AbortSignal.timeout(5000),
          });
        } catch (err: unknown) {
          logger.warn("backend_logout_failed", {
            error: err instanceof Error ? err.message : String(err),
          });
        }
        clearSessionCache();
        await signOut({ redirect: false });
        window.location.href = "/";
      },
      mode: "teacher" as const,
      homeUrl: "/dashboard",
      profileUrl: "/profile",
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
      homeUrl: operatorPath("/operator/organizations"),
      profileUrl: operatorPath("/operator/settings"),
    };
  }, [session, sessionStatus]);

  return (
    <ShellAuthContext.Provider value={value}>
      {children}
    </ShellAuthContext.Provider>
  );
}

// ParentShellProvider — same shape as OperatorShellProvider with a
// parent-scope NextAuth session. Used by app/parents/auth-guard.tsx
// to feed the AppShell user/profile/logout machinery on the parent
// portal. Cross-tenant ChildSummary data isn't carried here — the
// dashboard fetches it directly via /api/parent/me/children.
export function ParentShellProvider({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  const { data: session, status: sessionStatus } = useSession();

  const value = useMemo<ShellAuthContextType>(() => {
    const user: ShellUser | null = session?.user
      ? {
          name: session.user.name?.trim() || "Eltern",
          email: session.user.email ?? "",
          roles: session.user.roles ?? ["guardian"],
        }
      : null;

    // Elternkonten tragen im Session-Namen haeufig die E-Mail-Adresse. Die
    // darf nie als Vorname durchgereicht werden: "Guten Tag,
    // karin.klein@email.de" liest sich wie ein Datenbankauswurf. Ohne
    // brauchbaren Namen bleibt das Profil leer, und die Oberflaeche gruesst
    // ohne Anrede.
    const displayName = session?.user?.name?.trim() ?? "";
    const nameParts = displayName.includes("@") ? [] : displayName.split(" ");
    const firstName = session?.user?.firstName?.trim() || nameParts[0];
    const shellProfile: ShellProfile | null = firstName
      ? {
          firstName,
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
        // Parent backend has no logout endpoint yet — tokens expire
        // naturally. NextAuth signOut clears the parent.session-token
        // cookie locally and redirects to the parents login page.
        // Best-effort: drop this device's Web Push registration first.
        await unsubscribePushSilently("parent");
        clearSessionCache();
        await signOut({ callbackUrl: parentAbsoluteUrl("/parents/login") });
      },
      mode: "parent" as const,
      homeUrl: parentPath("/parents"),
      // Was null until #1671: the parents portal had no page of its own for
      // account settings, so the avatar menu offered nothing but "Abmelden".
      profileUrl: parentPath("/parents/settings"),
    };
  }, [session, sessionStatus]);

  return (
    <ShellAuthContext.Provider value={value}>
      {children}
    </ShellAuthContext.Provider>
  );
}
