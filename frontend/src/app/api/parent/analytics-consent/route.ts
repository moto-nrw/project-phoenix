import { proxyGet, proxyPut } from "~/lib/parent/route-wrapper.server";

// Einwilligung des Elternkontos zur Nutzungsanalyse (#4901), dieselbe
// Backend-Route wie im OGS-Portal; sie gilt für das Konto, nicht für eine Schule.
export const GET = proxyGet<{ granted: boolean | null }>("/analytics-consent/");
export const PUT = proxyPut<{ granted: boolean | null }>("/analytics-consent/");
