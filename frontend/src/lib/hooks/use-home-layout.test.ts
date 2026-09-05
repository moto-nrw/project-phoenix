import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next-auth/react", () => ({ useSession: vi.fn() }));
vi.mock("swr", () => ({ default: vi.fn() }));
vi.mock("~/lib/tenant-context", () => ({ useTenantSlugSafe: vi.fn() }));
vi.mock("~/lib/home-layout-api", () => ({
  fetchHomeLayout: vi.fn(),
  homeLayoutSWRKey: (tenantSlug: string, accountID: string) =>
    `home-layout:${tenantSlug}:${accountID}`,
  resetHomeLayout: vi.fn(),
  saveHomeBlockPolicies: vi.fn(),
  saveHomeLayout: vi.fn(),
}));

import { useSession } from "next-auth/react";
// eslint-disable-next-line no-restricted-imports -- test mock at module boundary
import useSWR from "swr";
import { useTenantSlugSafe } from "~/lib/tenant-context";
import { useHomeLayout } from "./use-home-layout";

describe("useHomeLayout", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useSWR).mockReturnValue({
      data: undefined,
      error: undefined,
      isLoading: false,
      isValidating: false,
      mutate: vi.fn(),
    });
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: { id: "account-a", token: "token" },
        expires: "2099-01-01",
      },
      status: "authenticated",
      update: vi.fn(),
    } as ReturnType<typeof useSession>);
    vi.mocked(useTenantSlugSafe).mockReturnValue("school-a");
  });

  it("isolates the layout cache by tenant and authenticated account", () => {
    renderHook(() => useHomeLayout());

    expect(useSWR).toHaveBeenCalledWith(
      "home-layout:school-a:account-a",
      expect.any(Function),
      { revalidateOnFocus: false },
    );

    vi.mocked(useSession).mockReturnValue({
      data: {
        user: { id: "account-b", token: "token" },
        expires: "2099-01-01",
      },
      status: "authenticated",
      update: vi.fn(),
    } as ReturnType<typeof useSession>);
    vi.mocked(useTenantSlugSafe).mockReturnValue("school-b");
    renderHook(() => useHomeLayout());

    expect(useSWR).toHaveBeenLastCalledWith(
      "home-layout:school-b:account-b",
      expect.any(Function),
      { revalidateOnFocus: false },
    );
  });
});
