import { SWRConfig } from "~/lib/swr/testing";
import { useSWRAuth } from "~/lib/swr/hooks";
vi.unmock("swr");
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { ApiError } from "~/lib/api-error";
import { reloadDocument } from "~/lib/reload-document";

import { useRefreshAfterWrite } from "./use-refresh-after-write";

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "A", tenantId: 1 }, expires: "2099-01-01" },
    status: "authenticated",
  }),
}));

function Probe({ fetcher }: { readonly fetcher: () => Promise<string> }) {
  const { data, mutate } = useSWRAuth("refresh-probe", fetcher);
  const { error, refresh } = useRefreshAfterWrite("die Liste", () => mutate());
  return (
    <>
      {data ? <p>{data}</p> : null}
      <button type="button" onClick={() => void refresh()}>
        Nachladen
      </button>
      <LoadErrorAlert error={error} />
    </>
  );
}

function renderProbe(fetcher: () => Promise<string>) {
  const cache = new Map();
  render(
    <SWRConfig value={{ provider: () => cache, shouldRetryOnError: false }}>
      <Probe fetcher={fetcher} />
    </SWRConfig>,
  );
}

// #6801: an SWR revalidation never rejects; a refusal of shown data still
// reaches the hook through the SWR middleware.
describe("useRefreshAfterWrite with an SWR reload", () => {
  it("loads the whole page again when the revalidation of shown data is refused", async () => {
    vi.mocked(reloadDocument).mockClear();
    const fetcher = vi
      .fn<() => Promise<string>>()
      .mockResolvedValueOnce("Hannah Schmitz")
      .mockRejectedValue(
        new ApiError("no", 403, { code: "general.permission" }),
      );
    renderProbe(fetcher);
    expect(await screen.findByText("Hannah Schmitz")).toBeVisible();

    fireEvent.click(screen.getByRole("button", { name: "Nachladen" }));

    await waitFor(() => expect(reloadDocument).toHaveBeenCalledTimes(1));
    expect(fetcher).toHaveBeenCalledTimes(2);
  });

  it("keeps the page when the revalidation fails for a while", async () => {
    vi.mocked(reloadDocument).mockClear();
    const fetcher = vi
      .fn<() => Promise<string>>()
      .mockResolvedValueOnce("Hannah Schmitz")
      .mockRejectedValue(
        new ApiError("down", 503, { code: "general.unavailable" }),
      );
    renderProbe(fetcher);
    expect(await screen.findByText("Hannah Schmitz")).toBeVisible();

    fireEvent.click(screen.getByRole("button", { name: "Nachladen" }));

    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(2));
    expect(screen.getByText("Hannah Schmitz")).toBeVisible();
    expect(reloadDocument).not.toHaveBeenCalled();
  });
});
