import { createGetHandler } from "@/lib/route-wrapper";
import { apiGet } from "@/lib/api-client";

export const GET = createGetHandler(async (request, token, params) => {
    const accountId = params.accountId as string;
    const response = await apiGet(`/auth/accounts/${accountId}/permissions`, token);
    return response.data;
});