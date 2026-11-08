import { AsyncLocalStorage } from "node:async_hooks";
import type { PortalBinding } from "~/lib/portal-handlers.server";
import type { PortalKey } from "~/server/auth/portals";
import { PORTALS } from "~/server/auth/portals";
import type { ServerSession } from "~/server/auth/session-types";
import { createMockContext } from "~/test/route-test-helpers";

type SessionReader = () => Promise<ServerSession | null | undefined>;

/** A test's session reader; its sessions carry only what the test needs. */
type MockSessionReader = () => Promise<unknown>;

/**
 * A portal binding for route tests, also as the portal object a mocked entry
 * module exports (`tenantPortal`, `parentPortal`, …): `withAuth` runs the
 * handler as the portal's response wrapper does, so the session helpers find
 * their request context there and nowhere else.
 */
export function testPortalBinding(
  auth: SessionReader,
  uncachedAuth: SessionReader = auth,
): PortalBinding {
  const context = new AsyncLocalStorage<true>();
  return {
    auth: async () => (await auth()) ?? null,
    uncachedAuth: async () => (await uncachedAuth()) ?? null,
    inRequestContext: () => context.getStore() === true,
    withAuth: (handler) => (request, routeContext) =>
      context.run(true, () =>
        handler(request as never, routeContext ?? createMockContext()),
      ),
  };
}

interface EntryModules {
  tenant: typeof import("~/server/auth");
  operator: typeof import("~/server/auth/operator");
  parent: typeof import("~/server/auth/parent");
  school: typeof import("~/server/auth/school");
  caterer: typeof import("~/server/auth/caterer");
  traeger: typeof import("~/server/auth/traeger");
}

type PortalObject<P extends PortalKey> = EntryModules[P][`${P}Portal` &
  keyof EntryModules[P]];

type Wrapper = (handler: never) => (...args: never[]) => Promise<Response>;

/** Members the binding provides; a mock replaces only the session readers. */
type BoundMembers = keyof PortalBinding | "portal" | "inRequestContext";

/**
 * What a test gives a mocked portal: its session readers, optionally a
 * wrapper that runs inside the portal's request context, and any other
 * member of the real portal object (`setSession`, `withSessionResponse`, …).
 * A name the real portal object does not have fails to compile.
 */
export type PortalMock<P extends PortalKey> = {
  auth?: MockSessionReader;
  /** The reader a backend 401 renews through; `auth` by default. */
  uncachedAuth?: MockSessionReader;
  /** Runs around the handler inside the request context. */
  withAuth?: Wrapper;
} & Partial<Record<Exclude<keyof PortalObject<P>, BoundMembers>, unknown>>;

/**
 * A mocked portal entry module (`vi.mock("~/server/auth/parent", …)`): the
 * module's only export, its portal object, with the given session readers.
 */
export function mockPortalModule<P extends PortalKey>(
  portal: P,
  mock: PortalMock<P>,
): Record<`${P}Portal`, unknown> {
  const {
    auth = async () => null,
    uncachedAuth = auth,
    withAuth,
    ...members
  } = mock as PortalMock<PortalKey> & Record<string, unknown>;
  const binding = testPortalBinding(
    auth as SessionReader,
    uncachedAuth as SessionReader,
  );
  return {
    [`${portal}Portal`]: {
      ...members,
      ...binding,
      portal: PORTALS[portal],
      withAuth: withAuth
        ? (handler: never) =>
            binding.withAuth(
              (withAuth as (h: never) => never)(handler) as never,
            )
        : binding.withAuth,
    },
  } as Record<`${P}Portal`, unknown>;
}
