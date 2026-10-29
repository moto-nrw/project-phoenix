"use client";

import React, { createContext, useContext, useMemo } from "react";
import { signOut, useSession } from "next-auth/react";
import { useProfile } from "~/lib/profile-context";
import { operatorAbsoluteUrl, operatorPath } from "~/lib/operator-url";
import { parentAbsoluteUrl, parentPath } from "~/lib/parent-url";
import { schoolAbsoluteUrl, schoolPath } from "~/lib/school-url";
import { catererAbsoluteUrl, catererPath } from "~/lib/caterer-url";
import { traegerPath } from "~/lib/traeger-url";
import { clearSessionCache, DELIBERATE_LOGOUT_KEY } from "~/lib/session-cache";
import { clearPersonalStorage } from "~/lib/personal-storage";
import { createLogger } from "~/lib/logger";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { apiErrorFromResponse } from "~/lib/api-error";
import { unsubscribePushSilently } from "~/lib/push-api";
import { hasPermission } from "~/lib/auth-utils";
import { getSmartRedirectPath } from "~/lib/redirect-utils";
import { performEndStaffPreview } from "~/lib/staff-preview-api";
import { mutate } from "~/lib/swr";

const logger = createLogger({ component: "ShellAuthContext" });

type ShowLogoutError = ReturnType<typeof useApiErrorDisplay>["show"];

/**
 * Wraps a portal's logout. A failure keeps the session signed in, is shown
 * once with a retry, and still reaches the caller, which only closes its
 * confirmation (#5194, #5126).
 */
function reportingLogout(
  endSession: () => Promise<void>,
  showLogoutError: ShowLogoutError,
): () => Promise<void> {
  const logout = async (): Promise<void> => {
    try {
      await endSession();
    } catch (err: unknown) {
      try {
        sessionStorage.removeItem(DELIBERATE_LOGOUT_KEY);
      } catch {
        // sessionStorage unavailable: the login page then shows its notice.
      }
      void showLogoutError(err, {
        object: "die Abmeldung",
        retry: () => void logout().catch(() => undefined),
      });
      throw err;
    }
  };
  return logout;
}

/**
 * Revokes the portal session's refresh-token family on the server (#5194).
 * Only the revoked family ends the session's access tokens, also in a cookie
 * a late response writes back; a failed revocation therefore throws and the
 * caller keeps the session. 401 means nothing is left to revoke.
 */
async function revokeSessionFamily(path: string): Promise<void> {
  const response = await fetch(path, {
    method: "POST",
    signal: AbortSignal.timeout(5000),
  });
  if (!response.ok && response.status !== 401) {
    throw await apiErrorFromResponse(response, "Logout failed");
  }
}

interface ShellUser {
  /**
   * Kontokennung der angemeldeten Person. Jede Portal-Hülle löst die Sitzung
   * ohnehin auf; Karten, die etwas pro Konto im Browser merken (die
   * Geräte-Einrichtung, #2831), holen sie hier statt eine eigene
   * Sitzungsabfrage zu öffnen. Optional, weil ältere Testdoppel der Hülle
   * ohne Kennung auskommen; fehlt sie, entfällt nur der Neustart-Knopf.
   */
  id?: string;
  name: string;
  email: string;
  roles: string[];
  roleIsSystem?: boolean[];
}

interface ShellProfile {
  firstName?: string;
  lastName?: string;
  avatar?: string;
}

type ShellStatus = "loading" | "authenticated" | "unauthenticated";

type ShellMode =
  "teacher" | "operator" | "parent" | "school" | "caterer" | "traeger";

interface ShellAuthContextType {
  user: ShellUser | null;
  profile: ShellProfile | null;
  status: ShellStatus;
  isSessionExpired: boolean;
  logout: () => Promise<void>;
  mode: ShellMode;
  homeUrl: string;
  profileUrl: string | null;
  /**
   * Name neben dem Logo, wenn das Portal einen eigenen hat. Das
   * Träger-Portal zeigt dort den Namen des Trägers (#5145); das
   * Personal-Portal nimmt den Namen der Schule aus dem Tenant-Kontext.
   */
  brandLabel?: string;
  // Admin staff-view preview (#2893) — only the teacher shell fills these.
  // isPreview: this session currently renders the read-only view of another
  // staff member; canStartStaffPreview: the signed-in admin may open one.
  isPreview?: boolean;
  previewTargetName?: string;
  previewTargetAccountId?: string;
  canStartStaffPreview?: boolean;
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

// useShellAuthSafe returns undefined outside a provider instead of throwing.
// For chrome that merely ADAPTS to shell state (the preview offset in
// AppShell) and must keep rendering where no shell provider exists.
export function useShellAuthSafe(): ShellAuthContextType | undefined {
  return useContext(ShellAuthContext);
}

export function TeacherShellProvider({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  const { data: session, status: sessionStatus, update } = useSession();
  const { show: showLogoutError } = useApiErrorDisplay();
  const { profile } = useProfile();
  // Das Logo folgt derselben Portalentscheidung wie der Login-Redirect. Der
  // relative Schulpfad ist auch beim serverseitigen Rendern sicher; der Proxy
  // leitet ihn auf den Schul-Host weiter.
  const redirectPath = getSmartRedirectPath(session);
  const homeUrl = redirectPath;

  const value = useMemo<ShellAuthContextType>(() => {
    const user: ShellUser | null = session?.user
      ? {
          ...(session.user.id ? { id: session.user.id } : {}),
          name: session.user.name?.trim() || "Benutzer",
          email: session.user.email ?? "",
          roles: session.user.roles ?? [],
          ...(session.user.roleIsSystem
            ? { roleIsSystem: session.user.roleIsSystem }
            : {}),
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
      logout: reportingLogout(async () => {
        // Mark as deliberate logout so the login page suppresses the
        // "session expired" banner that NextAuth's required-session
        // redirect would otherwise trigger (race with useSession).
        try {
          sessionStorage.setItem(DELIBERATE_LOGOUT_KEY, "1");
        } catch {
          // sessionStorage unavailable (e.g. private browsing quota)
        }
        // Abmelden aus einer laufenden Vorschau beendet zuerst die Vorschau
        // (#2893): sie endet hier genauso wie per Klick, also gehört ihr Ende
        // ins Protokoll — und der Aufruf braucht das Admin-Token, das erst das
        // Zurückschalten wiederherstellt. Danach widerruft der Logout die
        // Admin-Familie wie gewohnt.
        if (session?.user?.isPreview) {
          try {
            await performEndStaffPreview(update, mutate);
          } catch (err: unknown) {
            logger.warn("staff_preview_end_before_logout_failed", {
              error: err instanceof Error ? err.message : String(err),
            });
          }
        }
        // Best-effort: drop this device's Web Push registration while the
        // session is still valid. Must never block logout.
        await unsubscribePushSilently("tenant");
        // Finish backend revocation before clearing the Auth.js cookie. The
        // response-aware route may persist a just-refreshed session cookie;
        // awaiting it guarantees signOut remains the final cookie mutation.
        await revokeSessionFamily("/api/auth/logout");
        // Namen, Vorauswahl und Zähler dieses Kontos bleiben nicht auf dem
        // Gerät (#5000).
        clearPersonalStorage();
        clearSessionCache();
        await signOut({ redirect: false });
        window.location.href = "/";
      }, showLogoutError),
      mode: "teacher" as const,
      homeUrl,
      profileUrl: "/profile",
      // Admin staff-view preview (#2893): the entry point requires the admin
      // wildcard (same gate as the backend route); during a preview the
      // session carries the TARGET's permissions, so the flag is false then.
      isPreview: session?.user?.isPreview ?? false,
      previewTargetName: session?.user?.previewTargetName,
      previewTargetAccountId: session?.user?.previewTargetAccountId,
      canStartStaffPreview:
        !session?.user?.isPreview && hasPermission(session, "admin:*"),
    };
  }, [session, sessionStatus, profile, homeUrl, update, showLogoutError]);

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
  const { show: showLogoutError } = useApiErrorDisplay();

  const value = useMemo<ShellAuthContextType>(() => {
    const user: ShellUser | null = session?.user
      ? {
          ...(session.user.id ? { id: session.user.id } : {}),
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
      logout: reportingLogout(async () => {
        // The operator family is checked on every operator request; revoking
        // it ends this session's tokens at once (#5194).
        await revokeSessionFamily("/api/operator/auth/logout");
        // Namen, Vorauswahl und Zähler dieses Kontos bleiben nicht auf dem
        // Gerät (#5000).
        clearPersonalStorage();
        clearSessionCache();
        await signOut({ callbackUrl: operatorAbsoluteUrl("/operator/login") });
      }, showLogoutError),
      mode: "operator" as const,
      homeUrl: operatorPath("/operator/organizations"),
      profileUrl: operatorPath("/operator/settings"),
    };
  }, [session, sessionStatus, showLogoutError]);

  return (
    <ShellAuthContext.Provider value={value}>
      {children}
    </ShellAuthContext.Provider>
  );
}

// SchoolShellProvider — the school-portal ("moto schule", #2207) sibling of
// ParentShellProvider. Feeds the AppShell user/profile/logout machinery from
// the school-scope NextAuth session (school.session-token cookie).
export function SchoolShellProvider({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  const { data: session, status: sessionStatus } = useSession();
  const { show: showLogoutError } = useApiErrorDisplay();

  const value = useMemo<ShellAuthContextType>(() => {
    const user: ShellUser | null = session?.user
      ? {
          ...(session.user.id ? { id: session.user.id } : {}),
          name: session.user.name?.trim() || "Lehrkraft",
          email: session.user.email ?? "",
          roles: session.user.roles ?? ["lehrkraft"],
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
      logout: reportingLogout(async () => {
        // Suppress the "session expired" banner the login page would
        // otherwise show after the NextAuth redirect race.
        try {
          sessionStorage.setItem(DELIBERATE_LOGOUT_KEY, "1");
        } catch {
          // sessionStorage unavailable (e.g. private browsing quota)
        }
        // School sessions revoke through the shared scope-preserving
        // /auth/logout (tenant-style), proxied with the school session's
        // refresh token (#5194).
        await revokeSessionFamily("/api/school/auth/logout");
        // Dieses Gerät abmelden, bevor die Session weg ist (#2208): sonst
        // bliebe das Browser-Abo bestehen und der nächste Login auf diesem
        // Gerät fände ein Abo ohne Zuordnung vor.
        await unsubscribePushSilently("school");
        // Namen, Vorauswahl und Zähler dieses Kontos bleiben nicht auf dem
        // Gerät (#5000).
        clearPersonalStorage();
        clearSessionCache();
        await signOut({ callbackUrl: schoolAbsoluteUrl("/school/login") });
      }, showLogoutError),
      mode: "school" as const,
      homeUrl: schoolPath("/school"),
      // Einstellungen (#2208): Benachrichtigungen und Geräte, wie im
      // Elternportal seit #1671.
      profileUrl: schoolPath("/school/einstellungen"),
    };
  }, [session, sessionStatus, showLogoutError]);

  return (
    <ShellAuthContext.Provider value={value}>
      {children}
    </ShellAuthContext.Provider>
  );
}

// CatererShellProvider — the caterer portal (ADR 0077) sibling of
// SchoolShellProvider. Feeds the shared header from the caterer-scope
// NextAuth session (caterer.session-token cookie).
export function CatererShellProvider({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  const { data: session, status: sessionStatus } = useSession();
  const { show: showLogoutError } = useApiErrorDisplay();

  const value = useMemo<ShellAuthContextType>(() => {
    const user: ShellUser | null = session?.user
      ? {
          ...(session.user.id ? { id: session.user.id } : {}),
          name: session.user.name?.trim() || "Caterer",
          email: session.user.email ?? "",
          roles: session.user.roles ?? ["caterer"],
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
      profile: null,
      status,
      isSessionExpired: session?.error === "RefreshTokenExpired",
      logout: reportingLogout(async () => {
        try {
          sessionStorage.setItem(DELIBERATE_LOGOUT_KEY, "1");
        } catch {
          // sessionStorage unavailable: the login page shows its notice.
        }
        await revokeSessionFamily("/api/caterer/auth/logout");
        // Namen, Vorauswahl und Zähler dieses Kontos bleiben nicht auf dem
        // Gerät (#5000).
        clearPersonalStorage();
        clearSessionCache();
        await signOut({ callbackUrl: catererAbsoluteUrl("/caterer/login") });
      }, showLogoutError),
      mode: "caterer" as const,
      homeUrl: catererPath("/caterer"),
      // Die Konto-Seite mit „Passwort ändern“ (#4496).
      profileUrl: catererPath("/caterer/konto"),
    };
  }, [session, sessionStatus, showLogoutError]);

  return (
    <ShellAuthContext.Provider value={value}>
      {children}
    </ShellAuthContext.Provider>
  );
}

// TraegerShellProvider — the Träger portal ("moto träger", #4649) sibling of
// CatererShellProvider (#5145). Feeds the shared header from the org-scope
// NextAuth session (traeger.session-token cookie).
export function TraegerShellProvider({
  children,
  organizationName,
}: {
  readonly children: React.ReactNode;
  /**
   * Der Name des Trägers neben dem Logo. Das Token trägt ihn nicht; der
   * Guard lädt ihn einmal und reicht ihn herein. Solange er fehlt, steht
   * dort nur das Logo.
   */
  readonly organizationName?: string;
}) {
  const { data: session, status: sessionStatus } = useSession();
  const { show: showLogoutError } = useApiErrorDisplay();

  const value = useMemo<ShellAuthContextType>(() => {
    const user: ShellUser | null = session?.user
      ? {
          ...(session.user.id ? { id: session.user.id } : {}),
          name: session.user.name?.trim() || "Träger-Büro",
          email: session.user.email ?? "",
          roles: session.user.roles ?? [],
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
      profile: null,
      status,
      isSessionExpired: session?.error === "RefreshTokenExpired",
      logout: reportingLogout(async () => {
        try {
          sessionStorage.setItem(DELIBERATE_LOGOUT_KEY, "1");
        } catch {
          // sessionStorage unavailable: the login page then shows its
          // "session expired" note, which is harmless.
        }
        // Revoke the backend refresh session first (#5194).
        await revokeSessionFamily("/api/traeger/auth/logout");
        // Namen, Vorauswahl und Zähler dieses Kontos bleiben nicht auf dem
        // Gerät (#5000).
        clearPersonalStorage();
        clearSessionCache();
        // NextAuth resolves a callbackUrl against NEXTAUTH_URL, the tenant
        // origin, and would send the person to the school chooser there. End
        // the session without its redirect and stay on the Träger host.
        await signOut({ redirect: false });
        window.location.assign(traegerPath("/traeger/login"));
      }, showLogoutError),
      mode: "traeger" as const,
      homeUrl: traegerPath("/traeger"),
      // Kein eigenes Profil: Passwort und Code gehören zum moto-Konto.
      profileUrl: null,
      ...(organizationName ? { brandLabel: organizationName } : {}),
    };
  }, [organizationName, session, sessionStatus, showLogoutError]);

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
  accountName,
}: {
  readonly children: React.ReactNode;
  /**
   * Der Name aus dem Elternprofil. Das Eltern-Token trägt keine Namen, nur
   * den Benutzernamen, und der ist meist die E-Mail-Adresse. Das Profil ist
   * dieselbe Quelle wie die Begrüßung der Startseite, damit Kopfzeile und
   * Begrüßung dieselbe Person zeigen (#3891). Ohne Profil bleibt es beim
   * Namen der Sitzung.
   */
  readonly accountName?: {
    readonly firstName?: string;
    readonly lastName?: string;
  };
}) {
  const { data: session, status: sessionStatus } = useSession();
  const { show: showLogoutError } = useApiErrorDisplay();
  const profileFirstName = accountName?.firstName?.trim() ?? "";
  const profileLastName = accountName?.lastName?.trim() ?? "";

  const value = useMemo<ShellAuthContextType>(() => {
    // Elternkonten tragen im Session-Namen haeufig die E-Mail-Adresse. Die
    // darf weder als Name noch als Vorname durchgereicht werden: "Guten Tag,
    // karin.klein@email.de" liest sich wie ein Datenbankauswurf. Ohne
    // brauchbaren Namen bleibt das Profil leer, und die Oberflaeche gruesst
    // ohne Anrede.
    const sessionName = session?.user?.name?.trim() ?? "";
    const nameParts = profileFirstName
      ? [profileFirstName, profileLastName].filter(Boolean)
      : sessionName.includes("@")
        ? []
        : sessionName.split(" ");
    const displayName = nameParts.join(" ");

    const user: ShellUser | null = session?.user
      ? {
          ...(session.user.id ? { id: session.user.id } : {}),
          name: displayName || "Eltern",
          email: session.user.email ?? "",
          roles: session.user.roles ?? ["guardian"],
        }
      : null;

    const firstName =
      profileFirstName || session?.user?.firstName?.trim() || nameParts[0];
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
      logout: reportingLogout(async () => {
        // Best-effort: drop this device's Web Push registration first, while
        // the session can still authenticate it.
        await unsubscribePushSilently("parent");
        // Revoke the parent family before Auth.js clears the cookie (#5194).
        await revokeSessionFamily("/api/parent/auth/logout");
        // Namen, Vorauswahl und Zähler dieses Kontos bleiben nicht auf dem
        // Gerät (#5000).
        clearPersonalStorage();
        clearSessionCache();
        await signOut({ callbackUrl: parentAbsoluteUrl("/parents/login") });
      }, showLogoutError),
      mode: "parent" as const,
      homeUrl: parentPath("/parents"),
      // Was null until #1671: the parents portal had no page of its own for
      // account settings, so the avatar menu offered nothing but "Abmelden".
      profileUrl: parentPath("/parents/settings"),
    };
  }, [
    profileFirstName,
    profileLastName,
    session,
    sessionStatus,
    showLogoutError,
  ]);

  return (
    <ShellAuthContext.Provider value={value}>
      {children}
    </ShellAuthContext.Provider>
  );
}
