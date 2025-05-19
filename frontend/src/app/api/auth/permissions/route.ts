import { createGetHandler } from "@/lib/route-wrapper";
import { apiGet } from "@/lib/api-client";

export const GET = createGetHandler(async (request, token, params) => {
    // Extract query parameters
    const searchParams = request.nextUrl.searchParams;
    const resource = searchParams.get("resource");
    const action = searchParams.get("action");

    // Build query parameters for backend
    const queryParams = new URLSearchParams();
    if (resource) queryParams.set("resource", resource);
    if (action) queryParams.set("action", action);

    const url = queryParams.toString() 
        ? `/auth/permissions?${queryParams.toString()}`
        : "/auth/permissions";

    const response = await apiGet(url, token);
    
    // The backend returns { status: "success", data: [...], message: "..." }
    // We need to check if response.data has the expected structure
    if (response.data && 'data' in response.data) {
        // Backend returned the standard response format
        return response.data.data;
    }
    
    // Otherwise return as-is
    return response.data;
});