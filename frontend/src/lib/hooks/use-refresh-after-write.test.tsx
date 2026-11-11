import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { ApiError } from "~/lib/api-error";
import { useSWRAuth } from "~/lib/swr";
import { SWRConfig } from "~/lib/swr/testing";
import { catalogText } from "~/test/error-catalog-text";

import {
  SAVED_BUT_STALE_SUFFIX,
  useRefreshAfterWrite,
  type RefreshAfterWriteResult,
} from "./use-refresh-after-write";

vi.unmock("swr");
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "account-1" }, expires: "2099-01-01" },
    status: "authenticated",
  }),
}));

function SwrProbe({
  fetcher,
  onResult,
}: {
  readonly fetcher: () => Promise<string>;
  readonly onResult: (result: boolean | "threw") => void;
}) {
  const { data, mutate, readCurrent } = useSWRAuth("refresh-list", fetcher);
  return (
    <>
      <p>{data}</p>
      <Probe
        reload={async () => {
          await mutate();
          return { error: readCurrent?.().error };
        }}
        onResult={onResult}
      />
    </>
  );
}

function Probe({
  reload,
  onResult,
}: {
  readonly reload: () => Promise<RefreshAfterWriteResult>;
  readonly onResult: (result: boolean | "threw") => void;
}) {
  const { error, refresh } = useRefreshAfterWrite("die Liste", reload);
  return (
    <>
      <button
        type="button"
        onClick={() => void refresh().then(onResult, () => onResult("threw"))}
      >
        Nachladen
      </button>
      <LoadErrorAlert error={error} />
    </>
  );
}

// #6636: the reload after a confirmed write never throws into the caller's
// action catch; its failure is a load error whose retry only reloads.
describe("useRefreshAfterWrite", () => {
  it("detects a real SWR revalidation failure despite mutate resolving with cached data", async () => {
    const fetcher = vi
      .fn<() => Promise<string>>()
      .mockResolvedValueOnce("Bestätigte Angaben")
      .mockRejectedValueOnce(
        new ApiError("down", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValue("Aktuelle Angaben");
    const onResult = vi.fn();
    render(
      <SWRConfig
        value={{ provider: () => new Map(), shouldRetryOnError: false }}
      >
        <SwrProbe fetcher={fetcher} onResult={onResult} />
      </SWRConfig>,
    );
    expect(await screen.findByText("Bestätigte Angaben")).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "Nachladen" }));

    const message = `${catalogText("general.unavailable", "die Liste")} ${SAVED_BUT_STALE_SUFFIX}`;
    expect(await screen.findByText(message)).toBeVisible();
    expect(onResult).toHaveBeenCalledWith(false);
    expect(screen.getByText("Bestätigte Angaben")).toBeVisible();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(await screen.findByText("Aktuelle Angaben")).toBeVisible();
    expect(fetcher).toHaveBeenCalledTimes(3);
    await waitFor(() =>
      expect(screen.queryByText(message)).not.toBeInTheDocument(),
    );
  });

  it("resolves false and shows the failure as a load error that says the change is saved", async () => {
    const reload = vi
      .fn<() => Promise<RefreshAfterWriteResult>>()
      .mockRejectedValueOnce(
        new ApiError("down", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValue({ error: undefined });
    const onResult = vi.fn();

    render(<Probe reload={reload} onResult={onResult} />);
    fireEvent.click(screen.getByRole("button", { name: "Nachladen" }));

    const message = `${catalogText("general.unavailable", "die Liste")} ${SAVED_BUT_STALE_SUFFIX}`;
    expect(await screen.findByText(message)).toBeInTheDocument();
    expect(onResult).toHaveBeenCalledWith(false);

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() => expect(reload).toHaveBeenCalledTimes(2));
    await waitFor(() =>
      expect(screen.queryByText(message)).not.toBeInTheDocument(),
    );
  });

  it("resolves true when the reload succeeds", async () => {
    const onResult = vi.fn();

    render(
      <Probe
        reload={() => Promise.resolve({ error: undefined })}
        onResult={onResult}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Nachladen" }));

    await waitFor(() => expect(onResult).toHaveBeenCalledWith(true));
    expect(screen.getByRole("button", { name: "Nachladen" })).toBeVisible();
  });
});
