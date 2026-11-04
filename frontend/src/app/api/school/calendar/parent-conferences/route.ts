import { proxyGet, proxyPost } from "~/lib/school/route-wrapper.server";

// Elternsprechtage der Lehrkraft und ihre Klassen; neuen Tag anlegen (#4801).
export const GET = proxyGet("/school/calendar/parent-conferences");
export const POST = proxyPost("/school/calendar/parent-conferences");
