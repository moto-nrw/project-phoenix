import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Erlöskonten und Kostenstellen des Trägers (#4821), nur mit dem Recht
// „Finanz-Export“.
export const GET = proxyGet("/traeger/finance-exports/booking-accounts");
