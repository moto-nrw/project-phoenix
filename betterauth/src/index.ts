import { createServer, IncomingMessage, ServerResponse } from "node:http";
import { toNodeHandler } from "better-auth/node";
import { Pool } from "pg";
import { auth } from "./auth.js";
import {
  sendOrgApprovedEmail,
  sendOrgRejectedEmail,
  sendOrgInvitationEmail,
} from "./email.js";

const PORT = parseInt(process.env.PORT ?? "3001", 10);

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

// Internal API key for server-to-server calls (set in environment)
// When this header is present and matches, skip session verification
const INTERNAL_API_KEY = process.env.INTERNAL_API_KEY ?? "dev-internal-key";

// Helper to verify admin access
// For internal calls (from Next.js), we trust the X-Internal-API-Key header
// The actual admin email check happens in the Next.js API routes
function verifyInternalAccess(req: IncomingMessage): boolean {
  const apiKey = req.headers["x-internal-api-key"];
  return apiKey === INTERNAL_API_KEY;
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
  // Verify internal API access (auth check done in Next.js API routes)
  if (!verifyInternalAccess(req)) {
    res.writeHead(401, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ error: "Unauthorized - internal access required" }));
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

/**
 * Handler: Create organization directly with active status (for SaaS admin console)
 * This creates an organization without an owner - the first invited admin will become the owner
 */
async function handleCreateOrganization(
  req: IncomingMessage,
  res: ServerResponse,
): Promise<void> {
  // Verify internal API access (auth check done in Next.js API routes)
  if (!verifyInternalAccess(req)) {
    res.writeHead(401, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ error: "Unauthorized - internal access required" }));
    return;
  }

  try {
    const body = (await readJsonBody(req)) as {
      name?: string;
      slug?: string;
    };

    if (!body.name?.trim()) {
      res.writeHead(400, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ error: "Organization name is required" }));
      return;
    }

    // Generate slug from name if not provided
    const slug = body.slug?.trim() || body.name.trim()
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/^-|-$/g, "");

    // Check if slug already exists
    const existingOrg = await pool.query(
      `SELECT id FROM organization WHERE slug = $1`,
      [slug],
    );

    if (existingOrg.rows.length > 0) {
      res.writeHead(409, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ error: "Organization with this slug already exists" }));
      return;
    }

    // Create organization with active status
    const result = await pool.query(
      `INSERT INTO organization (id, name, slug, status, "createdAt", "allowPublicSignup", "requireMemberApproval")
       VALUES (gen_random_uuid(), $1, $2, 'active', NOW(), false, true)
       RETURNING id, name, slug, status, "createdAt"`,
      [body.name.trim(), slug],
    );

    const org = result.rows[0] as {
      id: string;
      name: string;
      slug: string;
      status: string;
      createdAt: Date;
    };

    res.writeHead(201, { "Content-Type": "application/json" });
    res.end(JSON.stringify({
      success: true,
      organization: {
        ...org,
        ownerEmail: null,
        ownerName: null,
      },
    }));
  } catch (error) {
    console.error("Failed to create organization:", error);
    res.writeHead(500, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ error: "Internal server error" }));
  }
}

// Go backend URL for internal API calls
const GO_BACKEND_URL = process.env.INTERNAL_API_URL ?? "http://server:8080";

// System user ID for SaaS admin operations (invitations without a real user)
const SAAS_ADMIN_EMAIL = "system@moto-app.de";
const SAAS_ADMIN_NAME = "SaaS System";

/**
 * Get or create the system user for SaaS admin operations.
 * This user serves as the inviter for invitations created via the admin console.
 */
async function getOrCreateSystemUser(): Promise<string> {
  // Check if system user already exists
  const existing = await pool.query(
    `SELECT id FROM "user" WHERE email = $1`,
    [SAAS_ADMIN_EMAIL],
  );

  if (existing.rows.length > 0) {
    return (existing.rows[0] as { id: string }).id;
  }

  // Create system user
  const result = await pool.query(
    `INSERT INTO "user" (id, email, name, "emailVerified", "createdAt", "updatedAt")
     VALUES (gen_random_uuid(), $1, $2, true, NOW(), NOW())
     RETURNING id`,
    [SAAS_ADMIN_EMAIL, SAAS_ADMIN_NAME],
  );

  return (result.rows[0] as { id: string }).id;
}

interface ProvisionInvitation {
  email: string;
  role: "admin" | "member" | "owner";
  firstName?: string;
  lastName?: string;
}

interface ProvisionRequest {
  orgName: string;
  orgSlug: string;
  invitations: ProvisionInvitation[];
}

interface ValidateEmailsResponse {
  available: string[];
  unavailable: string[];
}

/**
 * Handler: Atomic organization provisioning (for SaaS admin console)
 * This endpoint creates an organization AND its invitations atomically.
 * If any validation fails (slug taken, emails registered), nothing is created.
 *
 * POST /api/admin/organizations/provision
 */
async function handleProvisionOrganization(
  req: IncomingMessage,
  res: ServerResponse,
): Promise<void> {
  // Verify internal API access
  if (!verifyInternalAccess(req)) {
    res.writeHead(401, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ error: "Unauthorized - internal access required" }));
    return;
  }

  const client = await pool.connect();

  try {
    const body = (await readJsonBody(req)) as ProvisionRequest;

    // Validate request body
    if (!body.orgName?.trim()) {
      res.writeHead(400, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ error: "Organization name is required", field: "orgName" }));
      return;
    }

    if (!body.orgSlug?.trim()) {
      res.writeHead(400, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ error: "Organization slug is required", field: "orgSlug" }));
      return;
    }

    if (!body.invitations || body.invitations.length === 0) {
      res.writeHead(400, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ error: "At least one invitation is required", field: "invitations" }));
      return;
    }

    const orgName = body.orgName.trim();
    const orgSlug = body.orgSlug.trim().toLowerCase();

    // ============================================
    // PHASE 1: VALIDATE ALL (no writes yet)
    // ============================================

    // 1a. Validate slug not taken
    const slugExists = await client.query(
      `SELECT 1 FROM organization WHERE slug = $1`,
      [orgSlug],
    );

    if (slugExists.rows.length > 0) {
      res.writeHead(409, { "Content-Type": "application/json" });
      res.end(JSON.stringify({
        error: "Diese Subdomain ist bereits vergeben",
        field: "slug",
      }));
      return;
    }

    // 1b. Validate emails via Go backend
    const emails = body.invitations.map((inv) => inv.email.toLowerCase().trim());

    const emailValidation = await fetch(`${GO_BACKEND_URL}/api/internal/validate-emails`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ emails }),
    });

    if (!emailValidation.ok) {
      console.error("Failed to validate emails:", await emailValidation.text());
      res.writeHead(500, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ error: "Email validation service unavailable" }));
      return;
    }

    const emailResult = (await emailValidation.json()) as ValidateEmailsResponse;

    if (emailResult.unavailable.length > 0) {
      res.writeHead(409, { "Content-Type": "application/json" });
      res.end(JSON.stringify({
        error: `E-Mail-Adressen bereits registriert: ${emailResult.unavailable.join(", ")}`,
        field: "email",
        unavailableEmails: emailResult.unavailable,
      }));
      return;
    }

    // 1c. Check emails don't already have pending invitations for any org
    const existingInvitations = await client.query(
      `SELECT email FROM invitation WHERE email = ANY($1) AND status = 'pending'`,
      [emails],
    );

    if (existingInvitations.rows.length > 0) {
      const existingEmails = (existingInvitations.rows as { email: string }[]).map((r) => r.email);
      res.writeHead(409, { "Content-Type": "application/json" });
      res.end(JSON.stringify({
        error: `E-Mail-Adressen haben bereits ausstehende Einladungen: ${existingEmails.join(", ")}`,
        field: "email",
        unavailableEmails: existingEmails,
      }));
      return;
    }

    // ============================================
    // PHASE 2: CREATE (atomic within transaction)
    // ============================================

    // Get system user for inviter
    const systemUserId = await getOrCreateSystemUser();

    // Start transaction
    await client.query("BEGIN");

    try {
      // 2a. Create organization with active status
      const orgResult = await client.query(
        `INSERT INTO organization (id, name, slug, status, "createdAt", "allowPublicSignup", "requireMemberApproval")
         VALUES (gen_random_uuid(), $1, $2, 'active', NOW(), false, true)
         RETURNING id, name, slug, status, "createdAt"`,
        [orgName, orgSlug],
      );

      const org = orgResult.rows[0] as {
        id: string;
        name: string;
        slug: string;
        status: string;
        createdAt: Date;
      };

      // 2b. Create invitations
      const createdInvitations: Array<{
        id: string;
        email: string;
        role: string;
        firstName?: string;
        lastName?: string;
      }> = [];

      for (const inv of body.invitations) {
        const invResult = await client.query(
          `INSERT INTO invitation (id, "organizationId", email, role, status, "expiresAt", "createdAt", "inviterId")
           VALUES (gen_random_uuid(), $1, $2, $3, 'pending', NOW() + INTERVAL '48 hours', NOW(), $4)
           RETURNING id, email, role`,
          [org.id, inv.email.toLowerCase().trim(), inv.role, systemUserId],
        );

        createdInvitations.push({
          id: (invResult.rows[0] as { id: string }).id,
          email: inv.email.toLowerCase().trim(),
          role: inv.role,
          firstName: inv.firstName,
          lastName: inv.lastName,
        });
      }

      // Commit transaction
      await client.query("COMMIT");

      // 2c. Send invitation emails (async, fire-and-forget after commit)
      for (const invitation of createdInvitations) {
        sendOrgInvitationEmail({
          to: invitation.email,
          firstName: invitation.firstName,
          lastName: invitation.lastName,
          orgName: org.name,
          subdomain: org.slug,
          invitationId: invitation.id,
          role: invitation.role,
        }).catch((err: unknown) => {
          console.error(`Failed to send invitation email to ${invitation.email}:`, err);
        });
      }

      console.log(
        `Successfully provisioned org "${org.name}" (${org.slug}) with ${createdInvitations.length} invitations`,
      );

      res.writeHead(201, { "Content-Type": "application/json" });
      res.end(JSON.stringify({
        success: true,
        organization: {
          id: org.id,
          name: org.name,
          slug: org.slug,
          status: org.status,
          createdAt: org.createdAt,
        },
        invitations: createdInvitations.map((inv) => ({
          id: inv.id,
          email: inv.email,
          role: inv.role,
        })),
      }));
    } catch (txError) {
      // Rollback on any error during creation
      await client.query("ROLLBACK");
      throw txError;
    }
  } catch (error) {
    console.error("Failed to provision organization:", error);
    res.writeHead(500, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ error: "Internal server error" }));
  } finally {
    client.release();
  }
}

// Handler: Update organization status (approve, reject, suspend, reactivate)
async function handleUpdateOrgStatus(
  req: IncomingMessage,
  res: ServerResponse,
  orgId: string,
  newStatus: "active" | "rejected" | "suspended",
): Promise<void> {
  // Verify internal API access (auth check done in Next.js API routes)
  if (!verifyInternalAccess(req)) {
    res.writeHead(401, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ error: "Unauthorized - internal access required" }));
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
 * Public endpoint to search organizations by name or slug.
 * Used by the main domain org-selection page.
 * Only returns active organizations with minimal public fields.
 *
 * Query params:
 * - search: Optional search term (matches name or slug)
 * - limit: Max results (default 10, max 50)
 *
 * Returns: Array of { id, name, slug }
 */
async function handleSearchOrganizations(
  req: IncomingMessage,
  res: ServerResponse,
): Promise<void> {
  try {
    const urlObj = new URL(req.url ?? "", `http://${req.headers.host}`);
    const searchTerm = urlObj.searchParams.get("search") ?? "";
    const limitParam = urlObj.searchParams.get("limit");
    const limit = Math.min(Math.max(parseInt(limitParam ?? "10", 10) || 10, 1), 50);

    let query: string;
    let params: (string | number)[];

    if (searchTerm.trim()) {
      // Search by name or slug (case-insensitive)
      query = `
        SELECT id, name, slug
        FROM organization
        WHERE status = 'active'
          AND (LOWER(name) LIKE LOWER($1) OR LOWER(slug) LIKE LOWER($1))
        ORDER BY name ASC
        LIMIT $2
      `;
      params = [`%${searchTerm}%`, limit];
    } else {
      // No search term - return first N active organizations
      query = `
        SELECT id, name, slug
        FROM organization
        WHERE status = 'active'
        ORDER BY name ASC
        LIMIT $1
      `;
      params = [limit];
    }

    const result = await pool.query(query, params);

    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify(result.rows));
  } catch (error) {
    console.error("Failed to search organizations:", error);
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

    // Public: Search organizations (for org-selection page on main domain)
    // GET /api/auth/public/organizations?search=...&limit=10
    if (url.startsWith("/api/auth/public/organizations") && req.method === "GET") {
      await handleSearchOrganizations(req, res);
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

    // Admin: Create organization with active status (no owner)
    if (url === "/api/admin/organizations" && req.method === "POST") {
      await handleCreateOrganization(req, res);
      return;
    }

    // Admin: Atomic organization provisioning (org + invitations)
    // This is the atomic endpoint that creates org AND invitations together
    if (url === "/api/admin/organizations/provision" && req.method === "POST") {
      await handleProvisionOrganization(req, res);
      return;
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
