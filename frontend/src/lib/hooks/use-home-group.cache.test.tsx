import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const useSWRAuth = vi.hoisted(() => vi.fn());

vi.mock("next-auth/react", () => ({ useSession: vi.fn() }));
vi.mock("~/lib/swr", () => ({ useSWRAuth }));

import { useSession } from "next-auth/react";
import { useHomeGroup } from "./use-home-group";

describe("useHomeGroup", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useSWRAuth.mockReturnValue({
      data: undefined,
      error: undefined,
      isLoading: false,
    });
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: { id: "account-a", token: "token" },
        expires: "2099-01-01",
      },
      status: "authenticated",
      update: vi.fn(),
    } as ReturnType<typeof useSession>);
  });

  it("trennt den Gruppenstand nach angemeldetem Konto", () => {
    renderHook(() => useHomeGroup(true, "13:10"));

    expect(useSWRAuth).toHaveBeenCalledWith(
      "home-own-group:account-a",
      expect.any(Function),
      { revalidateOnFocus: false, errorRetryCount: 1 },
    );

    vi.mocked(useSession).mockReturnValue({
      data: {
        user: { id: "account-b", token: "token" },
        expires: "2099-01-01",
      },
      status: "authenticated",
      update: vi.fn(),
    } as ReturnType<typeof useSession>);
    renderHook(() => useHomeGroup(true, "13:10"));

    expect(useSWRAuth).toHaveBeenLastCalledWith(
      "home-own-group:account-b",
      expect.any(Function),
      { revalidateOnFocus: false, errorRetryCount: 1 },
    );
  });
});
