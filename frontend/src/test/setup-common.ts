import { afterEach, beforeEach, vi } from "vitest";

import { freezeTestClock } from "./clock";

process.env.API_URL = "http://server:8080";

// Deterministic test clock (#3101, see docs/agents/frontend-testing.md and
// src/test/clock.ts). Freeze `Date` at the shared instant before every test
// and hand the real clock back afterwards. Only `Date` is mocked; timers and
// promises stay real, so `waitFor` and debounce tests keep their behaviour.
// A test that installs `vi.useFakeTimers()` starts its fake clock at the
// frozen instant; the hook uninstalls it after the test unless the file
// installed it before the test (beforeAll / module scope), which stays the
// file's responsibility.
//
// The module-scope freeze covers collection time: a `vi.useFakeTimers()` at
// the top of a test file also starts at the shared instant.
freezeTestClock();

let fakeTimersInstalledBeforeTest = false;

beforeEach(() => {
  fakeTimersInstalledBeforeTest = vi.isFakeTimers();
  if (fakeTimersInstalledBeforeTest) return;
  freezeTestClock();
});

afterEach(() => {
  if (fakeTimersInstalledBeforeTest) return;
  vi.useRealTimers();
});

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

vi.mock("~/env.client", () => ({
  clientEnv: {
    NEXT_PUBLIC_API_URL: "http://localhost:8080",
    NEXT_PUBLIC_TENANT_DOMAIN: "localhost",
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
