/**
 * Absprung from the Träger portal into a school (#4654). The Träger portal
 * asks the backend for a short-lived hand-off token and opens the school's
 * host on TRAEGER_HANDOFF_PATH with the token in the URL fragment, which no
 * server logs. The entry page there redeems it into the school's own session.
 */

import { clientEnv } from "~/env.client";
import { TRAEGER_SIGN_IN_WITH_CODE_PARAM } from "~/lib/traeger-login-params";

/** The entry page on the school's host. */
const TRAEGER_HANDOFF_PATH = "/traeger-uebergabe";

/** Texts of the Absprung, shared by the Träger portal and the entry page. */
export const TRAEGER_HANDOFF_OPEN = "Schul-App öffnen";
/** What a click on „Schul-App öffnen“ does (#6747). */
export const TRAEGER_HANDOFF_OPEN_HINT =
  "Öffnet die Schul-App dieser Schule in einem neuen Tab.";
export const TRAEGER_HANDOFF_OPENING = "Schule wird geöffnet …";
export const TRAEGER_HANDOFF_FAILED_TITLE =
  "Die Schule konnte nicht geöffnet werden.";
export const TRAEGER_HANDOFF_FAILED_TEXT =
  "Der Link gilt nur kurz und nur einmal. Bitte öffnen Sie die Schule noch einmal im Träger-Portal.";
export const TRAEGER_HANDOFF_BACK = "Zum Träger-Portal";
/** The way back in the school's navigation (#5687). */
export const TRAEGER_RETURN_LABEL = "Zurück zur Träger-App";

/** Whether the path is the entry page, on the school's host or by path. */
export function isTraegerHandoffPath(
  pathname: string | null,
  tenantSlug: string | undefined,
): boolean {
  if (!pathname) return false;
  const path = pathname.replace(/\/+$/, "");
  return (
    path === TRAEGER_HANDOFF_PATH ||
    (!!tenantSlug && path === `/${tenantSlug}${TRAEGER_HANDOFF_PATH}`)
  );
}

/** The entry page of the school with the hand-off token in the fragment. */
export function traegerHandoffUrl(subdomain: string, token: string): string {
  const port = globalThis.location.port ? `:${globalThis.location.port}` : "";
  const fragment = new URLSearchParams({ token }).toString();
  return `${globalThis.location.protocol}//${subdomain}.${clientEnv.NEXT_PUBLIC_TENANT_DOMAIN}${port}${TRAEGER_HANDOFF_PATH}#${fragment}`;
}

/** The start page of the Träger portal, for the way back. */
export function traegerPortalUrl(): string {
  return `${globalThis.location.protocol}//${clientEnv.NEXT_PUBLIC_TRAEGER_HOSTNAME}/`;
}

/** Label of the way back when a school asks for a sign-in with code (#6955). */
export const TRAEGER_SIGN_IN_WITH_CODE_LABEL = "Mit Code neu anmelden";

/**
 * The Träger login that asks for the code (#6955): a school whose remembered
 * devices are off refused a session that came from the Träger-Portal. The
 * person may hold no account at the school, so the way back leads to the
 * Träger-Portal, which ends its session and ignores the device once.
 */
export function traegerSignInWithCodeUrl(): string {
  return `${traegerPortalUrl()}login?${TRAEGER_SIGN_IN_WITH_CODE_PARAM}=1`;
}

/**
 * Reads the hand-off token from the fragment once and removes it from the
 * address bar and the history entry.
 */
export function takeTraegerHandoffToken(): string | null {
  const fragment = new URLSearchParams(globalThis.location.hash.slice(1));
  globalThis.history.replaceState(
    null,
    "",
    globalThis.location.pathname + globalThis.location.search,
  );
  return fragment.get("token")?.trim() || null;
}
