import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

// The backend's class list is a Go slice, which a school without classes may
// encode as null; the client always gets an array.
export const GET = createGetHandler(async (_request, token) => {
  const response = await apiGet<{ data: string[] | null }>(
    "/api/students/school-classes",
    token,
  );
  return response.data ?? [];
});
