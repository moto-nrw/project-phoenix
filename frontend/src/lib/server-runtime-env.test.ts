import { beforeEach, describe, expect, it, vi } from "vitest";
import { validateServerRuntimeEnv } from "./server-runtime-env";

function stubValidRuntimeEnv() {
  vi.stubEnv("API_URL", "http://server:8080");
  vi.stubEnv("AUTH_JWT_EXPIRY", "15m");
  vi.stubEnv("AUTH_JWT_REFRESH_EXPIRY", "168h");
  vi.stubEnv("NODE_ENV", "production");
  vi.stubEnv("NEXTAUTH_URL", "https://moto-app.de");
  vi.stubEnv("NEXTAUTH_SECRET", "test-nextauth-secret");
  vi.stubEnv("TENANT_DOMAIN", "moto-app.de");
  vi.stubEnv("NEXT_PUBLIC_API_URL", "https://api.moto-app.de");
  vi.stubEnv("NEXT_PUBLIC_LOG_LEVEL", "info");
  vi.stubEnv("NEXT_PUBLIC_TENANT_DOMAIN", "moto-app.de");
  vi.stubEnv("NEXT_PUBLIC_OPERATOR_HOSTNAME", "operator.moto-app.de");
  vi.stubEnv("NEXT_PUBLIC_PARENTS_HOSTNAME", "eltern.moto-app.de");
  vi.stubEnv("METRICS_BEARER_TOKEN", "test-token-with-enough-length");
}

describe("validateServerRuntimeEnv", () => {
  beforeEach(() => {
    vi.unstubAllEnvs();
    stubValidRuntimeEnv();
  });

  it("validates the full frontend runtime environment", () => {
    expect(() => validateServerRuntimeEnv()).not.toThrow();
  });

  it("fails fast when NextAuth runtime config is missing", () => {
    vi.stubEnv("NEXTAUTH_SECRET", "");

    expect(() => validateServerRuntimeEnv()).toThrow(
      "Frontend runtime env validation failed: NEXTAUTH_SECRET: Too small",
    );
  });

  it("does not let SKIP_ENV_VALIDATION bypass runtime validation", () => {
    vi.stubEnv("SKIP_ENV_VALIDATION", "true");
    vi.stubEnv("AUTH_JWT_EXPIRY", "");

    expect(() => validateServerRuntimeEnv()).toThrow(
      "Frontend runtime env validation failed: AUTH_JWT_EXPIRY: Too small",
    );
  });
});
