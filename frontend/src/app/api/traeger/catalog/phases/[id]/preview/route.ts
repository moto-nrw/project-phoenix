import { proxyPost } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Vorschau des Übertragens je Schule (#4704). Der Workflow öffnet Träger- und
// Schul-Transaktionen selbst und verwirft die der Schulen wieder.
export const POST = proxyPost(
  (params) =>
    `/traeger/catalog/phases/${requirePathSegmentParam(params)}/preview`,
);
