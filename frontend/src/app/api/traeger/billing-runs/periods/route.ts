import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Ferienzeiträume, für die die Abrechnung einen Lauf starten kann (#4668).
export const GET = proxyGet("/traeger/billing-runs/periods");
