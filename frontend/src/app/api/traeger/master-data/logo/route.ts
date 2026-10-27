import {
  proxyDelete,
  proxyFile,
  proxyUpload,
} from "~/lib/traeger/route-wrapper.server";
import { TRAEGER_LOGO_MAX_BYTES } from "~/lib/traeger-api";

// Logo des Trägers (#4653): das gespeicherte Bild, ein neues hochladen oder
// es entfernen. Typ, Größe und Inhalt prüft das Backend.
export const GET = proxyFile("/traeger/master-data/logo");

export const PUT = proxyUpload(
  "/traeger/master-data/logo",
  TRAEGER_LOGO_MAX_BYTES,
);

export const DELETE = proxyDelete("/traeger/master-data/logo");
