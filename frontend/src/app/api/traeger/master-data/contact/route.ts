import { proxyPut } from "~/lib/traeger/route-wrapper.server";

// Ansprechpartner des Trägers für seine Dokumente (#4653).
export const PUT = proxyPut("/traeger/master-data/contact");
