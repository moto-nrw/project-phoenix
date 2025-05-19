import { type NextRequest, NextResponse } from "next/server";
import { env } from "~/env";

interface PasswordResetConfirmRequest {
    token: string;
    password: string;
}

interface PasswordResetConfirmResponse {
    message: string;
}

export async function POST(request: NextRequest) {
    try {
        const body = await request.json() as PasswordResetConfirmRequest;

        const response = await fetch(`${env.NEXT_PUBLIC_API_URL}/auth/password-reset/confirm`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(body),
        });

        if (!response.ok) {
            const errorText = await response.text();
            return NextResponse.json(
                { error: errorText },
                { status: response.status }
            );
        }

        const data = await response.json() as PasswordResetConfirmResponse;
        return NextResponse.json(data);
    } catch (error: unknown) {
        console.error("Password reset confirm route error:", error);
        return NextResponse.json(
            { error: "Internal Server Error" },
            { status: 500 }
        );
    }
}