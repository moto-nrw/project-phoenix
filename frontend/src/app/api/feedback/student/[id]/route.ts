import { NextResponse } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createTenantApiAdapter } from "~/lib/backend-proxy-route.server";

export const GET = createTenantApiAdapter(async (request, token) => {
  const pathParts = request.nextUrl.pathname.split("/");
  const studentIndex = pathParts.indexOf("student");
  const studentId = studentIndex >= 0 ? pathParts[studentIndex + 1] : undefined;
  if (!studentId) {
    return NextResponse.json(
      { error: "Invalid id parameter" },
      { status: 400 },
    );
  }
  const response = await apiGet<{ data: unknown }>(
    `/api/feedback/student/${studentId}`,
    token,
  );
  return NextResponse.json(
    { status: "success", data: response.data },
    { headers: { "Cache-Control": "no-store" } },
  );
});
