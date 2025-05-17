import { type NextRequest, NextResponse } from "next/server";
import { RouteWrapper } from "@/lib/route-wrapper";
import { env } from "@/env";
import { getServerSession } from "next-auth";
import { authOptions } from "@/server/auth/config";

export const POST = RouteWrapper(async (req: NextRequest) => {
    const session = await getServerSession(authOptions);
    if (!session?.user?.token) {
        return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
    }

    const data = await req.json();

    const response = await fetch(`${env.NEXT_PUBLIC_API_URL}/persons`, {
        method: "POST",
        headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${session.user.token}`,
        },
        body: JSON.stringify(data),
    });

    if (!response.ok) {
        const error = await response.text();
        return NextResponse.json(
            { success: false, message: error },
            { status: response.status }
        );
    }

    const result = await response.json();
    return NextResponse.json(result);
});