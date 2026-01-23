import "@testing-library/jest-dom/vitest";
import { vi } from "vitest";

// Mock ~/env globally to avoid Zod validation issues in tests
vi.mock("~/env", () => ({
  env: {
    NEXT_PUBLIC_API_URL: "http://localhost:8080",
    NODE_ENV: "test",
  },
}));

// Mock next/headers globally - required for Better Auth's cookies() calls
vi.mock("next/headers", () => ({
  cookies: vi.fn(() => ({
    get: vi.fn((name: string) => {
      if (name === "better-auth.session_token") {
        return { value: "test-session-token" };
      }
      return undefined;
    }),
    toString: vi.fn(() => "better-auth.session_token=test-session-token"),
  })),
  headers: vi.fn(() => new Map()),
}));

// Mock ~/server/auth globally - server-side auth helpers
vi.mock("~/server/auth", () => ({
  auth: vi.fn(() =>
    Promise.resolve({
      user: {
        id: "test-user-id",
        email: "test@example.com",
        name: "Test User",
      },
    }),
  ),
  getCookieHeader: vi.fn(() =>
    Promise.resolve("better-auth.session_token=test-session-token"),
  ),
  hasActiveSession: vi.fn(() => Promise.resolve(true)),
}));

// Mock ~/lib/auth-client globally - client-side Better Auth
const mockSession = {
  data: {
    user: {
      id: "test-user-id",
      email: "test@example.com",
      name: "Test User",
      emailVerified: true,
      image: null,
      createdAt: new Date(),
      updatedAt: new Date(),
    },
    session: {
      id: "test-session-id",
      userId: "test-user-id",
      expiresAt: new Date(Date.now() + 86400000),
      ipAddress: null,
      userAgent: null,
    },
    activeOrganizationId: "test-org-id",
  },
  isPending: false,
  error: null,
};

vi.mock("~/lib/auth-client", () => ({
  authClient: {
    signIn: { email: vi.fn() },
    signOut: vi.fn(),
    signUp: { email: vi.fn() },
    useSession: vi.fn(() => mockSession),
    getSession: vi.fn(() => Promise.resolve(mockSession.data)),
    organization: {
      getActiveMemberRole: vi.fn(() =>
        Promise.resolve({ data: { role: "supervisor" } }),
      ),
      getFullOrganization: vi.fn(() =>
        Promise.resolve({
          data: {
            id: "test-org-id",
            name: "Test OGS",
            slug: "test-ogs",
          },
        }),
      ),
      setActive: vi.fn(),
    },
  },
  signIn: { email: vi.fn() },
  signOut: vi.fn(),
  signUp: { email: vi.fn() },
  useSession: vi.fn(() => mockSession),
  getSession: vi.fn(() => Promise.resolve(mockSession.data)),
  organization: {
    getActiveMemberRole: vi.fn(() =>
      Promise.resolve({ data: { role: "supervisor" } }),
    ),
  },
  getActiveRole: vi.fn(() => Promise.resolve("supervisor")),
  isAdmin: vi.fn(() => Promise.resolve(false)),
  isSupervisor: vi.fn(() => Promise.resolve(true)),
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
