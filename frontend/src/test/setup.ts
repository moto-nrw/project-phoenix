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
// or `vi.unmock("~/components/tenant/tenant-provider")` when they need the real
// context wired up (see `binary-mode-guard.test.tsx` for an example).
vi.mock("~/components/tenant/tenant-provider", () => ({
  useTenant: vi.fn(() => ({
    tenantSlug: "test-tenant",
    tenant: null,
  })),
  useTenantSlugSafe: vi.fn(() => "test-tenant"),
  // Safe context accessor — never throws. Tests that need a populated
  // tenant override this mock locally; the default returns a slug-only
  // context so feature-flag hooks (useStudentPhotosEnabled) read `false`
  // by default and don't render avatars in unrelated test fixtures.
  useTenantSafe: vi.fn(() => ({
    tenantSlug: "test-tenant",
    tenant: null,
  })),
  // usePresenceMode defaults to "detailed" so components that decide between
  // LocationBadge / PresenceBadge render the richer detailed variant in tests
  // unless a specific test overrides the mock.
  usePresenceMode: vi.fn(() => "detailed"),
  // Most existing page tests exercise the NFC-capable surfaces themselves
  // (activities/devices). Default to true so those tests keep rendering the
  // page unless they explicitly cover the NFC-off branch.
  useNFCEnabled: vi.fn(() => true),
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
  // Top-level `mutate` (named export) — used for cross-component cache
  // invalidation (e.g. settings-page busts settings-schema cache so the
  // useStudentPhotosEnabled hook in other surfaces refetches).
  mutate: vi.fn(),
  useSWRConfig: vi.fn(() => ({
    mutate: vi.fn(),
    cache: new Map(),
  })),
}));
