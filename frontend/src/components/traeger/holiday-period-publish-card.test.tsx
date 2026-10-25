import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactElement } from "react";
import { ToastProvider } from "~/contexts/ToastContext";
import type { HolidayPeriod } from "~/lib/traeger-api";

const { mockPublish } = vi.hoisted(() => ({ mockPublish: vi.fn() }));

vi.mock("~/lib/traeger-api", () => ({
  publishHolidayPeriod: (id: string) => mockPublish(id),
}));

import { HolidayPeriodPublishCard } from "./holiday-period-publish-card";

const renderCard = (card: ReactElement) =>
  render(<ToastProvider>{card}</ToastProvider>);

const complete: HolidayPeriod = {
  id: "5",
  name: "Osterferien",
  registrationOpensOn: "2027-01-11",
  registrationClosesOn: "2027-02-26",
  binding: true,
  notes: "",
  status: "draft",
  publishedAt: null,
  weeks: [
    {
      id: "7",
      startsOn: "2027-03-22",
      endsOn: "2027-03-26",
      hostSchoolId: "31",
      timeModels: [
        {
          id: "9",
          name: "bis 13 Uhr",
          startsAt: "08:00",
          endsAt: "13:00",
          priceCents: 4500,
        },
      ],
    },
  ],
};

describe("HolidayPeriodPublishCard", () => {
  beforeEach(() => mockPublish.mockReset());

  it("names what is missing instead of offering the button", () => {
    renderCard(
      <HolidayPeriodPublishCard
        period={{ ...complete, weeks: [] }}
        unsavedChanges={false}
        onPublished={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("button", { name: "Veröffentlichen" }),
    ).toBeDisabled();
    expect(
      screen.getByText(/braucht der Ferienzeitraum mindestens eine Woche/),
    ).toBeInTheDocument();
  });

  it("asks to save unsaved changes first", () => {
    renderCard(
      <HolidayPeriodPublishCard
        period={complete}
        unsavedChanges
        onPublished={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("button", { name: "Veröffentlichen" }),
    ).toBeDisabled();
    expect(
      screen.getByText("Bitte speichern Sie zuerst Ihre Änderungen."),
    ).toBeInTheDocument();
  });

  it("publishes a complete period after the confirmation", async () => {
    const onPublished = vi.fn();
    mockPublish.mockResolvedValue({
      ...complete,
      status: "published",
      publishedAt: "2026-10-25T09:00:00Z",
    });
    renderCard(
      <HolidayPeriodPublishCard
        period={complete}
        unsavedChanges={false}
        onPublished={onPublished}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Veröffentlichen" }));
    expect(mockPublish).not.toHaveBeenCalled();
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(
      Array.from(dialog.querySelectorAll("button")).find(
        (button) => button.textContent === "Veröffentlichen",
      )!,
    );

    await waitFor(() => expect(mockPublish).toHaveBeenCalledWith("5"));
    await waitFor(() =>
      expect(onPublished).toHaveBeenCalledWith(
        expect.objectContaining({ status: "published" }),
      ),
    );
  });

  it("shows since when a published period is open for registration", () => {
    renderCard(
      <HolidayPeriodPublishCard
        period={{
          ...complete,
          status: "published",
          publishedAt: "2026-10-25T09:00:00Z",
        }}
        unsavedChanges={false}
        onPublished={vi.fn()}
      />,
    );
    expect(
      screen.getByText(/Seit 25\.10\.2026 für die Anmeldung freigegeben\./),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button")).toBeNull();
    expect(screen.getByText("Veröffentlicht")).toBeInTheDocument();
  });
});
