/**
 * API endpoint for client-side log ingestion
 *
 * Receives batched logs from browser and writes them to stdout
 * (Promtail captures from Docker logs → Grafana Loki)
 */

import { type NextRequest, NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { parseRequestBody } from "~/lib/route-wrapper-utils.server";
import { validateSessionToken } from "~/server/auth/token-validation";
import { redactSensitiveLogData } from "~/lib/log-redaction";

// Per-process limits are bounded independently of caller-controlled identities.
const budgets = new Map<string, { count: number; until: number }>();
function takeBudget(key: string, limit: number): boolean {
  const now = Date.now();
  for (const [id, budget] of budgets)
    if (budget.until <= now) budgets.delete(id);
  const budget = budgets.get(key);
  if (budget) {
    if (budget.count >= limit) return false;
    budget.count++;
  } else {
    if (budgets.size >= 10001) return false;
    budgets.set(key, { count: 1, until: now + 60000 });
  }
  return true;
}

function isBoundedLogValue(value: unknown, depth = 0): boolean {
  if (depth > 8) return false;
  if (typeof value === "string") return value.length <= 4096;
  if (value === null || typeof value !== "object") return true;
  const fields = Object.entries(value);
  return (
    fields.length <= 50 &&
    fields.every(
      ([key, nested]) =>
        key.length <= 128 && isBoundedLogValue(nested, depth + 1),
    )
  );
}

function validEntry(entry: unknown): entry is Record<string, unknown> {
  if (!entry || typeof entry !== "object" || Array.isArray(entry)) return false;
  const item = entry as Record<string, unknown>;
  return (
    typeof item.msg === "string" &&
    item.msg.length > 0 &&
    typeof item.level === "string" &&
    ["debug", "info", "warn", "error"].includes(item.level) &&
    (item.timestamp === undefined ||
      (typeof item.timestamp === "string" &&
        Number.isFinite(Date.parse(item.timestamp)))) &&
    isBoundedLogValue(item)
  );
}

async function POSTHandler(request: NextRequest) {
  const session = await auth();
  if (!session?.user?.token || session.error) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }
  if (!takeBudget("global", 1000)) return rateLimited();
  const identity = await validateSessionToken(session.user.token, "tenant");
  if (
    !identity ||
    identity.read_only ||
    String(identity.id) !== session.user.id
  ) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }
  if (!takeBudget(`actor:${identity.id}`, 20)) return rateLimited();
  let body: unknown;
  try {
    body = await parseRequestBody<unknown>(request, 64 * 1024);
  } catch (error) {
    const status =
      error instanceof Error && error.message.includes("API error (413)")
        ? 413
        : 400;
    return NextResponse.json({ error: "Invalid log payload" }, { status });
  }
  const entries =
    body && typeof body === "object" && "entries" in body
      ? body.entries
      : undefined;
  // Validate the complete batch before emitting anything.
  if (
    !Array.isArray(entries) ||
    entries.length > 50 ||
    !entries.every(validEntry)
  ) {
    return NextResponse.json({ error: "Invalid log payload" }, { status: 400 });
  }
  for (const entry of entries) {
    console.log(
      JSON.stringify(
        redactSensitiveLogData({
          ...entry,
          via_api: true,
          context: "client",
          provenance: "client_log",
          user_id: String(identity.id),
          tenant_id: identity.tenant_id,
          api_timestamp: new Date().toISOString(),
        }),
      ),
    );
  }
  return NextResponse.json({ status: "success", processed: entries.length });
}

function rateLimited() {
  return NextResponse.json(
    { error: "Rate limit exceeded" },
    { status: 429, headers: { "Retry-After": "60" } },
  );
}

export const POST = withTenantAuth(POSTHandler);
