import type { Session } from "next-auth";
import { useSession } from "next-auth/react";
import { render, screen, waitFor } from "@testing-library/react";
import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  vi,
  type MockInstance,
} from "vitest";

const { passthrough } = vi.hoisted(() => ({
  passthrough: ({ children }: { children?: React.ReactNode }) => children,
}));

vi.mock("~/components/analytics/parent-portal-analytics-session", () => ({
  ParentPortalAnalyticsSession: () => null,
}));
vi.mock("~/components/auth/sentry-session-context", () => ({
  SentrySessionContext: () => null,
}));
vi.mock("~/components/notifications/service-worker-registrar", () => ({
  PushSubscriptionSync: () => null,
}));
vi.mock("~/lib/module-portal-context", () => ({
  ModulePortalProvider: passthrough,
}));
vi.mock("~/lib/parent-locale-context", () => ({
  ParentLocaleProvider: passthrough,
}));

import { ParentProviders } from "./providers";

const snapshot: Session = {
  expires: "2026-09-10T12:00:00.000Z",
  user: { id: "41", token: "backend-access", scope: "parent" },
};

const statuses: string[] = [];

function SessionProbe() {
  const { status } = useSession();
  statuses.push(status);
  return <p>{status}</p>;
}

function sessionRequests(fetchSpy: MockInstance<typeof fetch>) {
  return fetchSpy.mock.calls.filter(([input]) =>
    String(input instanceof Request ? input.url : input).includes(
      "/api/parent/auth/session",
    ),
  );
}

let fetchSpy: MockInstance<typeof fetch>;

beforeEach(() => {
  statuses.length = 0;
  fetchSpy = vi.spyOn(globalThis, "fetch").mockImplementation(
    async () =>
      new Response(JSON.stringify(snapshot), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
  );
});

afterEach(() => {
  fetchSpy.mockRestore();
});

describe("ParentProviders", () => {
  it("starts authenticated from the server snapshot without a session request", async () => {
    render(
      <ParentProviders session={snapshot}>
        <SessionProbe />
      </ParentProviders>,
    );

    expect(await screen.findByText("authenticated")).toBeInTheDocument();
    expect(statuses[0]).toBe("authenticated");
    expect(statuses).not.toContain("loading");
    expect(sessionRequests(fetchSpy)).toHaveLength(0);
  });

  it("fetches the session in the browser when the server has no snapshot", async () => {
    render(
      <ParentProviders session={null}>
        <SessionProbe />
      </ParentProviders>,
    );

    expect(statuses[0]).toBe("loading");
    await waitFor(() => expect(sessionRequests(fetchSpy)).toHaveLength(1));
    expect(await screen.findByText("authenticated")).toBeInTheDocument();
  });
});
