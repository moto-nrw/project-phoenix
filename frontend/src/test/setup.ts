import "@testing-library/jest-dom/vitest";
import { vi } from "vitest";

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
