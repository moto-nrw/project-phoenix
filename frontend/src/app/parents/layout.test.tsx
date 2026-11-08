import type { ReactElement } from "react";
import type { Session } from "next-auth";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { parentAuthMock, readParentSessionSnapshotMock } = vi.hoisted(() => ({
  parentAuthMock: vi.fn(async () => null),
  readParentSessionSnapshotMock: vi.fn<() => Promise<Session | null>>(
    async () => null,
  ),
}));

vi.mock("~/server/auth/parent", async () =>
  (await import("~/test/portal-binding")).mockPortalModule("parent", {
    auth: parentAuthMock,
  }),
);

vi.mock("~/lib/portal-session-snapshot.server", () => ({
  readPortalSessionSnapshot: async (key: string) => {
    const session =
      key === "parent" ? await readParentSessionSnapshotMock() : null;
    return session ? { session, accessToken: "server-only" } : null;
  },
}));

const { default: ParentLayout } = await import("./layout");

type ProvidersElement = ReactElement<{ session: unknown }>;

async function renderedProviders(): Promise<ProvidersElement> {
  const element = (await ParentLayout({ children: null })) as ReactElement<{
    children: ProvidersElement;
  }>;
  return element.props.children;
}

beforeEach(() => {
  parentAuthMock.mockClear();
  readParentSessionSnapshotMock.mockReset();
  readParentSessionSnapshotMock.mockResolvedValue(null);
});

describe("ParentLayout", () => {
  it("hands the read-only session snapshot to the parent providers", async () => {
    const session: Session = {
      expires: "2026-09-10T12:00:00.000Z",
      user: { id: "41", scope: "parent" },
    };
    readParentSessionSnapshotMock.mockResolvedValue(session);

    const providers = await renderedProviders();

    expect(providers.props.session).toBe(session);
    expect(parentAuthMock).not.toHaveBeenCalled();
  });

  it("passes no session when the cookie yields no snapshot", async () => {
    const providers = await renderedProviders();

    expect(providers.props.session).toBeNull();
    expect(parentAuthMock).not.toHaveBeenCalled();
  });
});
