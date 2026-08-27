import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { StrictMode } from "react";

const mocks = vi.hoisted(() => ({
  createCareOffering: vi.fn(),
  createLateInvite: vi.fn(),
  createManualApprovedEnrollment: vi.fn(),
  createPhase: vi.fn(),
  createSchema: vi.fn(),
  cloneCareOffering: vi.fn(),
  deleteCareOffering: vi.fn(),
  deletePhase: vi.fn(),
  deleteSchema: vi.fn(),
  deleteEnrollmentLegalDocument: vi.fn(),
  fetchManualEnrollmentBootstrap: vi.fn(),
  fetchPublicLegalTexts: vi.fn(),
  listCareOfferings: vi.fn(),
  listPhaseExpiryWarnings: vi.fn(),
  listPhases: vi.fn(),
  listSchemas: vi.fn(),
  renameSchema: vi.fn(),
  updateCareOffering: vi.fn(),
  updatePhase: vi.fn(),
  updateSchema: vi.fn(),
  uploadEnrollmentLegalDocument: vi.fn(),
  getTemplates: vi.fn(),
  listCalendarPeriods: vi.fn(),
  fetchCareOfferingBookingStats: vi.fn(),
  gradeLevelMax: 13,
  enrollmentFormProps: [] as Array<{
    lockChildStructure?: boolean;
    gradeLevelMax?: number;
  }>,
  searchParams: new URLSearchParams(),
  routerReplace: vi.fn(),
  toast: {
    success: vi.fn(),
    error: vi.fn(),
    warning: vi.fn(),
  },
}));

// The date and datetime fields moved from native inputs to the kit pickers;
// these stubs keep them settable via fireEvent.change. Imported inside the
// factories because vi.mock is hoisted above the imports.
vi.mock("~/components/ui/date-picker", async (importOriginal) => {
  const { isoDatePickerMock } = await import("~/test/mocks/date-picker");
  return { ...(await importOriginal<object>()), ...isoDatePickerMock() };
});

vi.mock("~/components/ui/date-time-picker", async (importOriginal) => {
  const { dateTimePickerMock } = await import("~/test/mocks/date-time-picker");
  return { ...(await importOriginal<object>()), ...dateTimePickerMock() };
});

vi.mock("next/navigation", () => ({
  usePathname: () => "/demo/enrollment-phases",
  useRouter: () => ({ replace: mocks.routerReplace }),
  useSearchParams: () => mocks.searchParams,
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenant: () => ({
    tenantSlug: "demo",
    tenant: { gradeLevelMax: mocks.gradeLevelMax },
    routingMode: "path",
  }),
  useTenantSlugSafe: () => "demo",
  useTenantRoutingModeSafe: () => "path",
  useNFCEnabled: vi.fn(() => true),
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => mocks.toast,
}));

vi.mock("~/lib/swr", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  useSWRAuth: () => ({ data: [], error: undefined, isLoading: false }),
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
    listPhaseExpiryWarnings: mocks.listPhaseExpiryWarnings,
    listPhases: mocks.listPhases,
    updatePhase: mocks.updatePhase,
  };
});

vi.mock("~/lib/enrollment-form-schema-api", async (importOriginal) => {
  const actual = (await importOriginal()) as Record<string, unknown>;
  return {
    ...actual,
    createSchema: mocks.createSchema,
    deleteEnrollmentLegalDocument: mocks.deleteEnrollmentLegalDocument,
    deleteSchema: mocks.deleteSchema,
    fetchPublicLegalTexts: mocks.fetchPublicLegalTexts,
    listSchemas: mocks.listSchemas,
    renameSchema: mocks.renameSchema,
    updateSchema: mocks.updateSchema,
    uploadEnrollmentLegalDocument: mocks.uploadEnrollmentLegalDocument,
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

vi.mock("~/lib/care-offering-booking-stats", async (importOriginal) => {
  const actual = (await importOriginal()) as Record<string, unknown>;
  return {
    ...actual,
    fetchCareOfferingBookingStats: mocks.fetchCareOfferingBookingStats,
  };
});

vi.mock("~/lib/timetable-api", () => ({
  timetableService: {
    getTemplates: mocks.getTemplates,
  },
}));

vi.mock("~/lib/calendar-period-api", () => ({
  calendarPeriodService: {
    list: mocks.listCalendarPeriods,
  },
}));

vi.mock("~/lib/enrollment-admin-api", () => ({
  createLateInvite: mocks.createLateInvite,
  createManualApprovedEnrollment: mocks.createManualApprovedEnrollment,
  fetchManualEnrollmentBootstrap: mocks.fetchManualEnrollmentBootstrap,
}));

vi.mock("~/components/enrollment/enrollment-form", () => ({
  EnrollmentForm: (props: {
    lockChildStructure?: boolean;
    gradeLevelMax?: number;
  }) => {
    mocks.enrollmentFormProps.push(props);
    return (
      <div
        data-testid="manual-enrollment-form"
        data-lock-child-structure={String(props.lockChildStructure)}
      />
    );
  },
}));

import { CareOfferingsEditor } from "./care-offerings-editor";
import {
  EnrollmentFormEditor,
  prepareFieldsForSave,
} from "./enrollment-form-editor";
import { PhasesEditor } from "./phases-editor";
import type { CareOffering } from "~/lib/care-offering-api";
import type { FormField, FormSchema } from "~/lib/enrollment-form-schema-api";
import type { Phase } from "~/lib/enrollment-phase-api";
import type { CalendarPeriod } from "~/lib/calendar-period-helpers";

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
    counts_as_care: true,
    pickup_times: {
      mon: "14:30",
      tue: "14:30",
      wed: "14:30",
      thu: "14:30",
      fri: "14:30",
    },
    sort_order: 0,
    created_at: "2026-01-01T00:00:00.000Z",
    updated_at: "2026-01-01T00:00:00.000Z",
    ...overrides,
  };
}

function calendarPeriod(
  overrides: Partial<CalendarPeriod> = {},
): CalendarPeriod {
  return {
    id: "period-1",
    tenantId: "1",
    name: "Schuljahr 2026/27",
    periodType: "school_year",
    startDate: "2026-08-01",
    endDate: "2027-08-31",
    weekCycleLength: 1,
    weekCycleAnchor: null,
    isActive: true,
    createdAt: "2026-01-01T00:00:00.000Z",
    updatedAt: "2026-01-01T00:00:00.000Z",
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

function setPickupTime(day: string, value = "14:30") {
  fireEvent.change(screen.getByLabelText(`${day} Gehzeit`), {
    target: { value },
  });
}

async function chooseOption(
  label: string | RegExp,
  optionName: string | RegExp,
) {
  fireEvent.click(screen.getByLabelText(label));
  fireEvent.click(await screen.findByRole("option", { name: optionName }));
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

beforeEach(() => {
  mocks.createCareOffering.mockReset();
  mocks.createLateInvite.mockReset();
  mocks.createManualApprovedEnrollment.mockReset();
  mocks.createPhase.mockReset();
  mocks.createSchema.mockReset();
  mocks.cloneCareOffering.mockReset();
  mocks.deleteCareOffering.mockReset();
  mocks.deletePhase.mockReset();
  mocks.deleteSchema.mockReset();
  mocks.fetchPublicLegalTexts.mockReset();
  mocks.fetchManualEnrollmentBootstrap.mockReset();
  mocks.fetchManualEnrollmentBootstrap.mockResolvedValue({
    schema: schema(),
    offerings: [],
    care_offering_selection_mode: "optional",
    captcha_config: null,
    legal_texts: { blocks: [] },
  });
  // Without this mock the editor's loadAll() fires a real fetch
  // (ECONNREFUSED in CI). Empty blocks = no tenant legal texts
  // configured, so the standard blocks stay disabled by default.
  mocks.fetchPublicLegalTexts.mockResolvedValue({
    agb: "",
    agb_document_url: "",
    agb_display_mode: "text",
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
  mocks.fetchCareOfferingBookingStats.mockReset();
  mocks.fetchCareOfferingBookingStats.mockResolvedValue({});
  mocks.listPhaseExpiryWarnings.mockReset();
  mocks.listPhaseExpiryWarnings.mockResolvedValue([]);
  mocks.listPhases.mockReset();
  mocks.listSchemas.mockReset();
  mocks.renameSchema.mockReset();
  mocks.updateCareOffering.mockReset();
  mocks.updatePhase.mockReset();
  mocks.updateSchema.mockReset();
  mocks.uploadEnrollmentLegalDocument.mockReset();
  mocks.deleteEnrollmentLegalDocument.mockReset();
  mocks.getTemplates.mockReset();
  mocks.getTemplates.mockResolvedValue({ templates: [] });
  mocks.listCalendarPeriods.mockReset();
  mocks.listCalendarPeriods.mockResolvedValue([]);
  mocks.gradeLevelMax = 13;
  mocks.enrollmentFormProps = [];
  mocks.toast.success.mockReset();
  mocks.toast.error.mockReset();
  mocks.toast.warning.mockReset();
  mocks.searchParams = new URLSearchParams();
  mocks.routerReplace.mockReset();
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
    expect(screen.getByText("Betreuungstage & Mitbuchung")).toBeVisible();
    expect(screen.getByText("Als Betreuungstage zählen")).toBeVisible();
    // Weekdays start unselected (#1885): picking a day is a deliberate input.
    const mondayToggle = screen.getByRole("button", { name: "Mo" });
    expect(mondayToggle).toHaveAttribute("aria-pressed", "false");
    fireEvent.click(mondayToggle);
    setPickupTime("Mo");
    expect(mondayToggle).toHaveAttribute("aria-pressed", "true");
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

  it("edits availability rules without JSON and blocks incomplete rules", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings.mockResolvedValue([]);
    mocks.createCareOffering.mockResolvedValue(
      offering({ id: "new", name: "Randstunde" }),
    );

    render(<CareOfferingsEditor />);
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Erstes Betreuungsangebot anlegen",
      }),
    );

    expect(
      screen.getByRole("checkbox", {
        name: /Für alle Klassenstufen \/ ohne Bedingungen/,
      }),
    ).not.toBeChecked();
    fireEvent.change(await waitForInputByName("name"), {
      target: { value: "Randstunde" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Mo" }));
    setPickupTime("Mo");
    fireEvent.click(
      screen.getByRole("checkbox", {
        name: /Für alle Klassenstufen \/ ohne Bedingungen/,
      }),
    );

    expect(
      screen.getByText("Bedingung 1: Wähle mindestens eine Klassenstufe."),
    ).toBeVisible();
    expect(screen.getByRole("button", { name: "Erstellen" })).toBeDisabled();
    const availabilityFields = within(
      screen.getByRole("group", {
        name: "Bedingungen für die Verfügbarkeit",
      }),
    );
    fireEvent.click(
      availabilityFields.getByRole("button", { name: "Klasse 1" }),
    );
    fireEvent.click(
      availabilityFields.getByRole("button", { name: "Klasse 2" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Erstellen" }));

    await waitFor(() => {
      expect(mocks.createCareOffering).toHaveBeenCalledWith(
        expect.objectContaining({
          availability_rule: {
            match: "all",
            conditions: [
              {
                source: "grade_level",
                operator: "in",
                value: [1, 2],
              },
            ],
          },
        }),
      );
    });
  });

  it("shows care-day counting and booking behavior in the offerings list", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings.mockResolvedValue([
      offering({ id: "offer-1", name: "OGS Ganztag" }),
      offering({
        id: "offer-2",
        name: "Randstunde",
        counts_as_care: false,
        auto_add_trigger_offering_ids: ["offer-1"],
      }),
    ]);

    render(<CareOfferingsEditor />);

    expect(await screen.findByText("Randstunde")).toBeInTheDocument();
    expect(screen.getByText("Zählt nicht als Betreuungstag")).toBeVisible();
    expect(screen.getByText("Wird mitgebucht")).toBeVisible();
    expect(screen.getByText("Zählt als Betreuungstag")).toBeVisible();
  });

  it("blocks active care offerings whose Betreuungstage have no Gehzeit", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings.mockResolvedValue([
      offering({ name: "Unvollständig", pickup_times: {} }),
    ]);

    render(<CareOfferingsEditor />);
    expect(await screen.findByText("Unvollständig")).toBeInTheDocument();
    expect(screen.getByText("Gehzeiten fehlen")).toBeVisible();

    fireEvent.click(
      screen.getByRole("button", { name: "Neues Betreuungsangebot" }),
    );
    fireEvent.change(await waitForInputByName("name"), {
      target: { value: "Ohne Gehzeit" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Mo" }));
    fireEvent.click(screen.getByRole("button", { name: "Erstellen" }));

    expect(
      screen.getByText(
        "Bitte tragen Sie für jeden Betreuungstag eine Gehzeit ein: Mo.",
      ),
    ).toBeVisible();
    expect(mocks.createCareOffering).not.toHaveBeenCalled();
  });

  it("keeps auto-add trigger IDs as strings when saving", async () => {
    const unsafeID = "9007199254740993";
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings.mockResolvedValue([
      offering({ id: unsafeID, name: "Ganztag" }),
    ]);
    mocks.createCareOffering.mockResolvedValue(
      offering({ id: "new", name: "Randstunde" }),
    );

    render(<CareOfferingsEditor />);
    expect(await screen.findByText("Ganztag")).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Neues Betreuungsangebot" }),
    );
    fireEvent.change(await waitForInputByName("name"), {
      target: { value: "Randstunde" },
    });
    fireEvent.click(screen.getByRole("checkbox", { name: /Ganztag/ }));
    // Weekdays start unselected (#1885); save requires at least one day.
    fireEvent.click(screen.getByRole("button", { name: "Mo" }));
    setPickupTime("Mo");
    fireEvent.click(screen.getByRole("button", { name: "Erstellen" }));

    await waitFor(() => {
      expect(mocks.createCareOffering).toHaveBeenCalledWith(
        expect.objectContaining({
          auto_add_trigger_offering_ids: [unsafeID],
        }),
      );
    });
  });

  it("names the edited offering in the auto-add rule and previews the rule direction", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings.mockResolvedValue([
      offering({ id: "trigger", name: "Ganztag" }),
    ]);

    render(<CareOfferingsEditor />);
    expect(await screen.findByText("Ganztag")).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Neues Betreuungsangebot" }),
    );

    // Without a name yet, the generic wording stays.
    expect(
      screen.getByText(
        "Dieses Angebot wird automatisch mitgebucht, wenn Eltern eines dieser Angebote wählen:",
      ),
    ).toBeVisible();

    fireEvent.change(await waitForInputByName("name"), {
      target: { value: "Randstunde" },
    });
    expect(
      screen.getByText(
        "„Randstunde“ wird automatisch mitgebucht, wenn Eltern eines dieser Angebote wählen:",
      ),
    ).toBeVisible();

    // No preview until a trigger is checked.
    expect(
      screen.queryByText(/Andersherum gilt das nicht/),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("checkbox", { name: /Ganztag/ }));

    expect(
      screen.getByText(
        "Wenn Eltern „Ganztag“ wählen, wird „Randstunde“ automatisch mitgebucht.",
      ),
    ).toBeVisible();
    expect(
      screen.getByText(
        "Andersherum gilt das nicht: Wer nur „Randstunde“ wählt, bekommt die anderen Angebote nicht automatisch dazu.",
      ),
    ).toBeVisible();
  });

  it("clears hidden auto-add trigger IDs when an edited offering changes phase", async () => {
    mocks.listPhases.mockResolvedValue([
      phase(),
      phase({ id: "11", name: "Ferien" }),
    ]);
    mocks.listCareOfferings.mockResolvedValue([
      offering({ id: "trigger", name: "Ganztag" }),
      offering({
        id: "target",
        name: "Randstunde",
        auto_add_trigger_offering_ids: ["trigger"],
      }),
    ]);
    mocks.updateCareOffering.mockResolvedValue(
      offering({
        id: "target",
        name: "Randstunde",
        phase_id: "11",
        auto_add_trigger_offering_ids: [],
      }),
    );

    render(<CareOfferingsEditor />);
    expect(await screen.findByText("Randstunde")).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Randstunde" }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );
    fireEvent.click(document.querySelector("#care-offering-form-phase")!);
    fireEvent.click(await screen.findByRole("option", { name: "Ferien" }));
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(mocks.updateCareOffering).toHaveBeenCalledWith(
        "target",
        expect.objectContaining({
          phase_id: 11,
          auto_add_trigger_offering_ids: [],
        }),
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
    fireEvent.change(await waitForInputByName("name"), {
      target: { value: "Mittagessen" },
    });
    fireEvent.change(inputByName("capacity"), { target: { value: "30" } });

    // Marking the offering as required clears any capacity and locks the field.
    fireEvent.click(screen.getByText("Pflicht"));
    expect(inputByName("capacity")).toBeDisabled();

    // Weekdays start unselected (#1885); save requires at least one day.
    fireEvent.click(screen.getByRole("button", { name: "Mo" }));
    setPickupTime("Mo");
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

  it("blocks a Regeltermin whose slots omit selected offering weekdays", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings.mockResolvedValue([offering()]);
    mocks.listCalendarPeriods.mockResolvedValue([calendarPeriod()]);
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
              calendarPeriodId: "period-1",
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
    fireEvent.change(await waitForInputByName("name"), {
      target: { value: "Lernzeitangebot" },
    });
    // Weekdays start unselected (#1885); pick Mo-Fr explicitly so the
    // template (Mon + Sat) misses the selected Tue-Fri as before.
    for (const day of ["Mo", "Di", "Mi", "Do", "Fr"]) {
      fireEvent.click(screen.getByRole("button", { name: day }));
    }
    await chooseOption("Regeltermin", /Lernzeit/);

    expect(
      screen.getByText(
        /Regeltermin deckt die ausgewählten Angebotstage Di, Mi, Do, Fr nicht ab/,
      ),
    ).toHaveAttribute("role", "alert");
    expect(
      screen.getByText(
        "Der Regeltermin enthält Tage, die im Angebot nicht auswählbar sind.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Erstellen" })).toBeDisabled();

    for (const day of ["Di", "Mi", "Do", "Fr"]) {
      fireEvent.click(screen.getByRole("button", { name: day }));
    }

    expect(
      screen.queryByText(/Regeltermin deckt die ausgewählten Angebotstage/),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Erstellen" })).toBeEnabled();
  });

  it("warns when the offering name says one weekday but more days are selected", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings.mockResolvedValue([offering()]);

    render(<CareOfferingsEditor />);
    expect(await screen.findByText("Regelbetreuung")).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Neues Betreuungsangebot" }),
    );
    fireEvent.change(await waitForInputByName("name"), {
      target: {
        value:
          "Montags: Betreuung bis zum Beginn des privaten Musikunterrichts",
      },
    });
    for (const day of ["Mo", "Di", "Mi", "Do", "Fr"]) {
      fireEvent.click(screen.getByRole("button", { name: day }));
    }

    const warning = screen.getByText(
      /Der Name nennt nur einen Wochentag, ausgewählt sind zusätzlich Di, Mi, Do, Fr\./,
    );
    expect(warning).toBeVisible();
    // Soft hint, not an error: saving stays possible.
    expect(warning).not.toHaveAttribute("role", "alert");
    expect(screen.getByRole("button", { name: "Erstellen" })).toBeEnabled();

    // Deselecting the extra days resolves the mismatch.
    for (const day of ["Di", "Mi", "Do", "Fr"]) {
      fireEvent.click(screen.getByRole("button", { name: day }));
    }
    expect(
      screen.queryByText(/Der Name nennt nur einen Wochentag/),
    ).not.toBeInTheDocument();
  });

  function availabilityFieldset() {
    return within(
      screen.getByRole("group", { name: "Bedingungen für die Verfügbarkeit" }),
    );
  }

  async function openAvailabilityRuleEditor(offeringName: string) {
    fireEvent.click(
      screen.getByRole("button", { name: `Aktionen für ${offeringName}` }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );
    fireEvent.click(
      await screen.findByRole("checkbox", {
        name: /Für alle Klassenstufen \/ ohne Bedingungen/,
      }),
    );
  }

  // #2186: the catalog listed every other offering attribute as a pill but
  // not the availability rule, so a restriction was only discoverable by
  // opening the offering and scrolling to "Bedingungen für die Verfügbarkeit".
  it("shows a set availability rule as a pill in the catalog", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings.mockResolvedValue([
      offering({
        name: "Randstunde",
        availability_rule: {
          match: "all",
          conditions: [
            { source: "grade_level", operator: "in", value: [1, 2] },
          ],
        },
      }),
    ]);

    render(<CareOfferingsEditor />);

    expect(await screen.findByText("Randstunde")).toBeInTheDocument();
    expect(screen.getByText("Nur für Klassen 1–2")).toBeInTheDocument();
  });

  // #2186 review: a "mindestens eine Bedingung" rule built from two disjoint
  // not_in conditions admits every grade, so calling it a restriction in the
  // catalog misinforms the admin.
  it("shows no pill for a rule that turns nobody away", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings.mockResolvedValue([
      offering({
        name: "Randstunde",
        availability_rule: {
          match: "any",
          conditions: [
            { source: "grade_level", operator: "not_in", value: [1] },
            { source: "grade_level", operator: "not_in", value: [2] },
          ],
        },
      }),
    ]);

    render(<CareOfferingsEditor />);

    expect(await screen.findByText("Randstunde")).toBeInTheDocument();
    expect(screen.queryByText(/^Nur für/)).not.toBeInTheDocument();
    expect(screen.queryByText(/^Nicht für/)).not.toBeInTheDocument();
  });

  it("shows no availability pill for an unrestricted offering", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings.mockResolvedValue([offering()]);

    render(<CareOfferingsEditor />);

    expect(await screen.findByText("Regelbetreuung")).toBeInTheDocument();
    expect(screen.queryByText(/^Nur für Klasse/)).not.toBeInTheDocument();
    expect(screen.queryByText(/^Nicht für Klasse/)).not.toBeInTheDocument();
  });

  // #2186: tightening a rule leaves contradicting bookings in place
  // (Bestandsschutz). Without this hint the data just looks inconsistent.
  it("warns how many existing bookings a rule contradicts", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings.mockResolvedValue([
      offering({ id: "offer-1", name: "Randstunde" }),
    ]);
    mocks.fetchCareOfferingBookingStats.mockResolvedValue({
      "offer-1": {
        offering_id: "offer-1",
        capacity: 20,
        booked: 14,
        grade_levels: { "1": 12, "3": 2 },
        unknown_grade_count: 0,
      },
    });

    render(<CareOfferingsEditor />);
    expect(await screen.findByText("Randstunde")).toBeInTheDocument();

    await openAvailabilityRuleEditor("Randstunde");
    // Restrict to grade 1: the two grade-3 bookings now contradict the rule.
    fireEvent.click(
      availabilityFieldset().getByRole("button", { name: "Klasse 1" }),
    );

    expect(
      await screen.findByText(
        /2 bestehende Buchungen erfüllen diese Bedingung nicht\./,
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/die Regel gilt nur für neue Auswahlen/),
    ).toBeInTheDocument();
  });

  it("does not warn when every booking satisfies the rule", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings.mockResolvedValue([
      offering({ id: "offer-1", name: "Randstunde" }),
    ]);
    mocks.fetchCareOfferingBookingStats.mockResolvedValue({
      "offer-1": {
        offering_id: "offer-1",
        capacity: 20,
        booked: 12,
        grade_levels: { "1": 12 },
        unknown_grade_count: 0,
      },
    });

    render(<CareOfferingsEditor />);
    expect(await screen.findByText("Randstunde")).toBeInTheDocument();

    await openAvailabilityRuleEditor("Randstunde");
    fireEvent.click(
      availabilityFieldset().getByRole("button", { name: "Klasse 1" }),
    );

    await waitFor(() =>
      expect(screen.queryByText(/bestehende Buchung/)).not.toBeInTheDocument(),
    );
  });

  it("does not warn for a brand-new offering, which has no bookings", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings.mockResolvedValue([offering({ id: "offer-1" })]);
    mocks.fetchCareOfferingBookingStats.mockResolvedValue({
      "offer-1": {
        offering_id: "offer-1",
        capacity: 20,
        booked: 14,
        grade_levels: { "3": 14 },
        unknown_grade_count: 0,
      },
    });

    render(<CareOfferingsEditor />);
    expect(await screen.findByText("Regelbetreuung")).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Neues Betreuungsangebot" }),
    );
    fireEvent.click(
      await screen.findByRole("checkbox", {
        name: /Für alle Klassenstufen \/ ohne Bedingungen/,
      }),
    );
    fireEvent.click(
      availabilityFieldset().getByRole("button", { name: "Klasse 1" }),
    );

    await waitFor(() =>
      expect(screen.queryByText(/bestehende Buchung/)).not.toBeInTheDocument(),
    );
  });

  it("does not warn for names without a single-weekday claim", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings.mockResolvedValue([offering()]);

    render(<CareOfferingsEditor />);
    expect(await screen.findByText("Regelbetreuung")).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Neues Betreuungsangebot" }),
    );
    const nameInput = await waitForInputByName("name");
    for (const day of ["Mo", "Mi", "Fr"]) {
      fireEvent.click(screen.getByRole("button", { name: day }));
    }

    // Two weekday words: the name makes no single-day claim.
    fireEvent.change(nameInput, {
      target: { value: "Montag und Mittwoch AG" },
    });
    expect(
      screen.queryByText(/Der Name nennt nur einen Wochentag/),
    ).not.toBeInTheDocument();

    // No weekday word at all.
    fireEvent.change(nameInput, { target: { value: "Fußball AG" } });
    expect(
      screen.queryByText(/Der Name nennt nur einen Wochentag/),
    ).not.toBeInTheDocument();
  });

  it("does not block saving while the name-weekday warning shows", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings.mockResolvedValue([offering()]);
    mocks.createCareOffering.mockResolvedValue(
      offering({ id: "new", name: "Montags-Club" }),
    );

    render(<CareOfferingsEditor />);
    expect(await screen.findByText("Regelbetreuung")).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Neues Betreuungsangebot" }),
    );
    fireEvent.change(await waitForInputByName("name"), {
      target: { value: "Montags-Club" },
    });
    for (const day of ["Mo", "Di"]) {
      fireEvent.click(screen.getByRole("button", { name: day }));
      setPickupTime(day);
    }
    expect(
      screen.getByText(/Der Name nennt nur einen Wochentag/),
    ).toBeVisible();

    fireEvent.click(screen.getByRole("button", { name: "Erstellen" }));

    await waitFor(() => {
      expect(mocks.createCareOffering).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "Montags-Club",
          available_days: ["mon", "tue"],
        }),
      );
    });
    expect(mocks.toast.error).not.toHaveBeenCalled();
  });

  it("offers Regeltermine whose resolved period contains the phase", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings.mockResolvedValue([]);
    mocks.listCalendarPeriods.mockResolvedValue([
      calendarPeriod(),
      calendarPeriod({
        id: "period-short",
        name: "Kurzer Zeitraum",
        startDate: "2026-09-01",
        endDate: "2026-12-31",
      }),
      calendarPeriod({
        id: "period-inactive",
        name: "Inaktiver Zeitraum",
        isActive: false,
      }),
    ]);
    const baseTemplate = {
      type: "care" as const,
      categoryId: "cat-1",
      categoryName: "Betreuung",
      isOpen: true,
      maxParticipants: 20,
      enrollmentCount: 0,
      supervisorCount: 0,
      requiredStaffCount: 0,
      assignedStaffCount: 0,
      studentIds: [],
      staffIds: [],
      targetGroupType: "none" as const,
    };
    mocks.getTemplates.mockResolvedValue({
      templates: [
        {
          ...baseTemplate,
          id: "8",
          name: "Passender Regeltermin",
          schedules: [
            {
              id: "schedule-compatible",
              weekday: 1,
              startTime: "13:00",
              endTime: "14:00",
              weekPattern: 0,
              calendarPeriodId: "period-1",
            },
          ],
        },
        {
          ...baseTemplate,
          id: "9",
          name: "Zu kurzer Regeltermin",
          schedules: [
            {
              id: "schedule-short",
              weekday: 1,
              startTime: "13:00",
              endTime: "14:00",
              weekPattern: 0,
              calendarPeriodId: "period-short",
            },
          ],
        },
        {
          ...baseTemplate,
          id: "10",
          name: "Regeltermin ohne Slots",
          schedules: [],
        },
        {
          ...baseTemplate,
          id: "11",
          name: "Regeltermin mit Vorlagenperiode",
          calendarPeriodId: "period-1",
          schedules: [
            {
              id: "schedule-template-period",
              weekday: 2,
              startTime: "13:00",
              endTime: "14:00",
              weekPattern: 0,
            },
          ],
        },
        {
          ...baseTemplate,
          id: "12",
          name: "Regeltermin im inaktiven Zeitraum",
          calendarPeriodId: "period-inactive",
          schedules: [
            {
              id: "schedule-inactive-period",
              weekday: 2,
              startTime: "13:00",
              endTime: "14:00",
              weekPattern: 0,
            },
          ],
        },
      ],
    });

    render(<CareOfferingsEditor />);
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Erstes Betreuungsangebot anlegen",
      }),
    );
    fireEvent.click(screen.getByLabelText("Regeltermin"));

    expect(
      screen.getByRole("option", { name: /Passender Regeltermin/ }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("option", { name: /Zu kurzer Regeltermin/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("option", { name: /Regeltermin ohne Slots/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("option", {
        name: /Regeltermin mit Vorlagenperiode/,
      }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("option", {
        name: /Regeltermin im inaktiven Zeitraum/,
      }),
    ).not.toBeInTheDocument();
  });

  it("keeps inactive offerings linked to inactive periods but blocks activation", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings.mockResolvedValue([
      offering({
        activity_group_id: "12",
        available_days: ["tue"],
        is_active: false,
      }),
    ]);
    mocks.listCalendarPeriods.mockResolvedValue([
      calendarPeriod({ id: "period-inactive", isActive: false }),
    ]);
    mocks.getTemplates.mockResolvedValue({
      templates: [
        {
          id: "12",
          name: "Regeltermin im inaktiven Zeitraum",
          type: "care",
          categoryId: "cat-1",
          categoryName: "Betreuung",
          isOpen: true,
          maxParticipants: 20,
          calendarPeriodId: "period-inactive",
          targetGroupType: "none",
          enrollmentCount: 0,
          supervisorCount: 0,
          requiredStaffCount: 0,
          assignedStaffCount: 0,
          studentIds: [],
          staffIds: [],
          schedules: [
            {
              id: "schedule-inactive-period",
              weekday: 2,
              startTime: "13:00",
              endTime: "14:00",
              weekPattern: 0,
            },
          ],
        },
      ],
    });
    mocks.updateCareOffering.mockResolvedValue(
      offering({
        activity_group_id: "12",
        available_days: ["tue"],
        is_active: false,
      }),
    );

    render(<CareOfferingsEditor />);
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Regelbetreuung",
      }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );

    expect(screen.getByLabelText("Regeltermin")).toHaveTextContent(
      /Regeltermin im inaktiven Zeitraum/,
    );
    const activeCheckbox = screen.getByRole("checkbox", { name: /^Aktiv/ });
    const save = screen.getByRole("button", { name: "Speichern" });
    expect(activeCheckbox).not.toBeChecked();
    expect(save).toBeEnabled();

    fireEvent.click(activeCheckbox);
    expect(
      screen.getByText(/aktives Betreuungsangebot braucht einen aktiven/),
    ).toBeVisible();
    expect(save).toBeDisabled();

    fireEvent.click(activeCheckbox);
    expect(
      screen.queryByText(/aktives Betreuungsangebot braucht einen aktiven/),
    ).not.toBeInTheDocument();
    expect(save).toBeEnabled();
    fireEvent.click(save);

    await waitFor(() =>
      expect(mocks.updateCareOffering).toHaveBeenCalledWith(
        "offer-1",
        expect.objectContaining({
          activity_group_id: 12,
          is_active: false,
        }),
      ),
    );
  });

  it("shows an existing incompatible link and blocks saving it", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings.mockResolvedValue([
      offering({ activity_group_id: "9" }),
    ]);
    mocks.listCalendarPeriods.mockResolvedValue([
      calendarPeriod({
        id: "period-short",
        startDate: "2026-09-01",
        endDate: "2026-12-31",
      }),
    ]);
    mocks.getTemplates.mockResolvedValue({
      templates: [
        {
          id: "9",
          name: "Lernzeit",
          type: "care",
          categoryId: "cat-1",
          categoryName: "Betreuung",
          isOpen: true,
          maxParticipants: 20,
          enrollmentCount: 0,
          supervisorCount: 0,
          requiredStaffCount: 0,
          assignedStaffCount: 0,
          studentIds: [],
          staffIds: [],
          targetGroupType: "none",
          schedules: [
            {
              id: "schedule-short",
              weekday: 1,
              startTime: "13:00",
              endTime: "14:00",
              weekPattern: 0,
              calendarPeriodId: "period-short",
            },
          ],
        },
      ],
    });

    render(<CareOfferingsEditor />);
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Regelbetreuung",
      }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );

    expect(screen.getByLabelText("Regeltermin")).toHaveTextContent(
      /Lernzeit.*nicht kompatibel/,
    );
    expect(
      screen.getByText(/muss den gesamten Betreuungszeitraum/),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Speichern" })).toBeDisabled();
  });

  it("does not expose technical series-gap errors returned on save", async () => {
    const technicalError =
      "timetable recurrence does not cover care offering weekday 1 on 2027-03-01";
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings.mockResolvedValue([
      offering({ activity_group_id: "8", available_days: ["mon"] }),
    ]);
    mocks.listCalendarPeriods.mockResolvedValue([calendarPeriod()]);
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
          calendarPeriodId: "period-1",
          targetGroupType: "none",
          enrollmentCount: 0,
          supervisorCount: 0,
          requiredStaffCount: 0,
          assignedStaffCount: 0,
          studentIds: [],
          staffIds: [],
          schedules: [
            {
              id: "schedule-monday",
              weekday: 1,
              startTime: "13:00",
              endTime: "14:00",
              weekPattern: 0,
            },
          ],
        },
      ],
    });
    mocks.updateCareOffering.mockRejectedValue(new Error(technicalError));

    render(<CareOfferingsEditor />);
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Regelbetreuung",
      }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );
    await waitFor(() =>
      expect(screen.getByLabelText("Regeltermin")).toHaveTextContent(
        "Lernzeit",
      ),
    );
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        "Betreuungsangebot konnte nicht gespeichert werden",
      ),
    ).toBeVisible();
    expect(screen.queryByText(technicalError)).not.toBeInTheDocument();
    expect(mocks.toast.error).toHaveBeenCalledWith(
      "Betreuungsangebot konnte nicht gespeichert werden",
    );
  });

  it("clears a linked Regeltermin when the phase change makes it incompatible", async () => {
    mocks.listPhases.mockResolvedValue([
      phase(),
      phase({
        id: "11",
        name: "Schuljahr 2027/28",
        service_start_date: "2027-09-01",
        service_end_date: "2028-07-31",
      }),
    ]);
    mocks.listCareOfferings.mockResolvedValue([
      offering({ activity_group_id: "8" }),
    ]);
    mocks.listCalendarPeriods.mockResolvedValue([calendarPeriod()]);
    mocks.updateCareOffering.mockResolvedValue(
      offering({ phase_id: "11", activity_group_id: null }),
    );
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
          requiredStaffCount: 0,
          assignedStaffCount: 0,
          studentIds: [],
          staffIds: [],
          targetGroupType: "none",
          schedules: [
            {
              id: "schedule-linked",
              weekday: 1,
              startTime: "13:00",
              endTime: "14:00",
              weekPattern: 0,
              calendarPeriodId: "period-1",
            },
          ],
        },
      ],
    });

    render(<CareOfferingsEditor />);
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Regelbetreuung",
      }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );
    const form = screen
      .getByRole("heading", { name: "Betreuungsangebot bearbeiten" })
      .closest("form");
    expect(form).not.toBeNull();
    fireEvent.click(within(form!).getByLabelText("Anmeldephase"));
    fireEvent.click(
      await screen.findByRole("option", { name: "Schuljahr 2027/28" }),
    );

    expect(mocks.toast.warning).toHaveBeenCalledWith(
      expect.stringMatching(
        /Verknüpfung zum Regeltermin wurde entfernt.*neu gewählten Anmeldephase/,
      ),
    );
    expect(within(form!).getByLabelText("Regeltermin")).toHaveTextContent(
      "Keine automatische Regeltermin-Zuordnung",
    );
    fireEvent.click(within(form!).getByRole("button", { name: "Speichern" }));
    await waitFor(() => {
      expect(mocks.updateCareOffering).toHaveBeenCalledWith(
        "offer-1",
        expect.objectContaining({
          phase_id: 11,
          activity_group_id: null,
        }),
      );
    });
  });

  it("renders the offering catalog before optional planner metadata settles", async () => {
    const templatesRequest = deferred<{ templates: [] }>();
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings.mockResolvedValue([offering()]);
    mocks.getTemplates.mockReturnValue(templatesRequest.promise);

    render(<CareOfferingsEditor />);

    expect(await screen.findByText("Regelbetreuung")).toBeInTheDocument();
    expect(mocks.listCalendarPeriods).toHaveBeenCalledTimes(1);

    await act(async () => templatesRequest.resolve({ templates: [] }));
  });

  it("allows an existing link to be removed when planner metadata fails", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings.mockResolvedValue([
      offering({ activity_group_id: "8" }),
    ]);
    mocks.getTemplates.mockRejectedValue(new Error("Regeltermine verboten"));
    mocks.updateCareOffering.mockResolvedValue(
      offering({ activity_group_id: "8" }),
    );

    render(<CareOfferingsEditor />);

    expect(await screen.findByText("Regelbetreuung")).toBeInTheDocument();
    expect(await screen.findByRole("status")).toHaveTextContent(
      /Betreuungsangebote bleiben nutzbar/,
    );
    expect(screen.getByRole("button", { name: "Erneut laden" })).toBeEnabled();
    expect(mocks.toast.error).not.toHaveBeenCalled();

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Regelbetreuung" }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );

    const templateSelect = screen.getByLabelText("Regeltermin");
    expect(templateSelect).toBeEnabled();
    expect(
      screen.getByText(/Kompatibilität ist derzeit unbekannt/),
    ).toBeVisible();
    fireEvent.click(templateSelect);
    expect(
      screen.getByRole("option", {
        name: "Keine automatische Regeltermin-Zuordnung",
      }),
    ).toBeEnabled();
    expect(
      screen.getByRole("option", { name: /Regeltermin #8/ }),
    ).toBeDisabled();
    fireEvent.click(
      screen.getByRole("option", {
        name: "Keine automatische Regeltermin-Zuordnung",
      }),
    );
    const save = screen.getByRole("button", { name: "Speichern" });
    expect(save).toBeEnabled();
    fireEvent.click(save);

    await waitFor(() =>
      expect(mocks.updateCareOffering).toHaveBeenCalledWith(
        "offer-1",
        expect.objectContaining({ activity_group_id: null }),
      ),
    );
  });

  it("keeps existing links when calendar-period metadata fails", async () => {
    mocks.listPhases.mockResolvedValue([
      phase(),
      phase({ id: "11", name: "Schuljahr 2027/28" }),
    ]);
    mocks.listCareOfferings.mockResolvedValue([
      offering({ activity_group_id: "8" }),
    ]);
    mocks.listCalendarPeriods.mockRejectedValue(new Error("Perioden kaputt"));
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
          requiredStaffCount: 0,
          assignedStaffCount: 0,
          studentIds: [],
          staffIds: [],
          targetGroupType: "none",
          calendarPeriodId: "period-1",
          schedules: [
            {
              id: "schedule-linked",
              weekday: 1,
              startTime: "13:00",
              endTime: "14:00",
              weekPattern: 0,
            },
          ],
        },
      ],
    });
    mocks.updateCareOffering.mockResolvedValue(
      offering({ phase_id: "11", activity_group_id: "8" }),
    );

    render(<CareOfferingsEditor />);
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Regelbetreuung",
      }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );
    const form = screen
      .getByRole("heading", { name: "Betreuungsangebot bearbeiten" })
      .closest("form");
    expect(form).not.toBeNull();
    fireEvent.click(within(form!).getByLabelText("Anmeldephase"));
    fireEvent.click(
      await screen.findByRole("option", { name: "Schuljahr 2027/28" }),
    );

    fireEvent.click(within(form!).getByRole("button", { name: "Speichern" }));
    await waitFor(() =>
      expect(mocks.updateCareOffering).toHaveBeenCalledWith(
        "offer-1",
        expect.objectContaining({
          phase_id: 11,
          activity_group_id: 8,
        }),
      ),
    );
    expect(mocks.toast.warning).not.toHaveBeenCalled();
  });

  it("withholds a stale catalog when the newly selected phase fails to load", async () => {
    let targetPhaseFails = true;
    const metadataRequest = deferred<{ templates: [] }>();
    const failedCatalogRequest = deferred<CareOffering[]>();
    const successfulCatalogRequest = deferred<CareOffering[]>();
    mocks.getTemplates.mockReturnValue(metadataRequest.promise);
    mocks.listPhases.mockResolvedValue([
      phase(),
      phase({ id: "11", name: "Ferien", kind: "holiday" }),
    ]);
    mocks.listCareOfferings.mockImplementation((phaseID: string) => {
      if (phaseID === "11") {
        return targetPhaseFails
          ? failedCatalogRequest.promise
          : successfulCatalogRequest.promise;
      }
      return Promise.resolve([offering({ name: "Regelbetreuung" })]);
    });

    render(<CareOfferingsEditor />);
    expect(await screen.findByText("Regelbetreuung")).toBeInTheDocument();

    fireEvent.click(screen.getByLabelText("Anmeldephase"));
    const holidayPhaseOption = await screen.findByRole("option", {
      name: "Ferien (Ferienbetreuung)",
    });
    await act(async () => {
      holidayPhaseOption.click();
      await Promise.resolve();
    });
    await act(async () => {
      failedCatalogRequest.reject(new Error("Ferienkatalog nicht erreichbar"));
      await failedCatalogRequest.promise.catch(() => undefined);
      await new Promise((resolve) => setTimeout(resolve, 0));
    });

    expect(
      await screen.findByRole("heading", {
        name: "Betreuungsangebote konnten nicht geladen werden",
      }),
    ).toBeVisible();
    expect(screen.getByText("Ferienkatalog nicht erreichbar")).toBeVisible();
    expect(screen.queryByText("Regelbetreuung")).not.toBeInTheDocument();
    expect(
      screen.queryByText("Noch kein Betreuungsangebot angelegt"),
    ).not.toBeInTheDocument();

    targetPhaseFails = false;
    fireEvent.click(
      screen.getByRole("button", {
        name: "Betreuungsangebote erneut laden",
      }),
    );
    await act(async () => {
      successfulCatalogRequest.resolve([
        offering({
          id: "holiday-offer",
          phase_id: "11",
          name: "Ferienbetreuung",
        }),
      ]);
      await successfulCatalogRequest.promise;
      await new Promise((resolve) => setTimeout(resolve, 0));
    });

    expect(await screen.findByText("Ferienbetreuung")).toBeInTheDocument();
    expect(screen.queryByText("Regelbetreuung")).not.toBeInTheDocument();

    await act(async () => {
      metadataRequest.resolve({ templates: [] });
      await metadataRequest.promise;
    });
  });

  it("ignores an out-of-order catalog load under React StrictMode", async () => {
    const stalePhaseRequest = deferred<Phase[]>();
    mocks.listPhases
      .mockReturnValueOnce(stalePhaseRequest.promise)
      .mockResolvedValueOnce([
        phase({ id: "11", name: "Aktuelle Phase", kind: "custom" }),
      ]);
    mocks.listCareOfferings.mockResolvedValue([
      offering({
        id: "current-offer",
        phase_id: "11",
        name: "Aktuelles Angebot",
      }),
    ]);

    render(
      <StrictMode>
        <CareOfferingsEditor />
      </StrictMode>,
    );

    expect(await screen.findByText("Aktuelles Angebot")).toBeInTheDocument();

    await act(async () => {
      stalePhaseRequest.resolve([phase()]);
      await stalePhaseRequest.promise;
    });

    expect(screen.getByText("Aktuelles Angebot")).toBeInTheDocument();
    expect(mocks.listCareOfferings).toHaveBeenCalledTimes(1);
    expect(mocks.listCareOfferings).toHaveBeenCalledWith("11");
  });

  it("allows an unresolved linked template period to be removed", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings.mockResolvedValue([
      offering({ activity_group_id: "8" }),
    ]);
    mocks.listCalendarPeriods.mockResolvedValue([calendarPeriod()]);
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
          requiredStaffCount: 0,
          assignedStaffCount: 0,
          studentIds: [],
          staffIds: [],
          targetGroupType: "none",
          schedules: [
            {
              id: "schedule-unresolved",
              weekday: 1,
              startTime: "13:00",
              endTime: "14:00",
              weekPattern: 0,
            },
          ],
        },
      ],
    });
    mocks.updateCareOffering.mockResolvedValue(
      offering({ activity_group_id: "8" }),
    );

    render(<CareOfferingsEditor />);
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Regelbetreuung",
      }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );

    const templateSelect = screen.getByLabelText("Regeltermin");
    expect(templateSelect).toBeEnabled();
    expect(screen.getByText(/Planungsperiode.*nicht eindeutig/)).toBeVisible();
    fireEvent.click(templateSelect);
    expect(
      await screen.findByRole("option", {
        name: /Lernzeit.*Kompatibilität unbekannt/,
      }),
    ).toBeDisabled();
    fireEvent.click(
      screen.getByRole("option", {
        name: "Keine automatische Regeltermin-Zuordnung",
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(mocks.updateCareOffering).toHaveBeenCalledWith(
        "offer-1",
        expect.objectContaining({ activity_group_id: null }),
      ),
    );
  });

  it("derives automation grades from the tenant cap", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings.mockResolvedValue([offering()]);
    mocks.updateCareOffering.mockResolvedValue(offering());

    render(<CareOfferingsEditor />);
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Regelbetreuung",
      }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );

    const gradeThirteen = screen.getByRole("button", { name: "Klasse 13" });
    expect(gradeThirteen).toHaveAttribute("aria-pressed", "false");
    fireEvent.click(gradeThirteen);
    expect(gradeThirteen).toHaveAttribute("aria-pressed", "true");
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(mocks.updateCareOffering).toHaveBeenCalledWith(
        "offer-1",
        expect.objectContaining({ auto_add_grade_levels: [13] }),
      ),
    );
  });

  it("preserves existing supported automation grades above a lowered tenant cap", async () => {
    mocks.gradeLevelMax = 4;
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings.mockResolvedValue([
      offering({ auto_add_grade_levels: [5, 13] }),
    ]);
    mocks.updateCareOffering.mockResolvedValue(
      offering({ auto_add_grade_levels: [2, 5, 13] }),
    );

    render(<CareOfferingsEditor />);
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Regelbetreuung",
      }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );

    expect(
      screen.getByRole("button", { name: "Klasse 5 (bestehend)" }),
    ).toHaveAttribute("aria-pressed", "true");
    expect(
      screen.getByRole("button", { name: "Klasse 13 (bestehend)" }),
    ).toHaveAttribute("aria-pressed", "true");
    const gradeTwo = screen.getByRole("button", { name: "Klasse 2" });
    expect(gradeTwo).toHaveAttribute("aria-pressed", "false");
    fireEvent.click(gradeTwo);
    expect(gradeTwo).toHaveAttribute("aria-pressed", "true");
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(mocks.updateCareOffering).toHaveBeenCalledWith(
        "offer-1",
        expect.objectContaining({ auto_add_grade_levels: [2, 5, 13] }),
      ),
    );
  });

  // #2367: a reversed rule direction must be visible without a test
  // enrollment, so the phase catalog lists every active rule as a sentence.
  it("lists the phase's active Mitbuchungs-Regeln as plain sentences", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings.mockResolvedValue([
      offering({
        id: "1",
        name: "Ganztagsbetreuung bis 14.30 Uhr",
        auto_add_trigger_offering_ids: ["2"],
        availability_rule: {
          match: "all",
          conditions: [
            { source: "grade_level", operator: "in", value: [1, 2] },
          ],
        },
      }),
      offering({ id: "2", name: "Randstunde" }),
      offering({
        id: "3",
        name: "Inaktives Ziel",
        is_active: false,
        auto_add_trigger_offering_ids: ["2"],
      }),
    ]);

    const { unmount } = render(<CareOfferingsEditor />);

    expect(await screen.findByText("Mitbuchungs-Regeln")).toBeInTheDocument();
    expect(
      screen.getByText(
        "„Ganztagsbetreuung bis 14.30 Uhr“ wird automatisch mitgebucht, wenn „Randstunde“ gewählt wird (Klassen 1–2).",
      ),
    ).toBeInTheDocument();
    // A rule on an inactive offering cannot fire, so it stays out of the list.
    const sentences = screen.getAllByText(/wird automatisch mitgebucht/);
    expect(sentences).toHaveLength(1);
    unmount();

    // Without any rule the panel stays away entirely.
    mocks.listCareOfferings.mockResolvedValue([
      offering(),
      offering({ id: "2", name: "Randstunde" }),
    ]);
    render(<CareOfferingsEditor />);
    expect(await screen.findByText("Randstunde")).toBeInTheDocument();
    expect(screen.queryByText("Mitbuchungs-Regeln")).not.toBeInTheDocument();
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
    // Die Aktion liegt jetzt im Seitenkopf und rendert dort responsiv zweimal
    // (mobile Kopfzeile und Desktop-Zeile).
    fireEvent.click(
      screen.getAllByRole("button", { name: "Neue Anmeldephase" })[0]!,
    );
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

  it("does not save a phase without both service dates", async () => {
    mocks.listPhases.mockResolvedValue([]);
    mocks.listSchemas.mockResolvedValue([schema()]);

    render(<PhasesEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Erste Anmeldephase anlegen" }),
    );
    fireEvent.change(inputByName("name"), {
      target: { value: "Unvollständiger Zeitraum" },
    });
    fireEvent.change(inputByName("service_start_date"), {
      target: { value: "" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Erstellen" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Bitte Beginn und Ende des Betreuungszeitraums angeben.",
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

  it("uses tenant-aware phase detail links in the action menu", async () => {
    mocks.listPhases.mockResolvedValue([phase({ id: "12" })]);
    mocks.listSchemas.mockResolvedValue([schema()]);

    render(<PhasesEditor />);

    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Schuljahr 2026/27",
      }),
    );

    const link = await screen.findByRole("menuitem", {
      name: /Anmeldungen ansehen/,
    });
    expect(link).toHaveAttribute("href", "/demo/admin/enrollments/phases/12");
  });

  it("opens late invite and manual enrollment actions from the phase action menu", async () => {
    mocks.listPhases.mockResolvedValue([phase({ id: "12" })]);
    mocks.listSchemas.mockResolvedValue([schema()]);

    render(<PhasesEditor />);

    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Schuljahr 2026/27",
      }),
    );

    fireEvent.click(
      await screen.findByRole("menuitem", {
        name: "Nachzügler-Link erstellen",
      }),
    );
    expect(
      await screen.findByRole("dialog", {
        name: "Nachzügler-Link erstellen",
      }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Modal schließen" }));
    fireEvent.click(
      screen.getByRole("button", {
        name: "Aktionen für Schuljahr 2026/27",
      }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Manuelle Anmeldung" }),
    );

    expect(
      await screen.findByRole("dialog", {
        name: "Kind manuell über Anmeldung freigeben",
      }),
    ).toBeInTheDocument();
    await waitFor(() => {
      expect(mocks.fetchManualEnrollmentBootstrap).toHaveBeenCalledWith("12");
    });
    const form = await screen.findByTestId("manual-enrollment-form");
    expect(form).toHaveAttribute("data-lock-child-structure", "true");
    expect(mocks.enrollmentFormProps.at(-1)).toMatchObject({
      lockChildStructure: true,
      gradeLevelMax: 13,
    });
  });

  it("hides the untokenized public actions for a linked_parents phase but keeps the late-invite action (#1663)", async () => {
    mocks.listPhases.mockResolvedValue([
      phase({ id: "12", name: "Nur Konto", audience: "linked_parents" }),
    ]);
    mocks.listSchemas.mockResolvedValue([schema()]);

    render(<PhasesEditor />);

    // The row-level copy-link button is gone: the plain /enroll/{id} URL is
    // rejected by the backend without a late-invite token, so copying it would
    // hand out a 404.
    await screen.findByRole("button", { name: "Aktionen für Nur Konto" });
    expect(
      screen.queryByRole("button", { name: "Elternlink kopieren" }),
    ).not.toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Nur Konto" }),
    );

    // "Formular ansehen" (the untokenized public form) is hidden...
    expect(
      screen.queryByRole("menuitem", { name: "Formular ansehen" }),
    ).not.toBeInTheDocument();
    // ...while the tokenized "Nachzügler-Link erstellen" action stays available.
    expect(
      await screen.findByRole("menuitem", {
        name: "Nachzügler-Link erstellen",
      }),
    ).toBeInTheDocument();
  });

  it("keeps the public form actions for an open phase (#1663)", async () => {
    mocks.listPhases.mockResolvedValue([
      phase({ id: "12", name: "Offene Phase", audience: "open" }),
    ]);
    mocks.listSchemas.mockResolvedValue([schema()]);

    render(<PhasesEditor />);

    expect(
      await screen.findByRole("button", { name: "Elternlink kopieren" }),
    ).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Offene Phase" }),
    );
    expect(
      await screen.findByRole("menuitem", { name: "Formular ansehen" }),
    ).toBeInTheDocument();
  });

  it("round-trips the Zielgruppe audience and eligible classes into the create payload (#1663)", async () => {
    mocks.listPhases.mockResolvedValue([]);
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.createPhase.mockResolvedValue(phase({ id: "20", name: "Nur Konto" }));

    render(<PhasesEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Erste Anmeldephase anlegen" }),
    );
    fireEvent.change(inputByName("name"), { target: { value: "Nur Konto" } });

    // The audience radios render with a default of "open".
    expect(
      document.querySelector("#phase-audience-open") as HTMLInputElement,
    ).toBeChecked();
    fireEvent.click(
      document.querySelector(
        "#phase-audience-linked_parents",
      ) as HTMLInputElement,
    );

    // Add an eligible class. Scope to the Zielgruppe fieldset so we don't hit
    // the separate SchoolClassListEditor under "Konkrete Klassen".
    const audienceFieldset = screen
      .getByText("Zielgruppe")
      .closest("fieldset") as HTMLElement;
    fireEvent.change(
      within(audienceFieldset).getByLabelText("Klasse hinzufügen"),
      { target: { value: "3b" } },
    );
    fireEvent.click(
      within(audienceFieldset).getByRole("button", { name: "Hinzufügen" }),
    );

    fireEvent.click(screen.getByRole("button", { name: "Erstellen" }));

    await waitFor(() => {
      expect(mocks.createPhase).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "Nur Konto",
          audience: "linked_parents",
          eligible_school_classes: ["3b"],
        }),
      );
    });
  });

  it("keeps offered classes that are not eligible when saving a restricted phase (#1663)", async () => {
    // available_school_classes may legitimately be a superset of the eligible
    // list: late-invite recipients bypass the eligibility gate and get the
    // full offered list, so an unrelated edit must not drop those classes.
    const restricted = phase({
      id: "22",
      available_school_classes: ["2a", "2b", "3b"],
      eligible_school_classes: ["3b"],
      require_school_class: true,
    });
    mocks.listPhases.mockResolvedValue([restricted]);
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.updatePhase.mockResolvedValue(restricted);

    render(<PhasesEditor />);

    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Schuljahr 2026/27",
      }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );

    fireEvent.click(await screen.findByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(mocks.updatePhase).toHaveBeenCalledWith(
        "22",
        expect.objectContaining({
          available_school_classes: ["2a", "2b", "3b"],
          eligible_school_classes: ["3b"],
          require_school_class: true,
        }),
      );
    });
  });

  it("saves a grade-level restriction without touching the class config (#1663)", async () => {
    // A phase for a whole grade needs no concrete classes — that is the case
    // the class list cannot express, so selecting a grade must not force a
    // class pick list or the mandatory toggle.
    const target = phase({ id: "26" });
    mocks.listPhases.mockResolvedValue([target]);
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.updatePhase.mockResolvedValue(target);

    render(<PhasesEditor />);

    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Schuljahr 2026/27",
      }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );

    const gradeButtons = await screen.findAllByRole("button", { name: "3" });
    fireEvent.click(gradeButtons[0]!);
    fireEvent.click(await screen.findByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(mocks.updatePhase).toHaveBeenCalledWith(
        "26",
        expect.objectContaining({
          eligible_grade_levels: [3],
          eligible_school_classes: [],
          require_school_class: false,
        }),
      );
    });
  });

  it("refuses a class that contradicts the selected grade levels (#1663)", async () => {
    // Both restrictions apply together, so a grade-2 class under a grade-3
    // restriction can never be declared. The backend rejects the pair; the
    // editor names the offending class instead of surfacing that error.
    const conflicting = phase({
      id: "27",
      available_school_classes: ["2a"],
      eligible_school_classes: ["2a"],
      eligible_grade_levels: [3],
      require_school_class: true,
    });
    mocks.listPhases.mockResolvedValue([conflicting]);
    mocks.listSchemas.mockResolvedValue([schema()]);

    render(<PhasesEditor />);

    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Schuljahr 2026/27",
      }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );

    fireEvent.click(await screen.findByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(/passt nicht zu den gewählten Klassenstufen/),
    ).toBeInTheDocument();
    expect(mocks.updatePhase).not.toHaveBeenCalled();
  });

  it("adds an eligible class that is not yet offered to the offered list (#1663)", async () => {
    // The eligible list is edited free-form; the backend rejects a phase whose
    // eligible list is not a subset of the offered one, so the missing class
    // is added instead of the offered list being replaced.
    const restricted = phase({
      id: "23",
      available_school_classes: ["2a", "2b"],
      eligible_school_classes: [],
      require_school_class: false,
    });
    mocks.listPhases.mockResolvedValue([restricted]);
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.updatePhase.mockResolvedValue(restricted);

    render(<PhasesEditor />);

    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Schuljahr 2026/27",
      }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );

    const audienceFieldset = (await screen.findByText("Zielgruppe")).closest(
      "fieldset",
    ) as HTMLElement;
    fireEvent.change(
      within(audienceFieldset).getByLabelText("Klasse hinzufügen"),
      { target: { value: "3b" } },
    );
    fireEvent.click(
      within(audienceFieldset).getByRole("button", { name: "Hinzufügen" }),
    );

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(mocks.updatePhase).toHaveBeenCalledWith(
        "23",
        expect.objectContaining({
          available_school_classes: ["2a", "2b", "3b"],
          eligible_school_classes: ["3b"],
          require_school_class: true,
        }),
      );
    });
  });

  it("defaults a legacy phase without eligibility fields to audience open on save (#1663)", async () => {
    // phase() intentionally omits audience/eligible_school_classes, mirroring
    // a phase created before #1663.
    const legacy = phase({ id: "21" });
    mocks.listPhases.mockResolvedValue([legacy]);
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.updatePhase.mockResolvedValue(legacy);

    render(<PhasesEditor />);

    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Schuljahr 2026/27",
      }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );

    await waitFor(() => {
      expect(
        document.querySelector("#phase-audience-open") as HTMLInputElement,
      ).toBeChecked();
    });

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(mocks.updatePhase).toHaveBeenCalledWith(
        "21",
        expect.objectContaining({
          audience: "open",
          eligible_school_classes: [],
        }),
      );
    });
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
        // Content-only edit: no rename name rides along on the save.
        undefined,
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

  it("resets the saving state after creating a schema under React StrictMode", async () => {
    const create = deferred<FormSchema>();
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.createSchema.mockReturnValue(create.promise);

    render(
      <StrictMode>
        <EnrollmentFormEditor />
      </StrictMode>,
    );

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    fireEvent.change(
      screen.getByPlaceholderText("z. B. Ferienbetreuung Sommer 2026"),
      {
        target: { value: "Kontaktformular" },
      },
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Formularvorlage erstellen" }),
    );

    expect(
      await screen.findByRole("button", { name: "Speichert…" }),
    ).toBeDisabled();

    await act(async () => {
      create.resolve(schema({ id: "schema-new", name: "Kontaktformular" }));
    });

    await waitFor(() => {
      expect(mocks.toast.success).toHaveBeenCalledWith(
        "Formularvorlage erstellt.",
      );
    });
    expect(
      screen.queryByRole("button", { name: "Speichert…" }),
    ).not.toBeInTheDocument();
  });

  it("renames a schema via the actions menu", async () => {
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.renameSchema.mockResolvedValue(schema({ name: "Ferienprogramm" }));

    render(<EnrollmentFormEditor />);

    expect(
      await screen.findByText("Anmeldeformulare verwalten"),
    ).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Regelformular" }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Umbenennen" }),
    );

    const input = await screen.findByLabelText("Name");
    fireEvent.change(input, { target: { value: "  Ferienprogramm  " } });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      // The trimmed name reaches the API; the lineage id (latest version)
      // identifies which schema to rename.
      expect(mocks.renameSchema).toHaveBeenCalledWith(
        "schema-1",
        "Ferienprogramm",
      );
    });
  });

  it("keeps the rename dialog open and shows the error when the name is taken", async () => {
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.renameSchema.mockRejectedValue(
      new Error("Es gibt bereits ein Formular mit diesem Namen."),
    );

    render(<EnrollmentFormEditor />);

    expect(
      await screen.findByText("Anmeldeformulare verwalten"),
    ).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Regelformular" }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Umbenennen" }),
    );

    fireEvent.change(await screen.findByLabelText("Name"), {
      target: { value: "Ferienprogramm" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText("Es gibt bereits ein Formular mit diesem Namen."),
    ).toBeInTheDocument();
    // Dialog stays open so the admin can correct the name.
    expect(screen.getByLabelText("Name")).toBeInTheDocument();
  });

  it("renames via the builder name field when saving an edited template", async () => {
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.renameSchema.mockResolvedValue(schema({ name: "Ferienprogramm" }));
    mocks.updateSchema.mockResolvedValue(schema({ name: "Ferienprogramm" }));

    render(<EnrollmentFormEditor />);

    expect(
      await screen.findByText("Anmeldeformulare verwalten"),
    ).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Regelformular" }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );

    // The name input is now editable in the builder header (not only on
    // create). Changing ONLY the name and saving renames the lineage in
    // place; it must NOT publish a redundant identical version, matching
    // the standalone "Umbenennen" dialog's semantics.
    fireEvent.change(
      await screen.findByPlaceholderText("z. B. Ferienbetreuung Sommer 2026"),
      { target: { value: "Ferienprogramm" } },
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Änderungen speichern" }),
    );

    await waitFor(() => {
      expect(mocks.renameSchema).toHaveBeenCalledWith(
        "schema-1",
        "Ferienprogramm",
      );
    });
    expect(mocks.updateSchema).not.toHaveBeenCalled();
  });

  it("renames AND publishes in one atomic request when name and content both change", async () => {
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.updateSchema.mockResolvedValue(schema({ name: "Ferienprogramm" }));

    render(<EnrollmentFormEditor />);

    expect(
      await screen.findByText("Anmeldeformulare verwalten"),
    ).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Regelformular" }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );

    // Change the name AND a base-field requirement: the new name rides along
    // on the single updateSchema request so the backend renames + publishes in
    // one transaction. No separate rename round-trip the publish could leave
    // half-applied on failure.
    fireEvent.change(
      await screen.findByPlaceholderText("z. B. Ferienbetreuung Sommer 2026"),
      { target: { value: "Ferienprogramm" } },
    );
    fireEvent.click(
      await screen.findByRole("switch", {
        name: "Telefonnummer verpflichtend",
      }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Änderungen speichern" }),
    );

    await waitFor(() => {
      expect(mocks.updateSchema).toHaveBeenCalledWith(
        "schema-1",
        expect.anything(),
        expect.anything(),
        expect.anything(),
        "Ferienprogramm",
      );
    });
    // The combined save must NOT fire a separate rename request — that is the
    // non-atomic path the backend transaction replaced.
    expect(mocks.renameSchema).not.toHaveBeenCalled();
  });

  it("does not rename separately when the combined save fails", async () => {
    // Regression guard for the partial-save bug: a failed publish must not
    // leave the lineage renamed. Because the rename now rides inside the
    // single updateSchema transaction, the editor never calls renameSchema on
    // its own, so a rejected updateSchema commits nothing.
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.updateSchema.mockRejectedValue(
      new Error("Formularvorlage konnte nicht gespeichert werden"),
    );

    render(<EnrollmentFormEditor />);

    expect(
      await screen.findByText("Anmeldeformulare verwalten"),
    ).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Regelformular" }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );

    fireEvent.change(
      await screen.findByPlaceholderText("z. B. Ferienbetreuung Sommer 2026"),
      { target: { value: "Ferienprogramm" } },
    );
    fireEvent.click(
      await screen.findByRole("switch", {
        name: "Telefonnummer verpflichtend",
      }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Änderungen speichern" }),
    );

    await waitFor(() => {
      expect(mocks.updateSchema).toHaveBeenCalled();
    });
    expect(mocks.renameSchema).not.toHaveBeenCalled();
  });

  it("does not rename when the builder name is left unchanged", async () => {
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.updateSchema.mockResolvedValue(schema());

    render(<EnrollmentFormEditor />);

    expect(
      await screen.findByText("Anmeldeformulare verwalten"),
    ).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Regelformular" }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );

    // Toggle a base-field requirement so there is something to save without
    // touching the name.
    fireEvent.click(
      await screen.findByRole("switch", {
        name: "Telefonnummer verpflichtend",
      }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Änderungen speichern" }),
    );

    await waitFor(() => {
      expect(mocks.updateSchema).toHaveBeenCalled();
    });
    expect(mocks.renameSchema).not.toHaveBeenCalled();
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

  it("uploads and saves an AGB PDF source for a form template", async () => {
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.uploadEnrollmentLegalDocument.mockResolvedValue(
      "/uploads/enrollment-form-legal-documents/1_terms.pdf",
    );
    mocks.createSchema.mockResolvedValue(
      schema({ id: "schema-new", name: "PDF-Rechtstextformular" }),
    );

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    fireEvent.change(
      screen.getByPlaceholderText("z. B. Ferienbetreuung Sommer 2026"),
      {
        target: { value: "PDF-Rechtstextformular" },
      },
    );
    fireEvent.click(
      screen.getAllByRole("button", { name: /abweichend bearbeiten/i })[0]!,
    );
    fireEvent.click(
      screen.getByRole("button", { name: /PDF-Datei hochladen/ }),
    );

    const uploadInput = screen.getByLabelText("PDF hochladen");
    const file = new File(["%PDF-1.4"], "terms.pdf", {
      type: "application/pdf",
    });
    fireEvent.change(uploadInput, { target: { files: [file] } });

    await waitFor(() => {
      expect(mocks.uploadEnrollmentLegalDocument).toHaveBeenCalledWith(file);
    });
    expect(await screen.findByText("PDF gespeichert")).toBeInTheDocument();

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
      Array<{
        key: string;
        display_mode: string;
        document_url: string;
        enabled: boolean;
      }>,
    ];
    expect(legalBlocks).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          key: "agb",
          display_mode: "pdf",
          document_url: "/uploads/enrollment-form-legal-documents/1_terms.pdf",
          enabled: true,
        }),
      ]),
    );
    expect(mocks.deleteEnrollmentLegalDocument).not.toHaveBeenCalled();
  });

  it("keeps legal block edits made while an AGB PDF upload is pending", async () => {
    const upload = deferred<string>();
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.uploadEnrollmentLegalDocument.mockReturnValue(upload.promise);
    mocks.createSchema.mockResolvedValue(
      schema({ id: "schema-new", name: "PDF-Rechtstextformular" }),
    );

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    fireEvent.change(
      screen.getByPlaceholderText("z. B. Ferienbetreuung Sommer 2026"),
      {
        target: { value: "PDF-Rechtstextformular" },
      },
    );
    fireEvent.click(
      screen.getAllByRole("button", { name: /abweichend bearbeiten/i })[0]!,
    );
    fireEvent.click(
      screen.getByRole("button", { name: /PDF-Datei hochladen/ }),
    );

    fireEvent.change(screen.getByLabelText("PDF hochladen"), {
      target: {
        files: [
          new File(["%PDF-1.4"], "terms.pdf", {
            type: "application/pdf",
          }),
        ],
      },
    });
    fireEvent.change(screen.getByLabelText("Titel"), {
      target: { value: "Individuelle Teilnahmebedingungen" },
    });

    await act(async () => {
      upload.resolve("/uploads/enrollment-form-legal-documents/1_terms.pdf");
    });
    expect(await screen.findByText("PDF gespeichert")).toBeInTheDocument();

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
      Array<{ key: string; title: string; document_url: string }>,
    ];
    expect(legalBlocks).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          key: "agb",
          title: "Individuelle Teilnahmebedingungen",
          document_url: "/uploads/enrollment-form-legal-documents/1_terms.pdf",
        }),
      ]),
    );
  });

  it("clears stale AGB PDF URLs when saving the template in text mode", async () => {
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.uploadEnrollmentLegalDocument.mockResolvedValue(
      "/uploads/enrollment-form-legal-documents/1_terms.pdf",
    );
    mocks.deleteEnrollmentLegalDocument.mockResolvedValue(undefined);
    mocks.createSchema.mockResolvedValue(
      schema({ id: "schema-new", name: "Text-Rechtstextformular" }),
    );

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    fireEvent.change(
      screen.getByPlaceholderText("z. B. Ferienbetreuung Sommer 2026"),
      {
        target: { value: "Text-Rechtstextformular" },
      },
    );
    fireEvent.click(
      screen.getAllByRole("button", { name: /abweichend bearbeiten/i })[0]!,
    );
    fireEvent.click(
      screen.getByRole("button", { name: /PDF-Datei hochladen/ }),
    );
    fireEvent.change(screen.getByLabelText("PDF hochladen"), {
      target: {
        files: [
          new File(["%PDF-1.4"], "terms.pdf", {
            type: "application/pdf",
          }),
        ],
      },
    });
    await screen.findByText("PDF gespeichert");

    fireEvent.click(screen.getByRole("button", { name: /Text eingeben/ }));
    fireEvent.change(screen.getByLabelText("Rechtstext / Erklärung"), {
      target: { value: "AGB als Text" },
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
      Array<{
        key: string;
        display_mode: string;
        document_url: string;
        text: string;
      }>,
    ];
    expect(legalBlocks).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          key: "agb",
          display_mode: "text",
          document_url: "",
          text: "AGB als Text",
        }),
      ]),
    );
    expect(mocks.deleteEnrollmentLegalDocument).toHaveBeenCalledWith(
      "/uploads/enrollment-form-legal-documents/1_terms.pdf",
      expect.any(Object),
    );
  });

  it("does not delete a draft AGB PDF while its template save is in flight", async () => {
    const save = deferred<FormSchema>();
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.uploadEnrollmentLegalDocument.mockResolvedValue(
      "/uploads/enrollment-form-legal-documents/1_terms.pdf",
    );
    mocks.deleteEnrollmentLegalDocument.mockResolvedValue(undefined);
    mocks.createSchema.mockReturnValue(save.promise);

    const view = render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    fireEvent.change(
      screen.getByPlaceholderText("z. B. Ferienbetreuung Sommer 2026"),
      {
        target: { value: "PDF-Rechtstextformular" },
      },
    );
    fireEvent.click(
      screen.getAllByRole("button", { name: /abweichend bearbeiten/i })[0]!,
    );
    fireEvent.click(
      screen.getByRole("button", { name: /PDF-Datei hochladen/ }),
    );
    fireEvent.change(screen.getByLabelText("PDF hochladen"), {
      target: {
        files: [
          new File(["%PDF-1.4"], "terms.pdf", {
            type: "application/pdf",
          }),
        ],
      },
    });
    await screen.findByText("PDF gespeichert");

    fireEvent.click(
      screen.getByRole("button", { name: "Formularvorlage erstellen" }),
    );
    await waitFor(() => {
      expect(mocks.createSchema).toHaveBeenCalled();
    });

    view.unmount();
    expect(mocks.deleteEnrollmentLegalDocument).not.toHaveBeenCalled();

    await act(async () => {
      save.resolve(
        schema({ id: "schema-new", name: "PDF-Rechtstextformular" }),
      );
    });
    expect(mocks.deleteEnrollmentLegalDocument).not.toHaveBeenCalled();
  });

  it("does not enable inherited PDF AGB when the active PDF source has no document", async () => {
    mocks.fetchPublicLegalTexts.mockResolvedValue({
      agb: "Legacy AGB Text",
      agb_document_url: "",
      agb_display_mode: "pdf",
      dsgvo: "",
      email_contact: "",
      photo: "",
      terms_enabled: true,
      dsgvo_enabled: false,
      email_contact_enabled: false,
      photo_enabled: false,
      blocks: [
        {
          key: "agb",
          kind: "terms",
          title: "AGB",
          label: "Ich akzeptiere die AGB.",
          text: "Die AGB / Teilnahmebedingungen sind als PDF-Datei hinterlegt: [AGB-Dokument öffnen](/api/public/enrollment-legal-documents/legacy.pdf)",
          required: true,
          enabled: false,
          sort_order: 10,
          source: "standard",
        },
      ],
    });
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.createSchema.mockResolvedValue(
      schema({ id: "schema-new", name: "PDF ohne Datei" }),
    );

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    fireEvent.change(
      screen.getByPlaceholderText("z. B. Ferienbetreuung Sommer 2026"),
      { target: { value: "PDF ohne Datei" } },
    );
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
      Array<{
        key: string;
        enabled: boolean;
        text: string;
        display_mode: string;
        document_url: string;
      }>,
    ];
    expect(legalBlocks).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          key: "agb",
          enabled: false,
          text: "",
          display_mode: "pdf",
          document_url: "",
        }),
      ]),
    );
  });

  it("does not inherit stale AGB PDF URLs while settings are in text mode", async () => {
    mocks.fetchPublicLegalTexts.mockResolvedValue({
      agb: "",
      agb_document_url: "/uploads/enrollment-legal-documents/1_stale.pdf",
      agb_display_mode: "text",
      dsgvo: "",
      email_contact: "",
      photo: "",
      terms_enabled: true,
      dsgvo_enabled: false,
      email_contact_enabled: false,
      photo_enabled: false,
      blocks: [],
    });
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.createSchema.mockResolvedValue(
      schema({ id: "schema-new", name: "Text ohne Inhalt" }),
    );

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    fireEvent.change(
      screen.getByPlaceholderText("z. B. Ferienbetreuung Sommer 2026"),
      { target: { value: "Text ohne Inhalt" } },
    );
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
      Array<{ key: string; enabled: boolean; document_url: string }>,
    ];
    expect(legalBlocks).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          key: "agb",
          enabled: false,
          document_url: "",
        }),
      ]),
    );
  });

  it("keeps generated PDF link text out of the editable AGB text source", async () => {
    mocks.fetchPublicLegalTexts.mockResolvedValue({
      agb: "Die AGB / Teilnahmebedingungen sind als PDF-Datei hinterlegt: [AGB-Dokument öffnen](/api/public/enrollment-legal-documents/generated.pdf)",
      agb_document_url: "/uploads/enrollment-form-legal-documents/1_terms.pdf",
      agb_display_mode: "pdf",
      dsgvo: "",
      email_contact: "",
      photo: "",
      terms_enabled: true,
      dsgvo_enabled: false,
      email_contact_enabled: false,
      photo_enabled: false,
      blocks: [
        {
          key: "agb",
          kind: "terms",
          title: "AGB",
          label: "Ich akzeptiere die AGB.",
          text: "Die AGB / Teilnahmebedingungen sind als PDF-Datei hinterlegt: [AGB-Dokument öffnen](/api/public/enrollment-legal-documents/generated.pdf)",
          required: true,
          enabled: true,
          sort_order: 10,
          source: "standard",
        },
      ],
    });
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    fireEvent.click(
      screen.getAllByRole("button", { name: /abweichend bearbeiten/i })[0]!,
    );
    fireEvent.click(screen.getByRole("button", { name: /Text eingeben/ }));

    expect(screen.getByLabelText("Rechtstext / Erklärung")).toHaveValue("");
  });

  it("removes an uploaded AGB PDF source from a form template draft", async () => {
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.uploadEnrollmentLegalDocument.mockResolvedValue(
      "/uploads/enrollment-form-legal-documents/1_terms.pdf",
    );
    mocks.deleteEnrollmentLegalDocument.mockResolvedValue(undefined);

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    fireEvent.click(
      screen.getAllByRole("button", { name: /abweichend bearbeiten/i })[0]!,
    );
    fireEvent.click(
      screen.getByRole("button", { name: /PDF-Datei hochladen/ }),
    );

    const file = new File(["%PDF-1.4"], "terms.pdf", {
      type: "application/pdf",
    });
    fireEvent.change(screen.getByLabelText("PDF hochladen"), {
      target: { files: [file] },
    });

    await screen.findByText("PDF gespeichert");
    fireEvent.click(screen.getByRole("button", { name: "Entfernen" }));

    await waitFor(() => {
      expect(mocks.deleteEnrollmentLegalDocument).toHaveBeenCalledWith(
        "/uploads/enrollment-form-legal-documents/1_terms.pdf",
      );
    });
    expect(screen.queryByText("PDF gespeichert")).not.toBeInTheDocument();
  });

  it("cleans up uploaded AGB PDFs when a form template draft is discarded", async () => {
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.uploadEnrollmentLegalDocument.mockResolvedValue(
      "/uploads/enrollment-form-legal-documents/1_draft.pdf",
    );
    mocks.deleteEnrollmentLegalDocument.mockResolvedValue(undefined);

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    fireEvent.click(
      screen.getAllByRole("button", { name: /abweichend bearbeiten/i })[0]!,
    );
    fireEvent.click(
      screen.getByRole("button", { name: /PDF-Datei hochladen/ }),
    );

    const file = new File(["%PDF-1.4"], "terms.pdf", {
      type: "application/pdf",
    });
    fireEvent.change(screen.getByLabelText("PDF hochladen"), {
      target: { files: [file] },
    });
    await screen.findByText("PDF gespeichert");

    fireEvent.click(
      screen.getByRole("button", { name: "Zurück zur Übersicht" }),
    );
    fireEvent.click(await screen.findByRole("button", { name: "Verwerfen" }));

    await waitFor(() => {
      expect(mocks.deleteEnrollmentLegalDocument).toHaveBeenCalledWith(
        "/uploads/enrollment-form-legal-documents/1_draft.pdf",
        expect.any(Object),
      );
    });
  });

  it("keeps failed draft AGB PDF cleanup tracked for a later retry", async () => {
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.uploadEnrollmentLegalDocument.mockResolvedValue(
      "/uploads/enrollment-form-legal-documents/1_retry.pdf",
    );
    mocks.deleteEnrollmentLegalDocument
      .mockRejectedValueOnce(new Error("delete failed"))
      .mockResolvedValue(undefined);
    mocks.createSchema.mockResolvedValue(
      schema({ id: "schema-new", name: "Retry-Rechtstextformular" }),
    );

    const view = render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    fireEvent.change(
      screen.getByPlaceholderText("z. B. Ferienbetreuung Sommer 2026"),
      {
        target: { value: "Retry-Rechtstextformular" },
      },
    );
    fireEvent.click(
      screen.getAllByRole("button", { name: /abweichend bearbeiten/i })[0]!,
    );
    fireEvent.click(
      screen.getByRole("button", { name: /PDF-Datei hochladen/ }),
    );
    fireEvent.change(screen.getByLabelText("PDF hochladen"), {
      target: {
        files: [
          new File(["%PDF-1.4"], "terms.pdf", {
            type: "application/pdf",
          }),
        ],
      },
    });
    await screen.findByText("PDF gespeichert");

    fireEvent.click(screen.getByRole("button", { name: /Text eingeben/ }));
    fireEvent.change(screen.getByLabelText("Rechtstext / Erklärung"), {
      target: { value: "AGB als Text" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Formularvorlage erstellen" }),
    );

    await waitFor(() => {
      expect(mocks.createSchema).toHaveBeenCalled();
    });
    expect(mocks.toast.error).toHaveBeenCalledWith(
      "Nicht alle ungespeicherten PDF-Dateien konnten bereinigt werden.",
    );
    expect(mocks.deleteEnrollmentLegalDocument).toHaveBeenCalledTimes(1);

    view.unmount();

    expect(mocks.deleteEnrollmentLegalDocument).toHaveBeenCalledTimes(2);
    expect(mocks.deleteEnrollmentLegalDocument).toHaveBeenLastCalledWith(
      "/uploads/enrollment-form-legal-documents/1_retry.pdf",
      { keepalive: true },
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
      agb_document_url: "",
      agb_display_mode: "text",
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

  it("keeps a half-filled pickup-time row visible while editing", async () => {
    // The allowed-time rows are committed up to the field (deduped, non-empty)
    // and echo back through field.allowed_times. The reset effect now also
    // watches field.allowed_times so a schema/template switch refreshes the
    // rows, but it must NOT reset on the row's own commit — otherwise a freshly
    // added empty row (or a row mid-edit) would be wiped on every keystroke.
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    fireEvent.click(
      screen.getByRole("button", {
        name: /^Abholzeiten\b/,
      }),
    );

    // Adding a row leaves an empty time input on screen (it is not yet a
    // committed allowed_time, so a naive reset would drop it).
    fireEvent.click(screen.getByRole("button", { name: "Zeit hinzufügen" }));
    const firstRow = await screen.findByLabelText("Auswahlzeit 1");
    expect(firstRow).toBeInTheDocument();
    expect((firstRow as HTMLInputElement).value).toBe("");

    fireEvent.change(firstRow, { target: { value: "14:45" } });
    expect(
      (screen.getByLabelText("Auswahlzeit 1") as HTMLInputElement).value,
    ).toBe("14:45");

    // A second empty row coexists with the committed first one.
    fireEvent.click(screen.getByRole("button", { name: "Zeit hinzufügen" }));
    expect(
      (screen.getByLabelText("Auswahlzeit 1") as HTMLInputElement).value,
    ).toBe("14:45");
    expect(
      (screen.getByLabelText("Auswahlzeit 2") as HTMLInputElement).value,
    ).toBe("");
  });
});

describe("prepareFieldsForSave — fixed pickup times", () => {
  const pickupField = (overrides: Partial<FormField> = {}): FormField => ({
    key: "schedule_pickup",
    label: "Abholzeiten",
    type: "weekday_schedule",
    target: "schedule.pickup",
    applies_to_child: true,
    sort_order: 0,
    ...overrides,
  });

  it("keeps allowed_times on the rebuilt pickup target field", () => {
    const [out] = prepareFieldsForSave([
      pickupField({ allowed_times: ["14:45", "16:00"] }),
    ]);
    expect(out?.allowed_times).toEqual(["14:45", "16:00"]);
    expect(out?.target).toBe("schedule.pickup");
  });

  it("leaves allowed_times undefined when none configured", () => {
    const [out] = prepareFieldsForSave([pickupField()]);
    expect(out?.allowed_times).toBeUndefined();
  });
});
