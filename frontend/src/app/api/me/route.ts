import { createGetHandler } from "~/lib/route-wrapper";
import { apiGet } from "~/lib/api-helpers";

export const GET = createGetHandler(async (request, token, params) => {
    const response = await apiGet(`/api/me`, token);
    return response.data;
});