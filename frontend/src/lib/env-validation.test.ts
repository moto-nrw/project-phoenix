import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  isSkipEnvValidationEnabled,
  validateBuildEnv,
  validateRuntimeEnv,
} from "./env-validation";

const validBuildEnv = {
  API_URL: "http://server:8080",
  TENANT_DOMAIN: "moto-app.de",
  NEXT_PUBLIC_API_URL: "https://api.moto-app.de",
  NEXT_PUBLIC_TENANT_DOMAIN: "moto-app.de",
  NEXT_PUBLIC_POSTHOG_KEY: "",
  NEXT_PUBLIC_POSTHOG_HOST: "",
  NEXT_PUBLIC_OPERATOR_HOSTNAME: "operator.moto-app.de",
  NEXT_PUBLIC_PARENTS_HOSTNAME: "eltern.moto-app.de",
  NEXT_PUBLIC_SCHOOL_HOSTNAME: "schule.moto-app.de",
  NEXT_PUBLIC_SENTRY_DSN: "",
  NEXT_PUBLIC_SENTRY_ENVIRONMENT: "",
};

const validRuntimeEnv = {
  ...validBuildEnv,
  AUTH_JWT_EXPIRY: "15m",
  AUTH_JWT_REFRESH_EXPIRY: "168h",
  NODE_ENV: "production",
  NEXTAUTH_URL: "https://moto-app.de",
  NEXTAUTH_SECRET: "test-nextauth-secret",
  NEXT_PUBLIC_LOG_LEVEL: "info",
  METRICS_BEARER_TOKEN: "test-token-with-enough-length",
};

describe("env validation", () => {
  beforeEach(() => {
    vi.unstubAllEnvs();
  });

  it("fails build validation when a public build-time value is missing", () => {
    expect(() =>
      validateBuildEnv({
        ...validBuildEnv,
        NEXT_PUBLIC_API_URL: "",
      }),
    ).toThrow(
      "Frontend build env validation failed: NEXT_PUBLIC_API_URL: Invalid URL",
    );
  });

  it("does not require runtime secrets during build validation", () => {
    expect(() => validateBuildEnv(validBuildEnv)).not.toThrow();
  });

  it("fails build validation when a PostHog key has no ingestion host", () => {
    expect(() =>
      validateBuildEnv({
        ...validBuildEnv,
        NEXT_PUBLIC_POSTHOG_KEY: "phc_test_key_123",
        NEXT_PUBLIC_POSTHOG_HOST: "",
      }),
    ).toThrow(
      "Frontend build env validation failed: NEXT_PUBLIC_POSTHOG_HOST: Required when NEXT_PUBLIC_POSTHOG_KEY is set",
    );
  });

  it("fails build validation when the PostHog host is invalid", () => {
    expect(() =>
      validateBuildEnv({
        ...validBuildEnv,
        NEXT_PUBLIC_POSTHOG_KEY: "phc_test_key_123",
        NEXT_PUBLIC_POSTHOG_HOST: "not-a-url",
      }),
    ).toThrow(
      "Frontend build env validation failed: NEXT_PUBLIC_POSTHOG_HOST: Invalid URL",
    );
  });

  it("fails build validation when the PostHog host is not HTTP(S)", () => {
    expect(() =>
      validateBuildEnv({
        ...validBuildEnv,
        NEXT_PUBLIC_POSTHOG_KEY: "phc_test_key_123",
        NEXT_PUBLIC_POSTHOG_HOST: "data:text/plain,analytics",
      }),
    ).toThrow(
      "Frontend build env validation failed: NEXT_PUBLIC_POSTHOG_HOST: Must use the http or https protocol",
    );
  });

  it("fails runtime validation when a server secret is missing", () => {
    expect(() =>
      validateRuntimeEnv({
        ...validRuntimeEnv,
        NEXTAUTH_SECRET: "",
      }),
    ).toThrow(
      "Frontend runtime env validation failed: NEXTAUTH_SECRET: Too small",
    );
  });

  it("fails runtime validation when the metrics token is missing", () => {
    expect(() =>
      validateRuntimeEnv({
        ...validRuntimeEnv,
        METRICS_BEARER_TOKEN: " ",
      }),
    ).toThrow(
      "Frontend runtime env validation failed: METRICS_BEARER_TOKEN: Too small",
    );
  });

  it("only enables SKIP_ENV_VALIDATION when it is exactly true", () => {
    vi.stubEnv("SKIP_ENV_VALIDATION", "false");
    expect(isSkipEnvValidationEnabled()).toBe(false);

    vi.stubEnv("SKIP_ENV_VALIDATION", "true");
    expect(isSkipEnvValidationEnabled()).toBe(true);
  });
});
