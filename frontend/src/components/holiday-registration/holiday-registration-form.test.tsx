import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import deMessages from "~/i18n/messages/de.json";
import type { HolidayRegistrationOffer } from "~/lib/holiday-registration-api";

const { mockSubmit } = vi.hoisted(() => ({ mockSubmit: vi.fn() }));

vi.mock("~/lib/holiday-registration-api", () => ({
  submitHolidayRegistration: (token: string, payload: unknown) =>
    mockSubmit(token, payload),
}));

vi.mock("@marsidev/react-turnstile", () => ({
  Turnstile: ({ onSuccess }: { onSuccess: (token: string) => void }) => (
    <button type="button" onClick={() => onSuccess("captcha-token")}>
      Solve captcha
    </button>
  ),
}));

import { HolidayRegistrationForm } from "./holiday-registration-form";

const offer: HolidayRegistrationOffer = {
  traegerName: "Träger Bissendorf",
  name: "Winterferien 2027",
  registrationOpensOn: "2026-09-01",
  registrationClosesOn: "2026-12-04",
  binding: true,
  notes: "",
  open: true,
  weeks: [
    {
      id: "11",
      startsOn: "2027-01-04",
      endsOn: "2027-01-08",
      hostSchoolName: "OGS Nord",
      timeModels: [
        { id: "21", name: "bis 13 Uhr", startsAt: "08:00", endsAt: "13:00" },
      ],
    },
  ],
  schools: [{ id: "31", name: "OGS Nord" }],
  captchaSiteKey: "site",
};

function renderForm() {
  return render(
    <NextIntlClientProvider locale="de" messages={deMessages}>
      <ToastProvider>
        <HolidayRegistrationForm token="link-key" offer={offer} />
      </ToastProvider>
    </NextIntlClientProvider>,
  );
}

function choose(combobox: HTMLElement, option: string) {
  fireEvent.click(combobox);
  fireEvent.click(screen.getByRole("option", { name: option }));
}

function type(input: HTMLElement, value: string) {
  fireEvent.change(input, { target: { value } });
}

/** Fills a complete registration of one holiday guest. */
function fillRegistration() {
  const [childFirst, guardianFirst] = screen.getAllByLabelText("Vorname");
  const [childLast, guardianLast] = screen.getAllByLabelText("Nachname");
  type(childFirst!, "Ben");
  type(childLast!, "Muster");
  choose(screen.getByRole("combobox", { name: "Tag" }), "4");
  choose(screen.getByRole("combobox", { name: "Monat" }), "Mai");
  choose(screen.getByRole("combobox", { name: "Jahr" }), "2017");
  choose(
    screen.getByRole("combobox", { name: "Schule" }),
    "Eine andere Schule",
  );
  expect(screen.getByText(/Ihr Kind kommt als Feriengast/)).toBeInTheDocument();
  choose(
    screen.getByRole("combobox", { name: /04\.01\.2027 bis 08\.01\.2027/ }),
    "bis 13 Uhr (08:00 bis 13:00 Uhr)",
  );
  type(guardianFirst!, "Eva");
  type(guardianLast!, "Muster");
  type(screen.getByLabelText("E-Mail-Adresse"), "eva@example.org");
  type(screen.getAllByLabelText("Telefon")[0]!, "0151 1");
  type(screen.getAllByLabelText("Name")[0]!, "Oma Muster");
  type(screen.getAllByLabelText("Telefon")[1]!, "05402 1");
  fireEvent.click(
    screen.getByLabelText(/Ich bin einverstanden, dass der Träger/),
  );
  fireEvent.click(
    screen.getByLabelText("Ich weiß, dass die Anmeldung verbindlich ist."),
  );
}

async function renderFilledForm() {
  renderForm();
  fillRegistration();
  fireEvent.click(
    screen.getByRole("button", { name: "Weiter zur Zusammenfassung" }),
  );
  await screen.findByText("Bitte prüfen Sie Ihre Angaben");
}

describe("HolidayRegistrationForm", () => {
  it("marks every booking control when the server refuses one booking", async () => {
    mockSubmit.mockRejectedValue(
      new ApiError("refused", 400, {
        code: "enrollment.holiday_registration_invalid",
        errors: [{ field: "children.0.bookings.0", reason: "gone" }],
      }),
    );
    await renderFilledForm();
    fireEvent.click(screen.getByRole("button", { name: "Solve captcha" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Verbindlich anmelden" }),
    );

    const week = await screen.findByRole("combobox", {
      name: /04\.01\.2027 bis 08\.01\.2027/,
    });
    await waitFor(() =>
      expect(week).toHaveAttribute(
        "aria-describedby",
        "children.0-bookings-error",
      ),
    );
    expect(
      document.getElementById("children.0-bookings-error"),
    ).toHaveTextContent(/\S/);
  });

  beforeEach(() => {
    mockSubmit.mockReset();
    window.scrollTo = vi.fn();
  });

  it("marks the missing fields instead of showing the summary", async () => {
    renderForm();
    fireEvent.click(
      screen.getByRole("button", { name: "Weiter zur Zusammenfassung" }),
    );

    expect(
      await screen.findByText("Bitte prüfen Sie die markierten Felder."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Bitte wählen Sie mindestens eine Woche."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Bitte wählen Sie die Schule Ihres Kindes."),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Bitte prüfen Sie Ihre Angaben"),
    ).not.toBeInTheDocument();
    expect(mockSubmit).not.toHaveBeenCalled();
  });

  it("registers a holiday guest after the summary", async () => {
    mockSubmit.mockResolvedValue(undefined);
    renderForm();
    fillRegistration();
    fireEvent.click(
      screen.getByRole("button", { name: "Weiter zur Zusammenfassung" }),
    );

    expect(
      await screen.findByText("Bitte prüfen Sie Ihre Angaben"),
    ).toBeInTheDocument();
    expect(screen.getByText("Feriengast")).toBeInTheDocument();
    // The security check sits next to the send button and is solved there,
    // so its token is fresh when the registration leaves (quorum round 1).
    expect(
      screen.getByRole("button", { name: "Verbindlich anmelden" }),
    ).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Solve captcha" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Verbindlich anmelden" }),
    );

    await waitFor(() => expect(mockSubmit).toHaveBeenCalledTimes(1));
    expect(mockSubmit).toHaveBeenCalledWith(
      "link-key",
      expect.objectContaining({
        children: [
          expect.objectContaining({
            first_name: "Ben",
            birth_date: "2017-05-04",
            home_school_id: 0,
            bookings: [{ week_id: 11, time_model_id: 21 }],
          }),
        ],
        privacy_consent: true,
        binding_confirmed: true,
        health_consent: false,
        captcha_token: "captcha-token",
      }),
    );
    expect(
      await screen.findByText("Ihre Anmeldung ist eingegangen"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Wir schicken Ihnen eine Bestätigung an eva@example.org.",
      ),
    ).toBeInTheDocument();
  });
});
