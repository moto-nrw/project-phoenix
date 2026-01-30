import { describe, it, expect, vi, beforeEach } from "vitest";
import { GET } from "./route";

// ============================================================================
// Mocks
// ============================================================================

vi.mock("~/env", () => ({
  env: {
    AUTH_JWT_REFRESH_EXPIRY: "12h",
  },
}));

// ============================================================================
// Test Helpers
// ============================================================================

interface AuthConfig {
  accessTokenExpiry: string;
  refreshTokenExpiry: string;
  nextAuthSessionLength: string;
  proactiveRefreshWindow: string;
  refreshCooldown: string;
  maxRefreshRetries: number;
  tokenRefreshBehavior: string;
}

interface AuthConfigResponse {
  success: boolean;
  config: AuthConfig;
}

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/auth/config", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns auth configuration", async () => {
    const response = await GET();

    expect(response.status).toBe(200);
    const json = (await response.json()) as AuthConfigResponse;
    expect(json.success).toBe(true);
    expect(json.config).toBeDefined();
  });

  it("includes expected configuration fields", async () => {
    const response = await GET();
    const json = (await response.json()) as AuthConfigResponse;

    expect(json.config).toHaveProperty("accessTokenExpiry");
    expect(json.config).toHaveProperty("refreshTokenExpiry");
    expect(json.config).toHaveProperty("nextAuthSessionLength");
    expect(json.config).toHaveProperty("proactiveRefreshWindow");
    expect(json.config).toHaveProperty("refreshCooldown");
    expect(json.config).toHaveProperty("maxRefreshRetries");
    expect(json.config).toHaveProperty("tokenRefreshBehavior");
  });

  it("has correct access token expiry", async () => {
    const response = await GET();
    const json = (await response.json()) as AuthConfigResponse;

    expect(json.config.accessTokenExpiry).toBe("15 minutes");
  });

  it("reflects AUTH_JWT_REFRESH_EXPIRY from env", async () => {
    const response = await GET();
    const json = (await response.json()) as AuthConfigResponse;

    expect(json.config.refreshTokenExpiry).toBe("12h");
  });

  it("includes proactive refresh window", async () => {
    const response = await GET();
    const json = (await response.json()) as AuthConfigResponse;

    expect(json.config.proactiveRefreshWindow).toBe("10 minutes before expiry");
  });

  it("includes refresh cooldown period", async () => {
    const response = await GET();
    const json = (await response.json()) as AuthConfigResponse;

    expect(json.config.refreshCooldown).toBe("30 seconds between attempts");
  });

  it("includes max refresh retries", async () => {
    const response = await GET();
    const json = (await response.json()) as AuthConfigResponse;

    expect(json.config.maxRefreshRetries).toBe(3);
  });

  it("includes token refresh behavior description", async () => {
    const response = await GET();
    const json = (await response.json()) as AuthConfigResponse;

    expect(json.config.tokenRefreshBehavior).toContain("automatically");
    expect(json.config.tokenRefreshBehavior).toContain("10 minutes");
  });
});
