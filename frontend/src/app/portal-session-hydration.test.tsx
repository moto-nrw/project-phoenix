import type { ReactElement, ReactNode } from "react";
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

// The school, caterer, operator and Träger layouts hydrate their
// SessionProvider from a read-only cookie snapshot (#4991), as the parents
// portal does (#4990). Each portal's reader is tested against real cookies in
// src/lib/portal-session-snapshot.server.test.ts.

const { passthrough, readers, auths } = vi.hoisted(() => ({
  passthrough: ({ children }: { children?: ReactNode }) => children,
  readers: {
    school: vi.fn<() => Promise<Session | null>>(async () => null),
    caterer: vi.fn<() => Promise<Session | null>>(async () => null),
    operator: vi.fn<() => Promise<Session | null>>(async () => null),
    traeger: vi.fn<() => Promise<Session | null>>(async () => null),
  },
  auths: {
    school: vi.fn(async () => null),
    caterer: vi.fn(async () => null),
    operator: vi.fn(async () => null),
    traeger: vi.fn(async () => null),
  },
}));

vi.mock("~/lib/school-session-snapshot.server", () => ({
  readSchoolSessionSnapshot: readers.school,
}));
vi.mock("~/lib/caterer-session-snapshot.server", () => ({
  readCatererSessionSnapshot: readers.caterer,
}));
vi.mock("~/lib/operator-session-snapshot.server", () => ({
  readOperatorSessionSnapshot: readers.operator,
}));
vi.mock("~/lib/traeger-session-snapshot.server", () => ({
  readTraegerSessionSnapshot: readers.traeger,
}));
vi.mock("~/server/auth/school", () => ({ schoolAuth: auths.school }));
vi.mock("~/server/auth/caterer", () => ({ catererAuth: auths.caterer }));
vi.mock("~/server/auth/operator", () => ({ operatorAuth: auths.operator }));
vi.mock("~/server/auth/traeger", () => ({ traegerAuth: auths.traeger }));

vi.mock("~/components/analytics/portal-analytics-session", () => ({
  PortalAnalyticsSession: () => null,
}));
vi.mock("~/components/analytics/traeger-analytics-session", () => ({
  TraegerAnalyticsSession: () => null,
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

const { default: SchoolLayout } = await import("./school/layout");
const { default: CatererLayout } = await import("./caterer/layout");
const { default: OperatorLayout } = await import("./operator/layout");
const { default: TraegerLayout } = await import("./traeger/layout");
const { SchoolProviders } = await import("./school/providers");
const { CatererProviders } = await import("./caterer/providers");
const { OperatorProviders } = await import("./operator/providers");
const { TraegerProviders } = await import("./traeger/providers");

type Providers = (props: {
  children: ReactNode;
  session?: Session | null;
}) => ReactNode;

interface Portal {
  readonly name: keyof typeof readers;
  readonly layout: (props: {
    readonly children: ReactNode;
  }) => Promise<ReactNode>;
  readonly providers: Providers;
  readonly sessionPath: string;
  readonly snapshot: Session;
}

const expires = "2026-09-10T12:00:00.000Z";

const PORTALS: readonly Portal[] = [
  {
    name: "school",
    layout: SchoolLayout,
    providers: SchoolProviders,
    sessionPath: "/api/school/auth/session",
    snapshot: { expires, user: { id: "41", scope: "school", tenantId: 3 } },
  },
  {
    name: "caterer",
    layout: CatererLayout,
    providers: CatererProviders,
    sessionPath: "/api/caterer/auth/session",
    snapshot: { expires, user: { id: "41", scope: "caterer" } },
  },
  {
    name: "operator",
    layout: OperatorLayout,
    providers: OperatorProviders,
    sessionPath: "/api/operator/auth/session",
    snapshot: { expires, user: { id: "41", scope: "platform" } },
  },
  {
    name: "traeger",
    layout: TraegerLayout,
    providers: TraegerProviders,
    sessionPath: "/api/traeger/auth/session",
    snapshot: { expires, user: { id: "41", scope: "org", orgId: 7 } },
  },
];

const statuses: string[] = [];

function SessionProbe() {
  const { status } = useSession();
  statuses.push(status);
  return <p>{status}</p>;
}

let fetchSpy: MockInstance<typeof fetch>;

function sessionRequests(path: string) {
  return fetchSpy.mock.calls.filter(([input]) =>
    String(input instanceof Request ? input.url : input).includes(path),
  );
}

beforeEach(() => {
  statuses.length = 0;
  for (const reader of Object.values(readers)) {
    reader.mockReset();
    reader.mockResolvedValue(null);
  }
  for (const auth of Object.values(auths)) auth.mockClear();
});

afterEach(() => {
  fetchSpy?.mockRestore();
});

describe.each(PORTALS)("$name layout", (portal) => {
  async function renderedProviders() {
    const element = (await portal.layout({
      children: null,
    })) as ReactElement<{ session: unknown }>;
    return element;
  }

  it("hands the read-only session snapshot to the portal providers", async () => {
    readers[portal.name].mockResolvedValue(portal.snapshot);

    const providers = await renderedProviders();

    expect(providers.type).toBe(portal.providers);
    expect(providers.props.session).toBe(portal.snapshot);
    expect(readers[portal.name]).toHaveBeenCalledTimes(1);
    expect(auths[portal.name]).not.toHaveBeenCalled();
  });

  it("passes no session when the cookie yields no snapshot", async () => {
    const providers = await renderedProviders();

    expect(providers.props.session).toBeNull();
    expect(auths[portal.name]).not.toHaveBeenCalled();
  });
});

describe.each(PORTALS)("$name providers", (portal) => {
  const Providers = portal.providers;

  beforeEach(() => {
    fetchSpy = vi.spyOn(globalThis, "fetch").mockImplementation(
      async () =>
        new Response(JSON.stringify(portal.snapshot), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
    );
  });

  it("starts authenticated from the server snapshot without a session request", async () => {
    render(
      <Providers session={portal.snapshot}>
        <SessionProbe />
      </Providers>,
    );

    expect(await screen.findByText("authenticated")).toBeInTheDocument();
    expect(statuses[0]).toBe("authenticated");
    expect(statuses).not.toContain("loading");
    expect(sessionRequests(portal.sessionPath)).toHaveLength(0);
  });

  it("fetches the session in the browser when the server has no snapshot", async () => {
    render(
      <Providers session={null}>
        <SessionProbe />
      </Providers>,
    );

    expect(statuses[0]).toBe("loading");
    await waitFor(() =>
      expect(sessionRequests(portal.sessionPath)).toHaveLength(1),
    );
    expect(await screen.findByText("authenticated")).toBeInTheDocument();
  });
});
