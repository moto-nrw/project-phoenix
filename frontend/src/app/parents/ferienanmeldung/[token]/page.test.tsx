import { act, fireEvent, render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { Suspense } from "react";
import { describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import deMessages from "~/i18n/messages/de.generated";
import type { HolidayRegistrationStep } from "~/components/holiday-registration/holiday-registration-form";
import type { HolidayRegistrationOffer } from "~/lib/holiday-registration-api";

const offer: HolidayRegistrationOffer = {
  traegerName: "Träger Bissendorf",
  name: "Winterferien 2027",
  registrationOpensOn: "2026-09-01",
  registrationClosesOn: "2026-12-04",
  binding: true,
  notes: "",
  open: true,
  weeks: [],
  schools: [],
  captchaSiteKey: "site",
  form: { version: 1, fields: [] },
};

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: null, status: "unauthenticated" }),
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
  }: {
    onStepChange?: (step: HolidayRegistrationStep) => void;
  }) => (
    <div>
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
});
