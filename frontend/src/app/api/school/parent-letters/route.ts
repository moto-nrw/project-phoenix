import { proxyGet, proxyPost } from "~/lib/school/route-wrapper.server";

// Elternbriefe der Lehrkraft an ihre Klasse in moto schule (#4800). GET
// liefert nur die eigenen Briefe, POST legt einen Entwurf an. Das Backend
// prüft, dass die Klasse zur Lehrkraft gehört.
export const GET = proxyGet("/school/parent-letters/");
export const POST = proxyPost("/school/parent-letters/");
