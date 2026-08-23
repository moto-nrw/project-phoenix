import { describe, it, expect, vi, beforeEach } from "vitest";

const mockCaptureRequestError = vi.fn();
vi.mock("@sentry/nextjs", () => ({
  captureRequestError: mockCaptureRequestError,
}));

describe("instrumentation", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.unstubAllEnvs();
  });

  it("imports sentry.server.config when NEXT_RUNTIME is nodejs", async () => {
    vi.stubEnv("NEXT_RUNTIME", "nodejs");
    stubValidRuntimeEnv();

    const serverMod = vi.fn().mockResolvedValue({});
    vi.doMock("./sentry.server.config", serverMod);

    const { register } = await import("./instrumentation");
    await register();

    expect(serverMod).toHaveBeenCalled();

    vi.doUnmock("./sentry.server.config");
  });

  it("imports sentry.edge.config when NEXT_RUNTIME is edge", async () => {
    vi.stubEnv("NEXT_RUNTIME", "edge");

    const edgeMod = vi.fn().mockResolvedValue({});
    vi.doMock("./sentry.edge.config", edgeMod);

    const { register } = await import("./instrumentation");
    await register();

    expect(edgeMod).toHaveBeenCalled();

    vi.doUnmock("./sentry.edge.config");
  });

  it("does not import any config when NEXT_RUNTIME is unset", async () => {
    vi.stubEnv("NEXT_RUNTIME", "");

    const serverMod = vi.fn().mockResolvedValue({});
    const edgeMod = vi.fn().mockResolvedValue({});
    vi.doMock("./sentry.server.config", serverMod);
    vi.doMock("./sentry.edge.config", edgeMod);

    const { register } = await import("./instrumentation");
    await register();

    expect(serverMod).not.toHaveBeenCalled();
    expect(edgeMod).not.toHaveBeenCalled();

    vi.doUnmock("./sentry.server.config");
    vi.doUnmock("./sentry.edge.config");
  });

  it("exports onRequestError as Sentry.captureRequestError", async () => {
    const { onRequestError } = await import("./instrumentation");

    expect(onRequestError).toBe(mockCaptureRequestError);
  });
});

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
  vi.stubEnv("NEXT_PUBLIC_SCHOOL_HOSTNAME", "schule.moto-app.de");
  vi.stubEnv("METRICS_BEARER_TOKEN", "test-token-with-enough-length");
}
