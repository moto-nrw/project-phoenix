import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { ApiError } from "~/lib/api-error";
import { reloadDocument } from "~/lib/reload-document";
import { catalogText } from "~/test/error-catalog-text";

import {
  SAVED_BUT_STALE_SUFFIX,
  useRefreshAfterWrite,
} from "./use-refresh-after-write";

function Probe({
  reload,
  onResult,
}: {
  readonly reload: () => Promise<unknown>;
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
  it("resolves false and shows the failure as a load error that says the change is saved", async () => {
    const reload = vi
      .fn<() => Promise<unknown>>()
      .mockRejectedValueOnce(
        new ApiError("down", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValue(undefined);
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
      <Probe reload={() => Promise.resolve(undefined)} onResult={onResult} />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Nachladen" }));

    await waitFor(() => expect(onResult).toHaveBeenCalledWith(true));
    expect(screen.getByRole("button", { name: "Nachladen" })).toBeVisible();
  });

  // #6652: a reload with arguments (`onUpdated(nextState)`) retries with the
  // arguments of the failed attempt and still never throws.
  it("retries a reload with the arguments of the failed attempt", async () => {
    const reload = vi
      .fn<(state: string) => Promise<unknown>>()
      .mockRejectedValueOnce(
        new ApiError("down", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValue(undefined);

    function ArgsProbe() {
      const { error, refresh } = useRefreshAfterWrite("die Liste", reload);
      return (
        <>
          <button type="button" onClick={() => void refresh("aktiv")}>
            Nachladen
          </button>
          <LoadErrorAlert error={error} />
        </>
      );
    }

    render(<ArgsProbe />);
    fireEvent.click(screen.getByRole("button", { name: "Nachladen" }));
    fireEvent.click(await screen.findByRole("button", { name: "Wiederholen" }));

    await waitFor(() => expect(reload).toHaveBeenCalledTimes(2));
    expect(reload).toHaveBeenNthCalledWith(1, "aktiv");
    expect(reload).toHaveBeenNthCalledWith(2, "aktiv");
  });

  // #6801: a refused reload leaves no data standing that the account may no
  // longer see; the page loads anew, and its first load shows the refusal.
  it.each([
    [401, "general.permission"],
    [403, "general.permission"],
  ] as const)(
    "loads the whole page again on a %i reload, without the saved text or a retry",
    async (status, code) => {
      vi.mocked(reloadDocument).mockClear();
      const onResult = vi.fn();

      render(
        <Probe
          reload={() => Promise.reject(new ApiError("no", status, { code }))}
          onResult={onResult}
        />,
      );
      fireEvent.click(screen.getByRole("button", { name: "Nachladen" }));

      await waitFor(() => expect(onResult).toHaveBeenCalledWith(false));
      expect(reloadDocument).toHaveBeenCalledTimes(1);
      // The navigation guard can cancel the reload, leaving this document.
      expect(await screen.findByRole("alert")).toHaveTextContent(
        catalogText("general.permission", "die Liste"),
      );
      expect(
        screen.queryByText(SAVED_BUT_STALE_SUFFIX, { exact: false }),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByRole("button", { name: "Wiederholen" }),
      ).not.toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Nachladen" })).toBeVisible();
    },
  );

  it("keeps the page for a transient reload failure", async () => {
    vi.mocked(reloadDocument).mockClear();
    render(
      <Probe
        reload={() =>
          Promise.reject(
            new ApiError("down", 503, { code: "general.unavailable" }),
          )
        }
        onResult={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Nachladen" }));

    expect(
      await screen.findByRole("button", { name: "Wiederholen" }),
    ).toBeVisible();
    expect(reloadDocument).not.toHaveBeenCalled();
  });
});
