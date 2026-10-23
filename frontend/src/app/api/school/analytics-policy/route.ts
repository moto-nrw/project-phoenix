import { proxyGet } from "~/lib/school/route-wrapper.server";

export const GET = proxyGet<{ enabled: boolean }>("/analytics-policy");
