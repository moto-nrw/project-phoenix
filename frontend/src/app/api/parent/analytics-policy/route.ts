import { proxyGet } from "~/lib/parent/route-wrapper.server";

export const GET = proxyGet<{ enabled: boolean }>("/analytics-policy");
