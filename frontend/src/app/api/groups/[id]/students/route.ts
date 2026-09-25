import { NextResponse } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createTenantApiAdapter } from "~/lib/backend-proxy-route.server";

export const GET = createTenantApiAdapter(async (_request, token, context) => {
  const groupId = (await context?.params)?.id;
  if (typeof groupId !== "string" || !groupId) {
    return NextResponse.json(
      { error: "Group ID is required" },
      { status: 400 },
    );
  }
  const response = await apiGet(`/api/groups/${groupId}/students`, token);
  // Keep this endpoint's established bare-array success shape.
  if (response && typeof response === "object" && "data" in response) {
    return NextResponse.json(response.data);
  }
  return NextResponse.json(response);
});
