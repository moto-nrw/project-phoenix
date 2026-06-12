import { describe, it, expect, vi, beforeEach } from "vitest";
import {
  render,
  screen,
  fireEvent,
  waitFor,
  within,
} from "@testing-library/react";
import "@testing-library/jest-dom/vitest";

const {
  mockFetchPublicActiveSchema,
  mockFetchPublicCaptchaConfig,
  mockFetchPublicLegalTexts,
  mockFetchPublicCareOfferings,
  mockFetchMyEnrollmentProfile,
  mockSubmitEnrollment,
} = vi.hoisted(() => ({
  mockFetchPublicActiveSchema: vi.fn(),
  mockFetchPublicCaptchaConfig: vi.fn(),
  mockFetchPublicLegalTexts: vi.fn(),
  mockFetchPublicCareOfferings: vi.fn(),
  mockFetchMyEnrollmentProfile: vi.fn(),
  mockSubmitEnrollment: vi.fn(),
}));

vi.mock("~/lib/enrollment-form-schema-api", async (importOriginal) => {
  const actual = (await importOriginal()) as Record<string, unknown>;
  return {
    ...actual,
    fetchPublicActiveSchema: mockFetchPublicActiveSchema,
    fetchPublicCaptchaConfig: mockFetchPublicCaptchaConfig,
    fetchPublicLegalTexts: mockFetchPublicLegalTexts,
  };
});

vi.mock("~/lib/enrollment-submission-api", async (importOriginal) => {
  const actual = (await importOriginal()) as Record<string, unknown>;
  return {
    ...actual,
    fetchMyEnrollmentProfile: mockFetchMyEnrollmentProfile,
    fetchPublicCareOfferings: mockFetchPublicCareOfferings,
    submitEnrollment: mockSubmitEnrollment,
  };
});

import { EnrollmentForm } from "./enrollment-form";
import type {
  PublicFormSchema,
  PublicLegalTexts,
} from "~/lib/enrollment-form-schema-api";
import type {
  MeProfileResponse,
  PublicCareOffering,
  SubmitEnrollmentPayload,
} from "~/lib/enrollment-submission-api";

function schema(): PublicFormSchema {
  return {
    id: "7",
    version: 1,
    fields: [
      {
        key: "pickup_note",
        label: "Abholhinweis",
        type: "textarea",
        required: false,
        applies_to_child: false,
        sort_order: 0,
      },
      {
        key: "allergies",
        label: "Allergien",
        type: "text",
        required: false,
        applies_to_child: true,
        sort_order: 1,
      },
      {
        key: "lunch",
        label: "Mittagessen",
        type: "boolean",
        required: false,
        applies_to_child: true,
        sort_order: 2,
      },
      {
        key: "dismissal",
        label: "Abholzeit",
        type: "weekday_schedule",
        required: false,
        applies_to_child: true,
        sort_order: 3,
      },
      {
        key: "phones",
        label: "Weitere Telefonnummern",
        type: "phone_list",
        required: false,
        applies_to_child: true,
        sort_order: 4,
      },
      {
        key: "contacts",
        label: "Notfallkontakte",
        type: "contact_list",
        required: false,
        applies_to_child: true,
        sort_order: 5,
      },
      {
        key: "bus_days",
        label: "Buskind",
        type: "weekday_boolean",
        required: false,
        applies_to_child: true,
        sort_order: 6,
      },
    ],
  };
}

function legalTexts(
  blocks: PublicLegalTexts["blocks"] = [
    {
      key: "agb",
      kind: "terms",
      title: "AGB / Teilnahmebedingungen",
      label:
        "Ich akzeptiere die AGB / Teilnahmebedingungen / den Ganztag Info-Brief.",
      text: "AGB Text",
      required: true,
    },
    {
      key: "data_processing",
      kind: "privacy_notice",
      title: "Datenschutzinformation",
      label:
        "Ich habe die Datenschutzinformation der Schule zur Kenntnis genommen.",
      text: "Datenschutz Text",
      required: true,
    },
    {
      key: "photo",
      kind: "consent",
      title: "Fotoeinwilligung",
      label:
        "Mein Kind darf bei Schulveranstaltungen fotografiert werden. Diese Einwilligung ist freiwillig und jederzeit mit Wirkung für die Zukunft widerrufbar.",
      text: "Foto Text",
      required: false,
    },
    {
      key: "email_contact",
      kind: "notice",
      title: "E-Mail-Kontakt",
      label:
        "Die Schule nutzt Ihre E-Mail-Adresse für Rückfragen und Status-Benachrichtigungen zu dieser Anmeldung.",
      text: "E-Mail Text",
      required: false,
    },
  ],
): PublicLegalTexts {
  return {
    agb: blocks.find((block) => block.key === "agb")?.text ?? "",
    dsgvo: blocks.find((block) => block.key === "data_processing")?.text ?? "",
    email_contact:
      blocks.find((block) => block.key === "email_contact")?.text ?? "",
    photo: blocks.find((block) => block.key === "photo")?.text ?? "",
    terms_enabled: blocks.some((block) => block.key === "agb"),
    blocks,
  };
}

function offerings(): PublicCareOffering[] {
  return [
    {
      id: "11",
      phase_id: "5",
      name: "Flexible Betreuung",
      description: "Eltern wählen die Tage",
      days_of_week_mode: "parent_choice",
      available_days: ["mon", "wed", "fri"],
      includes_holiday_care: true,
      includes_lunch: true,
      is_active: true,
      is_required: false,
      capacity: null,
    },
    {
      id: "12",
      phase_id: "5",
      name: "Fixe Betreuung",
      description: null,
      days_of_week_mode: "fixed",
      available_days: ["tue", "thu"],
      includes_holiday_care: false,
      includes_lunch: false,
      is_active: true,
      is_required: false,
      capacity: 20,
    },
  ];
}

function profile(): MeProfileResponse {
  return {
    guardian: {
      first_name: "Mara",
      last_name: "Muster",
      email: "mara@example.test",
      phone: "+49 221 1234567",
    },
    children: [
      {
        id: "stu-1",
        first_name: "Lina",
        last_name: "Muster",
        school_class: "2a",
        grade_level: 2,
      },
    ],
  };
}

function renderForm(
  props: Partial<React.ComponentProps<typeof EnrollmentForm>> = {},
) {
  return render(
    <EnrollmentForm
      phaseID="5"
      gradeLevelMax={4}
      onSubmitted={vi.fn()}
      {...props}
    />,
  );
}

async function waitForLoaded() {
  await waitFor(() => {
    expect(
      screen.queryByText("Formular wird geladen…"),
    ).not.toBeInTheDocument();
  });
}

async function chooseOption(label: string, optionName: string) {
  fireEvent.click(screen.getByLabelText(label));
  fireEvent.click(await screen.findByRole("option", { name: optionName }));
}

async function fillRequiredFields() {
  const firstNameInputs = screen.getAllByLabelText("Vorname *");
  const lastNameInputs = screen.getAllByLabelText("Nachname *");
  fireEvent.change(firstNameInputs[0]!, { target: { value: "Mara" } });
  fireEvent.change(lastNameInputs[0]!, { target: { value: "Muster" } });
  fireEvent.change(screen.getByLabelText("E-Mail *"), {
    target: { value: "MARA@EXAMPLE.TEST" },
  });
  fireEvent.change(firstNameInputs[1]!, { target: { value: "Lina" } });
  fireEvent.change(lastNameInputs[1]!, { target: { value: "Muster" } });
  await chooseOption("Tag", "15");
  await chooseOption("Monat", "April");
  await chooseOption("Jahr", "2018");
  await chooseOption("Klassenstufe *", "2. Klasse");
  const agb = screen.queryByText(/AGB|Ganztag Info-Brief/);
  if (agb) {
    fireEvent.click(agb);
  }
  const privacy = screen.queryByText(
    /Datenschutzinformation der Schule zur Kenntnis/,
  );
  if (privacy) {
    fireEvent.click(privacy);
  }
}

describe("EnrollmentForm", () => {
  beforeEach(() => {
    mockFetchPublicActiveSchema.mockReset();
    mockFetchPublicCaptchaConfig.mockReset();
    mockFetchPublicLegalTexts.mockReset();
    mockFetchPublicCareOfferings.mockReset();
    mockFetchMyEnrollmentProfile.mockReset();
    mockSubmitEnrollment.mockReset();
    mockFetchPublicActiveSchema.mockResolvedValue(schema());
    mockFetchPublicCaptchaConfig.mockResolvedValue({
      enabled: false,
      site_key: "",
    });
    mockFetchPublicLegalTexts.mockResolvedValue(legalTexts());
    mockFetchPublicCareOfferings.mockResolvedValue({
      offerings: offerings(),
      careOfferingSelectionMode: "at_least_one",
      careRequired: true,
    });
    mockFetchMyEnrollmentProfile.mockResolvedValue(null);
    mockSubmitEnrollment.mockResolvedValue({ status_url: "/status/abc" });
  });

  it("loads schema, offerings, and renders validation for missing required fields", async () => {
    renderForm();
    await waitForLoaded();

    expect(mockFetchPublicActiveSchema).toHaveBeenCalledWith(
      "test-tenant",
      "5",
    );
    expect(mockFetchPublicCareOfferings).toHaveBeenCalledWith(
      "test-tenant",
      "5",
    );
    expect(mockFetchPublicLegalTexts).toHaveBeenCalledWith("test-tenant", "5");
    expect(screen.getByText("Flexible Betreuung")).toBeInTheDocument();
    expect(screen.getByText("Abholhinweis")).toBeInTheDocument();
    expect(screen.getByText("Allergien")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));

    expect(
      await screen.findByText(
        "Bitte korrigieren Sie die rot markierten Felder.",
      ),
    ).toBeInTheDocument();
    expect(screen.getAllByText("Bitte Vornamen angeben.")).toHaveLength(2);
    expect(mockSubmitEnrollment).not.toHaveBeenCalled();
  });

  it("renders custom field help texts so parents see the admin's guidance", async () => {
    // Regression guard: help_text was stored, served, and shown in the form
    // editor, but never rendered in the actual enrollment form (CustomFieldInput
    // and the structured field renderers dropped it). Cover both render paths:
    // a simple field via labelEl (textarea) and a structured one
    // (weekday_schedule), which render help_text independently.
    const withHelp = schema();
    withHelp.fields[0]!.help_text = "Bitte gewünschte Abholung beschreiben.";
    withHelp.fields[3]!.help_text = "Pro Tag die Uhrzeit eintragen.";
    mockFetchPublicActiveSchema.mockResolvedValue(withHelp);

    renderForm();
    await waitForLoaded();

    expect(
      screen.getByText("Bitte gewünschte Abholung beschreiben."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Pro Tag die Uhrzeit eintragen."),
    ).toBeInTheDocument();
  });

  it("fails closed when the legal texts cannot be loaded", async () => {
    // A real load failure (settings/DB/JSON error) must reject the whole
    // form load so the parent never submits legally relevant consent
    // without the configured documents. Unconfigured texts (empty
    // strings, a 200) do NOT trigger this — see the beforeEach default.
    mockFetchPublicLegalTexts.mockReset();
    mockFetchPublicLegalTexts.mockRejectedValue(
      new Error("legal texts request failed: 500"),
    );
    renderForm();
    await waitForLoaded();

    // The error banner surfaces and the schema-driven form body never
    // renders, so consent can't be collected without the documents.
    expect(
      screen.getByText("legal texts request failed: 500"),
    ).toBeInTheDocument();
    expect(screen.queryByText("Flexible Betreuung")).not.toBeInTheDocument();
    expect(mockSubmitEnrollment).not.toHaveBeenCalled();
  });

  it("renders only the configured AGB block and submits only its consent", async () => {
    mockFetchPublicLegalTexts.mockResolvedValueOnce(
      legalTexts([
        {
          key: "agb",
          kind: "terms",
          title: "AGB / Teilnahmebedingungen",
          label:
            "Ich akzeptiere die AGB / Teilnahmebedingungen / den Ganztag Info-Brief.",
          text: "Ganztag Info-Brief",
          required: true,
        },
      ]),
    );
    mockFetchPublicCareOfferings.mockResolvedValueOnce({
      offerings: [],
      careOfferingSelectionMode: "optional",
      careRequired: false,
    });

    renderForm();
    await waitForLoaded();

    expect(screen.getByText(/Ganztag Info-Brief/)).toBeInTheDocument();
    expect(
      screen.queryByText(/Datenschutzinformation der Schule zur Kenntnis/),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText(/Schulveranstaltungen fotografiert/),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText(/Status-Benachrichtigungen/),
    ).not.toBeInTheDocument();

    await fillRequiredFields();
    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));

    await waitFor(() => {
      expect(mockSubmitEnrollment).toHaveBeenCalledTimes(1);
    });
    const [, payload] = mockSubmitEnrollment.mock.calls[0] as [
      string,
      SubmitEnrollmentPayload,
    ];
    expect(payload.consent_flags).toEqual({ agb: true });
  });

  it("hides the legal section and submits empty consent flags when no blocks are configured", async () => {
    mockFetchPublicLegalTexts.mockResolvedValueOnce(legalTexts([]));
    mockFetchPublicCareOfferings.mockResolvedValueOnce({
      offerings: [],
      careOfferingSelectionMode: "optional",
      careRequired: false,
    });

    renderForm();
    await waitForLoaded();

    expect(
      screen.queryByText("Zustimmungen & Hinweise"),
    ).not.toBeInTheDocument();

    await fillRequiredFields();
    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));

    await waitFor(() => {
      expect(mockSubmitEnrollment).toHaveBeenCalledTimes(1);
    });
    const [, payload] = mockSubmitEnrollment.mock.calls[0] as [
      string,
      SubmitEnrollmentPayload,
    ];
    expect(payload.consent_flags).toEqual({});
  });

  it("requires and submits custom template consent blocks", async () => {
    mockFetchPublicLegalTexts.mockResolvedValueOnce(
      legalTexts([
        {
          key: "custom_pool",
          kind: "consent",
          title: "Schwimmbad",
          label: "Mein Kind darf am Schwimmbad-Ausflug teilnehmen.",
          text: "Schwimmbad Details",
          required: true,
          source: "custom",
        },
      ]),
    );
    mockFetchPublicCareOfferings.mockResolvedValueOnce({
      offerings: [],
      careOfferingSelectionMode: "optional",
      careRequired: false,
    });

    renderForm();
    await waitForLoaded();
    await fillRequiredFields();

    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));

    // The message renders twice by design: once in the error banner and
    // once inline at the unchecked block.
    expect(
      await screen.findAllByText(
        "Bitte diese erforderliche Bestätigung auswählen.",
      ),
    ).not.toHaveLength(0);
    expect(mockSubmitEnrollment).not.toHaveBeenCalled();

    fireEvent.click(
      screen.getByText("Mein Kind darf am Schwimmbad-Ausflug teilnehmen."),
    );
    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));

    await waitFor(() => {
      expect(mockSubmitEnrollment).toHaveBeenCalledTimes(1);
    });
    const [, payload] = mockSubmitEnrollment.mock.calls[0] as [
      string,
      SubmitEnrollmentPayload,
    ];
    expect(payload.consent_flags).toEqual({ custom_pool: true });
  });

  it("renders legal blocks from preview schemas without loading tenant legal texts", async () => {
    mockFetchPublicLegalTexts.mockClear();

    renderForm({
      phaseID: undefined,
      previewMode: true,
      previewSchema: {
        id: "preview-schema",
        version: 1,
        fields: [],
        legal_blocks: [
          {
            key: "custom_photo_trip",
            kind: "consent",
            title: "Fotoausflug",
            label: "Mein Kind darf beim Ausflug fotografiert werden.",
            text: "Details zum Fotoausflug",
            required: true,
            enabled: true,
            sort_order: 10,
            source: "custom",
          },
          {
            key: "disabled_consent",
            kind: "consent",
            title: "Ausgeblendet",
            label: "Diese Zustimmung ist deaktiviert.",
            text: "",
            required: false,
            enabled: false,
            sort_order: 20,
            source: "custom",
          },
        ],
      },
      skipCaptcha: true,
    });
    await waitForLoaded();

    expect(mockFetchPublicLegalTexts).not.toHaveBeenCalled();
    expect(
      screen.getByText("Mein Kind darf beim Ausflug fotografiert werden."),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Diese Zustimmung ist deaktiviert."),
    ).not.toBeInTheDocument();
  });

  it("submits guardian, child, custom field, offering, and consent payloads", async () => {
    const onSubmitted = vi.fn();
    renderForm({ onSubmitted });
    await waitForLoaded();

    await fillRequiredFields();
    fireEvent.change(screen.getByLabelText("Telefon"), {
      target: { value: "+49 221 1234567" },
    });
    fireEvent.change(screen.getByLabelText("Abholhinweis"), {
      target: { value: "Kommt mit Oma." },
    });
    fireEvent.change(screen.getByLabelText("Allergien"), {
      target: { value: "Nuesse" },
    });
    fireEvent.click(screen.getByLabelText("Ja"));
    fireEvent.click(screen.getByText(/Schulveranstaltungen fotografiert/));

    fireEvent.click(screen.getByRole("checkbox", { name: /Fixe Betreuung/ }));

    fireEvent.change(screen.getByLabelText("Montag"), {
      target: { value: "15:00" },
    });

    fireEvent.click(
      screen.getByRole("button", { name: "Telefonnummer hinzufügen" }),
    );
    fireEvent.change(screen.getByLabelText("Nummer"), {
      target: { value: "+49 221 7654321" },
    });
    fireEvent.click(screen.getByLabelText("Hauptnummer"));

    fireEvent.click(screen.getByRole("button", { name: "Kontakt hinzufügen" }));
    fireEvent.change(
      screen.getByLabelText("Beziehung (z. B. Oma, Onkel, Nachbarin)"),
      {
        target: { value: "Oma" },
      },
    );
    const contactInputs = screen.getAllByDisplayValue("");
    fireEvent.change(contactInputs[contactInputs.length - 4]!, {
      target: { value: "Eva" },
    });
    fireEvent.change(contactInputs[contactInputs.length - 3]!, {
      target: { value: "Muster" },
    });
    fireEvent.click(screen.getByLabelText("Abholberechtigt"));

    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));

    await waitFor(() => {
      expect(mockSubmitEnrollment).toHaveBeenCalledTimes(1);
    });
    const [tenantSlug, payload] = mockSubmitEnrollment.mock.calls[0] as [
      string,
      SubmitEnrollmentPayload,
    ];
    expect(tenantSlug).toBe("test-tenant");
    expect(payload).toMatchObject({
      phase_id: 5,
      guardian_first_name: "Mara",
      guardian_last_name: "Muster",
      guardian_email: "mara@example.test",
      guardian_phone: "+49 221 1234567",
      consent_flags: {
        agb: true,
        data_processing: true,
        email_contact: true,
        photo: true,
      },
      custom_data: { pickup_note: "Kommt mit Oma." },
    });
    expect(payload.children).toHaveLength(1);
    expect(payload.children[0]).toMatchObject({
      first_name: "Lina",
      last_name: "Muster",
      date_of_birth: "2018-04-15",
      target_grade_level: 2,
      offering_ids: [12],
    });
    expect(payload.children[0]?.custom_data).toMatchObject({
      allergies: "Nuesse",
      lunch: true,
      dismissal: { mon: "15:00" },
    });
    expect(onSubmitted).toHaveBeenCalledWith("/status/abc");
  });

  it("prefills from a parent profile and adopts an existing child", async () => {
    mockFetchMyEnrollmentProfile.mockResolvedValueOnce(profile());
    renderForm();
    await waitForLoaded();

    expect(screen.getByDisplayValue("Mara")).toBeInTheDocument();
    expect(screen.getByDisplayValue("Muster")).toBeInTheDocument();
    expect(screen.getByDisplayValue("mara@example.test")).toBeInTheDocument();
    expect(screen.getByText("Bestehende Kinder")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Übernehmen" }));

    expect(screen.getAllByDisplayValue("Lina")[0]).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "✓ übernommen" })).toBeDisabled();
    expect(screen.getByLabelText("Klassenstufe *")).toHaveTextContent(
      "2. Klasse",
    );
  });

  it("requires care offerings and parent-choice days before submit", async () => {
    renderForm();
    await waitForLoaded();
    await fillRequiredFields();

    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));
    expect(
      await screen.findByText(
        "Bitte wählen Sie für jedes Kind mindestens ein Betreuungsangebot aus.",
      ),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByText("Flexible Betreuung"));
    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));
    expect(
      await screen.findByText(
        'Kind 1: Beim Angebot „Flexible Betreuung" muss mindestens ein Tag ausgewählt werden.',
      ),
    ).toBeInTheDocument();
    expect(mockSubmitEnrollment).not.toHaveBeenCalled();
  });

  it("treats exactly-one care selection as a single-choice group", async () => {
    mockFetchPublicCareOfferings.mockResolvedValueOnce({
      offerings: offerings(),
      careOfferingSelectionMode: "exactly_one",
      careRequired: true,
    });
    renderForm();
    await waitForLoaded();

    const flexible = screen.getByRole("checkbox", {
      name: /Flexible Betreuung/,
    }) as HTMLInputElement;
    const fixed = screen.getByRole("checkbox", {
      name: /Fixe Betreuung/,
    }) as HTMLInputElement;

    fireEvent.click(fixed);
    expect(fixed).toBeChecked();
    expect(flexible).not.toBeChecked();

    fireEvent.click(flexible);
    expect(flexible).toBeChecked();
    expect(fixed).not.toBeChecked();
  });

  it("enforces configurable core required fields", async () => {
    mockFetchPublicActiveSchema.mockResolvedValueOnce({
      ...schema(),
      core_requirements: {
        guardian_phone: true,
      },
    });
    mockFetchPublicCareOfferings.mockResolvedValueOnce({
      offerings: [],
      careOfferingSelectionMode: "optional",
      careRequired: false,
    });
    renderForm();
    await waitForLoaded();
    await fillRequiredFields();

    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));

    expect(
      await screen.findAllByText("Bitte Telefonnummer angeben."),
    ).not.toHaveLength(0);
    expect(mockSubmitEnrollment).not.toHaveBeenCalled();

    fireEvent.change(screen.getByRole("textbox", { name: /Telefon/ }), {
      target: { value: "+49 221 1234567" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));

    await waitFor(() => {
      expect(mockSubmitEnrollment).toHaveBeenCalledTimes(1);
    });
    const [, payload] = mockSubmitEnrollment.mock.calls[0] as [
      string,
      SubmitEnrollmentPayload,
    ];
    expect(payload.consent_flags?.photo).toBe(false);
  });

  it("adds and removes children without submitting in preview mode", async () => {
    renderForm({ previewMode: true, previewSchema: schema() });
    await waitForLoaded();

    fireEvent.click(screen.getByRole("button", { name: "Weiteres Kind" }));
    expect(screen.getByText("Kind 2")).toBeInTheDocument();
    fireEvent.click(screen.getAllByRole("button", { name: "Entfernen" })[0]!);
    expect(screen.queryByText("Kind 2")).not.toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Vorschau, nicht absenden" }),
    );
    expect(mockSubmitEnrollment).not.toHaveBeenCalled();
  });

  it("uses injected submitter and skips captcha for authenticated parent paths", async () => {
    const submitter = vi
      .fn()
      .mockResolvedValue({ status_url: "/parents/status/1" });
    const onSubmitted = vi.fn();
    renderForm({
      submitter,
      onSubmitted,
      skipCaptcha: true,
      profileFetcher: vi.fn().mockResolvedValue(null),
    });
    await waitForLoaded();
    await fillRequiredFields();
    fireEvent.click(screen.getByText("Fixe Betreuung"));

    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));

    await waitFor(() => {
      expect(submitter).toHaveBeenCalledTimes(1);
    });
    const payload = submitter.mock.calls[0]?.[0] as SubmitEnrollmentPayload;
    expect(payload.captcha_token).toBeUndefined();
    expect(payload.children[0]?.offering_ids).toEqual([12]);
    expect(onSubmitted).toHaveBeenCalledWith("/parents/status/1");
  });

  it("submits selected bus weekdays for weekday boolean fields", async () => {
    renderForm();
    await waitForLoaded();
    await fillRequiredFields();

    fireEvent.click(screen.getByRole("checkbox", { name: "Mo" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "Fr" }));
    fireEvent.click(screen.getByRole("checkbox", { name: /Fixe Betreuung/ }));
    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));

    await waitFor(() => {
      expect(mockSubmitEnrollment).toHaveBeenCalledTimes(1);
    });
    const [, payload] = mockSubmitEnrollment.mock.calls[0] as [
      string,
      SubmitEnrollmentPayload,
    ];
    expect(payload.children[0]?.custom_data?.bus_days).toEqual({
      mon: true,
      fri: true,
    });
  });

  it("shows pickup-specific helper text for the Abholregelung weekday field", async () => {
    const withPickup = schema();
    withPickup.fields = [
      ...withPickup.fields,
      {
        key: "pickup_status",
        label: "Abholregelung",
        type: "weekday_boolean",
        target: "student.pickup_status",
        required: false,
        applies_to_child: true,
        sort_order: 7,
      },
    ];
    mockFetchPublicActiveSchema.mockResolvedValueOnce(withPickup);
    renderForm();
    await waitForLoaded();

    // The pickup field gets the pickup copy, not the bus copy — guards the
    // regression where the shared weekday_boolean input hardcoded bus text.
    expect(
      screen.getByText(/an denen Ihr Kind abgeholt wird/),
    ).toBeInTheDocument();
    // The Buskind field still shows its own bus copy.
    expect(
      screen.getByText(/an denen Ihr Kind mit dem Bus fährt/),
    ).toBeInTheDocument();
  });

  it("requires at least one selected weekday for required weekday boolean fields", async () => {
    const requiredBusDays = schema();
    requiredBusDays.fields = requiredBusDays.fields.map((field) =>
      field.key === "bus_days" ? { ...field, required: true } : field,
    );
    mockFetchPublicActiveSchema.mockResolvedValueOnce(requiredBusDays);
    mockFetchPublicCareOfferings.mockResolvedValueOnce({
      offerings: [],
      careOfferingSelectionMode: "optional",
      careRequired: false,
    });
    renderForm();
    await waitForLoaded();
    await fillRequiredFields();

    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));

    expect(
      await screen.findAllByText("Bitte mindestens einen Wochentag auswählen."),
    ).not.toHaveLength(0);
    expect(mockSubmitEnrollment).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("checkbox", { name: "Mo" }));
    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));

    await waitFor(() => {
      expect(mockSubmitEnrollment).toHaveBeenCalledTimes(1);
    });
  });

  it("required pickup rejects an untouched picker but accepts an explicit empty selection", async () => {
    const withRequiredPickup = schema();
    withRequiredPickup.fields = [
      ...withRequiredPickup.fields,
      {
        key: "pickup_status",
        label: "Abholregelung",
        type: "weekday_boolean",
        target: "student.pickup_status",
        required: true,
        applies_to_child: true,
        sort_order: 7,
      },
    ];
    mockFetchPublicActiveSchema.mockResolvedValueOnce(withRequiredPickup);
    mockFetchPublicCareOfferings.mockResolvedValueOnce({
      offerings: [],
      careOfferingSelectionMode: "optional",
      careRequired: false,
    });
    renderForm();
    await waitForLoaded();
    await fillRequiredFields();

    // Untouched required pickup is "missing" — submission is blocked and the
    // pickup-specific confirm message (not the bus "pick a day" message) shows.
    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));
    expect(
      await screen.findAllByText(
        "Bitte die Abholregelung bestätigen (Tage auswählen oder leer lassen).",
      ),
    ).not.toHaveLength(0);
    expect(mockSubmitEnrollment).not.toHaveBeenCalled();

    // Touch the pickup picker and clear it again → an explicit empty map. That
    // is the valid "geht alleine nach Hause" answer, so submission proceeds.
    const pickupGroup = screen.getByRole("group", { name: /Abholregelung/ });
    const pickupMonday = within(pickupGroup).getByRole("checkbox", {
      name: "Mo",
    });
    fireEvent.click(pickupMonday); // select
    fireEvent.click(pickupMonday); // deselect → touched, empty
    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));

    await waitFor(() => {
      expect(mockSubmitEnrollment).toHaveBeenCalledTimes(1);
    });
  });

  it("pre-selects and locks a mandatory offering and submits it for every child", async () => {
    const submitter = vi.fn().mockResolvedValue({ status_url: "/status/req" });
    mockFetchPublicCareOfferings.mockResolvedValue({
      offerings: [
        {
          id: "20",
          phase_id: "5",
          name: "Mittagessen",
          description: null,
          days_of_week_mode: "fixed",
          available_days: ["mon", "tue", "wed", "thu", "fri"],
          includes_holiday_care: false,
          includes_lunch: true,
          is_active: true,
          is_required: true,
          capacity: null,
        },
      ],
      careOfferingSelectionMode: "optional",
      careRequired: false,
    });
    renderForm({ submitter, skipCaptcha: true });
    await waitForLoaded();

    // The mandatory offering is checked and cannot be toggled off.
    const checkbox = screen.getByRole("checkbox", {
      name: /Mittagessen/,
    }) as HTMLInputElement;
    expect(checkbox).toBeChecked();
    expect(checkbox).toBeDisabled();
    expect(screen.getByText("Pflicht")).toBeInTheDocument();

    await fillRequiredFields();
    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));

    await waitFor(() => {
      expect(submitter).toHaveBeenCalledTimes(1);
    });
    const payload = submitter.mock.calls[0]?.[0] as SubmitEnrollmentPayload;
    expect(payload.children[0]?.offering_ids).toEqual([20]);
  });

  it("counts only choosable offerings for exactly_one when a required offering is present", async () => {
    mockFetchPublicCareOfferings.mockResolvedValueOnce({
      offerings: [
        {
          id: "20",
          phase_id: "5",
          name: "Mittagessen",
          description: null,
          days_of_week_mode: "fixed",
          available_days: ["mon", "tue", "wed", "thu", "fri"],
          includes_holiday_care: false,
          includes_lunch: true,
          is_active: true,
          is_required: true,
          capacity: null,
        },
        {
          id: "12",
          phase_id: "5",
          name: "Fixe Betreuung",
          description: null,
          days_of_week_mode: "fixed",
          available_days: ["tue", "thu"],
          includes_holiday_care: false,
          includes_lunch: false,
          is_active: true,
          is_required: false,
          capacity: 20,
        },
      ],
      careOfferingSelectionMode: "exactly_one",
      careRequired: true,
    });
    renderForm();
    await waitForLoaded();

    // The required offering renders in its own locked "Pflichtangebote" block,
    // checked and disabled - it is not the parent's pick.
    expect(screen.getByText("Pflichtangebote")).toBeInTheDocument();
    const required = screen.getByRole("checkbox", {
      name: /Mittagessen/,
    }) as HTMLInputElement;
    expect(required).toBeChecked();
    expect(required).toBeDisabled();

    await fillRequiredFields();

    // Submitting with ONLY the required offering must be rejected: exactly_one
    // counts the choosable offerings, and none is chosen yet. The required
    // offering must not satisfy the limit on its own.
    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));
    expect(
      await screen.findByText(
        "Bitte wählen Sie für jedes Kind genau ein Betreuungsangebot aus.",
      ),
    ).toBeInTheDocument();
    expect(mockSubmitEnrollment).not.toHaveBeenCalled();

    // Picking exactly one choosable offering satisfies the mode; the required
    // offering rides along without counting toward the limit.
    fireEvent.click(screen.getByRole("checkbox", { name: /Fixe Betreuung/ }));
    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));

    await waitFor(() => {
      expect(mockSubmitEnrollment).toHaveBeenCalledTimes(1);
    });
    const [, payload] = mockSubmitEnrollment.mock.calls[0] as [
      string,
      SubmitEnrollmentPayload,
    ];
    expect(payload.children[0]?.offering_ids).toHaveLength(2);
    expect(payload.children[0]?.offering_ids).toEqual(
      expect.arrayContaining([20, 12]),
    );
  });
});
