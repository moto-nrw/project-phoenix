import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const mocks = vi.hoisted(() => ({
  createCareOffering: vi.fn(),
  createPhase: vi.fn(),
  createSchema: vi.fn(),
  cloneCareOffering: vi.fn(),
  deleteCareOffering: vi.fn(),
  deletePhase: vi.fn(),
  deleteSchema: vi.fn(),
  fetchPublicLegalTexts: vi.fn(),
  listCareOfferings: vi.fn(),
  listPhases: vi.fn(),
  listSchemas: vi.fn(),
  updateCareOffering: vi.fn(),
  updatePhase: vi.fn(),
  updateSchema: vi.fn(),
  getTemplates: vi.fn(),
  searchParams: new URLSearchParams(),
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

vi.mock("next/navigation", () => ({
  useSearchParams: () => mocks.searchParams,
}));

vi.mock("~/components/tenant/tenant-provider", () => ({
  useTenantSlugSafe: () => "demo",
  useNFCEnabled: vi.fn(() => true),
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => mocks.toast,
}));

vi.mock("./rollover-form", () => ({
  RolloverForm: ({
    onCancel,
    onSuccess,
  }: {
    onCancel: () => void;
    onSuccess: (result: {
      phase: { name: string };
      summary: {
        rolled_count: number;
        review_count: number;
        enqueued_emails: number;
      };
    }) => void;
  }) => (
    <div>
      <button type="button" onClick={onCancel}>
        Rollover abbrechen
      </button>
      <button
        type="button"
        onClick={() =>
          onSuccess({
            phase: { name: "Schuljahr 2027/28" },
            summary: {
              rolled_count: 2,
              review_count: 1,
              enqueued_emails: 3,
            },
          })
        }
      >
        Rollover fertig
      </button>
    </div>
  ),
}));

vi.mock("~/lib/enrollment-phase-api", async (importOriginal) => {
  const actual = (await importOriginal()) as Record<string, unknown>;
  return {
    ...actual,
    createPhase: mocks.createPhase,
    deletePhase: mocks.deletePhase,
    listPhases: mocks.listPhases,
    updatePhase: mocks.updatePhase,
  };
});

vi.mock("~/lib/enrollment-form-schema-api", async (importOriginal) => {
  const actual = (await importOriginal()) as Record<string, unknown>;
  return {
    ...actual,
    createSchema: mocks.createSchema,
    deleteSchema: mocks.deleteSchema,
    fetchPublicLegalTexts: mocks.fetchPublicLegalTexts,
    listSchemas: mocks.listSchemas,
    updateSchema: mocks.updateSchema,
  };
});

vi.mock("~/lib/care-offering-api", () => ({
  cloneCareOffering: mocks.cloneCareOffering,
  createCareOffering: mocks.createCareOffering,
  deleteCareOffering: mocks.deleteCareOffering,
  listCareOfferings: mocks.listCareOfferings,
  updateCareOffering: mocks.updateCareOffering,
  // Real const consumed by CareOfferingForm's selection-rule dropdown.
  SELECTION_RULE_LABELS: {
    optional: "Optional (frei wählbar)",
    exactly_one: "Genau eines (Pflicht)",
    at_least_one: "Mindestens eines (Pflicht)",
    at_most_one: "Höchstens eines",
  },
}));

vi.mock("~/lib/timetable-api", () => ({
  timetableService: {
    getTemplates: mocks.getTemplates,
  },
}));

import { CareOfferingsEditor } from "./care-offerings-editor";
import { EnrollmentFormEditor } from "./enrollment-form-editor";
import { PhasesEditor } from "./phases-editor";
import type { CareOffering } from "~/lib/care-offering-api";
import type { FormSchema } from "~/lib/enrollment-form-schema-api";
import type { Phase } from "~/lib/enrollment-phase-api";

function phase(overrides: Partial<Phase> = {}): Phase {
  return {
    id: "10",
    name: "Schuljahr 2026/27",
    kind: "school_year",
    service_start_date: "2026-09-01",
    service_end_date: "2027-07-31",
    enrollment_open_at: "2026-02-01T08:00:00.000Z",
    enrollment_close_at: "2026-03-01T18:00:00.000Z",
    form_schema_id: "schema-1",
    show_status_reason_to_parent: false,
    care_overflow_mode: "waitlist",
    care_offering_selection_mode: "optional",
    is_active: true,
    created_at: "2026-01-01T00:00:00.000Z",
    updated_at: "2026-01-01T00:00:00.000Z",
    ...overrides,
  };
}

function schema(overrides: Partial<FormSchema> = {}): FormSchema {
  return {
    id: "schema-1",
    name: "Regelformular",
    version: 2,
    is_active: true,
    fields: [
      {
        key: "allergies",
        label: "Allergien",
        type: "textarea",
        required: true,
        applies_to_child: true,
        sort_order: 0,
        target: "student.health_info",
      },
    ],
    created_by: "1",
    created_at: "2026-01-01T00:00:00.000Z",
    ...overrides,
  };
}

function offering(overrides: Partial<CareOffering> = {}): CareOffering {
  return {
    id: "offer-1",
    phase_id: "10",
    activity_group_id: null,
    name: "Regelbetreuung",
    description: "Montag bis Freitag",
    days_of_week_mode: "fixed",
    available_days: ["mon", "tue", "wed", "thu", "fri"],
    includes_holiday_care: false,
    includes_lunch: true,
    capacity: 20,
    price_cents: 12500,
    is_active: true,
    is_required: false,
    sort_order: 0,
    created_at: "2026-01-01T00:00:00.000Z",
    updated_at: "2026-01-01T00:00:00.000Z",
    ...overrides,
  };
}

function inputByName(name: string): HTMLInputElement {
  return document.querySelector(`input[name="${name}"]`) as HTMLInputElement;
}

function textareaByName(name: string): HTMLTextAreaElement {
  return document.querySelector(
    `textarea[name="${name}"]`,
  ) as HTMLTextAreaElement;
}

async function waitForInputByName(name: string): Promise<HTMLInputElement> {
  await waitFor(() => expect(inputByName(name)).toBeInTheDocument());
  return inputByName(name);
}

async function chooseOption(
  label: string | RegExp,
  optionName: string | RegExp,
) {
  fireEvent.click(screen.getByLabelText(label));
  fireEvent.click(await screen.findByRole("option", { name: optionName }));
}

beforeEach(() => {
  mocks.createCareOffering.mockReset();
  mocks.createPhase.mockReset();
  mocks.createSchema.mockReset();
  mocks.cloneCareOffering.mockReset();
  mocks.deleteCareOffering.mockReset();
  mocks.deletePhase.mockReset();
  mocks.deleteSchema.mockReset();
  mocks.fetchPublicLegalTexts.mockReset();
  // Without this mock the editor's loadAll() fires a real fetch
  // (ECONNREFUSED in CI). Empty blocks = no tenant legal texts
  // configured, so the standard blocks stay disabled by default.
  mocks.fetchPublicLegalTexts.mockResolvedValue({
    agb: "",
    dsgvo: "",
    email_contact: "",
    photo: "",
    terms_enabled: false,
    dsgvo_enabled: false,
    email_contact_enabled: false,
    photo_enabled: false,
    blocks: [],
  });
  mocks.listCareOfferings.mockReset();
  mocks.listPhases.mockReset();
  mocks.listSchemas.mockReset();
  mocks.updateCareOffering.mockReset();
  mocks.updatePhase.mockReset();
  mocks.updateSchema.mockReset();
  mocks.getTemplates.mockReset();
  mocks.getTemplates.mockResolvedValue({ templates: [] });
  mocks.toast.success.mockReset();
  mocks.toast.error.mockReset();
  mocks.searchParams = new URLSearchParams();
  Object.defineProperty(window, "confirm", {
    value: vi.fn(() => true),
    configurable: true,
    writable: true,
  });
});

describe("CareOfferingsEditor", () => {
  it("creates, edits, deletes, and clones offerings", async () => {
    mocks.listPhases.mockResolvedValue([
      phase(),
      phase({ id: "11", name: "Ferien" }),
    ]);
    mocks.listCareOfferings.mockResolvedValue([offering()]);
    mocks.createCareOffering.mockResolvedValue(
      offering({ id: "new", name: "Frühbetreuung" }),
    );
    mocks.updateCareOffering.mockResolvedValue(
      offering({ name: "Spätbetreuung" }),
    );
    mocks.deleteCareOffering.mockResolvedValue(undefined);
    mocks.cloneCareOffering.mockResolvedValue(
      offering({ id: "clone", name: "Kopie" }),
    );

    render(<CareOfferingsEditor />);

    expect(await screen.findByText("Regelbetreuung")).toBeInTheDocument();
    expect(screen.getByText(/125,00/)).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Neues Betreuungsangebot" }),
    );
    fireEvent.change(await waitForInputByName("name"), {
      target: { value: "Frühbetreuung" },
    });
    fireEvent.change(textareaByName("description"), {
      target: { value: "Ab 7 Uhr" },
    });
    fireEvent.change(inputByName("capacity"), { target: { value: "12" } });
    fireEvent.change(inputByName("price_cents"), { target: { value: "9900" } });
    fireEvent.click(screen.getByText("Ferienbetreuung"));
    fireEvent.click(screen.getByRole("button", { name: "Erstellen" }));

    await waitFor(() => {
      expect(mocks.createCareOffering).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "Frühbetreuung",
          description: "Ab 7 Uhr",
          capacity: 12,
          price_cents: 9900,
          includes_holiday_care: true,
        }),
      );
    });
    await waitFor(() => {
      expect(mocks.listCareOfferings.mock.calls.length).toBeGreaterThanOrEqual(
        2,
      );
    });
  });

  it("locks and clears capacity when an offering is marked required", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings.mockResolvedValue([offering()]);
    mocks.createCareOffering.mockResolvedValue(
      offering({
        id: "req",
        name: "Mittagessen",
        is_required: true,
        capacity: null,
      }),
    );

    render(<CareOfferingsEditor />);
    expect(await screen.findByText("Regelbetreuung")).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Neues Betreuungsangebot" }),
    );
    fireEvent.change(inputByName("name"), {
      target: { value: "Mittagessen" },
    });
    fireEvent.change(inputByName("capacity"), { target: { value: "30" } });

    // Marking the offering as required clears any capacity and locks the field.
    fireEvent.click(screen.getByText("Pflicht"));
    expect(inputByName("capacity")).toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "Erstellen" }));
    await waitFor(() => {
      expect(mocks.createCareOffering).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "Mittagessen",
          is_required: true,
          capacity: null,
        }),
      );
    });
  });

  it("shows warnings for timetable template mismatches", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings.mockResolvedValue([offering()]);
    mocks.getTemplates.mockResolvedValue({
      templates: [
        {
          id: "8",
          name: "Lernzeit",
          type: "care",
          categoryId: "cat-1",
          categoryName: "Betreuung",
          isOpen: true,
          maxParticipants: 20,
          enrollmentCount: 0,
          supervisorCount: 0,
          studentIds: [],
          staffIds: [],
          schedules: [
            {
              id: "sch-1",
              weekday: 1,
              startTime: "13:00",
              endTime: "14:00",
              weekPattern: 0,
              calendarPeriodId: "period-1",
            },
            {
              id: "sch-2",
              weekday: 6,
              startTime: "13:00",
              endTime: "14:00",
              weekPattern: 0,
            },
          ],
        },
      ],
    });

    render(<CareOfferingsEditor />);
    expect(await screen.findByText("Regelbetreuung")).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Neues Betreuungsangebot" }),
    );
    await waitForInputByName("name");
    await chooseOption("Stundenplan-Vorlage", /Lernzeit/);

    expect(
      screen.getByText(
        "Das Angebot enthält Tage, an denen die ausgewählte Vorlage keinen Slot hat.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Die Vorlage enthält Tage, die im Angebot nicht auswählbar sind.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Die Vorlage muss genau eine Planungsperiode für alle Slots verwenden.",
      ),
    ).toBeInTheDocument();
  });

  it("shows empty and error states", async () => {
    mocks.listPhases.mockResolvedValueOnce([]);
    mocks.listCareOfferings.mockResolvedValue([]);

    const { unmount } = render(<CareOfferingsEditor />);
    expect(
      await screen.findByText("Erst eine Anmeldephase anlegen"),
    ).toBeInTheDocument();
    unmount();

    mocks.listPhases.mockRejectedValueOnce(new Error("Phasen kaputt"));
    render(<CareOfferingsEditor />);
    expect(await screen.findByRole("alert")).toHaveTextContent("Phasen kaputt");
  });
});

describe("PhasesEditor", () => {
  it("creates a phase, assigns schemas, toggles status, deletes, and completes rollover", async () => {
    mocks.searchParams = new URLSearchParams("assignForm=schema-1");
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.createPhase.mockResolvedValue(
      phase({ id: "12", name: "Sommerferien" }),
    );
    mocks.updatePhase.mockResolvedValue(phase({ is_active: false }));
    mocks.deletePhase.mockResolvedValue(undefined);

    render(<PhasesEditor />);

    expect(await screen.findByText("Schuljahr 2026/27")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Neue Anmeldephase" }));
    fireEvent.change(inputByName("name"), {
      target: { value: "Sommerferien" },
    });
    await chooseOption("Typ", "Ferienbetreuung");
    fireEvent.click(
      document.querySelector("#schema-source-reuse") as HTMLInputElement,
    );
    await chooseOption("Formular auswählen", "Regelformular");
    fireEvent.click(screen.getByText("Begründung für Eltern sichtbar"));
    fireEvent.click(screen.getByRole("button", { name: "Erstellen" }));

    await waitFor(() => {
      expect(mocks.createPhase).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "Sommerferien",
          kind: "holiday",
          form_schema_id: "schema-1",
          show_status_reason_to_parent: true,
        }),
      );
    });
    await waitFor(() => {
      expect(mocks.listPhases.mock.calls.length).toBeGreaterThanOrEqual(2);
    });
  });

  it("validates date windows before saving", async () => {
    mocks.listPhases.mockResolvedValue([]);
    mocks.listSchemas.mockResolvedValue([schema()]);

    render(<PhasesEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Erste Anmeldephase anlegen" }),
    );
    fireEvent.change(inputByName("name"), {
      target: { value: "Falsches Fenster" },
    });
    fireEvent.change(inputByName("enrollment_open_at"), {
      target: { value: "2026-03-10T10:00" },
    });
    fireEvent.change(inputByName("enrollment_close_at"), {
      target: { value: "2026-03-09T10:00" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Erstellen" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Schließung des Anmeldefensters muss nach der Öffnung liegen.",
    );
    expect(mocks.createPhase).not.toHaveBeenCalled();
  });

  it("shows a German validation error when the phase name is blank", async () => {
    mocks.listPhases.mockResolvedValue([]);
    mocks.listSchemas.mockResolvedValue([schema()]);

    render(<PhasesEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Erste Anmeldephase anlegen" }),
    );
    fireEvent.change(inputByName("name"), {
      target: { value: "   " },
    });
    fireEvent.click(screen.getByRole("button", { name: "Erstellen" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Bitte gib einen Namen für die Anmeldephase ein.",
    );
    expect(mocks.toast.error).toHaveBeenCalledWith(
      "Bitte gib einen Namen für die Anmeldephase ein.",
    );
    expect(mocks.createPhase).not.toHaveBeenCalled();
  });
});

// The editor always sends the template's legal blocks on save (PR #1632).
// With no tenant legal texts configured (see the fetchPublicLegalTexts
// default above), the four standard blocks are passed through disabled.
const defaultLegalBlocksForSave = [
  expect.objectContaining({ key: "agb", enabled: false, source: "standard" }),
  expect.objectContaining({
    key: "data_processing",
    enabled: false,
    source: "standard",
  }),
  expect.objectContaining({
    key: "photo",
    enabled: false,
    source: "standard",
  }),
  expect.objectContaining({
    key: "email_contact",
    enabled: false,
    source: "standard",
  }),
];

describe("EnrollmentFormEditor", () => {
  it("creates, previews, updates, and deletes schemas", async () => {
    const initialSchema = schema();
    mocks.listSchemas.mockResolvedValue([initialSchema]);
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.createSchema.mockResolvedValue(
      schema({ id: "schema-new", name: "Kontaktformular" }),
    );
    mocks.updateSchema.mockResolvedValue(schema({ name: "Regelformular" }));
    mocks.deleteSchema.mockResolvedValue(undefined);

    render(<EnrollmentFormEditor />);

    expect(
      await screen.findByText("Anmeldeformulare verwalten"),
    ).toBeInTheDocument();
    expect(screen.getByText("Regelformular")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Neue Vorlage" }));
    fireEvent.change(
      screen.getByPlaceholderText("z. B. Ferienbetreuung Sommer 2026"),
      {
        target: { value: "Kontaktformular" },
      },
    );
    fireEvent.click(screen.getByRole("button", { name: "Freie Zusatzfrage" }));
    fireEvent.change(screen.getByLabelText("Frage im Elternformular"), {
      target: { value: "Lieblingsessen" },
    });
    await chooseOption("Typ", "Auswahl");
    fireEvent.change(screen.getByLabelText("Auswahloptionen"), {
      target: { value: "Pasta\nReis" },
    });
    fireEvent.click(screen.getByText("Pflichtfrage"));
    fireEvent.click(
      screen.getByRole("button", { name: "Formularvorlage erstellen" }),
    );

    await waitFor(() => {
      expect(mocks.createSchema).toHaveBeenCalledWith(
        "Kontaktformular",
        expect.arrayContaining([
          expect.objectContaining({
            key: "lieblingsessen",
            label: "Lieblingsessen",
            type: "select",
            required: true,
            options: [
              { label: "Pasta", value: "pasta" },
              { label: "Reis", value: "reis" },
            ],
          }),
        ]),
        {},
        defaultLegalBlocksForSave,
      );
    });

    fireEvent.click(
      screen.getByRole("button", { name: "Zurück zur Übersicht" }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Regelformular" }),
    );
    fireEvent.click(await screen.findByRole("menuitem", { name: "Prüfen" }));
    expect(screen.getByText("Zuletzt gespeichert")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    fireEvent.change(screen.getByLabelText("Frage im Elternformular"), {
      target: { value: "Medizinische Hinweise" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Änderungen speichern" }),
    );
    await waitFor(() => {
      expect(mocks.updateSchema).toHaveBeenCalledWith(
        "schema-1",
        expect.arrayContaining([expect.objectContaining({ type: "textarea" })]),
        {},
        defaultLegalBlocksForSave,
      );
    });

    fireEvent.click(
      screen.getByRole("button", { name: "Zurück zur Übersicht" }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Regelformular" }),
    );
    fireEvent.click(await screen.findByRole("menuitem", { name: "Löschen" }));
    fireEvent.click(screen.getByRole("button", { name: "Löschen" }));

    await waitFor(() => {
      expect(mocks.deleteSchema).toHaveBeenCalledWith("schema-1");
    });
  });

  it("activates a standard legal block when an admin enters legal text", async () => {
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.createSchema.mockResolvedValue(
      schema({ id: "schema-new", name: "Rechtstextformular" }),
    );

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    fireEvent.change(
      screen.getByPlaceholderText("z. B. Ferienbetreuung Sommer 2026"),
      {
        target: { value: "Rechtstextformular" },
      },
    );
    fireEvent.click(
      screen.getAllByRole("button", { name: /abweichend bearbeiten/i })[0]!,
    );
    const legalTextAreas = await screen.findAllByLabelText(
      "Rechtstext / Erklärung",
    );
    fireEvent.change(legalTextAreas[0] as HTMLTextAreaElement, {
      target: { value: "AGB Text aus der Vorlage" },
    });

    fireEvent.click(
      screen.getByRole("button", { name: "Formularvorlage erstellen" }),
    );

    await waitFor(() => {
      expect(mocks.createSchema).toHaveBeenCalled();
    });
    const [, , , legalBlocks] = mocks.createSchema.mock.calls[0] as [
      string,
      unknown,
      unknown,
      Array<{ key: string; enabled: boolean; text: string }>,
    ];
    expect(legalBlocks).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          key: "agb",
          enabled: true,
          text: "AGB Text aus der Vorlage",
        }),
      ]),
    );
  });

  it("shows standard legal blocks as inherited summaries until an admin edits an override", async () => {
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );

    expect(
      screen.getAllByText("Aus Einstellungen übernommen").length,
    ).toBeGreaterThan(0);
    expect(
      screen.queryByLabelText("Rechtstext / Erklärung"),
    ).not.toBeInTheDocument();

    fireEvent.click(
      screen.getAllByRole("button", { name: /abweichend bearbeiten/i })[0]!,
    );

    expect(screen.getByLabelText("Rechtstext / Erklärung")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Einstellungen wieder verwenden" }),
    ).toBeInTheDocument();
  });

  it("keeps disabled inherited legal text available when editing a standard block", async () => {
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.fetchPublicLegalTexts.mockResolvedValue({
      agb: "",
      dsgvo: "",
      email_contact: "",
      photo: "Foto-Rechtstext aus den Einstellungen",
      terms_enabled: false,
      dsgvo_enabled: false,
      email_contact_enabled: false,
      photo_enabled: false,
      blocks: [],
    });

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    expect(
      screen.getAllByText("Ist für diese Vorlage ausgeblendet.").length,
    ).toBeGreaterThan(0);

    fireEvent.click(
      screen.getByRole("button", {
        name: "Fotoeinwilligung abweichend bearbeiten",
      }),
    );

    expect(screen.getByLabelText("Rechtstext / Erklärung")).toHaveValue(
      "Foto-Rechtstext aus den Einstellungen",
    );
  });

  it("uses compact right-side controls for inherited standard legal blocks", async () => {
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );

    expect(
      screen.queryByText("In dieser Vorlage anzeigen"),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Abweichend bearbeiten")).not.toBeInTheDocument();

    const displaySwitches = screen.getAllByRole("switch", {
      name: /in dieser Vorlage anzeigen/i,
    });
    const editButtons = screen.getAllByRole("button", {
      name: /abweichend bearbeiten/i,
    });

    expect(displaySwitches[0]?.tagName).toBe("BUTTON");
    expect(displaySwitches[0]).toHaveClass("h-5", "w-9");
    fireEvent.click(displaySwitches[0]!);
    expect(displaySwitches[0]!.querySelector("span")).toHaveClass(
      "translate-x-[18px]",
    );
    expect(editButtons[0]).toHaveTextContent("");
    expect(editButtons[0]).not.toHaveClass("border", "bg-white", "shadow-sm");
  });

  it("restores missing standard legal blocks when editing an older schema", async () => {
    mocks.listSchemas.mockResolvedValue([
      schema({
        legal_blocks: [
          {
            key: "data_processing",
            kind: "privacy_notice",
            title: "Datenschutzinformation",
            label: "Ich habe die Datenschutzinformation gelesen.",
            text: "",
            required: true,
            enabled: false,
            sort_order: 20,
            source: "standard",
          },
          {
            key: "photo",
            kind: "consent",
            title: "Fotoeinwilligung",
            label: "Fotos dürfen verwendet werden.",
            text: "",
            required: false,
            enabled: false,
            sort_order: 30,
            source: "standard",
          },
          {
            key: "email_contact",
            kind: "notice",
            title: "Kontakt per E-Mail",
            label: "Wir kontaktieren Sie per E-Mail.",
            text: "",
            required: false,
            enabled: false,
            sort_order: 40,
            source: "standard",
          },
        ],
      }),
    ]);
    mocks.listPhases.mockResolvedValue([]);

    render(<EnrollmentFormEditor />);

    expect(await screen.findByText("Regelformular")).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Regelformular" }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );

    expect(screen.getByText("AGB / Teilnahmebedingungen")).toBeInTheDocument();
    expect(screen.getAllByText("Aus Einstellungen übernommen")).toHaveLength(4);
  });

  it("keeps legal block keys hidden and generates unique keys for new custom consents", async () => {
    mocks.listSchemas.mockResolvedValue([
      schema({
        legal_blocks: [
          {
            key: "agb",
            kind: "terms",
            title: "AGB / Teilnahmebedingungen",
            label: "Ich akzeptiere die AGB.",
            text: "",
            required: true,
            enabled: false,
            sort_order: 10,
            source: "standard",
          },
          {
            key: "data_processing",
            kind: "privacy_notice",
            title: "Datenschutzinformation",
            label: "Ich habe die Datenschutzinformation gelesen.",
            text: "",
            required: true,
            enabled: false,
            sort_order: 20,
            source: "standard",
          },
          {
            key: "photo",
            kind: "consent",
            title: "Fotoeinwilligung",
            label: "Fotos dürfen verwendet werden.",
            text: "",
            required: false,
            enabled: false,
            sort_order: 30,
            source: "standard",
          },
          {
            key: "email_contact",
            kind: "notice",
            title: "Kontakt per E-Mail",
            label: "Wir kontaktieren Sie per E-Mail.",
            text: "",
            required: false,
            enabled: false,
            sort_order: 40,
            source: "standard",
          },
          {
            key: "custom_consent_6",
            kind: "consent",
            title: "Schwimmbad",
            label: "Mein Kind darf ins Schwimmbad gehen.",
            text: "",
            required: false,
            enabled: true,
            sort_order: 50,
            source: "custom",
          },
        ],
      }),
    ]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.updateSchema.mockResolvedValue(schema({ name: "Regelformular" }));

    render(<EnrollmentFormEditor />);

    expect(await screen.findByText("Regelformular")).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Regelformular" }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );

    expect(
      screen.queryByLabelText("Interner Schlüssel"),
    ).not.toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Eigene Zustimmung hinzufügen" }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Änderungen speichern" }),
    );

    await waitFor(() => {
      expect(mocks.updateSchema).toHaveBeenCalled();
    });
    const [, , , legalBlocks] = mocks.updateSchema.mock.calls[0] as [
      string,
      unknown,
      unknown,
      Array<{ key: string; source: string }>,
    ];
    expect(
      legalBlocks
        .filter((block) => block.source === "custom")
        .map((block) => block.key),
    ).toEqual(["custom_consent_6", "custom_consent_7"]);
  });

  it("renders legal block toggles with the shared styled checkbox treatment", async () => {
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    fireEvent.click(
      screen.getAllByRole("button", { name: /abweichend bearbeiten/i })[0]!,
    );

    const displayToggles = screen.getAllByRole("checkbox", {
      name: "Im Formular anzeigen",
    });
    const requiredToggles = screen.getAllByRole("checkbox", {
      name: "Muss bestätigt werden",
    });

    expect(displayToggles[0]?.tagName).toBe("BUTTON");
    expect(requiredToggles[0]?.tagName).toBe("BUTTON");
    expect(displayToggles[0]).not.toHaveAttribute("type", "checkbox");
    expect(requiredToggles[0]).not.toHaveAttribute("type", "checkbox");
  });

  it("shows a simple checkbox-or-notice mode instead of legal block kind names", async () => {
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    fireEvent.click(
      screen.getAllByRole("button", { name: /abweichend bearbeiten/i })[0]!,
    );

    expect(
      screen.getAllByRole("button", { name: "Mit Checkbox" }).length,
    ).toBeGreaterThan(0);
    expect(
      screen.getAllByRole("button", { name: "Nur Hinweis" }).length,
    ).toBeGreaterThan(0);
    expect(
      screen.queryByRole("button", { name: "Einwilligung" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Datenschutz-Bestätigung" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "AGB / Teilnahmebedingungen" }),
    ).not.toBeInTheDocument();
  });

  it("shows enabled legal blocks in the compact form preview", async () => {
    mocks.listSchemas.mockResolvedValue([
      schema({
        legal_blocks: [
          {
            key: "custom_photo_trip",
            kind: "consent",
            title: "Fotoausflug",
            label: "Mein Kind darf beim Ausflug fotografiert werden.",
            text: "Details",
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
      }),
    ]);
    mocks.listPhases.mockResolvedValue([]);

    render(<EnrollmentFormEditor />);
    expect(await screen.findByText("Regelformular")).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Regelformular" }),
    );
    fireEvent.click(await screen.findByRole("menuitem", { name: "Prüfen" }));

    expect(screen.getByText("Zustimmungen & Hinweise")).toBeInTheDocument();
    expect(screen.getByText("Fotoausflug")).toBeInTheDocument();
    expect(screen.getAllByText("Pflicht").length).toBeGreaterThan(0);
    expect(screen.queryByText("Ausgeblendet")).not.toBeInTheDocument();
  });

  it("handles load and save errors", async () => {
    mocks.listSchemas.mockRejectedValueOnce(new Error("Schema kaputt"));
    mocks.listPhases.mockResolvedValue([]);

    const { rerender } = render(<EnrollmentFormEditor />);
    expect(await screen.findByText("Schema kaputt")).toBeInTheDocument();

    mocks.listSchemas.mockResolvedValueOnce([]);
    mocks.createSchema.mockRejectedValueOnce(new Error("Speichern kaputt"));
    rerender(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    fireEvent.change(
      screen.getByPlaceholderText("z. B. Ferienbetreuung Sommer 2026"),
      {
        target: { value: "Fehlerformular" },
      },
    );
    fireEvent.click(screen.getByRole("button", { name: "Freie Zusatzfrage" }));
    fireEvent.change(screen.getByLabelText("Frage im Elternformular"), {
      target: { value: "Hinweis" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Formularvorlage erstellen" }),
    );

    expect(await screen.findByText("Speichern kaputt")).toBeInTheDocument();
  });
});
