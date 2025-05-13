import { NextRequest, NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { env } from "~/env";

export async function GET(request: NextRequest) {
    try {
        const session = await auth();

        if (!session?.user?.token) {
            return NextResponse.json(
                { error: "Unauthorized" },
                { status: 401 }
            );
        }

        const url = new URL(`${env.NEXT_PUBLIC_API_URL}/auth/roles`);
        const searchParams = request.nextUrl.searchParams;

        searchParams.forEach((value, key) => {
            url.searchParams.append(key, value);
        });

        const response = await fetch(url.toString(), {
            headers: {
                Authorization: `Bearer ${session.user.token}`,
                "Content-Type": "application/json",
            },
        });

        if (!response.ok) {
            const errorText = await response.text();
            return NextResponse.json(
                { error: errorText },
                { status: response.status }
            );
        }

        const data = await response.json();
        return NextResponse.json(data);
    } catch (error) {
        console.error("Get roles route error:", error);
        return NextResponse.json(
            { error: "Internal Server Error" },
            { status: 500 }
        );
    }
}

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

        const response = await fetch(`${env.NEXT_PUBLIC_API_URL}/auth/roles`, {
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

        const data = await response.json();
        return NextResponse.json(data);
    } catch (error) {
        console.error("Create role route error:", error);
        return NextResponse.json(
            { error: "Internal Server Error" },
            { status: 500 }
        );
    }
}