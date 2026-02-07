/**
 * API endpoint for client-side log ingestion
 *
 * Receives batched logs from browser and writes them to stdout
 * (Promtail captures from Docker logs → Grafana Loki)
 */

import { type NextRequest, NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "LogAPI" });

/**
 * POST /api/logs
 * Accept batched logs from client and write to stdout
 *
 * @example
 * ```typescript
 * fetch("/api/logs", {
 *   method: "POST",
 *   body: JSON.stringify({
 *     entries: [
 *       { timestamp: "2026-02-06T12:00:00Z", level: "info", msg: "test" }
 *     ],
 *     timestamp: "2026-02-06T12:00:00Z"
 *   })
 * });
 * ```
 */
export async function POST(request: NextRequest) {
  // Require authenticated session to prevent abuse
  const session = await auth();
  if (!session?.user) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  try {
    const body = (await request.json()) as {
      entries?: unknown[];
      timestamp?: string;
    };

    // Validate payload structure
    if (!body.entries || !Array.isArray(body.entries)) {
      logger.warn("invalid log payload received", {
        has_entries: !!body.entries,
        is_array: Array.isArray(body.entries),
      });

      return NextResponse.json(
        { error: "Invalid payload: missing entries array" },
        { status: 400 },
      );
    }

    // Write each log entry to stdout (Promtail captures)
    for (const entry of body.entries) {
      if (typeof entry === "object" && entry !== null) {
        // Add API metadata and session context
        const enrichedEntry = {
          ...entry,
          via_api: true,
          user_id: session.user.id,
          api_timestamp: new Date().toISOString(),
        };

        // Write JSON to stdout (same as server logger)
        console.log(JSON.stringify(enrichedEntry));
      }
    }

    logger.debug("client logs received", {
      count: body.entries.length,
      batch_timestamp: body.timestamp,
    });

    return NextResponse.json({
      status: "success",
      processed: body.entries.length,
    });
  } catch (error) {
    logger.error("failed to process client logs", {
      error: error instanceof Error ? error.message : String(error),
    });

    return NextResponse.json(
      { error: "Internal server error" },
      { status: 500 },
    );
  }
}
