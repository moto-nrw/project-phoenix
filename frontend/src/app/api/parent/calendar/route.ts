import { proxyGet } from "~/lib/parent/route-wrapper.server";

export const GET = proxyGet<unknown>("/parent/me/calendar");
