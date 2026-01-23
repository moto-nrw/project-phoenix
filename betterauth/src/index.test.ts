/**
 * Comprehensive tests for betterauth/src/index.ts
 *
 * This file tests all HTTP handlers and utility functions.
 * We mock external dependencies (pg, better-auth, email, fetch)
 * and capture the server's request handler for direct testing.
 */
import {
  describe,
  it,
  expect,
  vi,
  beforeEach,
  afterEach,
} from "vitest";
import type { IncomingMessage, ServerResponse } from "node:http";
import { EventEmitter } from "node:events";

// ============================================
// MOCK SETUP - Must be before imports
// ============================================

// Mock pg Pool
const mockPoolQuery = vi.fn();
const mockPoolConnect = vi.fn();
const mockClientQuery = vi.fn();
const mockClientRelease = vi.fn();

vi.mock("pg", () => ({
  Pool: vi.fn(() => ({
    query: mockPoolQuery,
    connect: mockPoolConnect,
  })),
}));

// Mock better-auth/node
const mockAuthHandler = vi.fn();
vi.mock("better-auth/node", () => ({
  toNodeHandler: vi.fn(() => mockAuthHandler),
}));

// Mock auth module
vi.mock("./auth.js", () => ({
  auth: {},
}));

// Mock email module
const mockSendOrgApprovedEmail = vi.fn().mockResolvedValue(undefined);
const mockSendOrgRejectedEmail = vi.fn().mockResolvedValue(undefined);
const mockSendOrgInvitationEmail = vi.fn().mockResolvedValue(undefined);
const mockSendOrgPendingEmail = vi.fn().mockResolvedValue(undefined);

vi.mock("./email.js", () => ({
  sendOrgApprovedEmail: mockSendOrgApprovedEmail,
  sendOrgRejectedEmail: mockSendOrgRejectedEmail,
  sendOrgInvitationEmail: mockSendOrgInvitationEmail,
  sendOrgPendingEmail: mockSendOrgPendingEmail,
}));

// Mock global fetch
const mockFetch = vi.fn();
vi.stubGlobal("fetch", mockFetch);

// Capture the server handler
let capturedHandler: (req: IncomingMessage, res: ServerResponse) => void;
const mockServer = {
  listen: vi.fn((_port: number, callback: () => void) => {
    callback();
  }),
  close: vi.fn((callback: () => void) => {
    callback();
  }),
};

vi.mock("node:http", async (importOriginal) => {
  const actual: Record<string, unknown> = await importOriginal();
  return {
    ...actual,
    createServer: vi.fn((handler: (req: IncomingMessage, res: ServerResponse) => void) => {
      capturedHandler = handler;
      return mockServer;
    }),
  };
});

// ============================================
// RESPONSE TYPE HELPERS
// ============================================

/**
 * Parse JSON response body with type assertion
 */
function parseResponse<T>(res: MockResponse): T {
  return JSON.parse(res._body) as T;
}

// Common response types
interface ErrorResponse {
  error: string;
  field?: string;
  code?: string;
  unavailableEmails?: string[];
}

interface SuccessOrgResponse {
  success: boolean;
  organization: {
    id: string;
    name: string;
    slug: string;
    status: string;
    createdAt?: string;
    ownerEmail?: string | null;
    ownerName?: string | null;
  };
  invitations?: Array<{
    id: string;
    email: string;
    role: string;
  }>;
}

interface SuccessSignupResponse {
  success: boolean;
  user: {
    id: string;
    email: string;
    name: string;
    emailVerified: boolean;
    createdAt: string;
  };
  organization: {
    id: string;
    name: string;
    slug: string;
    status: string;
  };
  session: {
    token: string;
    expiresAt: string;
  };
}

interface OrganizationListResponse {
  organizations: Array<{
    id: string;
    name: string;
    slug: string;
    status: string;
    createdAt: string;
    ownerEmail: string | null;
    ownerName: string | null;
  }>;
}

// ============================================
// TEST HELPERS
// ============================================

/**
 * Create a mock IncomingMessage with EventEmitter capabilities
 */
function createMockRequest(options: {
  url?: string;
  method?: string;
  headers?: Record<string, string | string[] | undefined>;
  body?: unknown;
}): IncomingMessage {
  const req = new EventEmitter() as IncomingMessage & EventEmitter;
  req.url = options.url ?? "/";
  req.method = options.method ?? "GET";
  req.headers = {
    host: "localhost:3001",
    ...options.headers,
  };

  // Simulate body data emission
  if (options.body !== undefined) {
    setImmediate(() => {
      req.emit("data", JSON.stringify(options.body));
      req.emit("end");
    });
  } else {
    setImmediate(() => {
      req.emit("end");
    });
  }

  return req;
}

/**
 * Mock response type for testing
 */
interface MockResponse {
  _statusCode: number;
  _headers: Record<string, string>;
  _body: string;
  headersSent: boolean;
  writeHead: ReturnType<typeof vi.fn>;
  end: ReturnType<typeof vi.fn>;
  setHeader: ReturnType<typeof vi.fn>;
}

/**
 * Create a mock ServerResponse that captures output
 */
function createMockResponse(): MockResponse {
  const res: MockResponse = {
    _statusCode: 200,
    _headers: {},
    _body: "",
    headersSent: false,
    writeHead: vi.fn(),
    end: vi.fn(),
    setHeader: vi.fn(),
  };

  res.writeHead = vi.fn((statusCode: number, headers?: Record<string, string>) => {
    res._statusCode = statusCode;
    if (headers) {
      Object.assign(res._headers, headers);
    }
    res.headersSent = true;
    return res;
  });

  res.end = vi.fn((body?: string) => {
    if (body) {
      res._body = body;
    }
    return res;
  });

  res.setHeader = vi.fn((name: string, value: string) => {
    res._headers[name] = value;
  });

  return res;
}

/**
 * Wait for async handler to complete
 */
async function handleRequest(
  req: IncomingMessage,
  res: MockResponse,
): Promise<void> {
  // Cast to ServerResponse for the handler
  capturedHandler(req, res as unknown as ServerResponse);
  // Wait for async operations to complete
  await new Promise((resolve) => setTimeout(resolve, 50));
}

// ============================================
// TESTS
// ============================================

describe("betterauth/src/index.ts", () => {
  beforeEach(async () => {
    vi.clearAllMocks();

    // Setup mock client for pool.connect()
    mockPoolConnect.mockResolvedValue({
      query: mockClientQuery,
      release: mockClientRelease,
    });

    // Reset client query mock
    mockClientQuery.mockReset();
    mockClientRelease.mockReset();

    // Import the module to capture the handler
    // Dynamic import to ensure mocks are in place
    await import("./index.js");
  });

  afterEach(() => {
    vi.resetModules();
  });

  // ============================================
  // HEALTH CHECK
  // ============================================

  describe("GET /health", () => {
    it("returns ok status", async () => {
      const req = createMockRequest({ url: "/health", method: "GET" });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(200);
      expect(parseResponse<{ status: string; service: string }>(res)).toEqual({
        status: "ok",
        service: "betterauth",
      });
    });
  });

  // ============================================
  // PUBLIC SEARCH ORGANIZATIONS
  // ============================================

  describe("GET /api/auth/public/organizations", () => {
    it("returns organizations without search term", async () => {
      mockPoolQuery.mockResolvedValueOnce({
        rows: [
          { id: "org-1", name: "Test Org", slug: "test-org" },
        ],
      });

      const req = createMockRequest({
        url: "/api/auth/public/organizations",
        method: "GET",
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(200);
      expect(parseResponse<Array<{ id: string; name: string; slug: string }>>(res)).toEqual([
        { id: "org-1", name: "Test Org", slug: "test-org" },
      ]);
    });

    it("returns organizations with search term", async () => {
      mockPoolQuery.mockResolvedValueOnce({
        rows: [
          { id: "org-1", name: "School Org", slug: "school-org" },
        ],
      });

      const req = createMockRequest({
        url: "/api/auth/public/organizations?search=school&limit=5",
        method: "GET",
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(200);
      expect(mockPoolQuery).toHaveBeenCalledWith(
        expect.stringContaining("LIKE LOWER"),
        ["%school%", 5],
      );
    });

    it("enforces max limit of 50", async () => {
      mockPoolQuery.mockResolvedValueOnce({ rows: [] });

      const req = createMockRequest({
        url: "/api/auth/public/organizations?limit=100",
        method: "GET",
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(mockPoolQuery).toHaveBeenCalledWith(
        expect.any(String),
        [50],
      );
    });

    it("handles database errors", async () => {
      mockPoolQuery.mockRejectedValueOnce(new Error("DB error"));

      const req = createMockRequest({
        url: "/api/auth/public/organizations",
        method: "GET",
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(500);
      expect(parseResponse<ErrorResponse>(res)).toEqual({
        error: "Internal server error",
      });
    });
  });

  // ============================================
  // ORG BY SLUG
  // ============================================

  describe("GET /api/auth/org/by-slug/:slug", () => {
    it("returns organization when found", async () => {
      mockPoolQuery.mockResolvedValueOnce({
        rows: [
          { id: "org-1", name: "Test Org", slug: "test-org", status: "active" },
        ],
      });

      const req = createMockRequest({
        url: "/api/auth/org/by-slug/test-org",
        method: "GET",
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(200);
      expect(parseResponse<{ id: string; name: string; slug: string; status: string }>(res)).toEqual({
        id: "org-1",
        name: "Test Org",
        slug: "test-org",
        status: "active",
      });
    });

    it("returns 404 when organization not found", async () => {
      mockPoolQuery.mockResolvedValueOnce({ rows: [] });

      const req = createMockRequest({
        url: "/api/auth/org/by-slug/nonexistent",
        method: "GET",
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(404);
      expect(parseResponse<ErrorResponse>(res)).toEqual({
        error: "Organization not found",
      });
    });

    it("handles database errors", async () => {
      mockPoolQuery.mockRejectedValueOnce(new Error("DB error"));

      const req = createMockRequest({
        url: "/api/auth/org/by-slug/test-org",
        method: "GET",
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(500);
    });

    it("decodes URL-encoded slugs", async () => {
      mockPoolQuery.mockResolvedValueOnce({
        rows: [{ id: "org-1", name: "Test Org", slug: "test-org", status: "active" }],
      });

      const req = createMockRequest({
        url: "/api/auth/org/by-slug/test%2Dorg",
        method: "GET",
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(mockPoolQuery).toHaveBeenCalledWith(
        expect.any(String),
        ["test-org"],
      );
    });
  });

  // ============================================
  // ADMIN LIST ORGANIZATIONS
  // ============================================

  describe("GET /api/admin/organizations", () => {
    it("returns 401 without internal API key", async () => {
      const req = createMockRequest({
        url: "/api/admin/organizations",
        method: "GET",
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(401);
      expect(parseResponse<ErrorResponse>(res)).toEqual({
        error: "Unauthorized - internal access required",
      });
    });

    it("returns organizations with valid API key", async () => {
      mockPoolQuery.mockResolvedValueOnce({
        rows: [
          {
            id: "org-1",
            name: "Test Org",
            slug: "test-org",
            status: "active",
            createdAt: "2024-01-01",
            ownerEmail: "owner@test.com",
            ownerName: "Owner",
          },
        ],
      });

      const req = createMockRequest({
        url: "/api/admin/organizations",
        method: "GET",
        headers: { "x-internal-api-key": "dev-internal-key" },
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(200);
      expect(parseResponse<OrganizationListResponse>(res)).toHaveProperty("organizations");
    });

    it("filters by status when provided", async () => {
      mockPoolQuery.mockResolvedValueOnce({ rows: [] });

      const req = createMockRequest({
        url: "/api/admin/organizations?status=pending",
        method: "GET",
        headers: { "x-internal-api-key": "dev-internal-key" },
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(mockPoolQuery).toHaveBeenCalledWith(
        expect.stringContaining("WHERE o.status = $1"),
        ["pending"],
      );
    });

    it("handles database errors", async () => {
      mockPoolQuery.mockRejectedValueOnce(new Error("DB error"));

      const req = createMockRequest({
        url: "/api/admin/organizations",
        method: "GET",
        headers: { "x-internal-api-key": "dev-internal-key" },
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(500);
    });
  });

  // ============================================
  // ADMIN CREATE ORGANIZATION
  // ============================================

  describe("POST /api/admin/organizations", () => {
    it("returns 401 without internal API key", async () => {
      const req = createMockRequest({
        url: "/api/admin/organizations",
        method: "POST",
        body: { name: "Test Org" },
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(401);
    });

    it("returns 400 when name is missing", async () => {
      const req = createMockRequest({
        url: "/api/admin/organizations",
        method: "POST",
        headers: { "x-internal-api-key": "dev-internal-key" },
        body: {},
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(400);
      expect(parseResponse<ErrorResponse>(res)).toEqual({
        error: "Organization name is required",
      });
    });

    it("returns 400 when name is empty", async () => {
      const req = createMockRequest({
        url: "/api/admin/organizations",
        method: "POST",
        headers: { "x-internal-api-key": "dev-internal-key" },
        body: { name: "   " },
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(400);
    });

    it("returns 409 when slug already exists", async () => {
      mockPoolQuery.mockResolvedValueOnce({ rows: [{ id: "existing" }] });

      const req = createMockRequest({
        url: "/api/admin/organizations",
        method: "POST",
        headers: { "x-internal-api-key": "dev-internal-key" },
        body: { name: "Test Org", slug: "test-org" },
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(409);
      expect(parseResponse<ErrorResponse>(res)).toEqual({
        error: "Organization with this slug already exists",
      });
    });

    it("creates organization with provided slug", async () => {
      mockPoolQuery.mockResolvedValueOnce({ rows: [] }); // slug check
      mockPoolQuery.mockResolvedValueOnce({
        rows: [
          {
            id: "new-org-id",
            name: "Test Org",
            slug: "custom-slug",
            status: "active",
            createdAt: new Date(),
          },
        ],
      });

      const req = createMockRequest({
        url: "/api/admin/organizations",
        method: "POST",
        headers: { "x-internal-api-key": "dev-internal-key" },
        body: { name: "Test Org", slug: "custom-slug" },
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(201);
      const body = parseResponse<SuccessOrgResponse>(res);
      expect(body.success).toBe(true);
      expect(body.organization.slug).toBe("custom-slug");
    });

    it("generates slug from name when not provided", async () => {
      mockPoolQuery.mockResolvedValueOnce({ rows: [] });
      mockPoolQuery.mockResolvedValueOnce({
        rows: [
          {
            id: "new-org-id",
            name: "Test Organization 123",
            slug: "test-organization-123",
            status: "active",
            createdAt: new Date(),
          },
        ],
      });

      const req = createMockRequest({
        url: "/api/admin/organizations",
        method: "POST",
        headers: { "x-internal-api-key": "dev-internal-key" },
        body: { name: "Test Organization 123" },
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(201);
      expect(mockPoolQuery).toHaveBeenCalledWith(
        expect.stringContaining("SELECT id FROM organization WHERE slug"),
        ["test-organization-123"],
      );
    });

    it("handles database errors", async () => {
      mockPoolQuery.mockRejectedValueOnce(new Error("DB error"));

      const req = createMockRequest({
        url: "/api/admin/organizations",
        method: "POST",
        headers: { "x-internal-api-key": "dev-internal-key" },
        body: { name: "Test Org" },
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(500);
    });
  });

  // ============================================
  // ADMIN PROVISION ORGANIZATION
  // ============================================

  describe("POST /api/admin/organizations/provision", () => {
    it("returns 401 without internal API key", async () => {
      const req = createMockRequest({
        url: "/api/admin/organizations/provision",
        method: "POST",
        body: {},
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(401);
    });

    it("returns 400 when orgName is missing", async () => {
      const req = createMockRequest({
        url: "/api/admin/organizations/provision",
        method: "POST",
        headers: { "x-internal-api-key": "dev-internal-key" },
        body: { orgSlug: "test", invitations: [{ email: "test@test.com", role: "admin" }] },
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(400);
      expect(parseResponse<ErrorResponse>(res)).toEqual({
        error: "Organization name is required",
        field: "orgName",
      });
    });

    it("returns 400 when orgSlug is missing", async () => {
      const req = createMockRequest({
        url: "/api/admin/organizations/provision",
        method: "POST",
        headers: { "x-internal-api-key": "dev-internal-key" },
        body: { orgName: "Test", invitations: [{ email: "test@test.com", role: "admin" }] },
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(400);
      expect(parseResponse<ErrorResponse>(res).field).toBe("orgSlug");
    });

    it("returns 400 when invitations is empty", async () => {
      const req = createMockRequest({
        url: "/api/admin/organizations/provision",
        method: "POST",
        headers: { "x-internal-api-key": "dev-internal-key" },
        body: { orgName: "Test", orgSlug: "test", invitations: [] },
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(400);
      expect(parseResponse<ErrorResponse>(res).field).toBe("invitations");
    });

    it("returns 409 when slug is taken", async () => {
      mockClientQuery.mockResolvedValueOnce({ rows: [{ id: "existing" }] });

      const req = createMockRequest({
        url: "/api/admin/organizations/provision",
        method: "POST",
        headers: { "x-internal-api-key": "dev-internal-key" },
        body: {
          orgName: "Test",
          orgSlug: "existing-slug",
          invitations: [{ email: "test@test.com", role: "admin" }],
        },
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(409);
      expect(parseResponse<ErrorResponse>(res).field).toBe("slug");
    });

    it("returns 500 when email validation service fails", async () => {
      mockClientQuery.mockResolvedValueOnce({ rows: [] }); // slug check
      mockFetch.mockResolvedValueOnce({
        ok: false,
        text: () => Promise.resolve("Service unavailable"),
      });

      const req = createMockRequest({
        url: "/api/admin/organizations/provision",
        method: "POST",
        headers: { "x-internal-api-key": "dev-internal-key" },
        body: {
          orgName: "Test",
          orgSlug: "test-slug",
          invitations: [{ email: "test@test.com", role: "admin" }],
        },
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(500);
      expect(parseResponse<ErrorResponse>(res).error).toBe("Email validation service unavailable");
    });

    it("returns 409 when emails are already registered", async () => {
      mockClientQuery.mockResolvedValueOnce({ rows: [] }); // slug check
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({
          available: [],
          unavailable: ["taken@test.com"],
        }),
      });

      const req = createMockRequest({
        url: "/api/admin/organizations/provision",
        method: "POST",
        headers: { "x-internal-api-key": "dev-internal-key" },
        body: {
          orgName: "Test",
          orgSlug: "test-slug",
          invitations: [{ email: "taken@test.com", role: "admin" }],
        },
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(409);
      expect(parseResponse<ErrorResponse>(res).unavailableEmails).toEqual(["taken@test.com"]);
    });

    it("returns 409 when emails have pending invitations", async () => {
      mockClientQuery.mockResolvedValueOnce({ rows: [] }); // slug check
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ available: ["test@test.com"], unavailable: [] }),
      });
      mockClientQuery.mockResolvedValueOnce({
        rows: [{ email: "test@test.com" }],
      }); // pending invitations check

      const req = createMockRequest({
        url: "/api/admin/organizations/provision",
        method: "POST",
        headers: { "x-internal-api-key": "dev-internal-key" },
        body: {
          orgName: "Test",
          orgSlug: "test-slug",
          invitations: [{ email: "test@test.com", role: "admin" }],
        },
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(409);
      expect(parseResponse<ErrorResponse>(res).field).toBe("email");
    });

    it("creates organization and invitations atomically", async () => {
      // Setup mocks for successful flow
      // Note: getOrCreateSystemUser uses pool.query, transaction uses client.query
      mockClientQuery
        .mockResolvedValueOnce({ rows: [] }) // slug check
        .mockResolvedValueOnce({ rows: [] }) // pending invitations check
        .mockResolvedValueOnce({}) // BEGIN
        .mockResolvedValueOnce({
          rows: [{
            id: "new-org-id",
            name: "Test Org",
            slug: "test-slug",
            status: "active",
            createdAt: new Date(),
          }],
        }) // INSERT org
        .mockResolvedValueOnce({
          rows: [{ id: "inv-1", email: "test@test.com", role: "admin" }],
        }) // INSERT invitation
        .mockResolvedValueOnce({}); // COMMIT

      // getOrCreateSystemUser uses pool.query (not client.query)
      mockPoolQuery.mockResolvedValueOnce({ rows: [{ id: "system-user-id" }] });

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ available: ["test@test.com"], unavailable: [] }),
      });

      const req = createMockRequest({
        url: "/api/admin/organizations/provision",
        method: "POST",
        headers: { "x-internal-api-key": "dev-internal-key" },
        body: {
          orgName: "Test Org",
          orgSlug: "test-slug",
          invitations: [{ email: "Test@Test.com", role: "admin", firstName: "John" }],
        },
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(201);
      const body = parseResponse<SuccessOrgResponse>(res);
      expect(body.success).toBe(true);
      expect(body.organization.slug).toBe("test-slug");
      expect(body.invitations).toHaveLength(1);

      // Check email was lowercased
      expect(mockClientQuery).toHaveBeenCalledWith(
        expect.stringContaining("INSERT INTO invitation"),
        expect.arrayContaining(["test@test.com"]),
      );
    });

    it("rolls back on transaction error", async () => {
      // Note: getOrCreateSystemUser uses pool.query, transaction uses client.query
      mockClientQuery
        .mockResolvedValueOnce({ rows: [] }) // slug check
        .mockResolvedValueOnce({ rows: [] }) // pending invitations check
        .mockResolvedValueOnce({}) // BEGIN
        .mockRejectedValueOnce(new Error("Insert failed")) // INSERT org fails
        .mockResolvedValueOnce({}); // ROLLBACK

      // getOrCreateSystemUser uses pool.query (not client.query)
      mockPoolQuery.mockResolvedValueOnce({ rows: [{ id: "system-user-id" }] });

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ available: ["test@test.com"], unavailable: [] }),
      });

      const req = createMockRequest({
        url: "/api/admin/organizations/provision",
        method: "POST",
        headers: { "x-internal-api-key": "dev-internal-key" },
        body: {
          orgName: "Test Org",
          orgSlug: "test-slug",
          invitations: [{ email: "test@test.com", role: "admin" }],
        },
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(500);
      expect(mockClientQuery).toHaveBeenCalledWith("ROLLBACK");
    });

    it("creates system user when it does not exist", async () => {
      // Note: getOrCreateSystemUser uses pool.query, transaction uses client.query
      mockClientQuery
        .mockResolvedValueOnce({ rows: [] }) // slug check
        .mockResolvedValueOnce({ rows: [] }) // pending invitations check
        .mockResolvedValueOnce({}) // BEGIN
        .mockResolvedValueOnce({
          rows: [{
            id: "new-org-id",
            name: "Test Org",
            slug: "test-slug",
            status: "active",
            createdAt: new Date(),
          }],
        })
        .mockResolvedValueOnce({
          rows: [{ id: "inv-1", email: "test@test.com", role: "admin" }],
        })
        .mockResolvedValueOnce({}); // COMMIT

      // getOrCreateSystemUser uses pool.query - first call checks if exists, second creates
      mockPoolQuery
        .mockResolvedValueOnce({ rows: [] }) // system user doesn't exist
        .mockResolvedValueOnce({ rows: [{ id: "new-system-user" }] }); // create system user

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ available: ["test@test.com"], unavailable: [] }),
      });

      const req = createMockRequest({
        url: "/api/admin/organizations/provision",
        method: "POST",
        headers: { "x-internal-api-key": "dev-internal-key" },
        body: {
          orgName: "Test Org",
          orgSlug: "test-slug",
          invitations: [{ email: "test@test.com", role: "admin" }],
        },
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(201);
      // Verify system user creation was attempted via pool.query (not client.query)
      expect(mockPoolQuery).toHaveBeenCalledWith(
        expect.stringContaining("SELECT id FROM \"user\" WHERE email"),
        ["system@moto-app.de"],
      );
    });
  });

  // ============================================
  // SELF-SERVICE SIGNUP WITH ORG
  // ============================================

  describe("POST /api/auth/signup-with-org", () => {
    it("returns 400 when name is missing", async () => {
      const req = createMockRequest({
        url: "/api/auth/signup-with-org",
        method: "POST",
        body: { email: "test@test.com", password: "password123", orgName: "Test", orgSlug: "test" },
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(400);
      expect(parseResponse<ErrorResponse>(res).field).toBe("name");
    });

    it("returns 400 when email is missing", async () => {
      const req = createMockRequest({
        url: "/api/auth/signup-with-org",
        method: "POST",
        body: { name: "Test User", password: "password123", orgName: "Test", orgSlug: "test" },
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(400);
      expect(parseResponse<ErrorResponse>(res).field).toBe("email");
    });

    it("returns 400 when password is too short", async () => {
      const req = createMockRequest({
        url: "/api/auth/signup-with-org",
        method: "POST",
        body: { name: "Test", email: "test@test.com", password: "short", orgName: "Test", orgSlug: "test" },
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(400);
      expect(parseResponse<ErrorResponse>(res).field).toBe("password");
    });

    it("returns 400 when orgName is missing", async () => {
      const req = createMockRequest({
        url: "/api/auth/signup-with-org",
        method: "POST",
        body: { name: "Test", email: "test@test.com", password: "password123", orgSlug: "test" },
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(400);
      expect(parseResponse<ErrorResponse>(res).field).toBe("orgName");
    });

    it("returns 400 when orgSlug is missing", async () => {
      const req = createMockRequest({
        url: "/api/auth/signup-with-org",
        method: "POST",
        body: { name: "Test", email: "test@test.com", password: "password123", orgName: "Test" },
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(400);
      expect(parseResponse<ErrorResponse>(res).field).toBe("orgSlug");
    });

    it("returns 409 when slug already exists", async () => {
      mockClientQuery.mockResolvedValueOnce({ rows: [{ id: "existing" }] });

      const req = createMockRequest({
        url: "/api/auth/signup-with-org",
        method: "POST",
        body: {
          name: "Test User",
          email: "test@test.com",
          password: "password123",
          orgName: "Test Org",
          orgSlug: "existing-slug",
        },
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(409);
      expect(parseResponse<ErrorResponse>(res).code).toBe("SLUG_ALREADY_EXISTS");
    });

    it("returns 409 when email already exists", async () => {
      mockClientQuery.mockResolvedValueOnce({ rows: [] }); // slug doesn't exist
      mockClientQuery.mockResolvedValueOnce({ rows: [{ id: "existing-user" }] }); // email exists

      const req = createMockRequest({
        url: "/api/auth/signup-with-org",
        method: "POST",
        body: {
          name: "Test User",
          email: "existing@test.com",
          password: "password123",
          orgName: "Test Org",
          orgSlug: "new-slug",
        },
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(409);
      expect(parseResponse<ErrorResponse>(res).code).toBe("USER_ALREADY_EXISTS");
    });

    it("creates user, org, membership, and session atomically", async () => {
      const now = new Date();
      mockClientQuery
        .mockResolvedValueOnce({ rows: [] }) // slug check
        .mockResolvedValueOnce({ rows: [] }) // email check
        .mockResolvedValueOnce({}) // BEGIN
        .mockResolvedValueOnce({
          rows: [{
            id: "user-id",
            email: "test@test.com",
            name: "Test User",
            emailVerified: false,
            createdAt: now,
            updatedAt: now,
          }],
        }) // INSERT user
        .mockResolvedValueOnce({}) // INSERT account
        .mockResolvedValueOnce({
          rows: [{
            id: "org-id",
            name: "Test Org",
            slug: "test-slug",
            status: "pending",
            createdAt: now,
          }],
        }) // INSERT org
        .mockResolvedValueOnce({}) // INSERT member
        .mockResolvedValueOnce({ rows: [{ id: "session-id" }] }) // INSERT session
        .mockResolvedValueOnce({}); // COMMIT

      const req = createMockRequest({
        url: "/api/auth/signup-with-org",
        method: "POST",
        body: {
          name: "Test User",
          email: "TEST@test.com", // Test email normalization
          password: "password123",
          orgName: "Test Org",
          orgSlug: "TEST-SLUG", // Test slug normalization
        },
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(201);
      const body = parseResponse<SuccessSignupResponse>(res);
      expect(body.success).toBe(true);
      expect(body.user.email).toBe("test@test.com");
      expect(body.organization.status).toBe("pending");
      expect(body.session.token).toBeDefined();

      // Verify email was lowercased
      expect(mockClientQuery).toHaveBeenCalledWith(
        expect.stringContaining("INSERT INTO \"user\""),
        ["test@test.com", "Test User"],
      );
    });

    it("rolls back on transaction error", async () => {
      mockClientQuery
        .mockResolvedValueOnce({ rows: [] }) // slug check
        .mockResolvedValueOnce({ rows: [] }) // email check
        .mockResolvedValueOnce({}) // BEGIN
        .mockRejectedValueOnce(new Error("Insert failed")); // user insert fails

      const req = createMockRequest({
        url: "/api/auth/signup-with-org",
        method: "POST",
        body: {
          name: "Test User",
          email: "test@test.com",
          password: "password123",
          orgName: "Test Org",
          orgSlug: "test-slug",
        },
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(500);
      expect(mockClientQuery).toHaveBeenCalledWith("ROLLBACK");
    });
  });

  // ============================================
  // ADMIN STATUS UPDATES (Approve/Reject/Suspend)
  // ============================================

  describe("POST /api/admin/organizations/:id/approve", () => {
    it("returns 401 without internal API key", async () => {
      const req = createMockRequest({
        url: "/api/admin/organizations/org-id/approve",
        method: "POST",
        body: {},
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(401);
    });

    it("returns 404 when organization not found", async () => {
      mockPoolQuery.mockResolvedValueOnce({ rows: [] });

      const req = createMockRequest({
        url: "/api/admin/organizations/nonexistent/approve",
        method: "POST",
        headers: { "x-internal-api-key": "dev-internal-key" },
        body: {},
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(404);
      expect(parseResponse<ErrorResponse>(res)).toEqual({ error: "Organization not found" });
    });

    it("approves organization and sends email", async () => {
      mockPoolQuery
        .mockResolvedValueOnce({
          rows: [{
            id: "org-id",
            name: "Test Org",
            slug: "test-org",
            status: "pending",
            ownerEmail: "owner@test.com",
            ownerName: "Owner Name",
          }],
        }) // get org
        .mockResolvedValueOnce({}); // update status

      const req = createMockRequest({
        url: "/api/admin/organizations/org-id/approve",
        method: "POST",
        headers: { "x-internal-api-key": "dev-internal-key" },
        body: {},
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(200);
      expect(mockPoolQuery).toHaveBeenCalledWith(
        expect.stringContaining("UPDATE organization SET status"),
        ["active", "org-id"],
      );
      expect(mockSendOrgApprovedEmail).toHaveBeenCalledWith({
        to: "owner@test.com",
        firstName: "Owner",
        orgName: "Test Org",
        subdomain: "test-org",
      });
    });

    it("handles org without owner gracefully", async () => {
      mockPoolQuery
        .mockResolvedValueOnce({
          rows: [{
            id: "org-id",
            name: "Test Org",
            slug: "test-org",
            status: "pending",
            ownerEmail: null,
            ownerName: null,
          }],
        })
        .mockResolvedValueOnce({});

      const req = createMockRequest({
        url: "/api/admin/organizations/org-id/approve",
        method: "POST",
        headers: { "x-internal-api-key": "dev-internal-key" },
        body: {},
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(200);
      expect(mockSendOrgApprovedEmail).not.toHaveBeenCalled();
    });
  });

  describe("POST /api/admin/organizations/:id/reject", () => {
    it("rejects organization with reason", async () => {
      mockPoolQuery
        .mockResolvedValueOnce({
          rows: [{
            id: "org-id",
            name: "Test Org",
            slug: "test-org",
            status: "pending",
            ownerEmail: "owner@test.com",
            ownerName: "Owner",
          }],
        })
        .mockResolvedValueOnce({});

      const req = createMockRequest({
        url: "/api/admin/organizations/org-id/reject",
        method: "POST",
        headers: { "x-internal-api-key": "dev-internal-key" },
        body: { reason: "Invalid documentation" },
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(200);
      expect(mockPoolQuery).toHaveBeenCalledWith(
        expect.stringContaining("UPDATE organization SET status"),
        ["rejected", "org-id"],
      );
      expect(mockSendOrgRejectedEmail).toHaveBeenCalledWith({
        to: "owner@test.com",
        firstName: "Owner",
        orgName: "Test Org",
        reason: "Invalid documentation",
      });
    });
  });

  describe("POST /api/admin/organizations/:id/suspend", () => {
    it("suspends organization", async () => {
      mockPoolQuery
        .mockResolvedValueOnce({
          rows: [{
            id: "org-id",
            name: "Test Org",
            slug: "test-org",
            status: "active",
            ownerEmail: "owner@test.com",
            ownerName: "Owner",
          }],
        })
        .mockResolvedValueOnce({});

      const req = createMockRequest({
        url: "/api/admin/organizations/org-id/suspend",
        method: "POST",
        headers: { "x-internal-api-key": "dev-internal-key" },
        body: {},
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(200);
      expect(mockPoolQuery).toHaveBeenCalledWith(
        expect.stringContaining("UPDATE organization SET status"),
        ["suspended", "org-id"],
      );
      // No email sent for suspend
      expect(mockSendOrgApprovedEmail).not.toHaveBeenCalled();
      expect(mockSendOrgRejectedEmail).not.toHaveBeenCalled();
    });
  });

  describe("POST /api/admin/organizations/:id/reactivate", () => {
    it("reactivates suspended organization", async () => {
      mockPoolQuery
        .mockResolvedValueOnce({
          rows: [{
            id: "org-id",
            name: "Test Org",
            slug: "test-org",
            status: "suspended",
            ownerEmail: "owner@test.com",
            ownerName: "Owner",
          }],
        })
        .mockResolvedValueOnce({});

      const req = createMockRequest({
        url: "/api/admin/organizations/org-id/reactivate",
        method: "POST",
        headers: { "x-internal-api-key": "dev-internal-key" },
        body: {},
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(200);
      expect(mockPoolQuery).toHaveBeenCalledWith(
        expect.stringContaining("UPDATE organization SET status"),
        ["active", "org-id"],
      );
    });

    it("handles database errors", async () => {
      mockPoolQuery.mockRejectedValueOnce(new Error("DB error"));

      const req = createMockRequest({
        url: "/api/admin/organizations/org-id/reactivate",
        method: "POST",
        headers: { "x-internal-api-key": "dev-internal-key" },
        body: {},
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(500);
    });
  });

  // ============================================
  // BETTERAUTH FALLBACK
  // ============================================

  describe("BetterAuth handler fallback", () => {
    it("delegates unknown routes to BetterAuth", async () => {
      mockAuthHandler.mockResolvedValueOnce(undefined);

      const req = createMockRequest({
        url: "/api/auth/session",
        method: "GET",
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(mockAuthHandler).toHaveBeenCalledWith(req, res);
    });

    it("handles BetterAuth handler errors", async () => {
      mockAuthHandler.mockRejectedValueOnce(new Error("Auth error"));

      const req = createMockRequest({
        url: "/api/auth/unknown",
        method: "GET",
      });
      const res = createMockResponse();

      await handleRequest(req, res);

      expect(res._statusCode).toBe(500);
      expect(parseResponse<ErrorResponse>(res)).toEqual({ error: "Internal server error" });
    });
  });

  // ============================================
  // JSON BODY PARSING
  // ============================================

  describe("readJsonBody error handling", () => {
    it("handles malformed JSON", async () => {
      const req = new EventEmitter() as IncomingMessage & EventEmitter;
      req.url = "/api/admin/organizations";
      req.method = "POST";
      req.headers = {
        host: "localhost:3001",
        "x-internal-api-key": "dev-internal-key",
      };

      setImmediate(() => {
        req.emit("data", "{ invalid json");
        req.emit("end");
      });

      const res = createMockResponse();

      capturedHandler(req, res as unknown as ServerResponse);
      await new Promise((resolve) => setTimeout(resolve, 50));

      expect(res._statusCode).toBe(500);
    });

    it("handles request stream errors", async () => {
      const req = new EventEmitter() as IncomingMessage & EventEmitter;
      req.url = "/api/admin/organizations";
      req.method = "POST";
      req.headers = {
        host: "localhost:3001",
        "x-internal-api-key": "dev-internal-key",
      };

      setImmediate(() => {
        req.emit("error", new Error("Stream error"));
      });

      const res = createMockResponse();

      capturedHandler(req, res as unknown as ServerResponse);
      await new Promise((resolve) => setTimeout(resolve, 50));

      expect(res._statusCode).toBe(500);
    });
  });
});
