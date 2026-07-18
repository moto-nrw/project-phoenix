import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth, uncachedAuth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({
  component: "EnrollmentStudentRequestsExportRoute",
});

interface RouteContext {
  params: Promise<{ studentId: string }>;
}

async function proxyExport(studentId: string, body: string, token: string) {
  const response = await fetch(
    `${getServerApiUrl()}/api/enrollment/admin/students/${encodeURIComponent(
      studentId,
    )}/requests/export`,
    {
      method: "POST",
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
      },
      body,
      cache: "no-store",
    },
  );

  if (!response.ok) {
    return NextResponse.json(
      { error: await response.text() },
      { status: response.status },
    );
  }

  const contentType =
    response.headers.get("content-type") ?? "application/octet-stream";
  const disposition = response.headers.get("content-disposition");
  const data = await response.arrayBuffer();
  const headers = new Headers({
    "Content-Type": contentType,
    "Content-Length": String(data.byteLength),
  });
  if (disposition) headers.set("Content-Disposition", disposition);
  return new NextResponse(data, { status: 200, headers });
}

async function POSTHandler(request: NextRequest, context: RouteContext) {
  try {
    const session = await auth();
    if (!session?.user?.token) {
      return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
    }

    const { studentId } = await context.params;
    const body = await request.text();
    const response = await proxyExport(studentId, body, session.user.token);
    if (response.status !== 401) return response;

    const refreshed = await uncachedAuth();
    if (
      !refreshed?.user?.token ||
      refreshed.user.token === session.user.token
    ) {
      return response;
    }
    return proxyExport(studentId, body, refreshed.user.token);
  } catch (error) {
    const message = error instanceof Error ? error.message : "Export failed";
    logger.error("student_enrollment_requests_export_failed", {
      error: message,
    });
    return NextResponse.json({ error: message }, { status: 500 });
  }
}

export const POST = withTenantAuth(POSTHandler);
