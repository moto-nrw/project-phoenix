import { proxyGet } from "~/lib/school/route-wrapper.server";

// Ungelesen-Zähler für die Navigation des Schul-Portals (#2208).
export const GET = proxyGet("/school/staff-messages/unread-count");
