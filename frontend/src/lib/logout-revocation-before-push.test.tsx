import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import {
  CatererShellProvider,
  OperatorShellProvider,
  ParentShellProvider,
  SchoolShellProvider,
  TeacherShellProvider,
  useShellAuth,
} from "./shell-auth-context";
import { reloginAtTenant } from "~/components/tenant/tenant-relogin";

const { mockUnsubscribe } = vi.hoisted(() => ({ mockUnsubscribe: vi.fn() }));

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: {
      user: { id: "7", name: "User", email: "u@example.invalid", roles: [] },
    },
    status: "authenticated",
  }),
  signOut: vi.fn(),
}));
vi.mock("~/contexts/ToastContext", () => ({
  useApiErrorDisplay: () => ({ show: vi.fn() }),
}));
vi.mock("~/lib/profile-context", () => ({
  useProfile: () => ({ profile: null }),
}));
vi.mock("~/lib/school-url", () => ({
  schoolAbsoluteUrl: (path: string) => path,
  schoolPath: (path: string) => path,
}));
vi.mock("~/lib/operator-url", () => ({
  operatorAbsoluteUrl: (path: string) => path,
  operatorPath: (path: string) => path,
}));
vi.mock("~/lib/caterer-url", () => ({
  catererAbsoluteUrl: (path: string) => path,
  catererPath: (path: string) => path,
}));
vi.mock("~/lib/parent-url", () => ({
  parentAbsoluteUrl: (path: string) => path,
  parentPath: (path: string) => path,
}));
vi.mock("~/lib/staff-preview-api", () => ({ recordStaffPreviewEnd: vi.fn() }));
vi.mock("~/lib/swr", () => ({ mutate: vi.fn() }));
vi.mock("~/env.client", () => ({
  clientEnv: { NEXT_PUBLIC_TENANT_DOMAIN: "localhost" },
}));
vi.mock("~/lib/push-api", () => ({ unsubscribePushSilently: mockUnsubscribe }));
vi.mock("~/lib/tenant-session-cookie", () => ({ signOutTenant: vi.fn() }));

beforeEach(() => {
  // A push cleanup that never settles (#5260).
  mockUnsubscribe.mockReturnValue(new Promise(() => undefined));
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => new Response(null, { status: 204 })),
  );
});

afterEach(() => {
  // End the deliberately unfinished document-owned tenant logout before
  // the next test installs a different session/fetch boundary.
  window.dispatchEvent(new Event("pagehide"));
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

// #5194: the family revocation never waits for the push cleanup; a hanging
// push cleanup must not keep the session's access tokens valid.
it.each([
  ["OGS", TeacherShellProvider, "/api/auth/logout"],
  ["Schule", SchoolShellProvider, "/api/school/auth/logout"],
  ["Eltern", ParentShellProvider, "/api/parent/auth/logout"],
  ["Betreiber", OperatorShellProvider, "/api/operator/auth/logout"],
  ["Caterer", CatererShellProvider, "/api/caterer/auth/logout"],
] as const)(
  "%s: revokes although the push cleanup hangs",
  async (_portal, Provider, path) => {
    const { result } = renderHook(() => useShellAuth(), {
      wrapper: ({ children }: { children: ReactNode }) => (
        <Provider>{children}</Provider>
      ),
    });

    void result.current.logout();

    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        path,
        expect.objectContaining({ method: "POST" }),
      ),
    );
  },
);

it("Zur Schulanmeldung: revokes although the push cleanup hangs", async () => {
  void reloginAtTenant("school-b");

  await waitFor(() =>
    expect(fetch).toHaveBeenCalledWith(
      "/api/auth/logout",
      expect.objectContaining({ method: "POST" }),
    ),
  );
});
