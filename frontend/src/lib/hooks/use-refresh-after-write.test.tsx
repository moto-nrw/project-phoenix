import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { ApiError } from "~/lib/api-error";
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
});
