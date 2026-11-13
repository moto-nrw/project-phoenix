import { render, screen, within } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import deMessages from "~/i18n/messages/de.generated";
import { ApiError } from "~/lib/api-error";
import type { HolidayRegistrationStatus } from "~/lib/holiday-registration-status-api";
import { HolidayRegistrationStatusView } from "./holiday-registration-status-view";

const mockFetch = vi.hoisted(() => vi.fn());

vi.mock("~/lib/holiday-registration-status-api", async (importOriginal) => ({
  ...(await importOriginal<
    typeof import("~/lib/holiday-registration-status-api")
  >()),
  fetchHolidayRegistrationStatus: (token: string) => mockFetch(token),
}));

const received: HolidayRegistrationStatus = {
  traegerName: "Gemeinde Bissendorf",
  periodName: "Weihnachtsferien 2026/27",
  firstDay: "2027-01-04",
  lastDay: "2027-01-08",
  registrationClosesOn: "2026-12-01",
  binding: true,
  allocationMode: "arrival",
  submittedAt: "2026-11-16T09:30:00Z",
  status: "received",
  children: [
    {
      firstName: "Lia",
      weeks: [
        {
          firstDay: "2027-01-04",
          lastDay: "2027-01-08",
          timeModelName: "Ganztags",
          startsAt: "08:00",
          endsAt: "16:00",
          hostSchoolName: "Grundschule Bissendorf",
          placement: "placed",
        },
      ],
    },
    {
      firstName: "Ben",
      weeks: [
        {
          firstDay: "2027-01-04",
          lastDay: "2027-01-08",
          timeModelName: "Vormittags",
          startsAt: "08:00",
          endsAt: "13:00",
          hostSchoolName: "Grundschule Bissendorf",
          placement: "waitlisted",
        },
      ],
    },
  ],
};

function renderView() {
  return render(
    <NextIntlClientProvider locale="de" messages={deMessages}>
      <ToastProvider>
        <HolidayRegistrationStatusView token="t0ken" />
      </ToastProvider>
    </NextIntlClientProvider>,
  );
}

describe("HolidayRegistrationStatusView (#6676)", () => {
  beforeEach(() => {
    mockFetch.mockReset();
  });

  it("shows the state, the next steps and every child's weeks with their place", async () => {
    mockFetch.mockResolvedValue(received);
    renderView();

    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "Ihre Anmeldung ist eingegangen",
      }),
    ).toBeInTheDocument();
    expect(mockFetch).toHaveBeenCalledWith("t0ken");
    expect(screen.getByText("Weihnachtsferien 2026/27")).toBeInTheDocument();
    expect(screen.getByText("16.11.2026")).toBeInTheDocument();
    // The window is still open on the test clock: the parent learns when the
    // Träger decides.
    expect(
      screen.getByText(
        "Anmelden geht noch bis 01.12.2026. Danach entscheidet der Träger.",
      ),
    ).toBeInTheDocument();

    const lia = screen.getByRole("region", { name: "Lia" });
    expect(
      within(lia).getByText(
        "Ganztags (08:00 bis 16:00 Uhr) · in Grundschule Bissendorf",
      ),
    ).toBeInTheDocument();
    expect(within(lia).getByText("Platz reserviert")).toBeInTheDocument();
    const ben = screen.getByRole("region", { name: "Ben" });
    expect(within(ben).getByText("Warteliste")).toBeInTheDocument();
    expect(
      screen.getByText(/Dort steht Ihr Kind auf der Warteliste/),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Die Anmeldung ist verbindlich."),
    ).toBeInTheDocument();
  });

  it("names a confirmed registration and marks every step done", async () => {
    mockFetch.mockResolvedValue({ ...received, status: "confirmed" });
    renderView();

    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "Ihre Anmeldung ist bestätigt",
      }),
    ).toBeInTheDocument();
    expect(screen.getByText("Die Zusage ist da.")).toBeInTheDocument();
    expect(screen.getAllByText(": erledigt")).toHaveLength(3);
  });

  it("shows no places of a cancelled registration and says who to ask", async () => {
    mockFetch.mockResolvedValue({ ...received, status: "cancelled" });
    renderView();

    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "Diese Anmeldung gilt nicht mehr",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Es gibt keinen nächsten Schritt. Bei Fragen wenden Sie sich bitte an den Träger.",
      ),
    ).toBeInTheDocument();
    const lia = screen.getByRole("region", { name: "Lia" });
    expect(within(lia).getByText("04.01.2027 bis 08.01.2027")).toBeVisible();
    expect(within(lia).queryByText("Platz reserviert")).not.toBeInTheDocument();
  });

  it("explains a link without a registration instead of an error", async () => {
    mockFetch.mockRejectedValue(
      new ApiError("not found", 404, {
        code: "enrollment.holiday_registration_status_not_found",
      }),
    );
    renderView();

    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "Wir finden diese Anmeldung nicht",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Bitte öffnen Sie ihn noch einmal aus Ihrer E-Mail/),
    ).toBeInTheDocument();
  });
});
