import { NextRequest, NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { env } from "~/env";

export async function POST(request: NextRequest) {
    try {
        const session = await auth();

        if (!session?.user?.token) {
            return NextResponse.json(
                { error: "Unauthorized" },
                { status: 401 }
            );
        }

        const body = await request.json();

        const response = await fetch(`${env.NEXT_PUBLIC_API_URL}/auth/password`, {
            method: "POST",
            headers: {
                Authorization: `Bearer ${session.user.token}`,
                "Content-Type": "application/json",
            },
            body: JSON.stringify(body),
        });

        if (!response.ok) {
            const errorText = await response.text();
            return NextResponse.json(
                { error: errorText },
                { status: response.status }
            );
        }

        return new NextResponse(null, { status: 204 });
    } catch (error) {
        console.error("Change password route error:", error);
        return NextResponse.json(
            { error: "Internal Server Error" },
            { status: 500 }
        );
    }
}