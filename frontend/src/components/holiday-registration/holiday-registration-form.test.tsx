import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import deMessages from "~/i18n/messages/de.generated";
import type {
  HolidayKnownFamily,
  HolidayRegistrationOffer,
} from "~/lib/holiday-registration-api";
import { HOLIDAY_CONSENT_TEXTS } from "~/test/fixtures/holiday-consent-texts";

const { mockSubmit, mockQuote, mockUpload } = vi.hoisted(() => ({
  mockSubmit: vi.fn(),
  mockQuote: vi.fn(),
  mockUpload: vi.fn(),
}));

vi.mock("~/lib/holiday-registration-api", () => ({
  submitHolidayRegistration: (token: string, payload: unknown) =>
    mockSubmit(token, payload),
  quoteHolidayContribution: (
    token: string,
    children: unknown,
    benefitsDeclared: boolean,
  ) => mockQuote(token, children, benefitsDeclared),
  uploadHolidayProof: (token: string, file: File, signal: AbortSignal) =>
    mockUpload(token, file, signal),
}));

/** The contribution of Ben's one week, with or without the exemption. */
function quoted(benefitsDeclared: boolean) {
  const base = {
    kind: "base",
    amountCents: 4500,
    weekStartsOn: "2027-01-04",
    weekEndsOn: "2027-01-08",
    timeModelName: "bis 13 Uhr",
    childRank: null,
    discountBasisPoints: null,
    discountCents: null,
    note: "",
  };
  const exemption = {
    ...base,
    kind: "benefits_exemption",
    amountCents: -4500,
    weekStartsOn: null,
    weekEndsOn: null,
    timeModelName: "",
    note: "Beitragsfrei mit BuT-Nachweis",
  };
  return {
    children: [
      {
        childId: "",
        position: 1,
        lines: benefitsDeclared ? [base, exemption] : [base],
        totalCents: benefitsDeclared ? 0 : 4500,
      },
    ],
    totalCents: benefitsDeclared ? 0 : 4500,
    exemptionAvailable: true,
    benefitsExemption: benefitsDeclared,
    frozen: false,
    confirmedAt: null,
  };
}

vi.mock("@marsidev/react-turnstile", () => ({
  Turnstile: ({ onSuccess }: { onSuccess: (token: string) => void }) => (
    <button type="button" onClick={() => onSuccess("captcha-token")}>
      Solve captcha
    </button>
  ),
}));

import {
  HolidayRegistrationForm,
  type HolidayRegistrationStep,
} from "./holiday-registration-form";

const offer: HolidayRegistrationOffer = {
  traegerName: "Träger Bissendorf",
  name: "Winterferien 2027",
  registrationOpensOn: "2026-09-01",
  registrationClosesOn: "2026-12-04",
  binding: true,
  holidayWishMode: "off",
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
  consentTexts: HOLIDAY_CONSENT_TEXTS,
  form: { version: 1, fields: [] },
};

const family: HolidayKnownFamily = {
  guardian: {
    firstName: "Sabine",
    lastName: "Schneider",
    email: "sabine@example.org",
    phone: "0151 2",
  },
  children: [
    {
      studentId: "41",
      schoolId: "31",
      firstName: "Felix",
      lastName: "Schneider",
      birthDate: "2018-03-01",
      holidayWish: "",
      holidayWishProof: "",
    },
  ],
  accountProof: "",
};

function renderForm(
  known: HolidayKnownFamily | null = null,
  shown: HolidayRegistrationOffer = offer,
  onFormChanged?: () => Promise<void>,
  onStepChange?: (step: HolidayRegistrationStep) => void,
) {
  return render(
    <NextIntlClientProvider locale="de" messages={deMessages}>
      <ToastProvider>
        <HolidayRegistrationForm
          token="link-key"
          offer={shown}
          family={known}
          onFormChanged={onFormChanged}
          onStepChange={onStepChange}
        />
      </ToastProvider>
    </NextIntlClientProvider>,
  );
}

function choose(combobox: HTMLElement, option: string | RegExp) {
  fireEvent.click(combobox);
  fireEvent.click(screen.getByRole("option", { name: option }));
}

function type(input: HTMLElement, value: string) {
  fireEvent.change(input, { target: { value } });
}

/** The time model of the week with its price; € follows a no-break space. */
const WEEK_TIME_MODEL = /^bis 13 Uhr · 45,00\s€ \(08:00 bis 13:00 Uhr\)$/;

/** Fills a complete registration of one holiday guest. */
function fillRegistration(timeModel: RegExp = WEEK_TIME_MODEL) {
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
    timeModel,
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
  fireEvent.click(
    screen.getByLabelText(/Ich darf die Anmeldung allein abgeben/),
  );
  type(screen.getByLabelText("Ihr Vor- und Nachname"), "Eva Maria Muster");
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
  it("shows a server day error at the selector and focuses its first day", async () => {
    mockSubmit.mockRejectedValue(
      new ApiError("refused", 400, {
        code: "enrollment.holiday_registration_invalid",
        errors: [{ field: "children.0.bookings.0.days", reason: "invalid" }],
      }),
    );
    renderForm(null, {
      ...offer,
      weeks: offer.weeks.map((week) => ({
        ...week,
        bookingUnit: "day",
        careDays: ["2027-01-04", "2027-01-05"],
      })),
    });
    fillRegistration(/^bis 13 Uhr/);
    fireEvent.click(screen.getByRole("button", { name: "Mo 04.01." }));
    fireEvent.click(
      screen.getByRole("button", { name: "Weiter zur Zusammenfassung" }),
    );
    await screen.findByText("Bitte prüfen Sie Ihre Angaben");
    fireEvent.click(screen.getByRole("button", { name: "Solve captcha" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Verbindlich anmelden" }),
    );

    const monday = await screen.findByRole("button", { name: "Mo 04.01." });
    await waitFor(() => expect(monday).toHaveFocus());
    const selector = monday.closest("fieldset")!;
    expect(selector).toHaveAttribute("aria-invalid", "true");
    const descriptions = selector.getAttribute("aria-describedby")!.split(" ");
    expect(descriptions).toHaveLength(2);
    expect(document.getElementById(descriptions[1]!)).toHaveTextContent(/\S/);
  });

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

  it("maps pickup_persons.1.name to the third person and shows both roles once in the summary", async () => {
    mockSubmit.mockRejectedValue(
      new ApiError("refused", 400, {
        code: "enrollment.holiday_registration_invalid",
        errors: [{ field: "pickup_persons.1.name", reason: "invalid" }],
      }),
    );
    renderForm();
    fillRegistration();
    fireEvent.click(screen.getByRole("button", { name: "Person hinzufügen" }));
    let people = screen.getAllByRole("group", { name: /Person \d/ });
    type(within(people[1]!).getByLabelText("Name"), "Opa");
    fireEvent.click(within(people[1]!).getByLabelText("Notfallkontakt"));
    fireEvent.click(
      within(people[1]!).getByLabelText("Darf die Kinder abholen"),
    );
    fireEvent.click(screen.getByRole("button", { name: "Person hinzufügen" }));
    people = screen.getAllByRole("group", { name: /Person \d/ });
    type(within(people[2]!).getByLabelText("Name"), "Tante");
    type(within(people[2]!).getByLabelText("Telefon"), "0123");
    fireEvent.click(
      within(people[2]!).getByLabelText("Darf die Kinder abholen"),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Weiter zur Zusammenfassung" }),
    );
    expect(
      await screen.findByText("Bitte prüfen Sie Ihre Angaben"),
    ).toBeInTheDocument();
    expect(screen.getByText(/Tante · 0123/)).toHaveTextContent(
      "Notfallkontakt, Darf die Kinder abholen",
    );
    expect(screen.getAllByText(/Tante · 0123/)).toHaveLength(1);
    fireEvent.click(screen.getByRole("button", { name: "Solve captcha" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Verbindlich anmelden" }),
    );
    const thirdName = await screen.findByDisplayValue("Tante");
    await waitFor(() =>
      expect(thirdName).toHaveAttribute("aria-invalid", "true"),
    );
    expect(thirdName).toHaveFocus();
    expect(screen.getByDisplayValue("Opa")).not.toHaveAttribute(
      "aria-invalid",
      "true",
    );
  });

  it.each([
    ["noRole", "Bitte wählen Sie mindestens eine Aufgabe."],
    ["noEmergency", "Bitte wählen Sie mindestens einen Notfallkontakt."],
    ["noPhone", "Bitte füllen Sie dieses Feld aus."],
    ["tooMany", "Sie können höchstens drei Notfallkontakte angeben."],
  ])("blocks the summary and submission for %s", async (scenario, message) => {
    renderForm();
    fillRegistration();
    if (scenario === "noRole" || scenario === "noEmergency") {
      fireEvent.click(screen.getByLabelText("Notfallkontakt"));
      if (scenario === "noEmergency")
        fireEvent.click(screen.getByLabelText("Darf die Kinder abholen"));
      else {
        fireEvent.click(
          screen.getByRole("button", { name: "Person hinzufügen" }),
        );
        const person = within(screen.getByRole("group", { name: "Person 2" }));
        type(person.getByLabelText("Name"), "Opa");
        type(person.getByLabelText("Telefon"), "0123");
      }
    } else if (scenario === "noPhone") {
      type(screen.getAllByLabelText("Telefon")[1]!, "");
    } else {
      for (let i = 2; i <= 4; i++) {
        fireEvent.click(
          screen.getByRole("button", { name: "Person hinzufügen" }),
        );
        const person = within(
          screen.getByRole("group", { name: `Person ${i}` }),
        );
        type(person.getByLabelText("Name"), `Kontakt ${i}`);
        type(person.getByLabelText("Telefon"), "0123");
      }
    }
    fireEvent.click(
      screen.getByRole("button", { name: "Weiter zur Zusammenfassung" }),
    );
    expect(await screen.findByText(message)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Weiter zur Zusammenfassung" }),
    ).toBeInTheDocument();
    expect(mockSubmit).not.toHaveBeenCalled();
  });

  beforeEach(() => {
    mockSubmit.mockReset();
    mockQuote.mockReset();
    mockQuote.mockResolvedValue(quoted(false));
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
    // The date of birth carries its hint like every other field (quorum
    // round 2): the day select is named, all three are described by it.
    for (const part of ["Tag", "Monat", "Jahr"]) {
      expect(screen.getByRole("combobox", { name: part })).toHaveAttribute(
        "aria-describedby",
        "children.0-birth-date-error",
      );
    }
    expect(
      document.getElementById("children.0-birth-date-error"),
    ).toHaveTextContent("Bitte füllen Sie dieses Feld aus.");
    expect(
      document.querySelector(
        'input[type="hidden"][name="children.0.birth_date"]',
      ),
    ).not.toBeNull();
    expect(
      screen.queryByText("Bitte prüfen Sie Ihre Angaben"),
    ).not.toBeInTheDocument();
    expect(mockSubmit).not.toHaveBeenCalled();
  });

  it("asks for the custody and the name, shows the name in the summary and sends the declaration (#5626)", async () => {
    mockSubmit.mockResolvedValue(undefined);
    renderForm();
    fillRegistration();
    const consents = within(
      screen
        .getByRole("heading", { name: "Einverständnis" })
        .closest("section")!,
    );
    // The binding German texts come with the offer; no consent is asked twice.
    expect(
      consents.getByLabelText(`${HOLIDAY_CONSENT_TEXTS.privacyConsent} *`),
    ).toBeChecked();
    expect(consents.getAllByRole("checkbox")).toHaveLength(3);
    expect(
      screen.getByText(HOLIDAY_CONSENT_TEXTS.declarationName),
    ).toBeInTheDocument();
    for (const word of [/Unterschrift/i, /unterschreib/i, /schriftlich/i]) {
      expect(document.body.textContent).not.toMatch(word);
    }

    const custody = screen.getByLabelText(
      `${HOLIDAY_CONSENT_TEXTS.custodyConfirmed} *`,
    );
    fireEvent.click(custody);
    const name = screen.getByLabelText("Ihr Vor- und Nachname");
    type(name, "  ");
    fireEvent.click(
      screen.getByRole("button", { name: "Weiter zur Zusammenfassung" }),
    );
    expect(
      await screen.findByText(
        "Bitte bestätigen Sie, dass Sie die Anmeldung abgeben dürfen.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Bitte geben Sie Ihren Namen ein."),
    ).toBeInTheDocument();
    expect(custody).toHaveAttribute("aria-invalid", "true");
    expect(name).toHaveAttribute("aria-invalid", "true");
    expect(mockSubmit).not.toHaveBeenCalled();

    fireEvent.click(custody);
    type(name, " Eva Maria Muster ");
    fireEvent.click(
      screen.getByRole("button", { name: "Weiter zur Zusammenfassung" }),
    );
    const declaration = within(
      (await screen.findByRole("heading", { name: "Ihre Erklärung" })).closest(
        "section",
      )!,
    );
    expect(declaration.getByText("Eva Maria Muster")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Solve captcha" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Verbindlich anmelden" }),
    );
    await waitFor(() => expect(mockSubmit).toHaveBeenCalledTimes(1));
    expect(mockSubmit).toHaveBeenCalledWith(
      "link-key",
      expect.objectContaining({
        privacy_consent: true,
        binding_confirmed: true,
        custody_confirmed: true,
        declaration_name: "Eva Maria Muster",
        consent_version: 4,
        retention_days: 365,
      }),
    );
  });

  it("shows the field of a declaration the server refuses (#5626)", async () => {
    mockSubmit.mockRejectedValue(
      new ApiError("refused", 400, {
        code: "enrollment.holiday_registration_invalid",
        errors: [{ field: "declaration_name", reason: "is required" }],
      }),
    );
    await renderFilledForm();
    fireEvent.click(screen.getByRole("button", { name: "Solve captcha" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Verbindlich anmelden" }),
    );
    const name = await screen.findByLabelText("Ihr Vor- und Nachname");
    await waitFor(() => expect(name).toHaveAttribute("aria-invalid", "true"));
  });

  it("asks the criteria as required yes/no questions and names the open place (#4700)", async () => {
    mockSubmit.mockResolvedValue([
      { childPosition: 1, weekId: "11", placement: "pending" },
    ]);
    renderForm(null, {
      ...offer,
      allocationMode: "criteria",
      criteria: [
        {
          id: "1",
          question: "Sind beide Eltern berufstätig?",
          scope: "family",
        },
        {
          id: "2",
          question: "Ist ein Geschwisterkind angemeldet?",
          scope: "child",
        },
      ],
    });
    expect(
      screen.getByText(
        "Das Büro vergibt die Plätze nach festen Regeln. Ihre Angaben helfen dabei. Das Büro kann nach einem Nachweis fragen.",
      ),
    ).toBeInTheDocument();
    fillRegistration();
    fireEvent.click(
      screen.getByRole("button", { name: "Weiter zur Zusammenfassung" }),
    );
    expect(
      await screen.findAllByText("Bitte wählen Sie Ja oder Nein."),
    ).toHaveLength(2);
    expect(screen.getByText("Für Ben Muster")).toBeInTheDocument();

    const family = screen.getByRole("group", {
      name: /Sind beide Eltern berufstätig\?/,
    });
    fireEvent.click(within(family).getByRole("radio", { name: "Ja" }));
    const sibling = screen.getByRole("group", {
      name: /Ist ein Geschwisterkind angemeldet\?/,
    });
    fireEvent.click(within(sibling).getByRole("radio", { name: "Nein" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Weiter zur Zusammenfassung" }),
    );
    expect(
      await screen.findByText("Bitte prüfen Sie Ihre Angaben"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Sind beide Eltern berufstätig?"),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Solve captcha" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Verbindlich anmelden" }),
    );
    await waitFor(() => expect(mockSubmit).toHaveBeenCalledTimes(1));
    expect(mockSubmit).toHaveBeenCalledWith(
      "link-key",
      expect.objectContaining({
        criteria: { "1": true },
        children: [expect.objectContaining({ criteria: { "2": false } })],
      }),
    );
    expect(await screen.findByText("Platz offen")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Das Büro vergibt die Plätze. Sie bekommen eine E-Mail.",
      ),
    ).toBeInTheDocument();
  });

  it.each([
    { scope: "family" as const, field: "criteria.2" },
    { scope: "child" as const, field: "children.0.criteria.2" },
  ])(
    "refreshes a missing $scope criterion and keeps the draft",
    async ({ scope, field }) => {
      const shown: HolidayRegistrationOffer = {
        ...offer,
        allocationMode: "criteria",
        criteria: [
          {
            id: "1",
            question: "Sind beide Eltern berufstätig?",
            scope: "family",
          },
        ],
      };
      const refreshed: HolidayRegistrationOffer = {
        ...shown,
        criteria: [
          ...shown.criteria!,
          { id: "2", question: "Ist ein Geschwisterkind angemeldet?", scope },
        ],
      };
      mockSubmit
        .mockRejectedValueOnce(
          new ApiError("missing criterion", 400, {
            code: "enrollment.holiday_registration_invalid",
            errors: [{ field, reason: "answer yes or no" }],
          }),
        )
        .mockResolvedValue([]);
      const reload = vi.fn(async () => {
        view.rerender(
          <NextIntlClientProvider locale="de" messages={deMessages}>
            <ToastProvider>
              <HolidayRegistrationForm
                token="link-key"
                offer={refreshed}
                onFormChanged={reload}
              />
            </ToastProvider>
          </NextIntlClientProvider>,
        );
      });
      const view = renderForm(null, shown, reload);
      fillRegistration();
      fireEvent.click(
        within(
          screen.getByRole("group", { name: /Sind beide Eltern berufstätig/ }),
        ).getByRole("radio", { name: "Ja" }),
      );
      fireEvent.click(
        screen.getByRole("button", { name: "Weiter zur Zusammenfassung" }),
      );
      await screen.findByText("Bitte prüfen Sie Ihre Angaben");
      fireEvent.click(screen.getByRole("button", { name: "Solve captcha" }));
      fireEvent.click(
        screen.getByRole("button", { name: "Verbindlich anmelden" }),
      );

      const question = await screen.findByRole("group", {
        name: /Ist ein Geschwisterkind angemeldet/,
      });
      expect(reload).toHaveBeenCalledTimes(1);
      expect(screen.getAllByLabelText("Vorname")[0]).toHaveValue("Ben");
      expect(
        within(
          screen.getByRole("group", { name: /Sind beide Eltern berufstätig/ }),
        ).getByRole("radio", { name: "Ja" }),
      ).toBeChecked();
      fireEvent.click(within(question).getByRole("radio", { name: "Nein" }));
      fireEvent.click(
        screen.getByRole("button", { name: "Weiter zur Zusammenfassung" }),
      );
      await screen.findByText("Bitte prüfen Sie Ihre Angaben");
      fireEvent.click(screen.getByRole("button", { name: "Solve captcha" }));
      fireEvent.click(
        screen.getByRole("button", { name: "Verbindlich anmelden" }),
      );
      await screen.findByText("Ihre Anmeldung ist eingegangen");
      expect(mockSubmit).toHaveBeenCalledTimes(2);
      expect(mockSubmit.mock.calls[1]?.[1]).toMatchObject({
        criteria:
          scope === "family" ? { "1": true, "2": false } : { "1": true },
        children: [
          expect.objectContaining({
            criteria: scope === "child" ? { "2": false } : {},
          }),
        ],
      });
    },
  );

  it("registers a holiday guest after the summary", async () => {
    mockSubmit.mockResolvedValue(undefined);
    const onStepChange = vi.fn();
    renderForm(null, offer, undefined, onStepChange);
    expect(onStepChange).toHaveBeenLastCalledWith("form");
    fillRegistration();
    fireEvent.click(
      screen.getByRole("button", { name: "Weiter zur Zusammenfassung" }),
    );

    expect(
      await screen.findByText("Bitte prüfen Sie Ihre Angaben"),
    ).toBeInTheDocument();
    // The page shows the required-field hint only on the form step (#5334).
    expect(onStepChange).toHaveBeenLastCalledWith("summary");
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
    expect(onStepChange).toHaveBeenLastCalledWith("done");
  });

  it("shows the contribution in the summary and sends the statement on benefits", async () => {
    mockSubmit.mockResolvedValue(undefined);
    mockQuote.mockImplementation((_token, _children, declared: boolean) =>
      Promise.resolve(quoted(declared)),
    );
    renderForm();
    fillRegistration();
    fireEvent.click(
      screen.getByRole("button", { name: "Weiter zur Zusammenfassung" }),
    );

    expect(
      await screen.findByText("04.01.2027 bis 08.01.2027 · bis 13 Uhr"),
    ).toBeInTheDocument();
    expect(mockQuote).toHaveBeenLastCalledWith(
      "link-key",
      [[{ week_id: 11, time_model_id: 21 }]],
      false,
    );
    expect(screen.getAllByText("45,00 €").length).toBeGreaterThan(0);
    fireEvent.click(
      screen.getByRole("checkbox", {
        name: /Wir bekommen Leistungen, zum Beispiel Bürgergeld/,
      }),
    );
    // The old amount goes at once; the new one follows.
    expect(
      screen.getByText("Der Beitrag wird berechnet …"),
    ).toBeInTheDocument();
    expect(
      await screen.findByText(
        "Befreit wegen Leistungsbezug: Beitragsfrei mit BuT-Nachweis",
      ),
    ).toBeInTheDocument();
    expect(mockQuote).toHaveBeenLastCalledWith(
      "link-key",
      [[{ week_id: 11, time_model_id: 21 }]],
      true,
    );

    fireEvent.click(screen.getByRole("button", { name: "Solve captcha" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Verbindlich anmelden" }),
    );
    await waitFor(() => expect(mockSubmit).toHaveBeenCalledTimes(1));
    expect(mockSubmit).toHaveBeenCalledWith(
      "link-key",
      expect.objectContaining({ benefits_declared: true }),
    );
  });

  it("uploads a proof of benefits in the summary and sends its token (#5238)", async () => {
    mockSubmit.mockResolvedValue(undefined);
    mockQuote.mockImplementation((_token, _children, declared: boolean) =>
      Promise.resolve(quoted(declared)),
    );
    let finishUpload: (value: unknown) => void = () => undefined;
    mockUpload.mockReturnValue(
      new Promise((resolve) => {
        finishUpload = resolve;
      }),
    );
    await renderFilledForm();
    const benefits = await screen.findByRole("checkbox", {
      name: /Wir bekommen Leistungen, zum Beispiel Bürgergeld/,
    });
    expect(benefits).not.toBeChecked();
    expect(
      screen.queryByText("Nachweis über die Leistungen"),
    ).not.toBeInTheDocument();

    fireEvent.click(benefits);
    expect(
      screen.getByText(/Freiwillig\. Laden Sie hier zum Beispiel/),
    ).toBeInTheDocument();
    const file = new File(["%PDF-1.4"], "bescheid.pdf", {
      type: "application/pdf",
    });
    fireEvent.change(screen.getByLabelText("Nachweis über die Leistungen"), {
      target: { files: [file] },
    });
    expect(mockUpload).toHaveBeenCalledWith(
      "link-key",
      file,
      expect.any(AbortSignal),
    );

    fireEvent.click(screen.getByRole("button", { name: "Solve captcha" }));
    // The send button waits until the file has arrived.
    expect(
      screen.getByText("Bitte warten Sie, bis alle Dateien hochgeladen sind."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Verbindlich anmelden" }),
    ).toBeDisabled();
    finishUpload({
      token: "proof-token",
      filename: "bescheid.pdf",
      size_bytes: 2048,
      content_type: "application/pdf",
    });
    expect(
      await screen.findByRole("button", { name: "bescheid.pdf entfernen" }),
    ).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Verbindlich anmelden" }),
    );
    await waitFor(() => expect(mockSubmit).toHaveBeenCalledTimes(1));
    expect(mockSubmit).toHaveBeenCalledWith(
      "link-key",
      expect.objectContaining({
        benefits_declared: true,
        benefits_proof_tokens: ["proof-token"],
      }),
    );
  });

  it("sends no proof once the statement on benefits is taken back (#5238)", async () => {
    mockSubmit.mockResolvedValue(undefined);
    mockQuote.mockImplementation((_token, _children, declared: boolean) =>
      Promise.resolve(quoted(declared)),
    );
    mockUpload.mockResolvedValue({
      token: "proof-token",
      filename: "bescheid.pdf",
      size_bytes: 2048,
      content_type: "application/pdf",
    });
    await renderFilledForm();
    const benefits = await screen.findByRole("checkbox", {
      name: /Wir bekommen Leistungen, zum Beispiel Bürgergeld/,
    });
    fireEvent.click(benefits);
    fireEvent.change(screen.getByLabelText("Nachweis über die Leistungen"), {
      target: {
        files: [
          new File(["%PDF-1.4"], "bescheid.pdf", { type: "application/pdf" }),
        ],
      },
    });
    expect(
      await screen.findByRole("button", { name: "bescheid.pdf entfernen" }),
    ).toBeInTheDocument();
    fireEvent.click(benefits);

    fireEvent.click(screen.getByRole("button", { name: "Solve captcha" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Verbindlich anmelden" }),
    );
    await waitFor(() => expect(mockSubmit).toHaveBeenCalledTimes(1));
    expect(mockSubmit).toHaveBeenCalledWith(
      "link-key",
      expect.objectContaining({
        benefits_declared: false,
        benefits_proof_tokens: [],
      }),
    );
  });

  it("shows the price per week at each time model, as the base line of the contribution", () => {
    renderForm();
    expect(
      screen.getByText(
        /Der Preis gilt pro Woche und Kind, ohne Ermäßigung oder Befreiung\. Ihren Beitrag sehen Sie in der Zusammenfassung\./,
      ),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("combobox", { name: /04\.01\.2027 bis 08\.01\.2027/ }),
    );
    // 45,00 € is the base line quoted(false) shows under "Beitrag".
    expect(
      screen.getByRole("option", { name: WEEK_TIME_MODEL }),
    ).toBeInTheDocument();
  });

  it("offers parallel weeks as one choice with the school (#5689)", () => {
    const firstWeek = offer.weeks[0]!;
    renderForm(null, {
      ...offer,
      weeks: [
        firstWeek,
        {
          ...firstWeek,
          id: "12",
          hostSchoolName: "OGS Süd",
          timeModels: [
            { ...firstWeek.timeModels[0]!, id: "22", name: "bis 16 Uhr" },
          ],
        },
      ],
    });

    expect(
      screen.getByText(
        "Diese Woche gibt es an mehreren Schulen. Wählen Sie eine Schule und die Zeit.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("combobox", { name: /04\.01\.2027 bis 08\.01\.2027/ }),
    );
    expect(
      screen.getByRole("option", { name: /^OGS Nord: bis 13 Uhr/ }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("option", { name: /^OGS Süd: bis 16 Uhr/ }),
    ).toBeInTheDocument();
  });

  it("offers no time model of a week without care at its school (#4702)", () => {
    const firstWeek = offer.weeks[0]!;
    renderForm(null, {
      ...offer,
      weeks: [
        {
          ...firstWeek,
          noCareDays: [{ date: "2027-01-06", name: "Heilige Drei Könige" }],
        },
        {
          ...firstWeek,
          id: "12",
          hostSchoolName: "OGS Süd",
          timeModels: [
            { ...firstWeek.timeModels[0]!, id: "22", name: "bis 16 Uhr" },
          ],
        },
      ],
    });

    expect(
      screen.getByText(/^OGS Nord: .*06\.01\.2027.*Heilige Drei Könige/),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("combobox", { name: /04\.01\.2027 bis 08\.01\.2027/ }),
    );
    expect(
      screen.getByRole("option", { name: /^OGS Süd: bis 16 Uhr/ }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("option", { name: /^OGS Nord: bis 13 Uhr/ }),
    ).not.toBeInTheDocument();
  });

  it("lets parents send the registration when the contribution cannot be shown", async () => {
    mockQuote.mockRejectedValue(new Error("offline"));
    renderForm();
    fillRegistration();
    fireEvent.click(
      screen.getByRole("button", { name: "Weiter zur Zusammenfassung" }),
    );

    expect(
      await screen.findByText(
        "Der Beitrag lässt sich gerade nicht anzeigen. Sie können trotzdem absenden. Der Träger nennt Ihnen dann den Beitrag.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Solve captcha" }));
    expect(
      screen.getByRole("button", { name: "Verbindlich anmelden" }),
    ).toBeEnabled();
  });

  it("prefills a known child that the parent corrects for this registration only", async () => {
    mockSubmit.mockResolvedValue(undefined);
    renderForm(family);

    expect(screen.getByLabelText("E-Mail-Adresse")).toHaveValue(
      "sabine@example.org",
    );
    const known = screen.getByRole("checkbox", { name: /Felix Schneider/ });
    expect(
      screen.getByText("OGS Nord · geboren am 01.03.2018"),
    ).toBeInTheDocument();
    fireEvent.click(known);
    expect(known).toBeChecked();
    const childFirst = screen.getAllByLabelText("Vorname")[0]!;
    expect(childFirst).toHaveValue("Felix");
    expect(screen.getAllByText("Kind 1")).toHaveLength(1);
    type(childFirst, "Feli");

    choose(
      screen.getByRole("combobox", { name: /04\.01\.2027 bis 08\.01\.2027/ }),
      WEEK_TIME_MODEL,
    );
    type(screen.getAllByLabelText("Name")[0]!, "Oma Schneider");
    type(screen.getAllByLabelText("Telefon")[1]!, "05402 2");
    fireEvent.click(
      screen.getByLabelText(/Ich bin einverstanden, dass der Träger/),
    );
    fireEvent.click(
      screen.getByLabelText("Ich weiß, dass die Anmeldung verbindlich ist. *"),
    );
    fireEvent.click(
      screen.getByLabelText(/Ich darf die Anmeldung allein abgeben/),
    );
    type(screen.getByLabelText("Ihr Vor- und Nachname"), "Sabine Schneider");
    fireEvent.click(
      screen.getByRole("button", { name: "Weiter zur Zusammenfassung" }),
    );
    await screen.findByText("Bitte prüfen Sie Ihre Angaben");
    fireEvent.click(screen.getByRole("button", { name: "Solve captcha" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Verbindlich anmelden" }),
    );

    await waitFor(() => expect(mockSubmit).toHaveBeenCalledTimes(1));
    const payload = mockSubmit.mock.calls[0]![1] as {
      guardian: Record<string, string>;
      children: Record<string, unknown>[];
    };
    expect(payload.guardian).toEqual({
      first_name: "Sabine",
      last_name: "Schneider",
      email: "sabine@example.org",
      phone: "0151 2",
    });
    expect(payload.children).toHaveLength(1);
    expect(payload.children[0]).toEqual(
      expect.objectContaining({
        first_name: "Feli",
        last_name: "Schneider",
        birth_date: "2018-03-01",
        home_school_id: 31,
      }),
    );
    // The registration is a copy: no school record is named in it.
    expect(JSON.stringify(payload)).not.toContain("41");
  });

  it("drops a known child again when the parent unpicks it", () => {
    renderForm(family);
    const known = screen.getByRole("checkbox", { name: /Felix Schneider/ });
    fireEvent.click(known);
    expect(screen.getAllByLabelText("Vorname")[0]).toHaveValue("Felix");
    fireEvent.click(known);
    expect(known).not.toBeChecked();
    expect(screen.getAllByLabelText("Vorname")[0]).toHaveValue("");
  });

  it("offers no known children without a parent account", () => {
    renderForm();
    expect(screen.getByText("Kind 1")).toBeInTheDocument();
    expect(
      screen.queryByText("Ihre Kinder aus dem Elternkonto"),
    ).not.toBeInTheDocument();
  });
});

describe("second guardian card", () => {
  beforeEach(() => {
    mockSubmit.mockReset();
    mockQuote.mockReset();
    mockQuote.mockResolvedValue(quoted(false));
    window.scrollTo = vi.fn();
  });

  function card() {
    const heading = screen.getByRole("heading", {
      name: "Zweite sorgeberechtigte Person",
    });
    return within(heading.closest("section")!);
  }

  it("requires names only while the optional card contains information", () => {
    renderForm();
    const second = card();
    const firstName = second.getByLabelText("Vorname");
    const lastName = second.getByLabelText("Nachname");
    expect(firstName).not.toBeRequired();
    expect(lastName).not.toBeRequired();
    const phone = second.getByLabelText("Telefon (freiwillig)");
    type(phone, "0151 2345678");
    expect(second.getByLabelText("Vorname")).toBeRequired();
    expect(second.getByLabelText("Nachname")).toBeRequired();
    type(phone, " ");
    expect(firstName).not.toBeRequired();
    expect(lastName).not.toBeRequired();
  });

  it("allows leaving the card blank", async () => {
    mockSubmit.mockResolvedValue(undefined);
    await renderFilledForm();
    expect(screen.getByText("Eva Muster")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Solve captcha" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Verbindlich anmelden" }),
    );
    await waitFor(() => expect(mockSubmit).toHaveBeenCalled());
    expect(mockSubmit.mock.calls[0]![1].second_guardian).toBeUndefined();
  });

  it("shows the second guardian in the summary and submits all four fields", async () => {
    mockSubmit.mockResolvedValue(undefined);
    renderForm();
    fillRegistration();
    const second = card();
    type(second.getByLabelText(/^Vorname\b/), "Tom");
    type(second.getByLabelText(/^Nachname\b/), "Anders");
    type(
      second.getByLabelText("E-Mail-Adresse (freiwillig)"),
      "tom@example.org",
    );
    type(second.getByLabelText("Telefon (freiwillig)"), "0151 2345678");
    fireEvent.click(
      screen.getByRole("button", { name: "Weiter zur Zusammenfassung" }),
    );
    await screen.findByText("Bitte prüfen Sie Ihre Angaben");
    expect(card().getByText("Tom Anders")).toBeInTheDocument();
    expect(card().getByText("tom@example.org")).toBeInTheDocument();
    expect(card().getByText("0151 2345678")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Solve captcha" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Verbindlich anmelden" }),
    );
    await waitFor(() => expect(mockSubmit).toHaveBeenCalled());
    expect(mockSubmit.mock.calls[0]![1].second_guardian).toEqual({
      first_name: "Tom",
      last_name: "Anders",
      email: "tom@example.org",
      phone: "0151 2345678",
    });
  });

  it.each([
    [/^Vorname\b/, "", /^Vorname\b/],
    [/^Nachname\b/, "", /^Nachname\b/],
    ["E-Mail-Adresse (freiwillig)", "bad", "E-Mail-Adresse (freiwillig)"],
    ["Telefon (freiwillig)", "abc", "Telefon (freiwillig)"],
  ])(
    "shows errors at the second guardian field %s",
    async (label, value, invalidLabel) => {
      renderForm();
      fillRegistration();
      const second = card();
      type(second.getByLabelText(/^Vorname\b/), "Tom");
      type(second.getByLabelText(/^Nachname\b/), "Anders");
      type(second.getByLabelText(label), value);
      fireEvent.click(
        screen.getByRole("button", { name: "Weiter zur Zusammenfassung" }),
      );
      await waitFor(() =>
        expect(second.getByLabelText(invalidLabel)).toHaveAttribute(
          "aria-invalid",
          "true",
        ),
      );
      expect(second.getByLabelText(invalidLabel)).toHaveAttribute(
        "aria-describedby",
      );
    },
  );
});

describe("address card (#5625)", () => {
  beforeEach(() => {
    mockSubmit.mockReset();
    mockQuote.mockReset();
    mockQuote.mockResolvedValue(quoted(false));
    window.scrollTo = vi.fn();
  });

  const asking = (
    addressRequirement: "off" | "optional" | "required",
  ): HolidayRegistrationOffer => ({
    ...offer,
    form: { ...offer.form, addressRequirement },
  });

  function addressCard(title: string) {
    const heading = screen.getByRole("heading", { name: title });
    return within(heading.closest("section")!);
  }

  async function send() {
    fireEvent.click(screen.getByRole("button", { name: "Solve captcha" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Verbindlich anmelden" }),
    );
    await waitFor(() => expect(mockSubmit).toHaveBeenCalled());
    return mockSubmit.mock.calls[0]![1] as Record<string, unknown>;
  }

  it("shows no address card when the form does not ask", () => {
    renderForm(null, asking("off"));
    expect(
      screen.getByRole("heading", { name: "Ihre Angaben" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: /^Anschrift/ }),
    ).not.toBeInTheDocument();
  });

  it("lets parents leave an optional address blank", async () => {
    mockSubmit.mockResolvedValue(undefined);
    renderForm(null, asking("optional"));
    const card = addressCard("Anschrift (freiwillig)");
    expect(card.getByLabelText("Straße und Hausnummer")).not.toBeRequired();
    expect(card.getByLabelText("PLZ")).not.toBeRequired();
    expect(card.getByLabelText("Ort")).not.toBeRequired();
    fillRegistration();
    fireEvent.click(
      screen.getByRole("button", { name: "Weiter zur Zusammenfassung" }),
    );
    await screen.findByText("Bitte prüfen Sie Ihre Angaben");
    expect((await send()).guardian_address).toBeUndefined();
  });

  it("marks every part of a required address and a short postal code", async () => {
    renderForm(null, asking("required"));
    fillRegistration();
    const card = addressCard("Anschrift");
    fireEvent.click(
      screen.getByRole("button", { name: "Weiter zur Zusammenfassung" }),
    );
    for (const label of ["Straße und Hausnummer", "PLZ", "Ort"]) {
      await waitFor(() =>
        expect(card.getByLabelText(label)).toHaveAttribute(
          "aria-invalid",
          "true",
        ),
      );
    }
    type(card.getByLabelText("Straße und Hausnummer"), "Musterweg 12");
    type(card.getByLabelText("PLZ"), "4914");
    type(card.getByLabelText("Ort"), "Bissendorf");
    fireEvent.click(
      screen.getByRole("button", { name: "Weiter zur Zusammenfassung" }),
    );
    expect(
      await card.findByText("Bitte geben Sie eine PLZ mit 5 Ziffern ein."),
    ).toBeInTheDocument();
    expect(card.getByLabelText("PLZ")).toHaveAttribute("aria-invalid", "true");
  });

  it("shows the address in the summary and sends it", async () => {
    mockSubmit.mockResolvedValue(undefined);
    renderForm(null, asking("required"));
    fillRegistration();
    const card = addressCard("Anschrift");
    expect(card.getByLabelText("PLZ")).toBeRequired();
    type(card.getByLabelText("Straße und Hausnummer"), " Musterweg 12 ");
    type(card.getByLabelText("PLZ"), "01234");
    type(card.getByLabelText("Ort"), "Bissendorf");
    fireEvent.click(
      screen.getByRole("button", { name: "Weiter zur Zusammenfassung" }),
    );
    await screen.findByText("Bitte prüfen Sie Ihre Angaben");
    expect(
      screen.getByText("Musterweg 12, 01234 Bissendorf"),
    ).toBeInTheDocument();
    expect((await send()).guardian_address).toEqual({
      street: "Musterweg 12",
      postal_code: "01234",
      city: "Bissendorf",
    });
  });

  it("puts the refusal of the server at the address field", async () => {
    mockSubmit.mockRejectedValue(
      new ApiError("refused", 400, {
        code: "enrollment.holiday_registration_invalid",
        errors: [
          { field: "guardian_address.postal_code", reason: "five digits" },
        ],
      }),
    );
    renderForm(null, asking("optional"));
    fillRegistration();
    const card = addressCard("Anschrift (freiwillig)");
    type(card.getByLabelText("Straße und Hausnummer"), "Musterweg 12");
    type(card.getByLabelText("PLZ"), "49143");
    type(card.getByLabelText("Ort"), "Bissendorf");
    fireEvent.click(
      screen.getByRole("button", { name: "Weiter zur Zusammenfassung" }),
    );
    await screen.findByText("Bitte prüfen Sie Ihre Angaben");
    await send();
    await waitFor(() =>
      expect(
        addressCard("Anschrift (freiwillig)").getByLabelText("PLZ"),
      ).toHaveAttribute("aria-invalid", "true"),
    );
  });
});

/** The Träger's questions (#5198): per child and per family. */
const offerWithQuestions: HolidayRegistrationOffer = {
  ...offer,
  form: {
    version: 3,
    fields: [
      {
        key: "ausfluege",
        label: "Darf an Ausflügen teilnehmen",
        type: "boolean",
        required: true,
        applies_to_child: true,
        sort_order: 0,
      },
      {
        key: "ausflug_hinweis",
        label: "Was müssen wir bei Ausflügen beachten?",
        type: "text",
        required: true,
        applies_to_child: true,
        sort_order: 1,
        visible_when: {
          source: "field",
          field: "ausfluege",
          operator: "eq",
          value: true,
        },
      },
      {
        key: "tetanus",
        label: "Tetanus-Impfschutz",
        type: "select",
        applies_to_child: true,
        health: true,
        sort_order: 2,
        options: [
          { label: "ja", value: "ja" },
          { label: "nein", value: "nein" },
        ],
      },
      {
        key: "newsletter",
        label: "Wie haben Sie von den Ferien erfahren?",
        type: "text",
        required: true,
        sort_order: 3,
      },
    ],
  },
};

describe("HolidayRegistrationForm questions of the Träger (#5198)", () => {
  beforeEach(() => {
    mockSubmit.mockReset();
    mockQuote.mockReset();
    mockQuote.mockResolvedValue(quoted(false));
    window.scrollTo = vi.fn();
  });

  it("marks a missing required answer and skips a hidden required question", async () => {
    renderForm(null, offerWithQuestions);
    fillRegistration();
    expect(screen.getByText("Fragen und Infos zum Kind")).toBeInTheDocument();
    expect(screen.getByText("Allgemeine Fragen und Infos")).toBeInTheDocument();
    // The follow-up question waits for its condition.
    expect(
      screen.queryByText("Was müssen wir bei Ausflügen beachten?"),
    ).not.toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Weiter zur Zusammenfassung" }),
    );

    expect(
      await screen.findByText("Bitte wählen Sie Ja oder Nein."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("textbox", {
        name: /Wie haben Sie von den Ferien erfahren/,
      }),
    ).toHaveAttribute("aria-invalid", "true");
    expect(mockSubmit).not.toHaveBeenCalled();
  });

  it("names the general card for an information text, not only for questions (#5467)", () => {
    renderForm(null, {
      ...offer,
      form: {
        version: 4,
        fields: [
          {
            key: "infotext_1",
            label: "Bitte Sonnencreme mitgeben",
            type: "information",
            content: "Wir sind viel draußen.",
            sort_order: 0,
          },
        ],
      },
    });
    expect(screen.getByText("Bitte Sonnencreme mitgeben")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Allgemeine Fragen und Infos" }),
    ).toBeInTheDocument();
  });

  it("asks for the health consent when a health question is answered and sends the answers", async () => {
    mockSubmit.mockResolvedValue(undefined);
    renderForm(null, offerWithQuestions);
    fillRegistration();
    fireEvent.click(screen.getByRole("radio", { name: "Nein" }));
    choose(screen.getByRole("combobox", { name: "Tetanus-Impfschutz" }), "ja");
    type(
      screen.getByRole("textbox", {
        name: /Wie haben Sie von den Ferien erfahren/,
      }),
      " Aus der Zeitung ",
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Weiter zur Zusammenfassung" }),
    );
    expect(
      await screen.findByText(
        "Bitte stimmen Sie zu oder löschen Sie die Gesundheitsangaben.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByLabelText(
        /^Ich bin einverstanden: Der Träger und die OGS sehen die Gesundheitsangaben meiner Kinder\./,
      ),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Weiter zur Zusammenfassung" }),
    );

    await screen.findByText("Bitte prüfen Sie Ihre Angaben");
    // The summary repeats the answers with the question.
    expect(
      screen.getByText("Darf an Ausflügen teilnehmen"),
    ).toBeInTheDocument();
    expect(screen.getByText("Tetanus-Impfschutz")).toBeInTheDocument();
    expect(screen.getByText("Aus der Zeitung")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Solve captcha" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Verbindlich anmelden" }),
    );

    await waitFor(() => expect(mockSubmit).toHaveBeenCalledTimes(1));
    expect(mockSubmit).toHaveBeenCalledWith(
      "link-key",
      expect.objectContaining({
        form_version: 3,
        consent_version: 4,
        retention_days: 365,
        custom_data: { newsletter: "Aus der Zeitung" },
        health_consent: true,
        children: [
          expect.objectContaining({
            custom_data: { ausfluege: false, tetanus: "ja" },
          }),
        ],
      }),
    );
  });

  it("loads the calendar of a date question when the form has one", async () => {
    renderForm(null, {
      ...offer,
      form: {
        version: 1,
        fields: [
          {
            key: "letzte_impfung",
            label: "Datum der letzten Tetanus-Impfung",
            type: "date",
            applies_to_child: true,
            sort_order: 0,
          },
        ],
      },
    });
    expect(
      await screen.findByText("Datum der letzten Tetanus-Impfung"),
    ).toBeInTheDocument();
    expect(
      await screen.findByLabelText("Datum der letzten Tetanus-Impfung"),
    ).toBeInTheDocument();
  });

  it("loads the form again when the Träger changed it and keeps the answers", async () => {
    mockSubmit.mockRejectedValue(
      new ApiError("form changed", 409, {
        code: "enrollment.holiday_registration_form_changed",
      }),
    );
    const reload = vi.fn().mockResolvedValue(undefined);
    renderForm(null, offerWithQuestions, reload);
    fillRegistration();
    fireEvent.click(screen.getByRole("radio", { name: "Nein" }));
    type(
      screen.getByRole("textbox", {
        name: /Wie haben Sie von den Ferien erfahren/,
      }),
      "Schule",
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Weiter zur Zusammenfassung" }),
    );
    await screen.findByText("Bitte prüfen Sie Ihre Angaben");
    fireEvent.click(screen.getByRole("button", { name: "Solve captcha" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Verbindlich anmelden" }),
    );

    await waitFor(() => expect(reload).toHaveBeenCalledTimes(1));
    expect(
      await screen.findByText(
        "Der Träger hat das Formular gerade geändert. Bitte prüfen Sie Ihre Angaben und senden Sie die Anmeldung noch einmal.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("textbox", {
        name: /Wie haben Sie von den Ferien erfahren/,
      }),
    ).toHaveValue("Schule");
    // The privacy text may have changed with the form (#6691): the parent
    // agrees again to the text the form shows now.
    expect(
      screen.getByLabelText(/Ich bin einverstanden, dass der Träger/),
    ).not.toBeChecked();
  });
  it("requires PDF acknowledgements and shows the accepted and declined blocks in the summary", async () => {
    const shown = {
      ...offer,
      form: {
        ...offer.form,
        legalBlocks: [
          {
            key: "hinweis",
            kind: "notice" as const,
            source: "custom" as const,
            title: "Ferienhinweise",
            label: "Ich habe die Ferienhinweise gelesen.",
            text: "Bitte eine Trinkflasche mitgeben.",
            enabled: true,
            required: true,
            sort_order: 0,
            attachments: [{ label: "Ferienbrief", document_url: "19" }],
          },
          {
            key: "foto",
            kind: "consent" as const,
            source: "custom" as const,
            title: "Erinnerungsfotos",
            label: "Fotos sind erlaubt.",
            text: "",
            enabled: true,
            required: false,
            sort_order: 1,
          },
        ],
      },
    };
    renderForm(null, shown);
    fillRegistration();
    fireEvent.click(
      screen.getByRole("button", { name: "Weiter zur Zusammenfassung" }),
    );
    expect(
      await screen.findByText("Bitte prüfen Sie die markierten Felder."),
    ).toBeInTheDocument();
    const acknowledgement = screen.getByRole("checkbox", {
      name: /Ich habe die Ferienhinweise gelesen/,
    });
    expect(acknowledgement).toHaveAttribute("aria-invalid", "true");
    fireEvent.click(
      screen.getByRole("button", {
        name: "Details zu Ferienhinweise anzeigen",
      }),
    );
    expect(
      await screen.findByText("Bitte eine Trinkflasche mitgeben."),
    ).toBeInTheDocument();
    expect(acknowledgement).not.toBeChecked();
    fireEvent.click(screen.getByRole("button", { name: "Modal schließen" }));
    fireEvent.click(acknowledgement);
    fireEvent.click(
      screen.getByRole("button", { name: "Weiter zur Zusammenfassung" }),
    );
    expect(
      await screen.findByText("Bitte prüfen Sie Ihre Angaben"),
    ).toBeInTheDocument();
    expect(screen.getByText("Ferienhinweise")).toBeInTheDocument();
    expect(screen.getByText("Bestätigt")).toBeInTheDocument();
    expect(screen.getByText("Nicht bestätigt")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Solve captcha" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Verbindlich anmelden" }),
    );
    await waitFor(() =>
      expect(mockSubmit).toHaveBeenCalledWith(
        "link-key",
        expect.objectContaining({
          legal_acceptances: { hinweis: true, foto: false },
        }),
      ),
    );
  });
  it("requires fresh legal acknowledgement after the version changes", async () => {
    mockSubmit.mockRejectedValueOnce(
      new ApiError("changed", 409, {
        code: "enrollment.holiday_registration_form_changed",
      }),
    );
    const reload = vi.fn().mockResolvedValue(undefined);
    renderForm(
      null,
      {
        ...offer,
        form: {
          ...offer.form,
          legalBlocks: [
            {
              key: "hinweis",
              kind: "terms",
              source: "custom",
              title: "Ferienhinweise",
              label: "Ich habe die Ferienhinweise gelesen.",
              text: "",
              enabled: true,
              required: true,
              sort_order: 0,
            },
          ],
        },
      },
      reload,
    );
    fillRegistration();
    fireEvent.click(
      screen.getByRole("checkbox", {
        name: /Ich habe die Ferienhinweise gelesen/,
      }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Weiter zur Zusammenfassung" }),
    );
    await screen.findByText("Bitte prüfen Sie Ihre Angaben");
    fireEvent.click(screen.getByRole("button", { name: "Solve captcha" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Verbindlich anmelden" }),
    );
    await waitFor(() => expect(reload).toHaveBeenCalledTimes(1));
    expect(
      await screen.findByRole("checkbox", {
        name: /Ich habe die Ferienhinweise gelesen/,
      }),
    ).not.toBeChecked();
  });
  it("keeps an optional information-only notice read-only", async () => {
    renderForm(null, {
      ...offer,
      form: {
        ...offer.form,
        legalBlocks: [
          {
            key: "info",
            kind: "notice",
            source: "custom",
            title: "Trinkflaschen",
            label: "Bitte eine Trinkflasche mitgeben.",
            text: "",
            enabled: true,
            required: false,
            sort_order: 0,
          },
        ],
      },
    });
    // Anker: the enabled information block is visible in the loaded form.
    expect(
      await screen.findByRole("heading", { name: "Trinkflaschen" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Bitte eine Trinkflasche mitgeben."),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("checkbox", {
        name: "Bitte eine Trinkflasche mitgeben.",
      }),
    ).not.toBeInTheDocument();
    fillRegistration();
    fireEvent.click(
      screen.getByRole("button", { name: "Weiter zur Zusammenfassung" }),
    );
    // Anker: the information-only notice remains visible in the summary.
    expect(await screen.findByText("Trinkflaschen")).toBeInTheDocument();
    expect(
      screen.getByText("Bitte eine Trinkflasche mitgeben."),
    ).toBeInTheDocument();
    expect(screen.queryByText("Nicht bestätigt")).not.toBeInTheDocument();
    expect(screen.queryByText("Bestätigt")).not.toBeInTheDocument();
  });
});

describe("HolidayRegistrationForm parent account (#6673)", () => {
  beforeEach(() => {
    mockSubmit.mockReset();
    mockQuote.mockReset();
    mockQuote.mockResolvedValue(quoted(false));
    window.scrollTo = vi.fn();
  });

  function renderWithAccount(
    parentAccount: "ask" | "signed-in",
    known: HolidayKnownFamily | null = null,
  ) {
    return render(
      <NextIntlClientProvider locale="de" messages={deMessages}>
        <ToastProvider>
          <HolidayRegistrationForm
            token="link-key"
            offer={offer}
            family={known}
            parentAccount={parentAccount}
          />
        </ToastProvider>
      </NextIntlClientProvider>,
    );
  }

  async function toSummary() {
    fillRegistration();
    fireEvent.click(
      screen.getByRole("button", { name: "Weiter zur Zusammenfassung" }),
    );
    await screen.findByText("Bitte prüfen Sie Ihre Angaben");
  }

  const question = () =>
    screen.getByRole("group", { name: /Möchten Sie ein Elternkonto anlegen/ });

  it("asks without a preselection and holds the send until the parent answers", async () => {
    mockSubmit.mockResolvedValue([]);
    renderWithAccount("ask");
    await toSummary();

    const group = question();
    const yes = within(group).getByRole("radio", { name: "Ja" });
    const no = within(group).getByRole("radio", { name: "Nein" });
    expect(yes).not.toBeChecked();
    expect(no).not.toBeChecked();
    expect(
      // The card names the benefit beside its title, above the group.
      screen.getByText(/den Stand der Anmeldung, die Bestätigung/),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Solve captcha" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Verbindlich anmelden" }),
    );

    expect(
      await within(group).findByText("Bitte wählen Sie Ja oder Nein."),
    ).toBeInTheDocument();
    expect(group).toHaveAttribute("aria-invalid", "true");
    expect(yes).toHaveFocus();
    expect(mockSubmit).not.toHaveBeenCalled();

    fireEvent.click(no);
    // Anker: the answer is taken and the hint of the group is gone.
    expect(no).toBeChecked();
    expect(
      within(group).queryByText("Bitte wählen Sie Ja oder Nein."),
    ).not.toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Verbindlich anmelden" }),
    );
    await waitFor(() => expect(mockSubmit).toHaveBeenCalledTimes(1));
    expect(mockSubmit).toHaveBeenCalledWith(
      "link-key",
      expect.objectContaining({ parent_account_requested: false }),
    );
    expect(mockSubmit.mock.calls[0]![1]).not.toHaveProperty(
      "parent_account_proof",
    );
    // Anker: the receipt shows; without „Ja“ it names no second mail.
    expect(
      await screen.findByText("Ihre Anmeldung ist eingegangen"),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Nächster Schritt: Ihr Elternkonto"),
    ).not.toBeInTheDocument();
  });

  it("keeps the answer over Zurück, names the address and announces the second mail", async () => {
    mockSubmit.mockResolvedValue([]);
    renderWithAccount("ask");
    await toSummary();

    fireEvent.click(within(question()).getByRole("radio", { name: "Ja" }));
    expect(
      screen.getByText(
        "Sie bekommen dazu eine E-Mail an eva@example.org. Mit dem Link darin legen Sie Ihr Passwort fest.",
      ),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Zurück und ändern" }));
    fireEvent.click(
      await screen.findByRole("button", { name: "Weiter zur Zusammenfassung" }),
    );
    await screen.findByText("Bitte prüfen Sie Ihre Angaben");
    expect(within(question()).getByRole("radio", { name: "Ja" })).toBeChecked();

    fireEvent.click(screen.getByRole("button", { name: "Solve captcha" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Verbindlich anmelden" }),
    );
    await waitFor(() =>
      expect(mockSubmit).toHaveBeenCalledWith(
        "link-key",
        expect.objectContaining({ parent_account_requested: true }),
      ),
    );
    expect(
      await screen.findByText("Nächster Schritt: Ihr Elternkonto"),
    ).toBeInTheDocument();
    const next = screen.getByText(/Wir haben Ihnen eine zweite E-Mail/);
    expect(next).toHaveTextContent(
      "Wir haben Ihnen eine zweite E-Mail an eva@example.org geschickt. Öffnen Sie den Link darin. Dort legen Sie Ihr Passwort fest oder melden sich an.",
    );
    // The receipt never says whether an account exists (ADR 0216).
    expect(next).not.toHaveTextContent(/schon ein Konto|neues Konto/);
  });

  it("asks a signed-in parent nothing, sends the account's proof and leads back to the app", async () => {
    mockSubmit.mockResolvedValue([]);
    renderWithAccount("signed-in", { ...family, accountProof: "sealed-proof" });
    await toSummary();

    // Anker: the summary with its send button is rendered.
    expect(
      screen.getByRole("button", { name: "Verbindlich anmelden" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("group", { name: /Möchten Sie ein Elternkonto/ }),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Solve captcha" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Verbindlich anmelden" }),
    );
    await waitFor(() =>
      expect(mockSubmit).toHaveBeenCalledWith(
        "link-key",
        expect.objectContaining({
          parent_account_requested: false,
          parent_account_proof: "sealed-proof",
        }),
      ),
    );
    expect(
      await screen.findByRole("link", { name: "Zu Ihren Ferienanmeldungen" }),
    ).toHaveAttribute("href", "/ferienanmeldungen");
  });
});
