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
      tenantId: number;
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
