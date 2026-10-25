/**
 * The help entry of the Träger portal (#4649): the article for the
 * Träger-Büro, with the Träger role so the help opens the right sidebar.
 * The help is host-agnostic, so the path is not rewritten.
 */
import { HELP_TOPICS } from "~/lib/help-topics";

export const TRAEGER_HELP_HREF = `/help/${HELP_TOPICS.traegerLogin}?${new URLSearchParams(
  { role: "traeger", return_to: "/" },
).toString()}`;
