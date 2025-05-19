import { createGetHandler } from "@/lib/route-wrapper";
import { apiGet } from "@/lib/api-client";

export const GET = createGetHandler(async (request, token, params) => {
    const resolvedParams = await params;
    const roleId = resolvedParams.roleId as string;
    const response = await apiGet(`/auth/roles/${roleId}/permissions`, token);
    return response.data;
});