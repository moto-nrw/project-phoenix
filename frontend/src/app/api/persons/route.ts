import { type NextRequest } from "next/server";
import { createPostHandler } from "@/lib/route-wrapper";
import { env } from "@/env";

export const POST = createPostHandler(
  async (req: NextRequest, body: unknown, token: string) => {
    const response = await fetch(`${env.NEXT_PUBLIC_API_URL}/persons`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${token}`,
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
