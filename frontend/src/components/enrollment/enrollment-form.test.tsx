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
  mockIntlLocale,
} = vi.hoisted(() => ({
  mockFetchPublicActiveSchema: vi.fn(),
  mockFetchPublicCaptchaConfig: vi.fn(),
  mockFetchPublicLegalTexts: vi.fn(),
  mockFetchPublicCareOfferings: vi.fn(),
  mockFetchMyEnrollmentProfile: vi.fn(),
  mockSubmitEnrollment: vi.fn(),
  mockIntlLocale: { value: "de" },
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

vi.mock("next-intl", async () => {
  const de = (await import("~/i18n/messages/de.json")).default as Record<
    string,
    unknown
  >;
  const en = (await import("~/i18n/messages/en.json")).default as Record<
    string,
    unknown
  >;
  // A German catalog with the weekday/departure label maps removed, exposed as
  // a pseudo-locale. Selecting it via mockIntlLocale lets a test render with
  // missing labels so the component's `labels[key] ?? key` fallbacks show the
  // raw keys. Uses the same locale-switching path as the real "en" catalog.
  const deNoLabels = structuredClone(de) as Record<string, unknown>;
  const deNoLabelsEnrollment = deNoLabels.enrollmentForm as Record<
    string,
    unknown
  >;
  delete deNoLabelsEnrollment.weekdaysShort;
  delete (deNoLabelsEnrollment.structured as Record<string, unknown>)
    .departureModes;
  const catalogs: Record<string, Record<string, unknown>> = {
    de,
    en,
    "de-no-labels": deNoLabels,
  };
  const currentCatalog = (): Record<string, unknown> =>
    catalogs[mockIntlLocale.value] ?? de;
  const resolve = (catalog: Record<string, unknown>, path: string): unknown =>
    path.split(".").reduce<unknown>((acc, part) => {
      if (acc && typeof acc === "object") {
        return (acc as Record<string, unknown>)[part];
      }
      return undefined;
    }, catalog);
  const interpolate = (
    value: string,
    values?: Record<string, unknown>,
  ): string =>
    values
      ? Object.entries(values).reduce(
          (str, [k, v]) => str.replaceAll(`{${k}}`, String(v)),
          value,
        )
      : value;
  const makeT = (namespace?: string) => {
    const prefix = namespace ? `${namespace}.` : "";
    const t = (key: string, values?: Record<string, unknown>) => {
      const catalog = currentCatalog();
      const val = resolve(catalog, `${prefix}${key}`);
      return typeof val === "string"
        ? interpolate(val, values)
        : `${prefix}${key}`;
    };
    t.raw = (key: string) => {
      const catalog = currentCatalog();
      return resolve(catalog, `${prefix}${key}`);
    };
    return t;
  };
  const cache = new Map<string, ReturnType<typeof makeT>>();
  return {
    useTranslations: (namespace?: string) => {
      const cacheKey = `${mockIntlLocale.value}:${namespace ?? ""}`;
      const existing = cache.get(cacheKey);
      if (existing) return existing;
      const t = makeT(namespace);
      cache.set(cacheKey, t);
      return t;
    },
    useLocale: () => mockIntlLocale.value,
    NextIntlClientProvider: ({ children }: { children: React.ReactNode }) =>
      children,
  };
});

import { EnrollmentForm } from "./enrollment-form";
import type {
  PublicFormSchema,
  PublicLegalTexts,
} from "~/lib/enrollment-form-schema-api";
import type {
  EnrollmentEditDraft,
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
    dsgvo_enabled: blocks.some((block) => block.key === "data_processing"),
    email_contact_enabled: blocks.some(
      (block) => block.key === "email_contact",
    ),
    photo_enabled: blocks.some((block) => block.key === "photo"),
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

function editDraft(
  children: { id: string; first_name: string; last_name: string }[],
): EnrollmentEditDraft {
  return {
    request_id: "req-1",
    status_token: "tok-1",
    tenant_id: "1",
    tenant_subdomain: "demo",
    phase_id: "5",
    guardian_first_name: "Mara",
    guardian_last_name: "Muster",
    guardian_email: "mara@example.test",
    guardian_phone: "+49 221 1234567",
    consent_flags: {},
    custom_data: {},
    children: children.map((child) => ({
      ...child,
      date_of_birth: "2018-04-15",
      target_grade_level: 2,
      // A fixed Tue/Thu offering gives each child care days so the
      // weekday_multi_mode field renders its controls.
      offering_ids: ["12"],
    })),
  };
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
        // The backend always emits the lifecycle status alongside the
        // permission flag; the reuse picker requires both (#1663).
        status: "active",
        enrollment_submit: true,
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

function dayButton(container: HTMLElement, label: string): HTMLButtonElement {
  const button = Array.from(container.querySelectorAll("button")).find(
    (item) => item.textContent === label,
  );
  if (!(button instanceof HTMLButtonElement)) {
    throw new Error(`Day button ${label} not found`);
  }
  return button;
}

function offeringCard(inputName: string): HTMLElement {
  const input = document.querySelector<HTMLInputElement>(
    `input[name="${inputName}"]`,
  );
  const card = input?.closest("label")?.parentElement ?? null;
  if (!(card instanceof HTMLElement)) {
    throw new Error(`Offering card ${inputName} not found`);
  }
  return card;
}

describe("EnrollmentForm", () => {
  beforeEach(() => {
    mockIntlLocale.value = "de";
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
      { lateInviteToken: undefined },
    );
    expect(mockFetchPublicCareOfferings).toHaveBeenCalledWith(
      "test-tenant",
      "5",
      { lateInviteToken: undefined },
    );
    expect(mockFetchPublicLegalTexts).toHaveBeenCalledWith("test-tenant", "5", {
      lateInviteToken: undefined,
    });
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

  it("shows offerings per child grade and clears an invalid selection after a grade change", async () => {
    const conditional: PublicCareOffering = {
      ...offerings()[1]!,
      id: "99",
      name: "Randstunde Klasse 1 und 2",
      availability_rule: {
        match: "all",
        conditions: [{ source: "grade_level", operator: "in", value: [1, 2] }],
      },
    };
    mockFetchPublicCareOfferings.mockResolvedValueOnce({
      offerings: [offerings()[0]!, conditional],
      careOfferingSelectionMode: "optional",
      careRequired: false,
      collectGradeLevel: true,
      careOfferingsEnabled: true,
    });

    renderForm();
    await waitForLoaded();

    expect(
      screen.queryByText("Randstunde Klasse 1 und 2"),
    ).not.toBeInTheDocument();
    await chooseOption("Klassenstufe *", "2. Klasse");
    expect(screen.getByText("Randstunde Klasse 1 und 2")).toBeInTheDocument();

    fireEvent.click(
      document.querySelector('input[name="children_0_offering_99"]')!,
    );
    await chooseOption("Klassenstufe *", "3. Klasse");

    expect(
      screen.queryByText("Randstunde Klasse 1 und 2"),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText(
        /zugehörige Auswahl, Betreuungstage und automatische Mitbuchungen wurden entfernt/,
      ),
    ).toBeInTheDocument();
  });

  it("hides grade and care offerings when the tenant disables both", async () => {
    mockFetchPublicCareOfferings.mockResolvedValue({
      offerings: offerings(),
      careOfferingSelectionMode: "at_least_one",
      careRequired: true,
      collectGradeLevel: false,
      careOfferingsEnabled: false,
    });

    renderForm();
    await waitForLoaded();

    expect(screen.queryByLabelText("Klassenstufe *")).not.toBeInTheDocument();
    expect(screen.queryByText("Flexible Betreuung")).not.toBeInTheDocument();
    expect(screen.queryByText("Fixe Betreuung")).not.toBeInTheDocument();
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

  it("shows the compulsory attendance notice near care offerings", async () => {
    renderForm();
    await waitForLoaded();

    expect(
      screen.getByText(
        /Mit der Anmeldung zum Ganztagsangebot ist Ihr Kind laut Ganztagsschulerlass zu den angemeldeten Zeiten schulpflichtig\./,
      ),
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

  it("localizes the e-mail contact notice CTA in localized public copy", async () => {
    mockIntlLocale.value = "en";

    renderForm({ localizedCopy: true });
    await waitForLoaded();

    const emailContactNotice = screen
      .getByText(/Status-Benachrichtigungen/)
      .closest("div");
    expect(emailContactNotice).not.toBeNull();
    expect(
      within(emailContactNotice!).getByRole("button", {
        name: "Show details for E-Mail-Kontakt",
      }),
    ).toBeInTheDocument();
    expect(
      within(emailContactNotice!).queryByRole("button", {
        name: "Mehr anzeigen",
      }),
    ).not.toBeInTheDocument();
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

  it("opens legal block details in a modal from a neutral details button", async () => {
    mockFetchPublicLegalTexts.mockResolvedValueOnce(
      legalTexts([
        {
          key: "agb",
          kind: "terms",
          title: "AGB / Teilnahmebedingungen",
          label:
            "Ich akzeptiere die AGB / Teilnahmebedingungen / den Ganztag Info-Brief.",
          text: "Ganztag Info-Brief Details",
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

    expect(screen.queryByText("Mehr anzeigen")).not.toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", {
        name: "Details zu AGB / Teilnahmebedingungen anzeigen",
      }),
    );

    expect(
      screen.getByRole("heading", { name: "AGB / Teilnahmebedingungen" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Ganztag Info-Brief Details")).toBeInTheDocument();
  });

  it("renders email contact as an active notice block without a checkbox", async () => {
    renderForm();
    await waitForLoaded();

    const notice = screen.getByText(/Status-Benachrichtigungen/).closest("div");
    expect(notice).not.toBeNull();
    expect(
      within(notice as HTMLElement).getByText("Hinweis"),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("checkbox", {
        name: /Status-Benachrichtigungen/,
      }),
    ).not.toBeInTheDocument();
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

    // The pickup field only renders the child's actual care days. "Fixe
    // Betreuung" covers Tue/Thu, so Monday is not offered here -- enter the
    // time on a care day instead.
    fireEvent.change(screen.getByLabelText("Dienstag"), {
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
      dismissal: { tue: "15:00" },
    });
    expect(onSubmitted).toHaveBeenCalledWith("/status/abc");
  });

  it("limits pickup weekdays to the child's selected care days", async () => {
    renderForm();
    await waitForLoaded();

    // Before any care offering is chosen there are no care days, so the
    // pickup field prompts the parent to pick care days first instead of
    // showing all weekdays.
    expect(
      screen.getByText("Wählen Sie zuerst die Betreuungstage aus."),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText("Montag")).not.toBeInTheDocument();

    // "Fixe Betreuung" covers Tue/Thu only -> exactly those two weekdays
    // appear under the pickup field; Mon/Wed/Fri stay hidden.
    fireEvent.click(screen.getByRole("checkbox", { name: /Fixe Betreuung/ }));

    expect(screen.getByLabelText("Dienstag")).toBeInTheDocument();
    expect(screen.getByLabelText("Donnerstag")).toBeInTheDocument();
    expect(screen.queryByLabelText("Montag")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Mittwoch")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Freitag")).not.toBeInTheDocument();
  });

  it("drops pickup times for days the child is no longer in care", async () => {
    // Two fixed offerings change the care-day set deterministically (no day
    // picker needed).
    mockFetchPublicCareOfferings.mockResolvedValueOnce({
      offerings: [
        {
          id: "31",
          phase_id: "5",
          name: "Block Mo Di",
          description: null,
          days_of_week_mode: "fixed",
          available_days: ["mon", "tue"],
          includes_holiday_care: false,
          includes_lunch: false,
          is_active: true,
          is_required: false,
          capacity: null,
        },
        {
          id: "32",
          phase_id: "5",
          name: "Block Do",
          description: null,
          days_of_week_mode: "fixed",
          available_days: ["thu"],
          includes_holiday_care: false,
          includes_lunch: false,
          is_active: true,
          is_required: false,
          capacity: null,
        },
      ],
      careOfferingSelectionMode: "at_least_one",
      careRequired: true,
    });
    const onSubmitted = vi.fn();
    renderForm({ onSubmitted });
    await waitForLoaded();

    await fillRequiredFields();

    // Mo/Di care -> enter pickup times for both days.
    fireEvent.click(screen.getByRole("checkbox", { name: /Block Mo Di/ }));
    fireEvent.change(screen.getByLabelText("Montag"), {
      target: { value: "15:00" },
    });
    fireEvent.change(screen.getByLabelText("Dienstag"), {
      target: { value: "15:30" },
    });

    // Add Do, then drop Mo/Di so only Thu remains a care day. The Mon/Tue
    // times are now stale and must not survive into the payload.
    fireEvent.click(screen.getByRole("checkbox", { name: /Block Do/ }));
    fireEvent.click(screen.getByRole("checkbox", { name: /Block Mo Di/ }));

    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));

    await waitFor(() => {
      expect(mockSubmitEnrollment).toHaveBeenCalledTimes(1);
    });
    const [, payload] = mockSubmitEnrollment.mock.calls[0] as [
      string,
      SubmitEnrollmentPayload,
    ];
    expect(payload.children[0]?.custom_data?.dismissal).toEqual({});
  });

  it("requires a pickup time on every care day when offerings constrain the form", async () => {
    // Make the dismissal (weekday_schedule) field required; a single fixed
    // Mo/Di offering pins the care days so the field shows exactly two days.
    const requiredScheduleSchema = schema();
    requiredScheduleSchema.fields[3] = {
      ...requiredScheduleSchema.fields[3]!,
      required: true,
    };
    mockFetchPublicActiveSchema.mockResolvedValueOnce(requiredScheduleSchema);
    mockFetchPublicCareOfferings.mockResolvedValueOnce({
      offerings: [
        {
          id: "31",
          phase_id: "5",
          name: "Block Mo Di",
          description: null,
          days_of_week_mode: "fixed",
          available_days: ["mon", "tue"],
          includes_holiday_care: false,
          includes_lunch: false,
          is_active: true,
          is_required: false,
          capacity: null,
        },
      ],
      careOfferingSelectionMode: "at_least_one",
      careRequired: true,
    });
    renderForm();
    await waitForLoaded();
    await fillRequiredFields();

    fireEvent.click(screen.getByRole("checkbox", { name: /Block Mo Di/ }));

    // The hint now asks for a time per day; the old "keine Angabe" copy is gone.
    expect(
      screen.getByText("Bitte für jeden Betreuungstag eine Uhrzeit angeben."),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Leere Felder bedeuten: an diesem Tag keine Angabe."),
    ).not.toBeInTheDocument();

    // Only Monday filled -> Tuesday still empty -> submit is blocked with the
    // per-day error (not the lenient "at least one" message).
    fireEvent.change(screen.getByLabelText("Montag"), {
      target: { value: "15:00" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));
    // The per-day message shows in both the summary banner and at the field.
    expect(
      (
        await screen.findAllByText(
          "Bitte für jeden Betreuungstag eine Uhrzeit angeben.",
        )
      ).length,
    ).toBeGreaterThan(0);
    expect(mockSubmitEnrollment).not.toHaveBeenCalled();

    // Fill Tuesday too -> every care day has a time -> the form submits.
    fireEvent.change(screen.getByLabelText("Dienstag"), {
      target: { value: "15:30" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));
    await waitFor(() => expect(mockSubmitEnrollment).toHaveBeenCalledTimes(1));
  });

  it("keeps a required pickup schedule lenient when no offerings constrain the form", async () => {
    // No care offerings -> all weekdays shown, unconstrained. A required
    // weekday_schedule must then accept at least one filled day (a child may
    // attend only some weekdays), and the hint/error stay on the lenient copy.
    const requiredScheduleSchema = schema();
    requiredScheduleSchema.fields[3] = {
      ...requiredScheduleSchema.fields[3]!,
      required: true,
    };
    mockFetchPublicActiveSchema.mockResolvedValueOnce(requiredScheduleSchema);
    mockFetchPublicCareOfferings.mockResolvedValueOnce({
      offerings: [],
      careOfferingSelectionMode: "optional",
      careRequired: false,
    });
    renderForm();
    await waitForLoaded();
    await fillRequiredFields();

    // Required + unconstrained -> the lenient "at least one time" hint, which
    // matches the actual validation gate. Neither the optional "keine Angabe"
    // copy nor the per-day demand may appear.
    expect(
      screen.getByText("Bitte mindestens eine Uhrzeit angeben."),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Leere Felder bedeuten: an diesem Tag keine Angabe."),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText("Bitte für jeden Betreuungstag eine Uhrzeit angeben."),
    ).not.toBeInTheDocument();

    // All days empty -> required gate fails with the lenient "at least one" copy.
    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));
    expect(
      await screen.findAllByText("Bitte mindestens eine Uhrzeit angeben."),
    ).not.toHaveLength(0);
    expect(mockSubmitEnrollment).not.toHaveBeenCalled();

    // One day filled is enough when unconstrained -> the form submits.
    fireEvent.change(screen.getByLabelText("Montag"), {
      target: { value: "15:00" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));
    await waitFor(() => expect(mockSubmitEnrollment).toHaveBeenCalledTimes(1));
  });

  it("keeps an optional pickup schedule lenient even when offerings constrain the form", async () => {
    // Dismissal stays optional (base schema). A fixed Mo/Di offering constrains
    // the care days, so the field shows exactly two days -- but because it is
    // NOT required, the hint must stay on the lenient "keine Angabe" copy (not
    // the per-day demand) and an entirely empty schedule must still submit.
    mockFetchPublicCareOfferings.mockResolvedValueOnce({
      offerings: [
        {
          id: "31",
          phase_id: "5",
          name: "Block Mo Di",
          description: null,
          days_of_week_mode: "fixed",
          available_days: ["mon", "tue"],
          includes_holiday_care: false,
          includes_lunch: false,
          is_active: true,
          is_required: false,
          capacity: null,
        },
      ],
      careOfferingSelectionMode: "at_least_one",
      careRequired: true,
    });
    renderForm();
    await waitForLoaded();
    await fillRequiredFields();

    fireEvent.click(screen.getByRole("checkbox", { name: /Block Mo Di/ }));

    // Optional + constrained -> lenient hint, never the per-day demand.
    expect(
      screen.getByText("Leere Felder bedeuten: an diesem Tag keine Angabe."),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Bitte für jeden Betreuungstag eine Uhrzeit angeben."),
    ).not.toBeInTheDocument();

    // Empty schedule submits because the field is optional.
    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));
    await waitFor(() => expect(mockSubmitEnrollment).toHaveBeenCalledTimes(1));
  });

  // Grade-level eligibility (#1663): a phase aimed at whole grades must offer
  // only those grades, otherwise the parent fills in the entire form and is
  // rejected with grade_not_eligible on submit.
  it("offers only the phase's eligible grade levels", async () => {
    renderForm({
      prefetchedData: {
        schema: schema(),
        offerings: offerings(),
        careOfferingSelectionMode: "optional",
        captchaConfig: null,
        legalTexts: legalTexts(),
        eligibleGradeLevels: [3],
      },
    });
    await waitForLoaded();

    fireEvent.click(screen.getByLabelText("Klassenstufe *"));
    expect(
      await screen.findByRole("option", { name: "3. Klasse" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("option", { name: "2. Klasse" }),
    ).not.toBeInTheDocument();
  });

  it("offers every grade when the phase carries no grade restriction", async () => {
    renderForm({
      prefetchedData: {
        schema: schema(),
        offerings: offerings(),
        careOfferingSelectionMode: "optional",
        captchaConfig: null,
        legalTexts: legalTexts(),
        eligibleGradeLevels: [],
      },
    });
    await waitForLoaded();

    fireEvent.click(screen.getByLabelText("Klassenstufe *"));
    // gradeLevelMax is 4 in renderForm.
    for (const grade of ["1. Klasse", "2. Klasse", "3. Klasse", "4. Klasse"]) {
      expect(
        await screen.findByRole("option", { name: grade }),
      ).toBeInTheDocument();
    }
  });

  it("prefills from a parent profile and adopts an existing child", async () => {
    // Reuse is scoped to existing_students phases (#1663): on other audiences
    // adopting a linked child would fresh-create a duplicate on approval, so
    // the panel only appears here.
    renderForm({
      prefetchedData: {
        schema: schema(),
        offerings: offerings(),
        careOfferingSelectionMode: "optional",
        captchaConfig: null,
        legalTexts: legalTexts(),
        profile: profile(),
        audience: "existing_students",
      },
    });
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

  it("drops an adopted child's grade when the phase does not accept it", async () => {
    // The reused child sits in grade 2 but the phase is restricted to grade 3
    // (#1663). Keeping the prefill would put a value in the draft that the
    // select never offers: invisible to the parent, accepted by the
    // client-side "grade is set" check, and rejected at submit with
    // grade_not_eligible for the whole form. It has to fall back to empty so
    // the required-field check forces an eligible grade.
    renderForm({
      prefetchedData: {
        schema: schema(),
        offerings: offerings(),
        careOfferingSelectionMode: "optional",
        captchaConfig: null,
        legalTexts: legalTexts(),
        profile: profile(),
        audience: "existing_students",
        eligibleGradeLevels: [3],
      },
    });
    await waitForLoaded();

    fireEvent.click(screen.getByRole("button", { name: "Übernehmen" }));

    expect(screen.getAllByDisplayValue("Lina")[0]).toBeInTheDocument();
    const gradeSelect = screen.getByLabelText("Klassenstufe *");
    expect(gradeSelect).not.toHaveTextContent("2. Klasse");
    expect(gradeSelect).toHaveTextContent("Bitte wählen");
  });

  it("hides a linked child the guardian may not enroll (no submit permission)", async () => {
    // Portal visibility is not enrollment-submit permission (#1663): a child
    // whose relationship lacks parent_portal.enrollment.submit must not be
    // offered as reusable, else it would 403 only after the form is filled.
    const noSubmit = profile();
    noSubmit.children = noSubmit.children.map((c) => ({
      ...c,
      enrollment_submit: false,
    }));
    renderForm({
      prefetchedData: {
        schema: schema(),
        offerings: offerings(),
        careOfferingSelectionMode: "optional",
        captchaConfig: null,
        legalTexts: legalTexts(),
        profile: noSubmit,
        audience: "existing_students",
      },
    });
    await waitForLoaded();

    expect(screen.queryByText("Bestehende Kinder")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Übernehmen" }),
    ).not.toBeInTheDocument();
  });

  it("hides a linked child that is no longer enrolled (inactive)", async () => {
    // The profile lists every non-alumnus child, but the existing_students
    // gate only accepts active/pending students (#1663): an inactive child
    // offered as reusable would be adopted into the form and only then
    // rejected as "nicht eingeschrieben" by the backend.
    const inactive = profile();
    inactive.children = inactive.children.map((c) => ({
      ...c,
      status: "inactive",
    }));
    renderForm({
      prefetchedData: {
        schema: schema(),
        offerings: offerings(),
        careOfferingSelectionMode: "optional",
        captchaConfig: null,
        legalTexts: legalTexts(),
        profile: inactive,
        audience: "existing_students",
      },
    });
    await waitForLoaded();

    expect(screen.queryByText("Bestehende Kinder")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Übernehmen" }),
    ).not.toBeInTheDocument();
  });

  it("hides linked-child reuse and explains it on a new_students phase", async () => {
    // Parent portal path: a new_students phase rejects any already-enrolled
    // child at submit, so the reuse panel must not be offered (#1663).
    renderForm({
      prefetchedData: {
        schema: schema(),
        offerings: offerings(),
        careOfferingSelectionMode: "optional",
        captchaConfig: null,
        legalTexts: legalTexts(),
        profile: profile(),
        audience: "new_students",
      },
    });
    await waitForLoaded();

    expect(screen.queryByText("Bestehende Kinder")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Übernehmen" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText(/richtet sich nur an neue Kinder/),
    ).toBeInTheDocument();
  });

  it("keeps linked-child reuse on an existing_students phase", async () => {
    renderForm({
      prefetchedData: {
        schema: schema(),
        offerings: offerings(),
        careOfferingSelectionMode: "optional",
        captchaConfig: null,
        legalTexts: legalTexts(),
        profile: profile(),
        audience: "existing_students",
      },
    });
    await waitForLoaded();

    expect(screen.getByText("Bestehende Kinder")).toBeInTheDocument();
    expect(
      screen.queryByText(/richtet sich nur an neue Kinder/),
    ).not.toBeInTheDocument();
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
        "Kind 1: Beim Angebot „Flexible Betreuung“ muss mindestens ein Tag ausgewählt werden.",
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

  it("renders demo weekdays for allowed departure modes in template preview", async () => {
    const previewSchema = schema();
    previewSchema.fields = [
      {
        key: "allowed_departure_modes",
        label: "Erlaubte Heimwege",
        type: "weekday_multi_mode",
        required: true,
        applies_to_child: true,
        sort_order: 1,
      },
    ];

    renderForm({ phaseID: undefined, previewMode: true, previewSchema });
    await waitForLoaded();

    expect(
      screen.queryByText("Wählen Sie zuerst die Betreuungstage aus."),
    ).not.toBeInTheDocument();
    const group = screen.getByRole("group", { name: /Erlaubte Heimwege/ });
    expect(within(group).getAllByRole("button")).toHaveLength(5);
    // Only the "same ways home" toggle is present up front; no per-day mode
    // checkboxes appear until a day is expanded.
    expect(
      within(group).queryByRole("checkbox", { name: "Bus" }),
    ).not.toBeInTheDocument();

    fireEvent.click(within(group).getByRole("button", { name: "Mo" }));

    expect(
      within(group).getByRole("checkbox", { name: "Geht zu Fuß" }),
    ).toBeInTheDocument();
    expect(
      within(group).getByRole("checkbox", { name: "Bus" }),
    ).toBeInTheDocument();
    expect(
      within(group).getByRole("checkbox", { name: "Wird abgeholt" }),
    ).toBeInTheDocument();
    fireEvent.click(within(group).getByRole("checkbox", { name: "Bus" }));
    expect(within(group).getByRole("checkbox", { name: "Bus" })).toBeChecked();
  });

  it("applies one selection to every care day when 'same ways home' is on", async () => {
    const previewSchema = schema();
    previewSchema.fields = [
      {
        key: "allowed_departure_modes",
        label: "Erlaubte Heimwege",
        type: "weekday_multi_mode",
        required: true,
        applies_to_child: true,
        sort_order: 1,
      },
    ];

    renderForm({ phaseID: undefined, previewMode: true, previewSchema });
    await waitForLoaded();

    const group = screen.getByRole("group", { name: /Erlaubte Heimwege/ });
    const uniformToggle = within(group).getByRole("checkbox", {
      name: "Gleiche Heimwege für alle Betreuungstage",
    });

    // Turn the uniform switch on: the per-day weekday buttons disappear in
    // favour of a single shared selection block.
    fireEvent.click(uniformToggle);
    expect(
      within(group).queryByRole("button", { name: "Mo" }),
    ).not.toBeInTheDocument();

    fireEvent.click(within(group).getByRole("checkbox", { name: "Bus" }));

    // Switch back to per-day to confirm the choice fanned out to all five days.
    fireEvent.click(
      within(group).getByRole("checkbox", {
        name: "Gleiche Heimwege für alle Betreuungstage",
      }),
    );
    const busBoxes = within(group).getAllByRole("checkbox", { name: "Bus" });
    expect(busBoxes).toHaveLength(5);
    busBoxes.forEach((box) => expect(box).toBeChecked());
  });

  it("deselecting a uniform mode clears it from every care day", async () => {
    const previewSchema = schema();
    previewSchema.fields = [
      {
        key: "allowed_departure_modes",
        label: "Erlaubte Heimwege",
        type: "weekday_multi_mode",
        required: true,
        applies_to_child: true,
        sort_order: 1,
      },
    ];

    renderForm({ phaseID: undefined, previewMode: true, previewSchema });
    await waitForLoaded();

    const group = screen.getByRole("group", { name: /Erlaubte Heimwege/ });
    const uniformLabel = "Gleiche Heimwege für alle Betreuungstage";
    fireEvent.click(
      within(group).getByRole("checkbox", { name: uniformLabel }),
    );

    const bus = () => within(group).getByRole("checkbox", { name: "Bus" });
    fireEvent.click(bus());
    expect(bus()).toBeChecked();
    // Toggle the same mode off: it must drop out of the shared selection.
    fireEvent.click(bus());
    expect(bus()).not.toBeChecked();

    // Back in per-day view the cleared selection leaves no active care day.
    fireEvent.click(
      within(group).getByRole("checkbox", { name: uniformLabel }),
    );
    expect(
      within(group).queryByRole("checkbox", { name: "Bus" }),
    ).not.toBeInTheDocument();
  });

  it("confirms before discarding divergent per-day choices when enabling uniform", async () => {
    const previewSchema = schema();
    previewSchema.fields = [
      {
        key: "allowed_departure_modes",
        label: "Erlaubte Heimwege",
        type: "weekday_multi_mode",
        required: true,
        applies_to_child: true,
        sort_order: 1,
      },
    ];

    renderForm({ phaseID: undefined, previewMode: true, previewSchema });
    await waitForLoaded();

    const group = screen.getByRole("group", { name: /Erlaubte Heimwege/ });
    const uniformLabel = "Gleiche Heimwege für alle Betreuungstage";

    // Give a single day a selection so the days now diverge (Mo=Bus, rest none).
    fireEvent.click(within(group).getByRole("button", { name: "Mo" }));
    fireEvent.click(within(group).getByRole("checkbox", { name: "Bus" }));
    expect(within(group).getByRole("checkbox", { name: "Bus" })).toBeChecked();

    // Enabling uniform must NOT switch immediately; it prompts first and the
    // per-day weekday buttons stay visible.
    fireEvent.click(
      within(group).getByRole("checkbox", { name: uniformLabel }),
    );
    expect(
      screen.getByText("Auswahl für alle Tage übernehmen?"),
    ).toBeInTheDocument();
    expect(
      within(group).getByRole("button", { name: "Mo" }),
    ).toBeInTheDocument();

    // Confirm: switch to the shared block with a clean slate (Bus unchecked).
    fireEvent.click(
      screen.getByRole("button", { name: "Löschen und fortfahren" }),
    );
    expect(
      within(group).queryByRole("button", { name: "Mo" }),
    ).not.toBeInTheDocument();
    expect(
      within(group).getByRole("checkbox", { name: "Bus" }),
    ).not.toBeChecked();

    // Back in per-day view the prior Monday selection is gone.
    fireEvent.click(
      within(group).getByRole("checkbox", { name: uniformLabel }),
    );
    expect(
      within(group).queryByRole("checkbox", { name: "Bus" }),
    ).not.toBeInTheDocument();
  });

  it("keeps per-day choices when the uniform confirmation is cancelled", async () => {
    const previewSchema = schema();
    previewSchema.fields = [
      {
        key: "allowed_departure_modes",
        label: "Erlaubte Heimwege",
        type: "weekday_multi_mode",
        required: true,
        applies_to_child: true,
        sort_order: 1,
      },
    ];

    renderForm({ phaseID: undefined, previewMode: true, previewSchema });
    await waitForLoaded();

    const group = screen.getByRole("group", { name: /Erlaubte Heimwege/ });
    const uniformLabel = "Gleiche Heimwege für alle Betreuungstage";

    fireEvent.click(within(group).getByRole("button", { name: "Mo" }));
    fireEvent.click(within(group).getByRole("checkbox", { name: "Bus" }));

    fireEvent.click(
      within(group).getByRole("checkbox", { name: uniformLabel }),
    );
    // Cancel: stay in per-day view with Monday's selection intact.
    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));
    expect(
      within(group).getByRole("button", { name: "Mo" }),
    ).toBeInTheDocument();
    expect(within(group).getByRole("checkbox", { name: "Bus" })).toBeChecked();
  });

  it("toggling uniform adopts the selection when every day already matches", async () => {
    const previewSchema = schema();
    previewSchema.fields = [
      {
        key: "allowed_departure_modes",
        label: "Erlaubte Heimwege",
        type: "weekday_multi_mode",
        required: true,
        applies_to_child: true,
        sort_order: 1,
      },
    ];

    renderForm({ phaseID: undefined, previewMode: true, previewSchema });
    await waitForLoaded();

    const group = screen.getByRole("group", { name: /Erlaubte Heimwege/ });
    const uniformLabel = "Gleiche Heimwege für alle Betreuungstage";

    // Spread Bus to every day via uniform, then drop back to per-day so all
    // five days share the identical selection.
    fireEvent.click(
      within(group).getByRole("checkbox", { name: uniformLabel }),
    );
    fireEvent.click(within(group).getByRole("checkbox", { name: "Bus" }));
    fireEvent.click(
      within(group).getByRole("checkbox", { name: uniformLabel }),
    );
    expect(
      within(group).getAllByRole("checkbox", { name: "Bus" }),
    ).toHaveLength(5);

    // Re-enabling uniform is lossless here, so no confirmation appears and the
    // shared Bus selection is adopted as-is.
    fireEvent.click(
      within(group).getByRole("checkbox", { name: uniformLabel }),
    );
    expect(
      screen.queryByText("Auswahl für alle Tage übernehmen?"),
    ).not.toBeInTheDocument();
    expect(within(group).getByRole("checkbox", { name: "Bus" })).toBeChecked();
  });

  it("prompts to pick care days before showing the home-route options", async () => {
    // Real (non-preview) flow: offerings load but the child has selected none,
    // so there are no care days and the field shows the placeholder instead of
    // the uniform toggle or weekday buttons.
    const customSchema = schema();
    customSchema.fields = [
      {
        key: "allowed_departure_modes",
        label: "Erlaubte Heimwege",
        type: "weekday_multi_mode",
        required: true,
        applies_to_child: true,
        sort_order: 1,
      },
    ];
    mockFetchPublicActiveSchema.mockResolvedValue(customSchema);

    renderForm();
    await waitForLoaded();

    const group = screen.getByRole("group", { name: /Erlaubte Heimwege/ });
    expect(
      within(group).getByText("Wählen Sie zuerst die Betreuungstage aus."),
    ).toBeInTheDocument();
    expect(within(group).queryByRole("checkbox")).not.toBeInTheDocument();
    expect(within(group).queryByRole("button")).not.toBeInTheDocument();
  });

  it("fans the uniform selection onto a care day added after uniform was enabled", async () => {
    // Real flow: the set of care days is driven by the selected offerings and
    // can grow AFTER uniform was switched on. A newly added day starts empty,
    // so without the sync effect the uniform block would claim coverage the
    // payload doesn't have. Critically the new day ("Mo") sorts BEFORE the
    // existing ones ("Di"/"Do"), which would make a naive "first day is the
    // source of truth" implementation wipe the existing selection instead.
    // Two fixed-day offerings drive the care days directly (no day picker).
    const customSchema = schema();
    customSchema.fields = [
      {
        key: "allowed_departure_modes",
        label: "Erlaubte Heimwege",
        type: "weekday_multi_mode",
        required: true,
        applies_to_child: true,
        sort_order: 1,
      },
    ];
    mockFetchPublicActiveSchema.mockResolvedValue(customSchema);
    mockFetchPublicCareOfferings.mockResolvedValue({
      offerings: [
        {
          id: "21",
          phase_id: "5",
          name: "Block Mo",
          description: null,
          days_of_week_mode: "fixed",
          available_days: ["mon"],
          includes_holiday_care: false,
          includes_lunch: false,
          is_active: true,
          is_required: false,
          capacity: null,
        },
        {
          id: "22",
          phase_id: "5",
          name: "Block Di Do",
          description: null,
          days_of_week_mode: "fixed",
          available_days: ["tue", "thu"],
          includes_holiday_care: false,
          includes_lunch: false,
          is_active: true,
          is_required: false,
          capacity: null,
        },
      ],
      careOfferingSelectionMode: "at_least_one",
      careRequired: true,
    });

    renderForm();
    await waitForLoaded();

    // Select the Di/Do block so the home-route field has two care days.
    fireEvent.click(screen.getByRole("checkbox", { name: /Block Di Do/ }));

    const group = screen.getByRole("group", { name: /Erlaubte Heimwege/ });
    const uniformLabel = "Gleiche Heimwege für alle Betreuungstage";

    // Turn uniform on (lossless: both days are empty) and choose Bus.
    fireEvent.click(
      within(group).getByRole("checkbox", { name: uniformLabel }),
    );
    fireEvent.click(within(group).getByRole("checkbox", { name: "Bus" }));
    expect(within(group).getByRole("checkbox", { name: "Bus" })).toBeChecked();

    // Add the Mo block: "mon" sorts before the existing Di/Do and starts empty.
    fireEvent.click(screen.getByRole("checkbox", { name: /Block Mo/ }));

    // The uniform block still shows Bus (existing selection preserved, not
    // wiped by the empty new day becoming the would-be "first day").
    expect(within(group).getByRole("checkbox", { name: "Bus" })).toBeChecked();

    // Switch back to per-day: Bus must now cover all three days (Mo/Di/Do).
    fireEvent.click(
      within(group).getByRole("checkbox", { name: uniformLabel }),
    );
    const busBoxes = within(group).getAllByRole("checkbox", { name: "Bus" });
    expect(busBoxes).toHaveLength(3);
    busBoxes.forEach((box) => expect(box).toBeChecked());
  });

  it("keeps the uniform selection when the entire care-day set is swapped out", async () => {
    // Real flow, harder than the "day added" case: every source day is removed
    // and a disjoint new day appears (Di/Do -> Mo) while uniform is on. The
    // shared selection lives in component state, not derived from the per-day
    // payload, so it survives even though no source day remains to read it from
    // -- a payload-derived value would read empty mid-swap and silently reset.
    const customSchema = schema();
    customSchema.fields = [
      {
        key: "allowed_departure_modes",
        label: "Erlaubte Heimwege",
        type: "weekday_multi_mode",
        required: true,
        applies_to_child: true,
        sort_order: 1,
      },
    ];
    mockFetchPublicActiveSchema.mockResolvedValue(customSchema);
    mockFetchPublicCareOfferings.mockResolvedValue({
      offerings: [
        {
          id: "21",
          phase_id: "5",
          name: "Block Mo",
          description: null,
          days_of_week_mode: "fixed",
          available_days: ["mon"],
          includes_holiday_care: false,
          includes_lunch: false,
          is_active: true,
          is_required: false,
          capacity: null,
        },
        {
          id: "22",
          phase_id: "5",
          name: "Block Di Do",
          description: null,
          days_of_week_mode: "fixed",
          available_days: ["tue", "thu"],
          includes_holiday_care: false,
          includes_lunch: false,
          is_active: true,
          is_required: false,
          capacity: null,
        },
      ],
      careOfferingSelectionMode: "at_least_one",
      careRequired: true,
    });

    renderForm();
    await waitForLoaded();

    fireEvent.click(screen.getByRole("checkbox", { name: /Block Di Do/ }));

    const group = screen.getByRole("group", { name: /Erlaubte Heimwege/ });
    const uniformLabel = "Gleiche Heimwege für alle Betreuungstage";

    fireEvent.click(
      within(group).getByRole("checkbox", { name: uniformLabel }),
    );
    fireEvent.click(within(group).getByRole("checkbox", { name: "Bus" }));
    expect(within(group).getByRole("checkbox", { name: "Bus" })).toBeChecked();

    // Drop the source days FIRST (no care days at all), THEN add the disjoint
    // Mo block. The selection has no per-day payload to be re-derived from at
    // any point during the swap.
    fireEvent.click(screen.getByRole("checkbox", { name: /Block Di Do/ }));
    fireEvent.click(screen.getByRole("checkbox", { name: /Block Mo/ }));

    // The shared Bus selection is intact and fanned onto the new day.
    expect(within(group).getByRole("checkbox", { name: "Bus" })).toBeChecked();

    fireEvent.click(
      within(group).getByRole("checkbox", { name: uniformLabel }),
    );
    const busBoxes = within(group).getAllByRole("checkbox", { name: "Bus" });
    expect(busBoxes).toHaveLength(1);
    busBoxes.forEach((box) => expect(box).toBeChecked());
  });

  it("falls back to raw weekday and mode keys when the locale lacks labels", async () => {
    // Render with a catalog missing the weekday/departure label maps so the
    // component's `labels[key] ?? key` fallbacks render the raw keys in both
    // the uniform and per-day views. `localizedCopy` routes the component
    // through next-intl (the mocked catalog) instead of its bundled German
    // copy, so the stripped pseudo-locale takes effect.
    mockIntlLocale.value = "de-no-labels";

    const previewSchema = schema();
    previewSchema.fields = [
      {
        key: "allowed_departure_modes",
        label: "Erlaubte Heimwege",
        type: "weekday_multi_mode",
        required: true,
        applies_to_child: true,
        sort_order: 1,
      },
    ];

    renderForm({
      phaseID: undefined,
      previewMode: true,
      previewSchema,
      localizedCopy: true,
    });
    await waitForLoaded();

    const group = screen.getByRole("group", { name: /Erlaubte Heimwege/ });
    const uniformLabel = "Gleiche Heimwege für alle Betreuungstage";

    // Uniform view: mode checkbox falls back to the raw mode key.
    fireEvent.click(
      within(group).getByRole("checkbox", { name: uniformLabel }),
    );
    expect(
      within(group).getByRole("checkbox", { name: "bus" }),
    ).toBeInTheDocument();

    // Per-day view: weekday button, day header, and mode checkbox fall back.
    fireEvent.click(
      within(group).getByRole("checkbox", { name: uniformLabel }),
    );
    fireEvent.click(within(group).getByRole("button", { name: "mon" }));
    expect(
      within(group).getByRole("button", { name: "mon" }),
    ).toBeInTheDocument();
    expect(
      within(group).getByRole("checkbox", { name: "bus" }),
    ).toBeInTheDocument();
  });

  it("prefills every child from an edit draft", async () => {
    // Exercises the draftChildren rebuild path (initialDraft -> ChildDraft[]):
    // each saved child must reappear with its own controlled values, and each
    // gets a deterministic clientId so the cards don't share identity.
    renderForm({
      initialDraft: editDraft([
        { id: "c-1", first_name: "Anton", last_name: "Alster" },
        { id: "c-2", first_name: "Berta", last_name: "Bach" },
      ]),
    });
    await waitForLoaded();

    // Index 0 is the guardian's own name field; the two children follow.
    const firstNames = screen.getAllByLabelText("Vorname *");
    const lastNames = screen.getAllByLabelText("Nachname *");
    expect(firstNames).toHaveLength(3);
    expect((firstNames[1] as HTMLInputElement).value).toBe("Anton");
    expect((lastNames[1] as HTMLInputElement).value).toBe("Alster");
    expect((firstNames[2] as HTMLInputElement).value).toBe("Berta");
    expect((lastNames[2] as HTMLInputElement).value).toBe("Bach");
  });

  it("adds a conditional required offering when hydrating an edit draft", async () => {
    const submitter = vi.fn().mockResolvedValue({ status_url: "/status/edit" });
    const noFieldSchema = schema();
    noFieldSchema.fields = [];
    const conditionalRequired: PublicCareOffering = {
      ...offerings()[0]!,
      id: "99",
      name: "Pflicht-Randstunde",
      days_of_week_mode: "fixed",
      available_days: ["mon"],
      is_required: true,
      availability_rule: {
        match: "all",
        conditions: [{ source: "grade_level", operator: "in", value: [1, 2] }],
      },
    };
    mockFetchPublicActiveSchema.mockResolvedValueOnce(noFieldSchema);
    mockFetchPublicLegalTexts.mockResolvedValueOnce(legalTexts([]));
    mockFetchPublicCareOfferings.mockResolvedValueOnce({
      offerings: [offerings()[1]!, conditionalRequired],
      careOfferingSelectionMode: "optional",
      careRequired: false,
      collectGradeLevel: true,
      careOfferingsEnabled: true,
    });

    renderForm({
      submitter,
      skipCaptcha: true,
      initialDraft: editDraft([
        { id: "c-1", first_name: "Anton", last_name: "Alster" },
      ]),
    });
    await waitForLoaded();

    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));

    await waitFor(() => expect(submitter).toHaveBeenCalledOnce());
    const payload = submitter.mock.calls[0]![0] as SubmitEnrollmentPayload;
    expect(payload.children[0]!.offering_ids).toEqual(
      expect.arrayContaining([12, 99]),
    );
  });

  it("does not turn automatic-only edit-draft offerings into manual selections", async () => {
    const submitter = vi.fn().mockResolvedValue({ status_url: "/status/edit" });
    const noFieldSchema = schema();
    noFieldSchema.fields = [];
    mockFetchPublicActiveSchema.mockResolvedValueOnce(noFieldSchema);
    mockFetchPublicLegalTexts.mockResolvedValueOnce(legalTexts([]));
    mockFetchPublicCareOfferings.mockResolvedValueOnce({
      offerings: [
        {
          id: "11",
          phase_id: "5",
          name: "Ganztag",
          description: null,
          days_of_week_mode: "parent_choice",
          available_days: ["mon", "wed"],
          includes_holiday_care: false,
          includes_lunch: false,
          is_active: true,
          is_required: false,
          counts_as_care: true,
          capacity: null,
        },
        {
          id: "22",
          phase_id: "5",
          name: "Randstunde",
          description: null,
          days_of_week_mode: "parent_choice",
          available_days: ["mon", "wed"],
          includes_holiday_care: false,
          includes_lunch: false,
          is_active: true,
          is_required: false,
          counts_as_care: false,
          auto_add_trigger_offering_ids: ["11"],
          capacity: null,
        },
      ],
      careOfferingSelectionMode: "optional",
      careRequired: false,
    });

    renderForm({
      submitter,
      skipCaptcha: true,
      initialDraft: {
        ...editDraft([{ id: "c-1", first_name: "Anton", last_name: "Alster" }]),
        consent_flags: {},
        children: [
          {
            id: "c-1",
            first_name: "Anton",
            last_name: "Alster",
            date_of_birth: "2018-04-15",
            target_grade_level: 2,
            offering_ids: ["11", "22"],
            offering_days: [
              {
                offering_id: "11",
                selected_days: ["mon"],
                manual_selected_days: ["mon"],
              },
              {
                offering_id: "22",
                selected_days: ["mon"],
                automatic_selected_days: ["mon"],
              },
            ],
          },
        ],
      },
    });
    await waitForLoaded();

    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));

    await waitFor(() => {
      expect(submitter).toHaveBeenCalledTimes(1);
    });
    const payload = submitter.mock.calls[0]?.[0] as SubmitEnrollmentPayload;
    expect(payload.children[0]?.offering_ids).toEqual([11]);
    expect(payload.children[0]?.offering_days).toEqual([
      { offering_id: 11, selected_days: ["mon"] },
    ]);
  });

  it("keeps each child's card state bound to that child when a sibling is removed", async () => {
    // Regression for the array-index key bug: the per-child WeekdayMultiModeInput
    // holds local-only state (the "same ways home" toggle). With index keys,
    // removing the first child would shift the survivor onto the removed slot's
    // React subtree and leak its toggle state. The stable clientId keeps each
    // card's local state with its own child.
    const customSchema = schema();
    customSchema.fields = [
      {
        key: "allowed_departure_modes",
        label: "Erlaubte Heimwege",
        type: "weekday_multi_mode",
        required: true,
        applies_to_child: true,
        sort_order: 1,
      },
    ];
    mockFetchPublicActiveSchema.mockResolvedValue(customSchema);

    renderForm({
      initialDraft: editDraft([
        { id: "c-1", first_name: "Anton", last_name: "Alster" },
        { id: "c-2", first_name: "Berta", last_name: "Bach" },
      ]),
    });
    await waitForLoaded();

    const uniformLabel = "Gleiche Heimwege für alle Betreuungstage";
    let toggles = screen.getAllByRole("checkbox", { name: uniformLabel });
    expect(toggles).toHaveLength(2);

    // Turn uniform on for the SECOND child (Berta) only.
    fireEvent.click(toggles[1]!);
    toggles = screen.getAllByRole("checkbox", { name: uniformLabel });
    expect(toggles[0]).not.toBeChecked();
    expect(toggles[1]).toBeChecked();

    // Remove the FIRST child (Anton). Berta becomes the only card.
    fireEvent.click(screen.getAllByRole("button", { name: "Entfernen" })[0]!);

    // Berta survived (index 0 is the guardian, index 1 the single remaining
    // child) and her uniform toggle is still on -- it did not leak onto Anton's
    // old slot and Anton's off-state did not bleed onto Berta.
    const remaining = screen.getAllByLabelText("Vorname *");
    expect(remaining).toHaveLength(2);
    expect((remaining[1] as HTMLInputElement).value).toBe("Berta");
    expect(screen.getByRole("checkbox", { name: uniformLabel })).toBeChecked();
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

  // The accompanied mode + coupled "mit wem" note is the headline of #1694.
  // A single fixed Monday offering keeps the home-route field to one care day.
  function accompaniedModeSchemaAndOfferings() {
    const customSchema = schema();
    customSchema.fields = [
      {
        key: "allowed_modes",
        label: "Erlaubte Heimwege",
        type: "weekday_multi_mode",
        target: "student.allowed_departure_modes",
        required: true,
        applies_to_child: true,
        sort_order: 1,
      },
    ];
    mockFetchPublicActiveSchema.mockResolvedValue(customSchema);
    mockFetchPublicCareOfferings.mockResolvedValue({
      offerings: [
        {
          id: "21",
          phase_id: "5",
          name: "Block Mo",
          description: null,
          days_of_week_mode: "fixed",
          available_days: ["mon"],
          includes_holiday_care: false,
          includes_lunch: false,
          is_active: true,
          is_required: false,
          capacity: null,
        },
      ],
      careOfferingSelectionMode: "at_least_one",
      careRequired: true,
    });
  }

  it("submits the coupled companion note when an accompanied day is selected (#1694)", async () => {
    accompaniedModeSchemaAndOfferings();
    renderForm();
    await waitForLoaded();
    await fillRequiredFields();
    fireEvent.click(screen.getByRole("checkbox", { name: /Block Mo/ }));

    const group = screen.getByRole("group", { name: /Erlaubte Heimwege/ });
    // No accompanied day yet → the "mit wem" input is not shown.
    expect(
      within(group).queryByPlaceholderText(/Geschwisterkind, Freund, Name/),
    ).not.toBeInTheDocument();

    fireEvent.click(within(group).getByRole("button", { name: "Mo" }));
    fireEvent.click(
      within(group).getByRole("checkbox", { name: "Mit anderem Kind" }),
    );
    const noteInput = within(group).getByPlaceholderText(
      /Geschwisterkind, Freund, Name/,
    );
    fireEvent.change(noteInput, { target: { value: "Geschwisterkind Mia" } });

    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));
    await waitFor(() => expect(mockSubmitEnrollment).toHaveBeenCalledTimes(1));
    const [, payload] = mockSubmitEnrollment.mock.calls[0] as [
      string,
      SubmitEnrollmentPayload,
    ];
    expect(payload.children[0]?.custom_data?.allowed_modes).toMatchObject({
      mon: ["accompanied"],
    });
    expect(
      payload.children[0]?.custom_data?.["student.departure_companion_note"],
    ).toBe("Geschwisterkind Mia");
  });

  it("does not offer accompanied for untargeted weekday multi-mode fields (#1694)", async () => {
    const customSchema = schema();
    customSchema.fields = [
      {
        key: "custom_modes",
        label: "Erlaubte Heimwege",
        type: "weekday_multi_mode",
        required: true,
        applies_to_child: true,
        sort_order: 1,
      },
    ];
    mockFetchPublicActiveSchema.mockResolvedValue(customSchema);
    mockFetchPublicCareOfferings.mockResolvedValue({
      offerings: [
        {
          id: "21",
          phase_id: "5",
          name: "Block Mo",
          description: null,
          days_of_week_mode: "fixed",
          available_days: ["mon"],
          includes_holiday_care: false,
          includes_lunch: false,
          is_active: true,
          is_required: false,
          capacity: null,
        },
      ],
      careOfferingSelectionMode: "at_least_one",
      careRequired: true,
    });

    renderForm({
      initialDraft: {
        ...editDraft([{ id: "c-1", first_name: "Lina", last_name: "Muster" }]),
        consent_flags: { agb: true, data_processing: true },
        children: [
          {
            id: "c-1",
            first_name: "Lina",
            last_name: "Muster",
            date_of_birth: "2018-04-15",
            target_grade_level: 2,
            offering_ids: ["21"],
            custom_data: {
              custom_modes: { mon: ["bus", "accompanied"] },
              "student.departure_companion_note": "Geschwisterkind Mia",
            },
          },
        ],
      },
    });
    await waitForLoaded();

    const group = screen.getByRole("group", { name: /Erlaubte Heimwege/ });
    expect(
      within(group).queryByRole("checkbox", { name: "Mit anderem Kind" }),
    ).not.toBeInTheDocument();
    expect(
      within(group).queryByPlaceholderText(/Geschwisterkind, Freund, Name/),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));
    await waitFor(() => expect(mockSubmitEnrollment).toHaveBeenCalledTimes(1));
    const [, payload] = mockSubmitEnrollment.mock.calls[0] as [
      string,
      SubmitEnrollmentPayload,
    ];
    expect(payload.children[0]?.custom_data?.custom_modes).toEqual({
      mon: ["bus"],
    });
    expect(
      payload.children[0]?.custom_data?.["student.departure_companion_note"],
    ).toBeUndefined();
  });

  // Regression (#1694): a schema may carry more than one field targeting
  // student.allowed_departure_modes (only keys are unique). The companion note
  // must couple back in and be enforced when accompanied is chosen in ANY of
  // them, not only the first — a single .find() on the first field dropped the
  // note (and skipped the required-note check) when accompanied lived in a
  // later field, so the backend rejected an otherwise-complete submission.
  function twoDepartureFieldsSchemaAndOfferings() {
    const customSchema = schema();
    customSchema.fields = [
      {
        key: "modes_a",
        label: "Heimweg A",
        type: "weekday_multi_mode",
        target: "student.allowed_departure_modes",
        required: true,
        applies_to_child: true,
        sort_order: 1,
      },
      {
        key: "modes_b",
        label: "Heimweg B",
        type: "weekday_multi_mode",
        target: "student.allowed_departure_modes",
        required: true,
        applies_to_child: true,
        sort_order: 2,
      },
    ];
    mockFetchPublicActiveSchema.mockResolvedValue(customSchema);
    mockFetchPublicCareOfferings.mockResolvedValue({
      offerings: [
        {
          id: "21",
          phase_id: "5",
          name: "Block Mo",
          description: null,
          days_of_week_mode: "fixed",
          available_days: ["mon"],
          includes_holiday_care: false,
          includes_lunch: false,
          is_active: true,
          is_required: false,
          capacity: null,
        },
      ],
      careOfferingSelectionMode: "at_least_one",
      careRequired: true,
    });
  }

  it("couples the companion note when accompanied is selected in a later departure field (#1694)", async () => {
    twoDepartureFieldsSchemaAndOfferings();
    renderForm();
    await waitForLoaded();
    await fillRequiredFields();
    fireEvent.click(screen.getByRole("checkbox", { name: /Block Mo/ }));

    // Field A gets a non-accompanied mode so the .find()-on-first-field bug
    // would see no accompanied and drop the note.
    const groupA = screen.getByRole("group", { name: /Heimweg A/ });
    fireEvent.click(within(groupA).getByRole("button", { name: "Mo" }));
    fireEvent.click(within(groupA).getByRole("checkbox", { name: "Bus" }));

    // Accompanied lives only in field B.
    const groupB = screen.getByRole("group", { name: /Heimweg B/ });
    fireEvent.click(within(groupB).getByRole("button", { name: "Mo" }));
    fireEvent.click(
      within(groupB).getByRole("checkbox", { name: "Mit anderem Kind" }),
    );
    fireEvent.change(
      within(groupB).getByPlaceholderText(/Geschwisterkind, Freund, Name/),
      { target: { value: "Geschwisterkind Mia" } },
    );

    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));
    await waitFor(() => expect(mockSubmitEnrollment).toHaveBeenCalledTimes(1));
    const [, payload] = mockSubmitEnrollment.mock.calls[0] as [
      string,
      SubmitEnrollmentPayload,
    ];
    expect(payload.children[0]?.custom_data?.modes_b).toMatchObject({
      mon: ["accompanied"],
    });
    expect(
      payload.children[0]?.custom_data?.["student.departure_companion_note"],
    ).toBe("Geschwisterkind Mia");
  });

  it("requires the note when accompanied is selected only in a later departure field (#1694)", async () => {
    twoDepartureFieldsSchemaAndOfferings();
    renderForm();
    await waitForLoaded();
    await fillRequiredFields();
    fireEvent.click(screen.getByRole("checkbox", { name: /Block Mo/ }));

    const groupA = screen.getByRole("group", { name: /Heimweg A/ });
    fireEvent.click(within(groupA).getByRole("button", { name: "Mo" }));
    fireEvent.click(within(groupA).getByRole("checkbox", { name: "Bus" }));

    const groupB = screen.getByRole("group", { name: /Heimweg B/ });
    fireEvent.click(within(groupB).getByRole("button", { name: "Mo" }));
    fireEvent.click(
      within(groupB).getByRole("checkbox", { name: "Mit anderem Kind" }),
    );

    // Note left empty → submit must be blocked even though accompanied lives in
    // the second departure field.
    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));
    await waitFor(() =>
      expect(
        screen.getAllByText(
          "Bitte angeben, mit welchem Kind das Kind nach Hause geht.",
        ).length,
      ).toBeGreaterThan(0),
    );
    expect(mockSubmitEnrollment).not.toHaveBeenCalled();
  });

  it("blocks submit with a required-note error when accompanied is selected but the note is empty (#1694)", async () => {
    accompaniedModeSchemaAndOfferings();
    renderForm();
    await waitForLoaded();
    await fillRequiredFields();
    fireEvent.click(screen.getByRole("checkbox", { name: /Block Mo/ }));

    const group = screen.getByRole("group", { name: /Erlaubte Heimwege/ });
    fireEvent.click(within(group).getByRole("button", { name: "Mo" }));
    fireEvent.click(
      within(group).getByRole("checkbox", { name: "Mit anderem Kind" }),
    );
    // Note left empty → submit must be blocked with the per-field error.
    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));
    await waitFor(() =>
      expect(
        screen.getAllByText(
          "Bitte angeben, mit welchem Kind das Kind nach Hause geht.",
        ).length,
      ).toBeGreaterThan(0),
    );
    expect(mockSubmitEnrollment).not.toHaveBeenCalled();

    // Filling the note clears the block and submit goes through.
    fireEvent.change(
      within(group).getByPlaceholderText(/Geschwisterkind, Freund, Name/),
      { target: { value: "Geschwisterkind Mia" } },
    );
    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));
    await waitFor(() => expect(mockSubmitEnrollment).toHaveBeenCalledTimes(1));
  });

  it("drops the companion note from the payload once accompanied is deselected (#1694)", async () => {
    accompaniedModeSchemaAndOfferings();
    renderForm();
    await waitForLoaded();
    await fillRequiredFields();
    fireEvent.click(screen.getByRole("checkbox", { name: /Block Mo/ }));

    const group = screen.getByRole("group", { name: /Erlaubte Heimwege/ });
    fireEvent.click(within(group).getByRole("button", { name: "Mo" }));
    fireEvent.click(
      within(group).getByRole("checkbox", { name: "Mit anderem Kind" }),
    );
    fireEvent.change(
      within(group).getByPlaceholderText(/Geschwisterkind, Freund, Name/),
      { target: { value: "Geschwisterkind Mia" } },
    );
    // Change of mind: switch the day to a non-accompanied mode. The note input
    // disappears and its value must NOT ride along on submit.
    fireEvent.click(
      within(group).getByRole("checkbox", { name: "Mit anderem Kind" }),
    );
    fireEvent.click(within(group).getByRole("checkbox", { name: "Bus" }));
    expect(
      within(group).queryByPlaceholderText(/Geschwisterkind, Freund, Name/),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));
    await waitFor(() => expect(mockSubmitEnrollment).toHaveBeenCalledTimes(1));
    const [, payload] = mockSubmitEnrollment.mock.calls[0] as [
      string,
      SubmitEnrollmentPayload,
    ];
    expect(
      payload.children[0]?.custom_data?.["student.departure_companion_note"],
    ).toBeUndefined();
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

  it("localizes weekday departure mode labels and validation copy", async () => {
    mockIntlLocale.value = "en";
    const withDeparture = schema();
    withDeparture.fields = [
      ...withDeparture.fields,
      {
        key: "departure",
        label: "Geh-/Abholregelung",
        type: "weekday_mode",
        target: "student.departure",
        required: true,
        applies_to_child: true,
        sort_order: 7,
      },
    ];
    mockFetchPublicActiveSchema.mockResolvedValueOnce(withDeparture);
    mockFetchPublicCareOfferings.mockResolvedValueOnce({
      offerings: [],
      careOfferingSelectionMode: "optional",
      careRequired: false,
    });

    renderForm({ localizedCopy: true });

    expect(await screen.findByText("Geh-/Abholregelung")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Choose how your child goes home for each weekday. The default is “Walks home”.",
      ),
    ).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "Walks home" })).toHaveLength(
      5,
    );
    expect(screen.getAllByRole("button", { name: "Bus" })).toHaveLength(5);
    expect(screen.getAllByRole("button", { name: "Picked up" })).toHaveLength(
      5,
    );

    fireEvent.click(screen.getByRole("button", { name: "Submit enrollment" }));

    expect(
      await screen.findAllByText("Please confirm the departure arrangement."),
    ).not.toHaveLength(0);
    expect(mockSubmitEnrollment).not.toHaveBeenCalled();
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

  it("auto-fills required lunch days from selected care days without submitting them as manual picks", async () => {
    const submitter = vi.fn().mockResolvedValue({ status_url: "/status/req" });
    mockFetchPublicCareOfferings.mockResolvedValueOnce({
      offerings: [
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
          counts_as_care: true,
          capacity: 20,
        },
        {
          id: "20",
          phase_id: "5",
          name: "Mittagessen",
          description: null,
          days_of_week_mode: "parent_choice",
          available_days: ["mon", "tue", "wed", "thu", "fri"],
          includes_holiday_care: false,
          includes_lunch: true,
          is_active: true,
          is_required: true,
          counts_as_care: false,
          capacity: null,
        },
      ],
      careOfferingSelectionMode: "optional",
      careRequired: false,
    });
    renderForm({ submitter, skipCaptcha: true });
    await waitForLoaded();

    fireEvent.click(screen.getByRole("checkbox", { name: /Fixe Betreuung/ }));
    expect(screen.queryByText("Di, Do automatisch")).not.toBeInTheDocument();

    await fillRequiredFields();
    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));

    await waitFor(() => {
      expect(submitter).toHaveBeenCalledTimes(1);
    });
    const payload = submitter.mock.calls[0]?.[0] as SubmitEnrollmentPayload;
    expect(payload.children[0]?.offering_ids).toEqual(
      expect.arrayContaining([12, 20]),
    );
    expect(payload.children[0]?.offering_days).toBeUndefined();
    expect(
      screen.queryByText("Bitte mindestens einen Tag auswählen."),
    ).toBeNull();
  });

  it("shows auto-added offerings as selected without exposing day provenance", async () => {
    mockFetchPublicCareOfferings.mockResolvedValueOnce({
      offerings: [
        {
          id: "11",
          phase_id: "5",
          name: "Ganztag",
          description: null,
          days_of_week_mode: "parent_choice",
          available_days: ["mon", "tue", "wed", "thu", "fri"],
          includes_holiday_care: false,
          includes_lunch: false,
          is_active: true,
          is_required: false,
          counts_as_care: true,
          capacity: null,
        },
        {
          id: "22",
          phase_id: "5",
          name: "Randstunde",
          description: null,
          days_of_week_mode: "parent_choice",
          available_days: ["mon", "tue", "wed", "thu", "fri"],
          includes_holiday_care: false,
          includes_lunch: false,
          is_active: true,
          is_required: false,
          counts_as_care: false,
          auto_add_trigger_offering_ids: ["11"],
          auto_add_grade_levels: [1, 2],
          capacity: null,
        },
      ],
      careOfferingSelectionMode: "optional",
      careRequired: false,
    });
    renderForm();
    await waitForLoaded();
    await chooseOption("Klassenstufe *", "2. Klasse");

    const ganztagInput = document.querySelector<HTMLInputElement>(
      'input[name="children_0_offering_11"]',
    );
    expect(ganztagInput).not.toBeNull();
    fireEvent.click(ganztagInput as HTMLInputElement);
    for (const day of ["Mo", "Di", "Mi", "Do"]) {
      fireEvent.click(dayButton(offeringCard("children_0_offering_11"), day));
    }
    const randstundeInput = document.querySelector<HTMLInputElement>(
      'input[name="children_0_offering_22"]',
    );
    expect(randstundeInput).not.toBeNull();
    expect(randstundeInput).toBeChecked();
    expect(randstundeInput).toBeDisabled();
    expect(
      screen.queryByText("Mo, Di, Mi, Do automatisch"),
    ).not.toBeInTheDocument();

    fireEvent.click(dayButton(offeringCard("children_0_offering_22"), "Fr"));

    expect(
      screen.queryByText("Mo, Di, Mi, Do automatisch; Fr manuell"),
    ).not.toBeInTheDocument();
    expect(
      dayButton(offeringCard("children_0_offering_22"), "Mo"),
    ).toBeDisabled();
    expect(
      dayButton(offeringCard("children_0_offering_22"), "Fr"),
    ).toHaveAttribute("aria-pressed", "true");
  });

  it("validates selection groups against auto-added offerings", async () => {
    mockFetchPublicCareOfferings.mockResolvedValueOnce({
      offerings: [
        {
          id: "11",
          phase_id: "5",
          name: "Ganztag",
          description: null,
          days_of_week_mode: "parent_choice",
          available_days: ["mon", "tue", "wed", "thu", "fri"],
          includes_holiday_care: false,
          includes_lunch: false,
          is_active: true,
          is_required: false,
          counts_as_care: true,
          capacity: null,
        },
        {
          id: "22",
          phase_id: "5",
          name: "Frühbetreuung",
          description: null,
          days_of_week_mode: "parent_choice",
          available_days: ["mon", "tue", "wed", "thu", "fri"],
          includes_holiday_care: false,
          includes_lunch: false,
          is_active: true,
          is_required: false,
          counts_as_care: false,
          selection_group: "Randzeiten",
          selection_rule: "at_most_one",
          auto_add_trigger_offering_ids: ["11"],
          capacity: null,
        },
        {
          id: "33",
          phase_id: "5",
          name: "Spätbetreuung",
          description: null,
          days_of_week_mode: "parent_choice",
          available_days: ["mon", "tue", "wed", "thu", "fri"],
          includes_holiday_care: false,
          includes_lunch: false,
          is_active: true,
          is_required: false,
          counts_as_care: false,
          selection_group: "Randzeiten",
          selection_rule: "at_most_one",
          capacity: null,
        },
      ],
      careOfferingSelectionMode: "optional",
      careRequired: false,
    });
    renderForm();
    await waitForLoaded();
    await fillRequiredFields();

    const ganztagInput = document.querySelector<HTMLInputElement>(
      'input[name="children_0_offering_11"]',
    );
    expect(ganztagInput).not.toBeNull();
    fireEvent.click(ganztagInput as HTMLInputElement);
    fireEvent.click(dayButton(offeringCard("children_0_offering_11"), "Mo"));
    fireEvent.click(screen.getByRole("checkbox", { name: /Spätbetreuung/ }));
    fireEvent.click(dayButton(offeringCard("children_0_offering_33"), "Mo"));
    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));

    expect(
      await screen.findByText(
        "Kind 1: Bitte bei „Randzeiten“ höchstens ein Angebot wählen.",
      ),
    ).toBeInTheDocument();
    expect(mockSubmitEnrollment).not.toHaveBeenCalled();
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

describe("EnrollmentForm — fixed pickup times", () => {
  // A schema variant whose per-child weekday_schedule field is the
  // pickup-times target constrained to a fixed list. Reuses the shared
  // schema() and only rewrites the weekday_schedule field (index 3).
  function pickupSchema(
    allowedTimes: string[] = ["14:45", "16:00"],
  ): PublicFormSchema {
    const s = schema();
    s.fields[3] = {
      ...s.fields[3]!,
      key: "schedule_pickup",
      label: "Abholzeit",
      type: "weekday_schedule",
      target: "schedule.pickup",
      applies_to_child: true,
      allowed_times: allowedTimes,
    };
    return s;
  }

  // A minimal valid draft that seeds one child whose pickup answer is
  // pre-filled — used to exercise the stale-value and off-list-rejection
  // paths, which a fresh form can't reach through the dropdown.
  function draftWithPickup(
    schedule: Record<string, string>,
  ): EnrollmentEditDraft {
    return {
      request_id: "1",
      status_token: "tok",
      tenant_id: "1",
      tenant_subdomain: "test-tenant",
      phase_id: "5",
      guardian_first_name: "Mara",
      guardian_last_name: "Muster",
      guardian_email: "mara@example.test",
      guardian_phone: "+49 221 1234567",
      consent_flags: { agb: true, data_processing: true },
      custom_data: {},
      children: [
        {
          id: "stu-1",
          first_name: "Lina",
          last_name: "Muster",
          date_of_birth: "2018-04-15",
          target_grade_level: 2,
          offering_ids: [],
          custom_data: { schedule_pickup: schedule },
        },
      ],
    };
  }

  beforeEach(() => {
    mockIntlLocale.value = "de";
    mockFetchPublicActiveSchema.mockReset();
    mockFetchPublicCaptchaConfig.mockReset();
    mockFetchPublicLegalTexts.mockReset();
    mockFetchPublicCareOfferings.mockReset();
    mockFetchMyEnrollmentProfile.mockReset();
    mockSubmitEnrollment.mockReset();
    mockFetchPublicActiveSchema.mockResolvedValue(pickupSchema());
    mockFetchPublicCaptchaConfig.mockResolvedValue({
      enabled: false,
      site_key: "",
    });
    mockFetchPublicLegalTexts.mockResolvedValue(legalTexts([]));
    mockFetchPublicCareOfferings.mockResolvedValue({
      offerings: [],
      careOfferingSelectionMode: "optional",
      careRequired: false,
    });
    mockFetchMyEnrollmentProfile.mockResolvedValue(null);
    mockSubmitEnrollment.mockResolvedValue({ status_url: "/status/abc" });
  });

  it("renders a dropdown limited to the configured pickup times", async () => {
    renderForm();
    await waitForLoaded();

    // The constrained field must NOT render the free-entry time input.
    expect(document.querySelectorAll('input[type="time"]')).toHaveLength(0);

    // The Monday dropdown offers exactly the configured times — nothing else.
    fireEvent.click(screen.getByLabelText("Montag – Abholzeit"));
    expect(
      await screen.findByRole("option", { name: "14:45" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "16:00" })).toBeInTheDocument();
    expect(
      screen.queryByRole("option", { name: "15:00" }),
    ).not.toBeInTheDocument();
  });

  it("falls back to a free time input when no fixed times are configured", async () => {
    // allowed_times empty → historical free-entry behaviour.
    mockFetchPublicActiveSchema.mockResolvedValue(pickupSchema([]));
    renderForm();
    await waitForLoaded();

    expect(
      document.querySelectorAll('input[type="time"]').length,
    ).toBeGreaterThan(0);
    // No constrained dropdown is rendered for the field.
    expect(
      screen.queryByLabelText("Montag – Abholzeit"),
    ).not.toBeInTheDocument();
  });

  it("keeps a saved off-list time visible and flags it as unavailable", async () => {
    // The admin shortened the list after this draft was saved: 12:00 is no
    // longer offered. It must stay visible (not silently blanked) and be
    // tagged so the parent re-picks it before submit.
    renderForm({ initialDraft: draftWithPickup({ mon: "12:00" }) });
    await waitForLoaded();

    expect(screen.getByText(/12:00.*nicht mehr verfügbar/)).toBeInTheDocument();
  });

  it("marks the offending pickup field when the backend rejects an off-list time", async () => {
    // Server-side defense-in-depth: a stale 12:00 reaches the backend, which
    // rejects it with the stable code. The form must mark the offending
    // schedule field (red), not only show the banner.
    const message =
      "Bitte wähle bei den Abholzeiten nur Uhrzeiten aus der vorgegebenen Liste.";
    mockSubmitEnrollment.mockRejectedValueOnce(
      Object.assign(new Error(message), {
        code: "enrollment.pickup_time_not_allowed",
      }),
    );

    renderForm({ initialDraft: draftWithPickup({ mon: "12:00" }) });
    await waitForLoaded();

    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));

    await waitFor(() => expect(mockSubmitEnrollment).toHaveBeenCalled());
    // The message appears in the error banner AND on the marked field, so a
    // banner-only fallback (length 1) fails this assertion.
    await waitFor(() =>
      expect(screen.getAllByText(message).length).toBeGreaterThanOrEqual(2),
    );
  });

  it("marks a constrained pickup field even when the rejected time still looks valid client-side", async () => {
    // Stale client schema: the admin removed 14:45 from the allowed list AFTER
    // this form loaded, so the client still considers 14:45 valid (it is in
    // allowed_times and the dropdown offers it). The backend validates against
    // the latest phase and rejects it. The precise off-list pass finds nothing,
    // so without the conservative fallback the rejection would be swallowed to
    // a banner-only message and no field would turn red.
    const message =
      "Bitte wähle bei den Abholzeiten nur Uhrzeiten aus der vorgegebenen Liste.";
    mockSubmitEnrollment.mockRejectedValueOnce(
      Object.assign(new Error(message), {
        code: "enrollment.pickup_time_not_allowed",
      }),
    );

    // 14:45 is in the client's allowed list — the precise pass cannot flag it.
    renderForm({ initialDraft: draftWithPickup({ mon: "14:45" }) });
    await waitForLoaded();

    fireEvent.click(screen.getByRole("button", { name: "Anmeldung absenden" }));

    await waitFor(() => expect(mockSubmitEnrollment).toHaveBeenCalled());
    await waitFor(() =>
      expect(screen.getAllByText(message).length).toBeGreaterThanOrEqual(2),
    );
  });
});
