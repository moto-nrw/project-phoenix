import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { env } from "~/env";

const API_URL = env.NEXT_PUBLIC_API_URL;

export async function DELETE(
  request: NextRequest,
  { params }: { params: { id: string; timeId: string } },
) {
  const session = await auth();

  if (!session?.user?.token) {
    return NextResponse.json(
      { error: "Unauthorized: No valid session" },
      { status: 401 },
    );
  }

  // Properly handle params
  const { id, timeId } = params;

  try {
    const response = await fetch(
      `${API_URL}/activities/${id}/times/${timeId}`,
      {
        method: "DELETE",
        headers: {
          Authorization: `Bearer ${session.user.token}`,
          "Content-Type": "application/json",
        },
      },
    );

    if (!response.ok) {
      const errorText = await response.text();
      console.error(`API error: ${response.status}`, errorText);
      return NextResponse.json(
        { error: `Backend error: ${response.status}` },
        { status: response.status },
      );
    }

    return NextResponse.json({ success: true });
  } catch (error: unknown) {
    console.error(
      `Error deleting time slot ${timeId} from activity ${id}:`,
      error,
    );
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}
