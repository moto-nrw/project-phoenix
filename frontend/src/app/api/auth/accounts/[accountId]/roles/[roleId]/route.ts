import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth, getCookieHeader } from "@/server/auth";
import { env } from "~/env";

// Custom POST handler to handle 204 No Content responses
export const POST = async (
  request: NextRequest,
  context: { params: Promise<Record<string, string | string[] | undefined>> },
) => {
  const session = await auth();

  if (!session?.user) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  try {
    const cookieHeader = await getCookieHeader();
    const contextParams = await context.params;
    const accountId = contextParams.accountId as string;
    const roleId = contextParams.roleId as string;

    console.log(`Assigning role ${roleId} to account ${accountId}`);

    // Call the API endpoint
    const response = await fetch(
      `${env.NEXT_PUBLIC_API_URL}/auth/accounts/${accountId}/roles/${roleId}`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Cookie: cookieHeader,
        },
      },
    );

    // If status is 204 No Content, return a success response with empty data
    if (response.status === 204) {
      return NextResponse.json({
        success: true,
        message: "Role assigned successfully",
        data: null,
      });
    }

    // If not a 2xx status, handle the error
    if (!response.ok) {
      const errorText = await response.text();
      console.error(
        `API error when assigning role: ${response.status}:`,
        errorText,
      );

      // Check if the error contains a specific SQL issue with account_role table
      if (
        errorText.includes("account_role") &&
        errorText.includes("missing FROM-clause")
      ) {
        return NextResponse.json(
          {
            success: false,
            message:
              "Database schema mismatch error. This is a backend issue with the auth database schema.",
            error:
              "Backend database configuration error - please contact the administrator.",
          },
          { status: 200 },
        ); // Return 200 to avoid further errors
      }

      throw new Error(`API error (${response.status}): ${errorText}`);
    }

    // If we have a JSON response, parse and return it
    const data = (await response.json()) as unknown;

    return NextResponse.json({
      success: true,
      message: "Role assigned successfully",
      data,
    });
  } catch (error) {
    console.error("Error assigning role:", error);
    return NextResponse.json(
      {
        error: error instanceof Error ? error.message : "Failed to assign role",
      },
      { status: 500 },
    );
  }
};

// Custom DELETE handler to handle 204 No Content responses
export const DELETE = async (
  request: NextRequest,
  context: { params: Promise<Record<string, string | string[] | undefined>> },
) => {
  const session = await auth();

  if (!session?.user) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  try {
    const cookieHeader = await getCookieHeader();
    const contextParams = await context.params;
    const accountId = contextParams.accountId as string;
    const roleId = contextParams.roleId as string;

    console.log(`Removing role ${roleId} from account ${accountId}`);

    // Call the API endpoint
    const response = await fetch(
      `${env.NEXT_PUBLIC_API_URL}/auth/accounts/${accountId}/roles/${roleId}`,
      {
        method: "DELETE",
        headers: {
          "Content-Type": "application/json",
          Cookie: cookieHeader,
        },
      },
    );

    // If status is 204 No Content, return a success response with empty data
    if (response.status === 204) {
      return NextResponse.json({
        success: true,
        message: "Role removed successfully",
        data: null,
      });
    }

    // If not a 2xx status, handle the error
    if (!response.ok) {
      const errorText = await response.text();
      console.error(
        `API error when removing role: ${response.status}:`,
        errorText,
      );

      // Check if the error contains a specific SQL issue with account_role table
      if (
        errorText.includes("account_role") &&
        errorText.includes("missing FROM-clause")
      ) {
        return NextResponse.json(
          {
            success: false,
            message:
              "Database schema mismatch error. This is a backend issue with the auth database schema.",
            error:
              "Backend database configuration error - please contact the administrator.",
          },
          { status: 200 },
        ); // Return 200 to avoid further errors
      }

      throw new Error(`API error (${response.status}): ${errorText}`);
    }

    // If we have a JSON response, parse and return it
    const data = (await response.json()) as unknown;

    return NextResponse.json({
      success: true,
      message: "Role removed successfully",
      data,
    });
  } catch (error) {
    console.error("Error removing role:", error);
    return NextResponse.json(
      {
        error: error instanceof Error ? error.message : "Failed to remove role",
      },
      { status: 500 },
    );
  }
};
