import "@testing-library/jest-dom/vitest";
import type React from "react";
import { vi } from "vitest";

function createStorageMock(): Storage {
  const store = new Map<string, string>();

  return {
    get length() {
      return store.size;
    },
    clear() {
      store.clear();
    },
    getItem(key: string) {
      return store.get(String(key)) ?? null;
    },
    key(index: number) {
      return Array.from(store.keys())[index] ?? null;
    },
    removeItem(key: string) {
      store.delete(String(key));
    },
    setItem(key: string, value: string) {
      store.set(String(key), String(value));
    },
  };
}

const localStorageMock = createStorageMock();
const sessionStorageMock = createStorageMock();

Object.defineProperty(globalThis, "localStorage", {
  configurable: true,
  writable: true,
  value: localStorageMock,
});

Object.defineProperty(globalThis, "sessionStorage", {
  configurable: true,
  writable: true,
  value: sessionStorageMock,
});

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

// Mock tenant provider globally so tenant-scoped components can render in tests.
// Individual tests can override by calling vi.mocked(useTenant).mockReturnValue(...)
vi.mock("~/components/tenant/tenant-provider", () => ({
  useTenant: vi.fn(() => ({
    tenantSlug: "test-tenant",
    tenant: null,
  })),
  useTenantSlugSafe: vi.fn(() => "test-tenant"),
  TenantProvider: ({
    children,
  }: {
    children: React.ReactNode;
    tenantSlug: string;
    tenant: unknown;
  }) => children,
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
