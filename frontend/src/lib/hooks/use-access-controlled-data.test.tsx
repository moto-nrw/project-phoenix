import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import { SWRConfig } from "~/lib/swr/testing";
import { useSWRAuth } from "~/lib/swr/hooks";
import { ApiError } from "~/lib/api-error";
import { useAccessControlledData } from "./use-access-controlled-data";

vi.unmock("swr");
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "refusal-test", tenantId: 1 }, expires: "2099-01-01" },
    status: "authenticated",
  }),
}));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => "school-a",
}));

function setup(fetcher: () => Promise<string>) {
  const cache = new Map();
  function wrapper({ children }: { children: ReactNode }) {
    return <SWRConfig value={{ provider: () => cache }}>{children}</SWRConfig>;
  }
  const useResource = () => {
    const response = useSWRAuth("protected-list", fetcher, {
      shouldRetryOnError: false,
      dedupingInterval: 0,
    });
    return {
      ...response,
      ...useAccessControlledData(
        "protected-list",
        response.data,
        response.error,
      ),
    };
  };
  return { cache, wrapper, useResource };
}

describe("protected SWR data", () => {
  it("discards refused cache data across remounts and failed revalidations", async () => {
    const fetcher = vi
      .fn()
      .mockResolvedValueOnce("confirmed")
      .mockRejectedValueOnce(new ApiError("refused", 403))
      .mockRejectedValue(new ApiError("server", 500));
    const { cache, wrapper, useResource } = setup(fetcher);
    const view = renderHook(useResource, { wrapper });
    await waitFor(() => expect(view.result.current.data).toBe("confirmed"));
    await act(async () => {
      await view.result.current.mutate();
    });
    await waitFor(() => expect(view.result.current.accessRefused).toBe(true));
    expect(view.result.current.data).toBeUndefined();
    expect(
      [...cache.values()].some((entry) => entry.data === "confirmed"),
    ).toBe(false);
    view.unmount();

    const remounted = renderHook(useResource, { wrapper });
    await waitFor(() =>
      expect(remounted.result.current.error).toMatchObject({ status: 500 }),
    );
    expect(remounted.result.current.data).toBeUndefined();
    fetcher.mockResolvedValue("fresh");
    await act(async () => {
      await remounted.result.current.mutate();
    });
    await waitFor(() => expect(remounted.result.current.data).toBe("fresh"));
    expect(remounted.result.current.accessRefused).toBe(false);
  });

  it.each([500, 503])(
    "keeps confirmed data after a transient %s failure",
    async (status) => {
      const fetcher = vi
        .fn()
        .mockResolvedValueOnce("confirmed")
        .mockRejectedValueOnce(new ApiError("failed", status));
      const { wrapper, useResource } = setup(fetcher);
      const view = renderHook(useResource, { wrapper });
      await waitFor(() => expect(view.result.current.data).toBe("confirmed"));
      await act(async () => {
        await view.result.current.mutate();
      });
      await waitFor(() =>
        expect(view.result.current.error).toMatchObject({ status }),
      );
      expect(view.result.current.data).toBe("confirmed");
      expect(view.result.current.accessRefused).toBe(false);
    },
  );
});
