import { fireEvent, render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { LanguageSwitcher } from "~/components/parent/language-switcher";
import { ToastProvider } from "~/contexts/ToastContext";
import { LOCALE_COOKIE_NAME } from "~/i18n/locales";
import deMessages from "~/i18n/messages/de.generated";
import enMessages from "~/i18n/messages/en.generated";
import type { HolidayRegistrationOffer } from "~/lib/holiday-registration-api";
import { ParentLocaleProvider } from "~/lib/parent-locale-context";
import { HolidayRegistrationForm } from "./holiday-registration-form";

// A language switch must keep the half-filled registration (#5327). The real
// catalogs show that the form re-renders in the new language.
vi.unmock("next-intl");

// router.refresh() brings the server tree back in the cookie's language. The
// harness below plays that part: it hands the new locale to the provider
// without remounting the page.
const refresh = vi.hoisted(() => ({ current: () => {} }));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ refresh: () => refresh.current() }),
}));

// The public link needs no parent account.
vi.mock("next-auth/react", () => ({
  useSession: () => ({ status: "unauthenticated", data: null }),
}));

vi.mock("~/lib/holiday-registration-api", () => ({
  submitHolidayRegistration: vi.fn(),
  quoteHolidayContribution: vi.fn(() => new Promise(() => {})),
  uploadHolidayProof: vi.fn(),
}));

vi.mock("@marsidev/react-turnstile", () => ({
  Turnstile: () => null,
}));

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
        {
          id: "21",
          name: "bis 13 Uhr",
          startsAt: "08:00",
          endsAt: "13:00",
          priceCents: 4500,
        },
      ],
    },
  ],
  schools: [{ id: "31", name: "OGS Nord" }],
  captchaSiteKey: "site",
  form: { version: 1, fields: [] },
};

const MESSAGES = { de: deMessages, en: enMessages } as const;

function Harness() {
  const [locale, setLocale] = useState<"de" | "en">("de");
  refresh.current = () => {
    const cookie = new RegExp(`${LOCALE_COOKIE_NAME}=([a-z]+)`).exec(
      document.cookie,
    )?.[1];
    setLocale(cookie === "en" ? "en" : "de");
  };
  return (
    <NextIntlClientProvider locale={locale} messages={MESSAGES[locale]}>
      <ToastProvider>
        <ParentLocaleProvider>
          <LanguageSwitcher />
          <HolidayRegistrationForm token="link-key" offer={offer} />
        </ParentLocaleProvider>
      </ToastProvider>
    </NextIntlClientProvider>
  );
}

function choose(combobox: HTMLElement, option: string | RegExp) {
  fireEvent.click(combobox);
  fireEvent.click(screen.getByRole("option", { name: option }));
}

function type(input: HTMLElement, value: string) {
  fireEvent.change(input, { target: { value } });
}

function fillRegistration() {
  const [childFirst, guardianFirst] = screen.getAllByLabelText("Vorname");
  const [childLast, guardianLast] = screen.getAllByLabelText("Nachname");
  type(childFirst!, "Ben");
  type(childLast!, "Muster");
  choose(screen.getByRole("combobox", { name: "Tag" }), "4");
  choose(screen.getByRole("combobox", { name: "Monat" }), "Mai");
  choose(screen.getByRole("combobox", { name: "Jahr" }), "2017");
  choose(screen.getByRole("combobox", { name: "Schule" }), "OGS Nord");
  choose(
    screen.getByRole("combobox", { name: /04\.01\.2027 bis 08\.01\.2027/ }),
    /^bis 13 Uhr/,
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
    screen.getByLabelText("Ich weiß, dass die Anmeldung verbindlich ist. *"),
  );
}

describe("HolidayRegistrationForm language switch", () => {
  it("keeps every answer and the summary step when the parent switches the language", async () => {
    window.scrollTo = vi.fn();
    render(<Harness />);
    fillRegistration();
    fireEvent.click(
      screen.getByRole("button", { name: "Weiter zur Zusammenfassung" }),
    );
    await screen.findByText("Bitte prüfen Sie Ihre Angaben");

    fireEvent.click(screen.getByRole("button", { name: /Sprache/ }));
    fireEvent.click(screen.getByRole("option", { name: "English" }));

    // Same step, now in English, with the answers still there.
    expect(
      await screen.findByText("Please check your details"),
    ).toBeInTheDocument();
    expect(screen.getByText("eva@example.org")).toBeInTheDocument();
    expect(screen.getByText(/Oma Muster/)).toBeInTheDocument();

    // Back in the form, the fields still hold what was typed.
    fireEvent.click(screen.getByRole("button", { name: "Back and change" }));
    const [childFirst, guardianFirst] = screen.getAllByLabelText("First name");
    expect(childFirst).toHaveValue("Ben");
    expect(guardianFirst).toHaveValue("Eva");
    expect(screen.getByLabelText("E-mail address")).toHaveValue(
      "eva@example.org",
    );
  });
});
