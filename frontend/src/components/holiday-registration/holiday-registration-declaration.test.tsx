import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, expect, it, vi } from "vitest";
import enMessages from "~/i18n/messages/en.generated";
import ruMessages from "~/i18n/messages/ru.generated";
import { HOLIDAY_CONSENT_TEXTS } from "~/test/fixtures/holiday-consent-texts";
import { HolidayRegistrationDeclaration } from "./holiday-registration-declaration";

// The real next-intl: the test reads the catalogue of each language.
vi.unmock("next-intl");

const DRAFT = {
  privacyConsent: false,
  healthConsent: false,
  bindingConfirmed: false,
  custodyConfirmed: false,
  declarationName: "",
};

function renderDeclaration(locale: "en" | "ru", texts = HOLIDAY_CONSENT_TEXTS) {
  return render(
    <NextIntlClientProvider
      locale={locale}
      messages={locale === "en" ? enMessages : ruMessages}
    >
      <HolidayRegistrationDeclaration
        texts={texts}
        draft={DRAFT}
        binding={false}
        hasHealth={false}
        fieldError={() => undefined}
        onChange={() => undefined}
      />
    </NextIntlClientProvider>,
  );
}

// The reading aid below the German privacy text names the retention the
// German text names (#6691), in the number forms of its language.
describe("HolidayRegistrationDeclaration reading aid", () => {
  it("names the retention of the offer in English", () => {
    renderDeclaration("en");
    expect(
      screen.getByText(/It deletes it 365 days after the holidays\./),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        /It deletes health information already 92 days after the holidays\./,
      ),
    ).toBeInTheDocument();
  });

  it("says that the provider keeps the debtor number and when it goes (#6692)", () => {
    renderDeclaration("en");
    expect(
      screen.getByText(
        /the provider keeps it for later registrations\. It deletes it at the end of the third year after the last registration\./,
      ),
    ).toBeInTheDocument();
  });

  it("uses the Russian number forms for the days", () => {
    renderDeclaration("ru", { ...HOLIDAY_CONSENT_TEXTS, retentionDays: 181 });
    expect(
      screen.getByText(/через 181 день после каникул/),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/уже через 92 дня после каникул/),
    ).toBeInTheDocument();
  });

  it("stays away for texts of another version", () => {
    renderDeclaration("en", { ...HOLIDAY_CONSENT_TEXTS, version: 3 });
    expect(screen.queryByText(/It deletes it/)).not.toBeInTheDocument();
    expect(
      screen.getByText(/Er löscht sie 365 Tage nach den Ferien\./),
    ).toBeInTheDocument();
  });
});
