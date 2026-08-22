import { proxyGet, proxyPost } from "~/lib/route-proxy.server";

// Klassenlisteneinträge (#2382): Kinder ohne OGS-Datensatz, nur Name +
// Klasse. Lesen users:read, Anlegen users:create — beides erzwingt das
// Backend.
export const GET = proxyGet("/api/class-list-entries/");
export const POST = proxyPost("/api/class-list-entries/");
