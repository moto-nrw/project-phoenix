import { proxyGet } from "@/lib/route-proxy.server";

// GET /api/guardians/search - guardian picker search (#1513).
//
// Shares the users:read gate with the admin guardian list (GET /api/guardians),
// so any staff member who manages students can link a sibling's existing
// guardian. Unlike that full-profile list, the backend returns a minimal,
// enumeration-resistant projection: name, email, and only a COUNT of other
// linked children (never their names) — address, notes, language, contact
// method, and account are all withheld server-side.
export const GET = proxyGet("/api/guardians/search");
