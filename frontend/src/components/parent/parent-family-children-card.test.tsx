import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { ParentFamilyChildren } from "~/lib/parent-family-children-api";
import { ParentFamilyChildrenCard } from "./parent-family-children-card";

const mocks = vi.hoisted(() => ({ fetch: vi.fn(), state: vi.fn() }));
vi.mock("~/lib/parent-family-children-api", () => ({
  fetchFamilyChildren: mocks.fetch,
  stateFamilyChildren: mocks.state,
}));

const family: ParentFamilyChildren = {
  statedChildren: 3,
  statedAt: "2026-09-01T10:00:00Z",
  confirmedChildren: 2,
  confirmedAt: "2026-09-02T10:00:00Z",
  validFrom: "2026-09-01",
  validUntil: null,
  canState: false,
};

describe("ParentFamilyChildrenCard", () => {
  beforeEach(() => vi.clearAllMocks());

  it.each([
    ["an office correction finishes the review", family, false],
    [
      "a later changed statement waits for review",
      { ...family, statedChildren: 4, statedAt: "2026-09-03T10:00:00Z" },
      true,
    ],
    [
      "restating the confirmed number needs no review, as in the office",
      { ...family, statedChildren: 2, statedAt: "2026-09-03T10:00:00Z" },
      false,
    ],
    [
      "a first statement waits for confirmation",
      { ...family, confirmedChildren: null, confirmedAt: null },
      true,
    ],
  ] as const)("%s", async (_name, count, pending) => {
    mocks.fetch.mockResolvedValue(count);
    render(<ParentFamilyChildrenCard organizationId="4" contractId="75" />, {
      wrapper: ToastProvider,
    });
    // Anker: the family response has loaded, not just the card skeleton.
    expect(
      await screen.findByText(
        "Wir speichern nur die Zahl. Namen der anderen Kinder brauchen wir nicht.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Wird geprüft") !== null).toBe(pending);
    expect(
      screen.queryByRole("button", { name: "Angabe senden" }),
    ).not.toBeInTheDocument();
    expect(mocks.fetch).toHaveBeenCalledWith("4", "75");
  });
  it("offers submission only when the response permits it", async () => {
    mocks.fetch.mockResolvedValue({ ...family, canState: true });
    render(<ParentFamilyChildrenCard organizationId="4" contractId="75" />, {
      wrapper: ToastProvider,
    });
    expect(
      await screen.findByRole("button", { name: "Angabe senden" }),
    ).toBeInTheDocument();
  });

  it("shows a failed statement inside the form, not only as a toast", async () => {
    mocks.fetch.mockResolvedValue({ ...family, canState: true });
    mocks.state.mockRejectedValue(new Error("network down"));
    render(<ParentFamilyChildrenCard organizationId="4" contractId="75" />, {
      wrapper: ToastProvider,
    });
    const send = await screen.findByRole("button", { name: "Angabe senden" });
    const form = send.closest("form") as HTMLFormElement;
    fireEvent.click(send);
    expect(await within(form).findByRole("alert")).toBeInTheDocument();
    expect(mocks.state).toHaveBeenCalledWith("4", "75", 3);
  });
});
