import { createPostHandler, createDeleteHandler } from "@/lib/route-wrapper";
import { apiPost, apiDelete } from "@/lib/api-client";

export const POST = createPostHandler(async (request, body, token, params) => {
    const accountId = params.accountId as string;
    const roleId = params.roleId as string;
    const response = await apiPost(`/auth/accounts/${accountId}/roles/${roleId}`, null, token);
    return response.data;
});

export const DELETE = createDeleteHandler(async (request, token, params) => {
    const accountId = params.accountId as string;
    const roleId = params.roleId as string;
    const response = await apiDelete(`/auth/accounts/${accountId}/roles/${roleId}`, token);
    return response.data;
});