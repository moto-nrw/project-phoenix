import { vi } from "vitest";

/**
 * Default session data matching the backend's JWT structure.
 * Overrides merge shallowly into user; top-level fields merge into session.
 */
export function mockSessionData(
  overrides?: Partial<{
    user: Partial<{
      id: string;
      name: string;
      email: string;
      role: string;
      token: string;
      accessToken: string;
      refreshToken: string;
    }>;
    expires: string;
  }>,
) {
  return {
    user: {
      id: "1",
      name: "Test User",
      email: "test@test.com",
      role: "admin",
      token: "test-token",
      accessToken: "test-access-token",
      refreshToken: "test-refresh-token",
      ...overrides?.user,
    },
    expires: overrides?.expires ?? "2099-01-01",
  };
}

/**
 * Returns a mock useSession() return value for authenticated state.
 * Pass overrides to customize the session data.
 */
export function mockUseSessionReturn(
  overrides?: Parameters<typeof mockSessionData>[0],
) {
  return {
    data: mockSessionData(overrides),
    status: "authenticated" as const,
    update: vi.fn(),
  };
}

/**
 * Returns a vi.fn() pre-configured to resolve with session data.
 * Use as the return value inside vi.mock("next-auth") or vi.mock("next-auth/react").
 */
export function setupGetSessionMock(
  session?: ReturnType<typeof mockSessionData> | null,
) {
  return vi.fn().mockResolvedValue(session ?? mockSessionData());
}
