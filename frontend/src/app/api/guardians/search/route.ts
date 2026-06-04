import { createGetHandler } from "@/lib/route-wrapper.server";
import { apiGet } from "@/lib/api-helpers.server";

// GET /api/guardians/search - guardian picker search (#1513).
//
// Shares the users:read gate with the admin guardian list (GET /api/guardians),
// so any staff member who manages students can link a sibling's existing
// guardian. Unlike that full-profile list, the backend returns a minimal,
// enumeration-resistant projection: name, email, and only a COUNT of other
// linked children (never their names) — address, notes, language, contact
// method, and account are all withheld server-side.
export const GET = createGetHandler(async (request, token, _params) => {
  const searchParams = request.nextUrl.searchParams;
  const queryString = searchParams.toString();

  const endpoint = queryString
    ? `/api/guardians/search?${queryString}`
    : "/api/guardians/search";

  const response = await apiGet(endpoint, token);
  // Return the bare data array (mirrors the /api/guardians list route); the
  // route wrapper re-wraps it into the client's { data, status } envelope.
  // @ts-expect-error - API helper returns unknown type
  return response.data;
});
