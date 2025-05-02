import { NextRequest, NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { env } from "~/env";

interface RouteContext {
  params: {
    id: string;
    timeId: string;
  };
}

/**
 * DELETE handler for removing a time slot from an activity
 */
export async function DELETE(
  request: NextRequest,
  context: RouteContext
) {
  const { id, timeId } = context.params;
  const session = await auth();
  
  if (!session?.user?.token) {
    return NextResponse.json(
      { error: "Unauthorized" },
      { status: 401 }
    );
  }
  
  try {
    const apiUrl = `${env.NEXT_PUBLIC_API_URL}/activities/${id}/times/${timeId}`;
    const response = await fetch(apiUrl, {
      method: 'DELETE',
      headers: {
        'Authorization': `Bearer ${session.user.token}`,
        'Content-Type': 'application/json',
      },
    });
    
    if (!response.ok) {
      const errorText = await response.text();
      console.error(`API error: ${response.status}`, errorText);
      
      return NextResponse.json(
        { error: `Error from API: ${response.statusText}` },
        { status: response.status }
      );
    }
    
    return new NextResponse(null, { status: 204 });
  } catch (error) {
    console.error(`Error deleting time slot ${timeId} from activity ${id}:`, error);
    return NextResponse.json(
      { error: 'Failed to delete time slot' },
      { status: 500 }
    );
  }
}