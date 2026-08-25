import { vi } from "vitest";

// Prevent ClientLogger from accessing the browser, starting timers, or making
// requests. Pass through to console.* so existing spies keep working.
vi.mock("~/lib/logger", () => {
  const createMockLogger = (): Record<string, unknown> => ({
    debug: (msg: string, ctx?: Record<string, unknown>) =>
      console.debug(msg, ctx),
    info: (msg: string, ctx?: Record<string, unknown>) =>
      console.info(msg, ctx),
    warn: (msg: string, ctx?: Record<string, unknown>) =>
      console.warn(msg, ctx),
    error: (msg: string, ctx?: Record<string, unknown>) =>
      console.error(msg, ctx),
    flush: () => Promise.resolve(),
    child: () => createMockLogger(),
  });
  return {
    createLogger: vi.fn(() => createMockLogger()),
  };
});

// Avoid Zod validation while keeping required runtime values explicit.
vi.mock("~/env", () => ({
  env: {
    API_URL: "http://server:8080",
    NEXT_PUBLIC_API_URL: "http://localhost:8080",
    NEXTAUTH_URL: "http://localhost:3000",
    NEXTAUTH_SECRET: "test-secret",
    AUTH_SECRET: "test-auth-secret",
    AUTH_JWT_EXPIRY: "15m",
    AUTH_JWT_REFRESH_EXPIRY: "12h",
    NODE_ENV: "test",
  },
}));

// Focused tests exercise the auth wrappers directly. Route tests mock each
// portal auth module, so their wrapper layer stays transparent here.
const passthroughAuthWrapper = <Handler>(handler: Handler): Handler => handler;
vi.mock("~/server/auth/tenant-route", () => ({
  withTenantAuth: passthroughAuthWrapper,
}));
vi.mock("~/server/auth/operator-route", () => ({
  withOperatorAuth: passthroughAuthWrapper,
}));
vi.mock("~/server/auth/parent-route", () => ({
  withParentAuth: passthroughAuthWrapper,
}));
