import { type NextRequest, NextResponse } from "next/server";
import { env } from "~/env";

interface PasswordResetRequestBody {
    email: string;
}

interface PasswordResetResponseData {
    message: string;
}

interface ErrorResponse {
    error: string;
}

export async function POST(request: NextRequest) {
    try {
        const body = await request.json() as PasswordResetRequestBody;

        const response = await fetch(`${env.NEXT_PUBLIC_API_URL}/auth/password-reset`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(body),
        });

        if (!response.ok) {
            const errorText = await response.text();
            return NextResponse.json(
                { error: errorText } as ErrorResponse,
                { status: response.status }
            );
        }

        const data = await response.json() as PasswordResetResponseData;
        return NextResponse.json(data);
    } catch (error) {
        console.error("Password reset route error:", error);
        return NextResponse.json(
            { error: "Internal Server Error" } as ErrorResponse,
            { status: 500 }
        );
    }
}