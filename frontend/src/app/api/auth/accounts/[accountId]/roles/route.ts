import { createGetHandler } from "@/lib/route-wrapper";
import { apiGet } from "@/lib/api-client";
import type { BackendRole } from "@/lib/auth-helpers";

interface RolesResponse {
    status: string;
    data: BackendRole[];
    message?: string;
}

export const GET = createGetHandler(async (request, token, params) => {
    // Extract accountId from params, ensuring it's defined
    if (!params.accountId) {
        throw new Error("Account ID is required");
    }
    
    const accountId = params.accountId as string;
    console.log("Fetching roles for account:", accountId);
    
    // Make the API call with the validated account ID
    const response = await apiGet<RolesResponse>(`/auth/accounts/${accountId}/roles`, token);
    return response.data;
});