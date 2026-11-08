import { AsyncLocalStorage } from "node:async_hooks";
import type { PortalBinding } from "~/lib/portal-handlers.server";
import { PORTALS } from "~/server/auth/portals";
import type { ServerSession } from "~/server/auth/session-types";
import { createMockContext } from "~/test/route-test-helpers";

type SessionReader = () => Promise<ServerSession | null | undefined>;

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

const ENTRY_NAMES = {
  tenant: ["tenantPortal", "auth", "uncachedAuth", "withTenantAuth", "Tenant"],
  operator: [
    "operatorPortal",
    "operatorAuth",
    "uncachedOperatorAuth",
    "withOperatorAuth",
    "Operator",
  ],
  parent: [
    "parentPortal",
    "parentAuth",
    "uncachedParentAuth",
    "withParentAuth",
    "Parent",
  ],
  school: [
    "schoolPortal",
    "schoolAuth",
    "uncachedSchoolAuth",
    "withSchoolAuth",
    "School",
  ],
  caterer: [
    "catererPortal",
    "catererAuth",
    "uncachedCatererAuth",
    "withCatererAuth",
    "Caterer",
  ],
  traeger: [
    "traegerPortal",
    "traegerAuth",
    "uncachedTraegerAuth",
    "withTraegerAuth",
    "Traeger",
  ],
} as const;

type Wrapper = (handler: unknown) => (...args: unknown[]) => Promise<Response>;

/**
 * A mocked portal entry module (`vi.mock("~/server/auth/parent", …)`) with
 * its portal object, built from the module's own mocked session readers. The
 * module's mocked wrapper runs its handler in the portal's request context,
 * as the real one does.
 */
export function mockPortalModule<M extends Record<string, unknown>>(
  portal: keyof typeof ENTRY_NAMES,
  mocked: M,
): M {
  const [portalName, authName, uncachedName, wrapperName, cap] =
    ENTRY_NAMES[portal];
  const reader = (name: string) => () =>
    ((mocked[name] ?? mocked[authName]) as SessionReader | undefined)?.() ??
    Promise.resolve(null);
  const binding = testPortalBinding(reader(authName), reader(uncachedName));
  const wrapper = mocked[wrapperName] as Wrapper | undefined;
  return {
    ...mocked,
    ...(wrapper && {
      [wrapperName]: (handler: unknown) =>
        binding.withAuth(wrapper(handler) as never),
    }),
    [portalName]: {
      ...binding,
      portal: PORTALS[portal],
      setSession: mocked[`set${cap}Session`],
      withSessionResponse: mocked[`with${cap}SessionResponse`],
    },
  };
}
