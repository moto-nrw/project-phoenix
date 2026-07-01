/**
 * Storybook-only duplicate of src/test/mocks/next-auth.ts's mockSessionData().
 *
 * Not a re-export: that file imports `vi` from "vitest" at module scope for
 * its other exports (mockUseSessionReturn, setupGetSessionMock), which drags
 * `vitest`/chai into every story's browser bundle and crashes at runtime
 * (Storybook renders in a real browser, not under the Vitest test runner).
 * Keep this shape in sync with mockSessionData() if that one changes.
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
