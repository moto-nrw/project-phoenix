import { NextRequest } from "next/server";
import { createGetHandler, createPostHandler } from "@/lib/route-wrapper";
import { apiGet, apiPut } from "@/lib/api-client";
import { auth } from "@/server/auth";

export const GET = createGetHandler(async (request, token, params) => {
    // Extract query parameters
    const searchParams = request.nextUrl.searchParams;
    const email = searchParams.get("email");
    const active = searchParams.get("active");

    // Build query parameters for backend
    const queryParams = new URLSearchParams();
    if (email) queryParams.set("email", email);
    if (active) queryParams.set("active", active);

    const url = queryParams.toString() 
        ? `/auth/accounts?${queryParams.toString()}`
        : "/auth/accounts";

    const response = await apiGet(url, token);
    return response.data;
});

// POST handler for updating accounts
export async function POST(request: NextRequest) {
    const session = await auth();
    if (!session?.user?.token) {
        return Response.json({ error: "Unauthorized" }, { status: 401 });
    }

    try {
        const body = await request.json();
        const { id, ...updateData } = body;
        const response = await apiPut(`/auth/accounts/${id}`, updateData, session.user.token);
        return Response.json(response.data);
    } catch (error) {
        return Response.json({ error: "Failed to update account" }, { status: 500 });
    }
}