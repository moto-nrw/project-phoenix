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

const TRAEGER_HELP_HREF = traegerHelpLink(HELP_TOPICS.traegerLogin, "/");

const TRAEGER_SETTINGS_HELP_HREF = traegerHelpLink(
  HELP_TOPICS.traegerSettings,
  "/einstellungen",
);

const TRAEGER_BRIEFKOPF_HELP_HREF = traegerHelpLink(
  HELP_TOPICS.traegerBriefkopf,
  "/briefkopf",
);

function onPage(page: string, path: string): boolean {
  return page === path || page.startsWith(`${path}/`);
}

/** The help article for a Träger page (pathname with or without /traeger). */
export function traegerHelpHref(pathname: string): string {
  const page = pathname.replace(/^\/traeger/, "") || "/";
  if (onPage(page, "/einstellungen")) return TRAEGER_SETTINGS_HELP_HREF;
  if (onPage(page, "/briefkopf")) return TRAEGER_BRIEFKOPF_HELP_HREF;
  return TRAEGER_HELP_HREF;
}
