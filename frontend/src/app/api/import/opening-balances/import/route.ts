import { NextResponse, type NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "OpeningBalanceImportRoute" });

async function POSTHandler(request: NextRequest) {
  try {
    const session = await auth();
    if (!session?.user?.token) {
      return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
    }

    const formData = await request.formData();

    const response = await fetch(
      `${getServerApiUrl()}/api/import/opening-balances/import`,
      {
        method: "POST",
        headers: {
          Authorization: `Bearer ${session.user.token}`,
        },
        body: formData,
      },
    );

    const data = (await response.json()) as Record<string, unknown>;

    if (!response.ok) {
      return NextResponse.json(data, { status: response.status });
    }

    return NextResponse.json(data);
  } catch (error) {
    logger.error("opening_balance_import_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal server error" },
      { status: 500 },
    );
  }
}

export const POST = withTenantAuth(POSTHandler);
