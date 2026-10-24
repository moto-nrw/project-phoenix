import { proxyPost } from "~/lib/school/route-wrapper.server";

// Neuer Abo-Link; der bisherige gilt danach nicht mehr (#4798).
export const POST = proxyPost("/school/calendar/feed/rotate");
