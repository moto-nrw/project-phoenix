import { type NextRequest, NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { getServerApiUrl } from "~/lib/server-api-url";

export async function GET(request: NextRequest) {
  try {
    const session = await auth();
    if (!session?.user?.token) {
      return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
    }

    // Get format parameter from query string
    const searchParams = request.nextUrl.searchParams;
    const format = searchParams.get("format") ?? "csv";

    const response = await fetch(
      `${getServerApiUrl()}/api/import/students/template?format=${format}`,
      {
        headers: {
          Authorization: `Bearer ${session.user.token}`,
        },
      },
    );

    if (!response.ok) {
      const error = await response.text();
      return NextResponse.json(
        { error: error || "Failed to download template" },
        { status: response.status },
      );
    }

    // Get content type and filename from backend response
    const contentType =
      response.headers.get("Content-Type") ?? "text/csv; charset=utf-8";
    const contentDisposition =
      response.headers.get("Content-Disposition") ??
      'attachment; filename="schueler-import-vorlage.csv"';

    const blob = await response.blob();
    return new NextResponse(blob, {
      headers: {
        "Content-Type": contentType,
        "Content-Disposition": contentDisposition,
      },
    });
  } catch (error) {
    console.error("Template download error:", error);
    return NextResponse.json(
      { error: "Internal server error" },
      { status: 500 },
    );
  }
}
