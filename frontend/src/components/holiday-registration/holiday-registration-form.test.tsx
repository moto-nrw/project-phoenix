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
import deMessages from "~/i18n/messages/de.json";
import type {
  HolidayKnownFamily,
  HolidayRegistrationOffer,
} from "~/lib/holiday-registration-api";

const { mockSubmit, mockQuote } = vi.hoisted(() => ({
  mockSubmit: vi.fn(),
  mockQuote: vi.fn(),
}));

vi.mock("~/lib/holiday-registration-api", () => ({
  submitHolidayRegistration: (token: string, payload: unknown) =>
    mockSubmit(token, payload),
  quoteHolidayContribution: (
    token: string,
    children: unknown,
    benefitsDeclared: boolean,
  ) => mockQuote(token, children, benefitsDeclared),
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
    },
  ],
};

function renderForm(
  known: HolidayKnownFamily | null = null,
  shown: HolidayRegistrationOffer = offer,
  onFormChanged?: () => Promise<void>,
) {
  return render(
    <NextIntlClientProvider locale="de" messages={deMessages}>
      <ToastProvider>
        <HolidayRegistrationForm
          token="link-key"
          offer={shown}
          family={known}
          onFormChanged={onFormChanged}
        />
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
    screen.getByLabelText("Ich weiß, dass die Anmeldung verbindlich ist. *"),
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
      "bis 13 Uhr (08:00 bis 13:00 Uhr)",
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
    expect(screen.getByText("Weitere Angaben")).toBeInTheDocument();
    expect(screen.getByText("Weitere Angaben zur Familie")).toBeInTheDocument();
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
        /^Ich bin einverstanden, dass die Betreuung die Gesundheitsangaben meiner Kinder sieht\./,
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
  });
});
