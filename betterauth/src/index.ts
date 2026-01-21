import { createServer, IncomingMessage, ServerResponse } from "node:http";
import { toNodeHandler } from "better-auth/node";
import { Pool } from "pg";
import { auth } from "./auth.js";
import {
  sendOrgApprovedEmail,
  sendOrgRejectedEmail,
} from "./email.js";

const PORT = parseInt(process.env.PORT ?? "3001", 10);

// SaaS admin emails (comma-separated in env, or default for development)
const SAAS_ADMIN_EMAILS = (
  process.env.SAAS_ADMIN_EMAILS ?? "admin@example.com"
)
  .split(",")
  .map((e) => e.trim().toLowerCase());

// Database pool for custom queries (separate from BetterAuth's internal pool)
const pool = new Pool({
  connectionString: process.env.DATABASE_URL,
});

// Helper to read JSON body from request
async function readJsonBody(req: IncomingMessage): Promise<unknown> {
  return new Promise((resolve, reject) => {
    let body = "";
    req.on("data", (chunk: string) => {
      body += chunk;
    });
    req.on("end", () => {
      try {
        resolve(body ? JSON.parse(body) : {});
      } catch (err) {
        reject(err);
      }
    });
    req.on("error", reject);
  });
}

// Helper to verify session and check admin permissions
async function verifyAdminSession(
  req: IncomingMessage,
): Promise<{ valid: false } | { valid: true; userId: string; email: string }> {
  const cookies = req.headers.cookie ?? "";
  const sessionToken = cookies
    .split(";")
    .find((c) => c.trim().startsWith("better-auth.session_token="))
    ?.split("=")[1]
    ?.trim();

  if (!sessionToken) {
    return { valid: false };
  }

  try {
    // Look up session in database
    const sessionResult = await pool.query(
      `SELECT s.id, s."userId", u.email
       FROM session s
       JOIN "user" u ON u.id = s."userId"
       WHERE s.token = $1 AND s."expiresAt" > NOW()`,
      [sessionToken],
    );

    if (sessionResult.rows.length === 0) {
      return { valid: false };
    }

    const session = sessionResult.rows[0] as {
      id: string;
      userId: string;
      email: string;
    };

    // Check if user is a SaaS admin
    if (!SAAS_ADMIN_EMAILS.includes(session.email.toLowerCase())) {
      return { valid: false };
    }

    return { valid: true, userId: session.userId, email: session.email };
  } catch (error) {
    console.error("Failed to verify admin session:", error);
    return { valid: false };
  }
}

interface OrgWithOwner {
  id: string;
  name: string;
  slug: string;
  status: string;
  createdAt: string;
  ownerEmail: string | null;
  ownerName: string | null;
}

// Handler: List organizations (with optional status filter)
async function handleListOrganizations(
  req: IncomingMessage,
  res: ServerResponse,
): Promise<void> {
  const adminCheck = await verifyAdminSession(req);
  if (!adminCheck.valid) {
    res.writeHead(401, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ error: "Unauthorized" }));
    return;
  }

  try {
    // Parse query params for status filter
    const urlObj = new URL(req.url ?? "", `http://${req.headers.host}`);
    const statusFilter = urlObj.searchParams.get("status");

    let query = `
      SELECT
        o.id, o.name, o.slug, o.status, o."createdAt",
        u.email as "ownerEmail", u.name as "ownerName"
      FROM organization o
      LEFT JOIN member m ON m."organizationId" = o.id AND m.role = 'owner'
      LEFT JOIN "user" u ON u.id = m."userId"
    `;
    const params: string[] = [];

    if (statusFilter) {
      query += ` WHERE o.status = $1`;
      params.push(statusFilter);
    }

    query += ` ORDER BY o."createdAt" DESC`;

    const result = await pool.query(query, params);

    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ organizations: result.rows as OrgWithOwner[] }));
  } catch (error) {
    console.error("Failed to list organizations:", error);
    res.writeHead(500, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ error: "Internal server error" }));
  }
}

// Handler: Update organization status (approve, reject, suspend, reactivate)
async function handleUpdateOrgStatus(
  req: IncomingMessage,
  res: ServerResponse,
  orgId: string,
  newStatus: "active" | "rejected" | "suspended",
): Promise<void> {
  const adminCheck = await verifyAdminSession(req);
  if (!adminCheck.valid) {
    res.writeHead(401, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ error: "Unauthorized" }));
    return;
  }

  try {
    // Read optional reason from body
    const body = (await readJsonBody(req)) as { reason?: string };
    const reason = body.reason;

    // Get current org state
    const orgResult = await pool.query(
      `SELECT o.id, o.name, o.slug, o.status,
              u.email as "ownerEmail", u.name as "ownerName"
       FROM organization o
       LEFT JOIN member m ON m."organizationId" = o.id AND m.role = 'owner'
       LEFT JOIN "user" u ON u.id = m."userId"
       WHERE o.id = $1`,
      [orgId],
    );

    if (orgResult.rows.length === 0) {
      res.writeHead(404, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ error: "Organization not found" }));
      return;
    }

    const org = orgResult.rows[0] as OrgWithOwner;

    // Update status
    await pool.query(`UPDATE organization SET status = $1 WHERE id = $2`, [
      newStatus,
      orgId,
    ]);

    // Send email notification to owner
    if (org.ownerEmail) {
      const firstName = org.ownerName?.split(" ")[0];

      if (newStatus === "active") {
        await sendOrgApprovedEmail({
          to: org.ownerEmail,
          firstName,
          orgName: org.name,
          subdomain: org.slug,
        });
      } else if (newStatus === "rejected") {
        await sendOrgRejectedEmail({
          to: org.ownerEmail,
          firstName,
          orgName: org.name,
          reason,
        });
      }
      // Note: No email for suspend/reactivate - add if needed
    }

    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(
      JSON.stringify({
        success: true,
        organization: { ...org, status: newStatus },
      }),
    );
  } catch (error) {
    console.error(`Failed to update org status to ${newStatus}:`, error);
    res.writeHead(500, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ error: "Internal server error" }));
  }
}

/**
 * Public endpoint to look up an organization by slug.
 * Used by the subdomain middleware to validate tenant context.
 *
 * Returns: { id, name, slug, status } or 404 if not found
 */
async function handleOrgBySlug(
  _req: IncomingMessage,
  res: ServerResponse,
  slug: string,
): Promise<void> {
  try {
    // Query the organization table directly
    // BetterAuth uses "organization" as the table name by default
    const result = await pool.query(
      `SELECT id, name, slug, status FROM organization WHERE slug = $1 LIMIT 1`,
      [slug],
    );

    if (result.rows.length === 0) {
      res.writeHead(404, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ error: "Organization not found" }));
      return;
    }

    const org = result.rows[0] as {
      id: string;
      name: string;
      slug: string;
      status: string;
    };
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify(org));
  } catch (error) {
    console.error("Failed to look up organization by slug:", error);
    res.writeHead(500, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ error: "Internal server error" }));
  }
}

// Create BetterAuth Node.js handler
const authHandler = toNodeHandler(auth);

/**
 * Simple HTTP server that handles BetterAuth API requests.
 *
 * All requests come through the Next.js proxy (server-to-server),
 * so no CORS handling is needed.
 *
 * BetterAuth provides a handler that processes all auth-related endpoints:
 * - POST /api/auth/sign-up/email
 * - POST /api/auth/sign-in/email
 * - POST /api/auth/sign-out
 * - GET /api/auth/session
 * - GET /api/auth/ok (health check)
 * - Organization endpoints (from plugin)
 * - etc.
 */
const server = createServer(
  async (req: IncomingMessage, res: ServerResponse) => {
    const url = req.url ?? "";

    // Custom health check endpoint (outside BetterAuth)
    if (url === "/health" && req.method === "GET") {
      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ status: "ok", service: "betterauth" }));
      return;
    }

    // Custom org lookup by slug endpoint (public, no auth required)
    // Used by subdomain middleware to validate tenant context
    const orgBySlugMatch = url.match(/^\/api\/auth\/org\/by-slug\/([^/?]+)/);
    if (orgBySlugMatch && req.method === "GET") {
      const slug = decodeURIComponent(orgBySlugMatch[1] ?? "");
      if (slug) {
        await handleOrgBySlug(req, res, slug);
        return;
      }
    }

    // Admin: List organizations
    if (url.startsWith("/api/admin/organizations") && req.method === "GET") {
      // Check for specific org ID
      const orgIdMatch = url.match(/^\/api\/admin\/organizations\/([^/?]+)$/);
      if (!orgIdMatch) {
        // List all organizations (with optional ?status=pending filter)
        await handleListOrganizations(req, res);
        return;
      }
    }

    // Admin: Approve organization
    const approveMatch = url.match(
      /^\/api\/admin\/organizations\/([^/?]+)\/approve$/,
    );
    if (approveMatch && req.method === "POST") {
      const orgId = decodeURIComponent(approveMatch[1] ?? "");
      if (orgId) {
        await handleUpdateOrgStatus(req, res, orgId, "active");
        return;
      }
    }

    // Admin: Reject organization
    const rejectMatch = url.match(
      /^\/api\/admin\/organizations\/([^/?]+)\/reject$/,
    );
    if (rejectMatch && req.method === "POST") {
      const orgId = decodeURIComponent(rejectMatch[1] ?? "");
      if (orgId) {
        await handleUpdateOrgStatus(req, res, orgId, "rejected");
        return;
      }
    }

    // Admin: Suspend organization
    const suspendMatch = url.match(
      /^\/api\/admin\/organizations\/([^/?]+)\/suspend$/,
    );
    if (suspendMatch && req.method === "POST") {
      const orgId = decodeURIComponent(suspendMatch[1] ?? "");
      if (orgId) {
        await handleUpdateOrgStatus(req, res, orgId, "suspended");
        return;
      }
    }

    // Admin: Reactivate (un-suspend) organization
    const reactivateMatch = url.match(
      /^\/api\/admin\/organizations\/([^/?]+)\/reactivate$/,
    );
    if (reactivateMatch && req.method === "POST") {
      const orgId = decodeURIComponent(reactivateMatch[1] ?? "");
      if (orgId) {
        await handleUpdateOrgStatus(req, res, orgId, "active");
        return;
      }
    }

    // Let BetterAuth handle the request
    // toNodeHandler handles all /api/auth/* routes
    try {
      await authHandler(req, res);
    } catch (error) {
      console.error("BetterAuth handler error:", error);
      if (!res.headersSent) {
        res.writeHead(500, { "Content-Type": "application/json" });
        res.end(JSON.stringify({ error: "Internal server error" }));
      }
    }
  }
);

server.listen(PORT, () => {
  console.log(`BetterAuth service listening on port ${PORT}`);
  console.log(`Health check: http://localhost:${PORT}/health`);
  console.log(`Auth endpoints: http://localhost:${PORT}/api/auth/*`);
});

// Graceful shutdown
process.on("SIGTERM", () => {
  console.log("SIGTERM received, shutting down gracefully...");
  server.close(() => {
    console.log("Server closed");
    process.exit(0);
  });
});

process.on("SIGINT", () => {
  console.log("SIGINT received, shutting down gracefully...");
  server.close(() => {
    console.log("Server closed");
    process.exit(0);
  });
});
