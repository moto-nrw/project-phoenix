import "@testing-library/jest-dom/vitest";
import type React from "react";
import { beforeEach, vi } from "vitest";

// Reset the module-level session cache between tests. A static import would
// bind next-auth/react before the test file's vi.mock registers. Importing here
// also resolves the current module after a test calls vi.resetModules(). Some
// tests mock session-cache without the clear export, and Vitest throws while
// accessing that missing export.
beforeEach(async () => {
  const sessionCache = await import("~/lib/session-cache");
  if ("clearSessionCache" in sessionCache) {
    sessionCache.clearSessionCache();
  }
});

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
    flush: () => Promise.resolve(),
    child: () => createMockLogger(),
  });
  return {
    createLogger: vi.fn(() => createMockLogger()),
  };
});

// Mock next-intl globally. EnrollmentForm, the parent shell, and several
// shared components now call useTranslations/useLocale unconditionally, which
// throws without a NextIntlClientProvider. The mock resolves keys against the
// real German catalog so existing assertions (which match the German chrome)
// keep passing, and useLocale reports the default locale ("de").
vi.mock("next-intl", async () => {
  const de = (await import("~/i18n/messages/de.json")).default as Record<
    string,
    unknown
  >;
  const resolve = (path: string): unknown =>
    path.split(".").reduce<unknown>((acc, part) => {
      if (acc && typeof acc === "object") {
        return (acc as Record<string, unknown>)[part];
      }
      return undefined;
    }, de);
  const interpolate = (
    value: string,
    values?: Record<string, unknown>,
  ): string =>
    values
      ? Object.entries(values).reduce(
          (str, [k, v]) => str.replaceAll(`{${k}}`, String(v)),
          value,
        )
      : value;
  const makeT = (namespace?: string) => {
    const prefix = namespace ? `${namespace}.` : "";
    const t = (key: string, values?: Record<string, unknown>) => {
      const val = resolve(`${prefix}${key}`);
      return typeof val === "string"
        ? interpolate(val, values)
        : `${prefix}${key}`;
    };
    t.raw = (key: string) => resolve(`${prefix}${key}`);
    return t;
  };
  // Cache the translator per namespace so useTranslations returns a stable
  // reference across renders, exactly as next-intl does. Without this, effects
  // that depend on `t` (e.g. the status-view fetch effect) would re-run every
  // render and exhaust mockResolvedValueOnce queues.
  const cache = new Map<string, ReturnType<typeof makeT>>();
  return {
    useTranslations: (namespace?: string) => {
      const cacheKey = namespace ?? "";
      const existing = cache.get(cacheKey);
      if (existing) return existing;
      const t = makeT(namespace);
      cache.set(cacheKey, t);
      return t;
    },
    useLocale: () => "de",
    NextIntlClientProvider: ({ children }: { children: React.ReactNode }) =>
      children,
  };
});

// Mock the server entrypoint of next-intl (used by the root layout's
// getLocale()). It needs a request context that doesn't exist under jsdom.
vi.mock("next-intl/server", () => ({
  getLocale: () => Promise.resolve("de"),
}));

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

// Route-auth wrappers are exercised directly in focused unit tests. Route
// tests mock the portal auth modules themselves, so keep their wrapper layer
// transparent and preserve the existing one-session-read contract.
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

const tenantProviderMock = vi.hoisted(() => ({
  useTenant: vi.fn(() => ({
    tenantSlug: "test-tenant",
    tenant: null,
  })),
  useTenantSlugSafe: vi.fn(() => "test-tenant"),
  useTenantRoutingModeSafe: vi.fn(() => "path"),
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
  // Info-Point Dashboard is opt-in and defaults off in production; mirror
  // that default here so unrelated tests don't accidentally exercise the
  // hidden nav item. Tests covering the feature override this mock locally.
  useDisplayEnabled: vi.fn(() => false),
  // Tagesauswertung / Anwesenheitsprotokoll (#1456) is opt-in and defaults
  // off; same reasoning as useDisplayEnabled above.
  useAttendanceLogEnabled: vi.fn(() => false),
  // Care offerings were historically always enabled. Preserve that behaviour
  // in unrelated fixtures unless a capability test opts out explicitly.
  useCareOfferingsEnabled: vi.fn(() => true),
  useAttendanceWebEnabled: vi.fn(() => true),
  useOpenCareGroupMode: vi.fn(() => false),
  useOperationalOverviewScope: vi.fn(() => "own"),
  useShowTimetableCounts: vi.fn(() => true),
  useWaitlistEnabled: vi.fn(() => true),
  TenantProvider: ({
    children,
  }: {
    children: React.ReactNode;
    tenantSlug: string;
    tenant: unknown;
  }) => children,
}));

// Mock tenant context globally so tenant-scoped components can render in tests.
// Tests that need the real context must unmock this module path.
vi.mock("~/lib/tenant-context", () => tenantProviderMock);

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
