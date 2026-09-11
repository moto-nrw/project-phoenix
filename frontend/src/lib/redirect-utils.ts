/**
 * Wohin eine angemeldete Person nach dem Login geht.
 *
 * Seit #2180 gibt es darauf eine Antwort für alle Rollen: die Startseite
 * `/home`. Sie setzt sich aus Bausteinen zusammen, die an Berechtigungen
 * hängen — die Verteilung auf verschiedene Einstiegsseiten (Tagesplan,
 * Kindersuche, Meine Gruppen, Dashboard) ist damit hinfällig. Wer in den
 * laufenden Betreuungstag will, kommt von der Startseite mit einem Klick
 * dorthin.
 */

import type { Session } from "next-auth";

const SCHOOL_PORTAL_HANDOFF_PATH = "/school/login";

/** Die Startseite der App (#2180). */
export const HOME_PATH = "/home";

export function isSchoolPortalHandoffPath(path: string): boolean {
  return path === SCHOOL_PORTAL_HANDOFF_PATH;
}

/**
 * Der Einstieg nach dem Login.
 *
 * Die einzige Ausnahme von `/home` ist eine Sitzung, die gar nicht ins
 * Mitarbeiter-Portal gehört: Zugriffstoken, die vor dem Umzug des Schul-
 * Portals ausgestellt wurden, werden dorthin weitergereicht, bis sie
 * auslaufen.
 */
export function getSmartRedirectPath(session: Session | null): string {
  // Exact matching avoids treating tenant-defined roles with similar names as
  // the system role.
  const isExistingSchoolPortalOnlySession =
    session?.user?.roles?.length === 1 && session.user.roles[0] === "lehrkraft";

  return isExistingSchoolPortalOnlySession
    ? SCHOOL_PORTAL_HANDOFF_PATH
    : HOME_PATH;
}

/**
 * Dieselbe Entscheidung in der Form, die die Weiterleitungs-Komponente nutzt.
 *
 * `isReady` bleibt im Vertrag, ist aber immer wahr: das Ziel hängt seit #2180
 * an nichts mehr, das erst nachgeladen werden müsste.
 */
export function useSmartRedirectPath(session: Session | null): {
  redirectPath: string;
  isReady: boolean;
} {
  return { redirectPath: getSmartRedirectPath(session), isReady: true };
}
