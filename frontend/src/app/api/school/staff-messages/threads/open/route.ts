import { proxyPost } from "~/lib/school/route-wrapper.server";

// Unterhaltung mit einer Person öffnen oder anlegen (#2208).
export const POST = proxyPost("/school/staff-messages/threads/open");
