import { type NextRequest, NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { env } from "~/env";

export async function POST(_request: NextRequest) {
    try {
        const session = await auth();

        if (!session?.user?.token) {
            return NextResponse.json(
                { error: "No active session" },
                { status: 401 }
            );
        }

        // Forward to backend
        const response = await fetch(`${env.NEXT_PUBLIC_API_URL}/auth/logout`, {
            method: "POST",
            headers: {
                Authorization: `Bearer ${session.user.token}`,
                "Content-Type": "application/json",
            },
        });

        if (!response.ok && response.status !== 204) {
            const errorText = await response.text();
            console.error(`Logout error: ${response.status}`, errorText);
        }

        // Always return success to client
        return new NextResponse(null, { status: 204 });
    } catch (error) {
        console.error("Logout route error:", error);
        // Still return success - logout should always succeed on client side
        return new NextResponse(null, { status: 204 });
    }
}