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
  asked: true,
};

const inbox = { kind: "inbox", organizationId: "4", contractId: "75" } as const;

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
    render(<ParentFamilyChildrenCard source={inbox} />, {
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
    expect(mocks.fetch).toHaveBeenCalledWith(inbox);
  });
  it("offers submission only when the response permits it", async () => {
    mocks.fetch.mockResolvedValue({ ...family, canState: true });
    render(<ParentFamilyChildrenCard source={inbox} />, {
      wrapper: ToastProvider,
    });
    expect(
      await screen.findByRole("button", { name: "Angabe senden" }),
    ).toBeInTheDocument();
  });

  it("shows a failed statement inside the form, not only as a toast", async () => {
    mocks.fetch.mockResolvedValue({ ...family, canState: true });
    mocks.state.mockRejectedValue(new Error("network down"));
    render(<ParentFamilyChildrenCard source={inbox} />, {
      wrapper: ToastProvider,
    });
    const send = await screen.findByRole("button", { name: "Angabe senden" });
    const form = send.closest("form") as HTMLFormElement;
    fireEvent.click(send);
    expect(await within(form).findByRole("alert")).toBeInTheDocument();
    expect(mocks.state).toHaveBeenCalledWith(inbox, 3);
  });

  // #6335: the mail link and the registration's status page use the same
  // card; each sends to its own source only.
  it("states the number through an open mail link", async () => {
    const link = { kind: "link", link: "4.abc" } as const;
    mocks.fetch.mockResolvedValue({
      ...family,
      statedChildren: null,
      statedAt: null,
      confirmedChildren: null,
      confirmedAt: null,
      canState: true,
    });
    mocks.state.mockResolvedValue({
      ...family,
      statedChildren: 2,
      confirmedChildren: null,
      confirmedAt: null,
      canState: true,
    });
    render(<ParentFamilyChildrenCard source={link} />, {
      wrapper: ToastProvider,
    });
    expect(
      await screen.findByText(
        "Bis zur Unterschrift können Sie die Zahl hier ändern.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Ein Kind mehr" }));
    fireEvent.click(screen.getByRole("button", { name: "Angabe senden" }));
    expect(await screen.findByText("Wird geprüft")).toBeInTheDocument();
    expect(mocks.fetch).toHaveBeenCalledWith(link);
    expect(mocks.state).toHaveBeenCalledWith(link, 2);
  });

  it("asks on the registration only where the school uses the number", async () => {
    const enrollment = { kind: "enrollment", token: "tok" } as const;
    mocks.fetch.mockResolvedValue({ ...family, asked: false });
    render(
      <div data-testid="host">
        <ParentFamilyChildrenCard source={enrollment} />
      </div>,
      { wrapper: ToastProvider },
    );
    await vi.waitFor(() =>
      expect(mocks.fetch).toHaveBeenCalledWith(enrollment),
    );
    await vi.waitFor(() =>
      expect(screen.getByTestId("host")).toBeEmptyDOMElement(),
    );
  });

  it("points to the contract once a child of the registration is accepted", async () => {
    mocks.fetch.mockResolvedValue({
      ...family,
      statedChildren: 3,
      confirmedChildren: null,
      confirmedAt: null,
      canState: false,
    });
    render(
      <ParentFamilyChildrenCard
        source={{ kind: "enrollment", token: "tok" }}
      />,
      { wrapper: ToastProvider },
    );
    expect(
      await screen.findByText(
        "Ihr Kind ist angenommen. Die Zahl ändern Sie jetzt in Ihrem Betreuungsvertrag.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Der Träger prüft sie, wenn Ihr Vertrag kommt/),
    ).toBeInTheDocument();
  });
});
