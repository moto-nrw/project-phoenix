import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
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
import type { PublicFormSchema } from "~/lib/enrollment-form-schema-api";
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

function fillRequiredFields() {
  const firstNameInputs = screen.getAllByLabelText("Vorname *");
  const lastNameInputs = screen.getAllByLabelText("Nachname *");
  fireEvent.change(firstNameInputs[0]!, { target: { value: "Mara" } });
  fireEvent.change(lastNameInputs[0]!, { target: { value: "Muster" } });
  fireEvent.change(screen.getByLabelText("E-Mail *"), {
    target: { value: "MARA@EXAMPLE.TEST" },
  });
  fireEvent.change(firstNameInputs[1]!, { target: { value: "Lina" } });
  fireEvent.change(lastNameInputs[1]!, { target: { value: "Muster" } });
  fireEvent.change(screen.getByLabelText("Tag"), { target: { value: "15" } });
  fireEvent.change(screen.getByLabelText("Monat"), { target: { value: "4" } });
  fireEvent.change(screen.getByLabelText("Jahr"), {
    target: { value: "2018" },
  });
  fireEvent.change(screen.getByLabelText("Klassenstufe *"), {
    target: { value: "2" },
  });
  // AGB checkbox only renders when the tenant enabled the terms block
  // (default mock sets terms_enabled: true). Datenschutz is an
  // acknowledgement; e-mail contact is now a notice with no checkbox.
  fireEvent.click(screen.getByText(/AGB/));
  fireEvent.click(
    screen.getByText(/Datenschutzinformation der Schule zur Kenntnis/),
  );
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
    // Unconfigured tenant: the public legal endpoint returns empty
    // strings (a 200 response), so the consent labels render without a
    // document link. A real load failure rejects instead — covered by
    // the dedicated fail-closed test below.
    mockFetchPublicLegalTexts.mockResolvedValue({
      agb: "",
      dsgvo: "",
      email_contact: "",
      photo: "",
      // Terms block on so the AGB checkbox renders for the submit-payload
      // tests (fillRequiredFields ticks it). Tenants default to false.
      terms_enabled: true,
    });
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
    expect(screen.getByText("Flexible Betreuung")).toBeInTheDocument();
    expect(screen.getByText("Abholhinweis")).toBeInTheDocument();
    expect(screen.getByText("Allergien")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));

    expect(
      await screen.findByText("Bitte korrigiere die rot markierten Felder."),
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

  it("submits guardian, child, custom field, offering, and consent payloads", async () => {
    const onSubmitted = vi.fn();
    renderForm({ onSubmitted });
    await waitForLoaded();

    fillRequiredFields();
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
    expect(
      (screen.getByLabelText("Klassenstufe *") as HTMLSelectElement).value,
    ).toBe("2");
  });

  it("requires care offerings and parent-choice days before submit", async () => {
    renderForm();
    await waitForLoaded();
    fillRequiredFields();

    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));
    expect(
      await screen.findByText(
        "Bitte wähle für jedes Kind mindestens ein Betreuungsangebot aus.",
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
    fillRequiredFields();

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
    fillRequiredFields();
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
    fillRequiredFields();

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
    fillRequiredFields();

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

    fillRequiredFields();
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

    fillRequiredFields();

    // Submitting with ONLY the required offering must be rejected: exactly_one
    // counts the choosable offerings, and none is chosen yet. The required
    // offering must not satisfy the limit on its own.
    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));
    expect(
      await screen.findByText(
        "Bitte wähle für jedes Kind genau ein Betreuungsangebot aus.",
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
