import "@testing-library/jest-dom/vitest";
import { vi } from "vitest";

// Mock ~/lib/logger globally to prevent ClientLogger from:
// - Accessing window.location.pathname (crashes in test env)
// - Starting setInterval batch timers (leaks into tests)
// - Making fetch calls to /api/logs
// The mock passes through to console.* so existing spies still work.
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
    child: () => createMockLogger(),
  });
  return {
    createLogger: vi.fn(() => createMockLogger()),
    getLogger: vi.fn(() => createMockLogger()),
  };
});

// Mock ~/env globally to avoid Zod validation issues in tests
vi.mock("~/env", () => ({
  env: {
    NEXT_PUBLIC_API_URL: "http://localhost:8080",
    NEXTAUTH_URL: "http://localhost:3000",
    NEXTAUTH_SECRET: "test-secret",
    AUTH_SECRET: "test-auth-secret",
    AUTH_JWT_EXPIRY: "15m",
    AUTH_JWT_REFRESH_EXPIRY: "12h",
    NODE_ENV: "test",
  },
}));

// Mock SWR globally - individual tests can override with vi.mocked()
vi.mock("swr", () => ({
  default: vi.fn(() => ({
    data: undefined,
    error: undefined,
    isLoading: true,
    isValidating: false,
    mutate: vi.fn(),
  })),
  useSWRConfig: vi.fn(() => ({
    mutate: vi.fn(),
    cache: new Map(),
  })),
}));
