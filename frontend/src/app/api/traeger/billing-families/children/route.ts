import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Die Kinder der Familien mit ihren eigenen Debitorennummern (#7007).
export const GET = proxyGet("/traeger/billing-families/children");
