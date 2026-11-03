import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type { HolidaySibling } from "~/lib/holiday-contribution";

const { mockSplit } = vi.hoisted(() => ({ mockSplit: vi.fn() }));
vi.mock("~/lib/traeger-sibling-groups-api", () => ({
  splitHolidaySibling: (periodId: string, registrationId: string) =>
    mockSplit(periodId, registrationId),
}));

import { HolidayRegistrationSiblingsCard } from "./holiday-registration-siblings-card";

const SIBLINGS: HolidaySibling[] = [
  {
    registrationId: "8",
    cancelled: false,
    children: [{ id: "81", name: "Tom Muster" }],
  },
  {
    registrationId: "9",
    cancelled: true,
    children: [{ id: "91", name: "Ida Muster" }],
  },
];

function renderCard(
  invoiced = false,
  onSplit = vi.fn(() => Promise.resolve()),
) {
  render(
    <ToastProvider>
      <HolidayRegistrationSiblingsCard
        periodId="5"
        registrationId="7"
        siblings={SIBLINGS}
        invoiced={invoiced}
        onSplit={onSplit}
      />
    </ToastProvider>,
  );
  return onSplit;
}

// The siblings of a registration in its page and the split (#5628).
describe("HolidayRegistrationSiblingsCard", () => {
  beforeEach(() => {
    mockSplit.mockReset();
    vi.stubEnv("NEXT_PUBLIC_TRAEGER_HOSTNAME", "traeger.localhost:3000");
  });

  it("names the children of the other registrations and splits after asking", async () => {
    mockSplit.mockResolvedValue(undefined);
    const onSplit = renderCard();

    expect(
      screen.getByRole("link", { name: "Anmeldung Nr. 8" }),
    ).toHaveAttribute("href", "/traeger/ferien/5/anmeldungen/8");
    expect(screen.getByText(/Tom Muster/)).toBeInTheDocument();
    expect(
      screen.getByText(/Ida Muster \(storniert, zählt nicht mit\)/),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Trennen" }));
    expect(
      await screen.findByText("Von den Geschwistern trennen?"),
    ).toBeInTheDocument();
    const buttons = screen.getAllByRole("button", { name: "Trennen" });
    fireEvent.click(buttons[buttons.length - 1]!);

    await waitFor(() => expect(mockSplit).toHaveBeenCalledWith("5", "7"));
    expect(
      await screen.findByText(
        "Die Anmeldung ist von den Geschwistern getrennt.",
      ),
    ).toBeInTheDocument();
    expect(onSplit).toHaveBeenCalled();
  });

  it("shows the issued invoice when the split is refused", async () => {
    mockSplit.mockRejectedValue(
      new ApiError("invoiced", 409, {
        code: "parent_billing.sibling_group_invoiced",
      }),
    );
    const onSplit = renderCard();
    fireEvent.click(screen.getByRole("button", { name: "Trennen" }));
    const buttons = await screen.findAllByRole("button", { name: "Trennen" });
    fireEvent.click(buttons[buttons.length - 1]!);

    expect(
      await screen.findByText(
        "Für diese Anmeldung gibt es schon eine Rechnung. Geschwister können Sie jetzt nicht mehr ändern.",
      ),
    ).toBeInTheDocument();
    expect(onSplit).not.toHaveBeenCalled();
  });

  it("offers no split once an invoice bills the registration", () => {
    renderCard(true);

    expect(
      screen.getByText(
        "Für diese Anmeldung gibt es schon eine Rechnung. Geschwister können Sie jetzt nicht mehr ändern.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Trennen" }),
    ).not.toBeInTheDocument();
  });
});
