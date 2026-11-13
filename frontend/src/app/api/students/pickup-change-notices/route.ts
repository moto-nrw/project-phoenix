import { proxyGet } from "~/lib/route-wrapper.server";

// GET /api/students/pickup-change-notices - Geänderte Abholzeiten von
// Kindern, für die ich zuständig bin, die ich noch nicht gesehen habe (#6855)
export const GET = proxyGet("/api/students/pickup-change-notices");
