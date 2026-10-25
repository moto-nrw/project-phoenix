import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
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
  publicToken: "abc123",
  registrationCount: 0,
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
  beforeEach(() => {
    mockPublish.mockReset();
    vi.stubEnv("NEXT_PUBLIC_PARENTS_HOSTNAME", "parents.localhost:3000");
    // The published card links to the review of its registrations (#4663).
    vi.stubEnv("NEXT_PUBLIC_TRAEGER_HOSTNAME", "traeger.localhost:3000");
  });
  afterEach(() => vi.unstubAllEnvs());

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
    // The only control left is the copy button of the parents' link (#4661).
    expect(screen.getAllByRole("button")).toHaveLength(1);
    expect(
      screen.getByRole("button", { name: "Link kopieren" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Veröffentlicht")).toBeInTheDocument();
  });

  it("shows the one link for parents and the registrations received", () => {
    renderCard(
      <HolidayPeriodPublishCard
        period={{
          ...complete,
          status: "published",
          publishedAt: "2026-10-25T09:00:00Z",
          registrationCount: 3,
        }}
        unsavedChanges={false}
        onPublished={vi.fn()}
      />,
    );
    expect(
      screen.getByText("http://parents.localhost:3000/ferienanmeldung/abc123"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Bisher sind 3 Anmeldungen eingegangen."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Anmeldungen prüfen" }),
    ).toHaveAttribute("href", "/traeger/ferien/5/anmeldungen");
  });

  it("tells a draft that the link for parents comes with publishing", () => {
    renderCard(
      <HolidayPeriodPublishCard
        period={complete}
        unsavedChanges={false}
        onPublished={vi.fn()}
      />,
    );
    expect(
      screen.getByText("Den Link für Eltern gibt es nach dem Veröffentlichen."),
    ).toBeInTheDocument();
    expect(screen.queryByText(/ferienanmeldung/)).toBeNull();
  });
});
