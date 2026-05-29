import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";

const {
  mockFetchPublicActiveSchema,
  mockFetchPublicCareOfferings,
  mockFetchMyEnrollmentProfile,
  mockSubmitEnrollment,
} = vi.hoisted(() => ({
  mockFetchPublicActiveSchema: vi.fn(),
  mockFetchPublicCareOfferings: vi.fn(),
  mockFetchMyEnrollmentProfile: vi.fn(),
  mockSubmitEnrollment: vi.fn(),
}));

vi.mock("~/lib/enrollment-form-schema-api", async (importOriginal) => {
  const actual = (await importOriginal()) as Record<string, unknown>;
  return {
    ...actual,
    fetchPublicActiveSchema: mockFetchPublicActiveSchema,
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
  fireEvent.click(screen.getByText(/AGB/));
  fireEvent.click(screen.getByText(/Verarbeitung der angegebenen Daten/));
  fireEvent.click(screen.getByText(/E-Mail kontaktieren/));
}

describe("EnrollmentForm", () => {
  beforeEach(() => {
    mockFetchPublicActiveSchema.mockReset();
    mockFetchPublicCareOfferings.mockReset();
    mockFetchMyEnrollmentProfile.mockReset();
    mockSubmitEnrollment.mockReset();
    mockFetchPublicActiveSchema.mockResolvedValue(schema());
    mockFetchPublicCareOfferings.mockResolvedValue({
      offerings: offerings(),
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

  it("enforces configurable core required fields", async () => {
    mockFetchPublicActiveSchema.mockResolvedValueOnce({
      ...schema(),
      core_requirements: {
        guardian_phone: true,
      },
    });
    mockFetchPublicCareOfferings.mockResolvedValueOnce({
      offerings: [],
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
});
