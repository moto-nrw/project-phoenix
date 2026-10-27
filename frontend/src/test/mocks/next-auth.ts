/**
 * Default session data matching the browser's session (#5005): no access or
 * refresh token, those stay in the server's session cookie.
 * Overrides merge shallowly into user; top-level fields merge into session.
 */
export function mockSessionData(
  overrides?: Partial<{
    user: Partial<{
      id: string;
      name: string;
      email: string;
      role: string;
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
      ...overrides?.user,
    },
    expires: overrides?.expires ?? "2099-01-01",
  };
}

/**
 * The server's view of the same session (ServerSession): what a portal's
 * auth() and route wrappers hand to route handlers, tokens included.
 */
export function mockServerSessionData(
  overrides?: Parameters<typeof mockSessionData>[0],
) {
  const session = mockSessionData(overrides);
  return {
    ...session,
    user: {
      token: "test-token",
      refreshToken: "test-refresh-token",
      ...session.user,
    },
  };
}
