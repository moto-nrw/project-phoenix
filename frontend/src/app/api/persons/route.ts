import { type NextRequest } from "next/server";
import { createPostHandler } from "@/lib/route-wrapper";
import { env } from "@/env";

// BetterAuth: cookieHeader is passed for session validation
export const POST = createPostHandler(
  async (_req: NextRequest, body: unknown, cookieHeader: string) => {
    // BetterAuth: Forward cookies instead of Bearer token
    const response = await fetch(`${env.NEXT_PUBLIC_API_URL}/persons`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Cookie: cookieHeader,
      },
      body: JSON.stringify(body),
    });

    if (!response.ok) {
      const error = await response.text();
      return {
        success: false,
        message: error,
        data: null,
      };
    }

    const result: unknown = await response.json();
    return {
      success: true,
      message: "Person created successfully",
      data: result,
    };
  },
);
