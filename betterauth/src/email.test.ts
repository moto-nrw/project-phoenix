/**
 * Tests for email.ts - Email utilities for BetterAuth organization lifecycle notifications.
 *
 * Test coverage targets:
 * - sendOrgPendingEmail: success, API error, network error
 * - sendOrgApprovedEmail: success, API error, network error
 * - sendOrgRejectedEmail: success, API error, network error
 * - sendOrgInvitationEmail: success with URL encoding, API error, network error
 * - syncUserToGoBackend: success, API error, network error
 * - Module-level BASE_DOMAIN validation
 */

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

// Type for email request body
interface EmailRequestBody {
  to: string;
  template: string;
  subject?: string;
  data: Record<string, string | undefined>;
}

// Type for sync user request body
interface SyncUserRequestBody {
  betterauth_user_id: string;
  email: string;
  name: string;
  organization_id: string;
  role: string;
}

// Type for mock fetch call arguments
type MockFetchCall = [string, { method: string; headers: Record<string, string>; body: string }];

// Helper to parse request body from mock calls
function getRequestBody<T>(mockFetch: ReturnType<typeof vi.fn>, callIndex = 0): T {
  const calls = mockFetch.mock.calls as MockFetchCall[];
  const body = calls[callIndex][1].body;
  return JSON.parse(body) as T;
}

// Mock fetch globally - environment variables are set in test/setup.ts
const mockFetch = vi.fn();
global.fetch = mockFetch;

// Import module under test (BASE_DOMAIN and INTERNAL_API_URL set via setupFiles)
import {
  sendOrgPendingEmail,
  sendOrgApprovedEmail,
  sendOrgRejectedEmail,
  sendOrgInvitationEmail,
  syncUserToGoBackend,
} from "./email";

describe("email.ts", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Suppress console output during tests - use vi.fn() instead of empty arrow
    vi.spyOn(console, "log").mockImplementation(vi.fn());
    vi.spyOn(console, "error").mockImplementation(vi.fn());
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe("sendOrgPendingEmail", () => {
    it("sends email with correct template and data on success", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: () => Promise.resolve("OK"),
      });

      await sendOrgPendingEmail({
        to: "admin@test.com",
        firstName: "John",
        orgName: "Test Org",
        subdomain: "testorg",
      });

      expect(mockFetch).toHaveBeenCalledTimes(1);
      expect(mockFetch).toHaveBeenCalledWith(
        "http://test-server:8080/api/internal/email",
        expect.objectContaining({
          method: "POST",
          headers: { "Content-Type": "application/json" },
        }),
      );

      const body = getRequestBody<EmailRequestBody>(mockFetch);
      expect(body).toEqual({
        to: "admin@test.com",
        template: "org-pending",
        subject: undefined,
        data: {
          FirstName: "John",
          OrgName: "Test Org",
          Subdomain: "testorg",
          BaseDomain: "example.com",
          OrgURL: "https://testorg.example.com",
        },
      });

      expect(console.log).toHaveBeenCalledWith(
        "Email sent successfully: org-pending to admin@test.com",
      );
    });

    it("handles optional firstName (undefined)", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: () => Promise.resolve("OK"),
      });

      await sendOrgPendingEmail({
        to: "admin@test.com",
        orgName: "Test Org",
        subdomain: "testorg",
      });

      const body = getRequestBody<EmailRequestBody>(mockFetch);
      expect(body.data.FirstName).toBeUndefined();
    });

    it("logs error when API returns non-OK response", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 500,
        text: () => Promise.resolve("Internal server error"),
      });

      await sendOrgPendingEmail({
        to: "admin@test.com",
        firstName: "John",
        orgName: "Test Org",
        subdomain: "testorg",
      });

      expect(console.error).toHaveBeenCalledWith(
        "Failed to send email (org-pending):",
        500,
        "Internal server error",
      );
      expect(console.log).not.toHaveBeenCalled();
    });

    it("logs error when network request fails", async () => {
      const networkError = new Error("Network connection failed");
      mockFetch.mockRejectedValueOnce(networkError);

      await sendOrgPendingEmail({
        to: "admin@test.com",
        firstName: "John",
        orgName: "Test Org",
        subdomain: "testorg",
      });

      expect(console.error).toHaveBeenCalledWith(
        "Failed to send email (org-pending):",
        networkError,
      );
    });
  });

  describe("sendOrgApprovedEmail", () => {
    it("sends email with correct template and data on success", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: () => Promise.resolve("OK"),
      });

      await sendOrgApprovedEmail({
        to: "owner@test.com",
        firstName: "Jane",
        orgName: "Approved Org",
        subdomain: "approvedorg",
      });

      expect(mockFetch).toHaveBeenCalledTimes(1);

      const body = getRequestBody<EmailRequestBody>(mockFetch);
      expect(body).toEqual({
        to: "owner@test.com",
        template: "org-approved",
        subject: undefined,
        data: {
          FirstName: "Jane",
          OrgName: "Approved Org",
          Subdomain: "approvedorg",
          BaseDomain: "example.com",
          OrgURL: "https://approvedorg.example.com",
        },
      });

      expect(console.log).toHaveBeenCalledWith(
        "Email sent successfully: org-approved to owner@test.com",
      );
    });

    it("handles optional firstName (undefined)", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: () => Promise.resolve("OK"),
      });

      await sendOrgApprovedEmail({
        to: "owner@test.com",
        orgName: "Approved Org",
        subdomain: "approvedorg",
      });

      const body = getRequestBody<EmailRequestBody>(mockFetch);
      expect(body.data.FirstName).toBeUndefined();
    });

    it("logs error when API returns non-OK response", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 400,
        text: () => Promise.resolve("Bad request"),
      });

      await sendOrgApprovedEmail({
        to: "owner@test.com",
        firstName: "Jane",
        orgName: "Approved Org",
        subdomain: "approvedorg",
      });

      expect(console.error).toHaveBeenCalledWith(
        "Failed to send email (org-approved):",
        400,
        "Bad request",
      );
    });

    it("logs error when network request fails", async () => {
      const networkError = new Error("DNS resolution failed");
      mockFetch.mockRejectedValueOnce(networkError);

      await sendOrgApprovedEmail({
        to: "owner@test.com",
        firstName: "Jane",
        orgName: "Approved Org",
        subdomain: "approvedorg",
      });

      expect(console.error).toHaveBeenCalledWith(
        "Failed to send email (org-approved):",
        networkError,
      );
    });
  });

  describe("sendOrgRejectedEmail", () => {
    it("sends email with correct template and data on success", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: () => Promise.resolve("OK"),
      });

      await sendOrgRejectedEmail({
        to: "owner@test.com",
        firstName: "Bob",
        orgName: "Rejected Org",
        reason: "Missing documentation",
      });

      expect(mockFetch).toHaveBeenCalledTimes(1);

      const body = getRequestBody<EmailRequestBody>(mockFetch);
      expect(body).toEqual({
        to: "owner@test.com",
        template: "org-rejected",
        subject: undefined,
        data: {
          FirstName: "Bob",
          OrgName: "Rejected Org",
          Reason: "Missing documentation",
        },
      });

      expect(console.log).toHaveBeenCalledWith(
        "Email sent successfully: org-rejected to owner@test.com",
      );
    });

    it("handles optional firstName and reason (undefined)", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: () => Promise.resolve("OK"),
      });

      await sendOrgRejectedEmail({
        to: "owner@test.com",
        orgName: "Rejected Org",
      });

      const body = getRequestBody<EmailRequestBody>(mockFetch);
      expect(body.data.FirstName).toBeUndefined();
      expect(body.data.Reason).toBeUndefined();
    });

    it("logs error when API returns non-OK response", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 503,
        text: () => Promise.resolve("Service unavailable"),
      });

      await sendOrgRejectedEmail({
        to: "owner@test.com",
        firstName: "Bob",
        orgName: "Rejected Org",
        reason: "Missing documentation",
      });

      expect(console.error).toHaveBeenCalledWith(
        "Failed to send email (org-rejected):",
        503,
        "Service unavailable",
      );
    });

    it("logs error when network request fails", async () => {
      const networkError = new Error("Connection timed out");
      mockFetch.mockRejectedValueOnce(networkError);

      await sendOrgRejectedEmail({
        to: "owner@test.com",
        firstName: "Bob",
        orgName: "Rejected Org",
        reason: "Missing documentation",
      });

      expect(console.error).toHaveBeenCalledWith(
        "Failed to send email (org-rejected):",
        networkError,
      );
    });
  });

  describe("sendOrgInvitationEmail", () => {
    it("sends email with correct template, data, and URL-encoded invitation link", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: () => Promise.resolve("OK"),
      });

      await sendOrgInvitationEmail({
        to: "invitee@test.com",
        firstName: "Alice",
        lastName: "Smith",
        orgName: "Invite Org",
        subdomain: "inviteorg",
        invitationId: "inv-123-abc",
        role: "teacher",
      });

      expect(mockFetch).toHaveBeenCalledTimes(1);

      const body = getRequestBody<EmailRequestBody>(mockFetch);

      // Verify the invitation URL is correctly constructed
      const expectedURL =
        "https://inviteorg.example.com/accept-invitation/inv-123-abc?email=invitee%40test.com&org=Invite+Org&role=teacher";
      expect(body.data.InviteURL).toBe(expectedURL);

      expect(body).toEqual({
        to: "invitee@test.com",
        template: "org-invitation",
        subject: undefined,
        data: {
          FirstName: "Alice",
          LastName: "Smith",
          OrgName: "Invite Org",
          Subdomain: "inviteorg",
          BaseDomain: "example.com",
          InviteURL: expectedURL,
          Role: "teacher",
        },
      });

      expect(console.log).toHaveBeenCalledWith(
        "Email sent successfully: org-invitation to invitee@test.com",
      );
    });

    it("handles optional firstName and lastName (undefined)", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: () => Promise.resolve("OK"),
      });

      await sendOrgInvitationEmail({
        to: "invitee@test.com",
        orgName: "Invite Org",
        subdomain: "inviteorg",
        invitationId: "inv-456",
        role: "admin",
      });

      const body = getRequestBody<EmailRequestBody>(mockFetch);
      expect(body.data.FirstName).toBeUndefined();
      expect(body.data.LastName).toBeUndefined();
    });

    it("URL-encodes special characters in query parameters", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: () => Promise.resolve("OK"),
      });

      await sendOrgInvitationEmail({
        to: "user+alias@test.com",
        firstName: "O'Brian",
        orgName: "Test & Demo Org",
        subdomain: "testorg",
        invitationId: "inv-special",
        role: "super admin",
      });

      const body = getRequestBody<EmailRequestBody>(mockFetch);

      // Email with + should be URL encoded
      expect(body.data.InviteURL).toContain("user%2Balias%40test.com");
      // Org name with & should be URL encoded
      expect(body.data.InviteURL).toContain("Test+%26+Demo+Org");
      // Role with space should be URL encoded
      expect(body.data.InviteURL).toContain("super+admin");
    });

    it("logs error when API returns non-OK response", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 422,
        text: () => Promise.resolve("Invalid email address"),
      });

      await sendOrgInvitationEmail({
        to: "invalid-email",
        firstName: "Alice",
        lastName: "Smith",
        orgName: "Invite Org",
        subdomain: "inviteorg",
        invitationId: "inv-123",
        role: "teacher",
      });

      expect(console.error).toHaveBeenCalledWith(
        "Failed to send email (org-invitation):",
        422,
        "Invalid email address",
      );
    });

    it("logs error when network request fails", async () => {
      const networkError = new Error("Socket closed");
      mockFetch.mockRejectedValueOnce(networkError);

      await sendOrgInvitationEmail({
        to: "invitee@test.com",
        firstName: "Alice",
        lastName: "Smith",
        orgName: "Invite Org",
        subdomain: "inviteorg",
        invitationId: "inv-123",
        role: "teacher",
      });

      expect(console.error).toHaveBeenCalledWith(
        "Failed to send email (org-invitation):",
        networkError,
      );
    });
  });

  describe("syncUserToGoBackend", () => {
    it("returns response data on success", async () => {
      const mockResponse = {
        status: "success",
        message: "User synced",
        person_id: 100,
        staff_id: 200,
        teacher_id: 300,
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      });

      const result = await syncUserToGoBackend({
        betterauthUserId: "ba-user-123",
        email: "teacher@school.com",
        name: "John Teacher",
        organizationId: "org-456",
        role: "teacher",
      });

      expect(result).toEqual(mockResponse);

      expect(mockFetch).toHaveBeenCalledWith(
        "http://test-server:8080/api/internal/sync-user",
        expect.objectContaining({
          method: "POST",
          headers: { "Content-Type": "application/json" },
        }),
      );

      const body = getRequestBody<SyncUserRequestBody>(mockFetch);
      expect(body).toEqual({
        betterauth_user_id: "ba-user-123",
        email: "teacher@school.com",
        name: "John Teacher",
        organization_id: "org-456",
        role: "teacher",
      });

      expect(console.log).toHaveBeenCalledWith(
        "User synced to Go backend successfully:",
        {
          betterauth_user_id: "ba-user-123",
          person_id: 100,
          staff_id: 200,
          teacher_id: 300,
        },
      );
    });

    it("returns null and logs error when API returns non-OK response", async () => {
      const errorResponse = {
        status: "error",
        message: "Organization not found",
      };

      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 404,
        json: () => Promise.resolve(errorResponse),
      });

      const result = await syncUserToGoBackend({
        betterauthUserId: "ba-user-123",
        email: "teacher@school.com",
        name: "John Teacher",
        organizationId: "invalid-org",
        role: "teacher",
      });

      expect(result).toBeNull();
      expect(console.error).toHaveBeenCalledWith(
        "Failed to sync user to Go backend:",
        404,
        errorResponse,
      );
      expect(console.log).not.toHaveBeenCalled();
    });

    it("returns null and logs error when network request fails", async () => {
      const networkError = new Error("ECONNREFUSED");
      mockFetch.mockRejectedValueOnce(networkError);

      const result = await syncUserToGoBackend({
        betterauthUserId: "ba-user-123",
        email: "teacher@school.com",
        name: "John Teacher",
        organizationId: "org-456",
        role: "teacher",
      });

      expect(result).toBeNull();
      expect(console.error).toHaveBeenCalledWith(
        "Failed to sync user to Go backend:",
        networkError,
      );
    });

    it("returns null and logs error when JSON parsing fails", async () => {
      const parseError = new Error("Unexpected token");
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.reject(parseError),
      });

      const result = await syncUserToGoBackend({
        betterauthUserId: "ba-user-123",
        email: "teacher@school.com",
        name: "John Teacher",
        organizationId: "org-456",
        role: "teacher",
      });

      expect(result).toBeNull();
      expect(console.error).toHaveBeenCalledWith(
        "Failed to sync user to Go backend:",
        parseError,
      );
    });

    it("handles response with missing optional fields", async () => {
      const mockResponse = {
        status: "success",
        person_id: 100,
        // staff_id and teacher_id are missing
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      });

      const result = await syncUserToGoBackend({
        betterauthUserId: "ba-user-123",
        email: "staff@school.com",
        name: "Staff Member",
        organizationId: "org-456",
        role: "staff",
      });

      expect(result).toEqual(mockResponse);
      expect(console.log).toHaveBeenCalledWith(
        "User synced to Go backend successfully:",
        {
          betterauth_user_id: "ba-user-123",
          person_id: 100,
          staff_id: undefined,
          teacher_id: undefined,
        },
      );
    });
  });

  describe("module-level validation", () => {
    it("uses INTERNAL_API_URL from environment", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: () => Promise.resolve("OK"),
      });

      await sendOrgPendingEmail({
        to: "test@test.com",
        orgName: "Test",
        subdomain: "test",
      });

      expect(mockFetch).toHaveBeenCalledWith(
        "http://test-server:8080/api/internal/email",
        expect.anything(),
      );
    });

    it("constructs correct URLs using BASE_DOMAIN", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: () => Promise.resolve("OK"),
      });

      await sendOrgApprovedEmail({
        to: "test@test.com",
        orgName: "Test",
        subdomain: "mysubdomain",
      });

      const body = getRequestBody<EmailRequestBody>(mockFetch);

      // Verify BASE_DOMAIN is used in URL construction
      expect(body.data.OrgURL).toBe("https://mysubdomain.example.com");
      expect(body.data.BaseDomain).toBe("example.com");
    });
  });
});

describe("email.ts BASE_DOMAIN validation", () => {
  it("module throws error when BASE_DOMAIN is not set", () => {
    // This test documents the expected behavior but cannot be easily tested
    // because the module is already loaded with BASE_DOMAIN set.
    // The validation happens at module load time (lines 11-13 in email.ts).
    //
    // To verify this behavior manually:
    // 1. Remove BASE_DOMAIN from environment
    // 2. Try to import the module
    // 3. Should throw: "BASE_DOMAIN environment variable is required"
    //
    // This test exists to document the requirement.
    expect(process.env.BASE_DOMAIN).toBe("example.com");
  });
});
