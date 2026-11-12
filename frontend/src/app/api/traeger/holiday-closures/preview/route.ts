import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Ferienbuchungen, die ein geplanter Schließtag betrifft (#6434).
export const POST = proxyPost("/traeger/holiday-closures/preview");
