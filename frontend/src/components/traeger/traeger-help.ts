/**
 * The help entry of the Träger portal (#4649): the article for the page the
 * Träger-Büro is on, with the Träger role so the help opens the right
 * sidebar. The help is host-agnostic, so the article path is not rewritten;
 * `return_to` is the page's path on the Träger host.
 */
import { HELP_TOPICS } from "~/lib/help-topics";

function traegerHelpLink(topic: string, returnTo: string): string {
  return `/help/${topic}?${new URLSearchParams({
    role: "traeger",
    return_to: returnTo,
  }).toString()}`;
}

export const TRAEGER_HELP_HREF = traegerHelpLink(HELP_TOPICS.traegerLogin, "/");

export const TRAEGER_SETTINGS_HELP_HREF = traegerHelpLink(
  HELP_TOPICS.traegerSettings,
  "/einstellungen",
);

/** The help article for a Träger page (pathname with or without /traeger). */
export function traegerHelpHref(pathname: string): string {
  const page = pathname.replace(/^\/traeger/, "") || "/";
  return page === "/einstellungen" || page.startsWith("/einstellungen/")
    ? TRAEGER_SETTINGS_HELP_HREF
    : TRAEGER_HELP_HREF;
}
