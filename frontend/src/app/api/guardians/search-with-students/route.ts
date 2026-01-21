import { createGetHandler } from "@/lib/route-wrapper";
import { apiGet } from "@/lib/api-helpers";

// GET /api/guardians/search-with-students - Search guardians with their linked students
export const GET = createGetHandler(async (request, token, _params) => {
  const searchParams = request.nextUrl.searchParams;
  const queryString = searchParams.toString();

  const endpoint = queryString
    ? `/api/guardians/search-with-students?${queryString}`
    : "/api/guardians/search-with-students";

  const response = await apiGet(endpoint, token);
  return response;
});
