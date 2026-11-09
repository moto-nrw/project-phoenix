// Dokumente, die der Träger für die Schule oder die Eltern an ihr freigegeben
// hat (#4692). Wer sie sehen darf (files:manage), entscheidet das Backend.
import { proxyGet } from "~/lib/route-wrapper.server";

export const GET = proxyGet(() => "/api/traeger-documents/");
