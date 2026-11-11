import { act, fireEvent, render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { Suspense } from "react";
import { describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import deMessages from "~/i18n/messages/de.generated";
import type { HolidayRegistrationStep } from "~/components/holiday-registration/holiday-registration-form";
import type { HolidayRegistrationOffer } from "~/lib/holiday-registration-api";
import { HOLIDAY_CONSENT_TEXTS } from "~/test/fixtures/holiday-consent-texts";

const offer: HolidayRegistrationOffer = {
  traegerName: "Träger Bissendorf",
  name: "Winterferien 2027",
  registrationOpensOn: "2026-09-01",
  registrationClosesOn: "2026-12-04",
  binding: true,
  holidayWishMode: "off",
  notes: "",
  open: true,
  weeks: [],
  schools: [],
  captchaSiteKey: "site",
  consentTexts: HOLIDAY_CONSENT_TEXTS,
  form: { version: 1, fields: [] },
};

const session = vi.hoisted(() => ({
  current: { data: null, status: "unauthenticated" } as {
    data: { user: { scope: string } } | null;
    status: "authenticated" | "unauthenticated";
  },
}));

vi.mock("next-auth/react", () => ({
  useSession: () => session.current,
}));

vi.mock("~/components/parent/language-switcher", () => ({
  LanguageSwitcher: () => null,
}));

vi.mock("~/lib/hooks/use-analytics-school-policy", () => ({
  useAnalyticsSchoolPolicy: () => undefined,
}));

vi.mock("~/lib/holiday-registration-api", () => ({
  fetchHolidayRegistrationOffer: () => Promise.resolve(offer),
  fetchHolidayRegistrationFamily: () => Promise.resolve(null),
}));

// The form itself is covered by its own tests; here it only reports its step.
vi.mock("~/components/holiday-registration/holiday-registration-form", () => ({
  HolidayRegistrationForm: ({
    onStepChange,
    parentAccount,
  }: {
    onStepChange?: (step: HolidayRegistrationStep) => void;
    parentAccount?: string;
  }) => (
    <div>
      <p>{`Elternkonto: ${parentAccount ?? "keins"}`}</p>
      {(["form", "summary", "done"] as const).map((step) => (
        <button key={step} type="button" onClick={() => onStepChange?.(step)}>
          {`Schritt ${step}`}
        </button>
      ))}
    </div>
  ),
}));

import HolidayRegistrationPage from "./page";

const HINT = "Felder mit * müssen Sie ausfüllen.";

async function renderPage() {
  await act(async () => {
    render(
      <NextIntlClientProvider locale="de" messages={deMessages}>
        <ToastProvider>
          <Suspense>
            <HolidayRegistrationPage
              params={Promise.resolve({ token: "link-key" })}
            />
          </Suspense>
        </ToastProvider>
      </NextIntlClientProvider>,
    );
  });
  await screen.findByText("Winterferien 2027");
}

describe("HolidayRegistrationPage", () => {
  it("shows the required-field hint only while the form step shows (#5334)", async () => {
    await renderPage();
    expect(screen.getByText(HINT)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Schritt summary" }));
    // Anker: the offer header is still rendered without the hint.
    expect(screen.getByText("Winterferien 2027")).toBeInTheDocument();
    expect(screen.queryByText(HINT)).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Schritt done" }));
    // Anker: the offer header is still rendered without the hint.
    expect(screen.getByText("Winterferien 2027")).toBeInTheDocument();
    expect(screen.queryByText(HINT)).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Schritt form" }));
    expect(screen.getByText(HINT)).toBeInTheDocument();
  });

  it("offers the login of the parents app at the start to a visitor without a session (#6671)", async () => {
    session.current = { data: null, status: "unauthenticated" };
    await renderPage();

    expect(
      screen.getByText("Sie haben schon die Eltern-App?"),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Anmelden" })).toHaveAttribute(
      "href",
      "/login?next=%2Fferienanmeldung%2Flink-key",
    );
    // A visitor without a session did not come from the app.
    expect(
      screen.queryByRole("link", { name: "Zurück zur Eltern-App" }),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Schritt summary" }));
    // Anker: the offer header is still rendered; the hint belongs to the form.
    expect(screen.getByText("Winterferien 2027")).toBeInTheDocument();
    expect(
      screen.queryByText("Sie haben schon die Eltern-App?"),
    ).not.toBeInTheDocument();
  });

  it("shows a signed-in parent the way back to the app instead of the login hint (#6671)", async () => {
    session.current = {
      data: { user: { scope: "parent" } },
      status: "authenticated",
    };
    await renderPage();

    expect(
      screen.getByRole("link", { name: "Zurück zur Eltern-App" }),
    ).toHaveAttribute("href", "/");
    expect(
      screen.queryByText("Sie haben schon die Eltern-App?"),
    ).not.toBeInTheDocument();
  });

  it("lets the form ask for an account only without a parent session (#6673)", async () => {
    session.current = { data: null, status: "unauthenticated" };
    await renderPage();
    expect(screen.getByText("Elternkonto: ask")).toBeInTheDocument();
  });

  it("tells the form that a parent is signed in (#6673)", async () => {
    session.current = {
      data: { user: { scope: "parent" } },
      status: "authenticated",
    };
    await renderPage();
    expect(screen.getByText("Elternkonto: signed-in")).toBeInTheDocument();
  });
});
