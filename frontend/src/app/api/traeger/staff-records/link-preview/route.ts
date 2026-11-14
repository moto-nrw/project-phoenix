import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Was eine Verknüpfung zusammenbringt, Feld für Feld, vor dem Bestätigen
// (#4705).
export const POST = proxyPost("/traeger/staff-records/link-preview");
