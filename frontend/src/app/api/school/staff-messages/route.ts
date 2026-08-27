import { proxyGet } from "~/lib/school/route-wrapper.server";

// Posteingang des Team-Chats im Schul-Portal (#2208); `only_unread` läuft als
// Query mit.
export const GET = proxyGet("/school/staff-messages/");
