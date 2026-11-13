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

// useTenantMutate scopes its keys to the signed-in account (#5006).
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "1" }, expires: "2099-01-01" },
    status: "authenticated",
  }),
}));

vi.mock("vaul", async () => {
  const original = await (await import("~/test/enrollment-editors")).vaulMock();
  return {
    Drawer: {
      ...original.Drawer,
      Root: ({
        children,
        open,
        dismissible,
        onOpenChange,
      }: {
        children: React.ReactNode;
        open?: boolean;
        dismissible?: boolean;
        onOpenChange?: (open: boolean) => void;
      }) =>
        open === false ? null : (
          <div>
            <button
              type="button"
              disabled={dismissible === false}
              onClick={() => onOpenChange?.(false)}
            >
              Dismiss panel
            </button>
            {children}
          </div>
        ),
    },
  };
});

const mocks = await vi.hoisted(async () =>
  (await import("~/test/enrollment-editors")).createEditorMocks(),
);

// The date and datetime fields moved from native inputs to the kit pickers;
// these stubs keep them settable via fireEvent.change. Imported inside the
// factories because vi.mock is hoisted above the imports.
vi.mock("~/components/ui/date-picker", async () =>
  (await import("~/test/mocks/date-picker")).datePickerModuleMock(),
);

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

// Form, load and dialog errors run through the real hooks (#2515); only the
// toast surface and the toast path for actions without a form are doubles.
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  useToast: () => mocks.toast,
  useApiErrorDisplay: () => ({
    show: mocks.showActionError,
    showMessage: mocks.showActionMessage,
  }),
}));

vi.mock("~/lib/swr", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  useSWRAuth: () => ({ data: [], error: undefined, isLoading: false }),
}));

vi.mock("./rollover-form", async () =>
  (await import("~/test/enrollment-editors")).rolloverFormMock(),
);

vi.mock("~/lib/enrollment-phase-api", async (importOriginal) => {
  const actual = (await importOriginal()) as Record<string, unknown>;
  return {
    ...actual,
    createPhase: mocks.createPhase,
    deletePhase: mocks.deletePhase,
    getPhaseDeleteImpact: mocks.getPhaseDeleteImpact,
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
    withoutSignature?: boolean;
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
import type { CareOffering } from "~/lib/care-offering-api";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";
import { SAVED_BUT_STALE_SUFFIX } from "~/lib/hooks/use-refresh-after-write";
import type { Phase } from "~/lib/enrollment-phase-api";
import {
  phase,
  offering,
  calendarPeriod,
  inputByName,
  textareaByName,
  waitForInputByName,
  setPickupTime,
  chooseOption,
  deferred,
  resetEditorMocks,
} from "~/test/enrollment-editors";
import {
  expectPageReloadedAfterRefusal,
  resetPageReloads,
} from "~/test/helpers/refused-refresh";

beforeEach(() => {
  resetEditorMocks(mocks);
});

describe("CareOfferingsEditor", () => {
  it("keeps the editor open while saving through outside dismissal", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings.mockResolvedValue([offering()]);
    let finish!: (value: ReturnType<typeof offering>) => void;
    mocks.updateCareOffering.mockImplementation(
      () =>
        new Promise<ReturnType<typeof offering>>((resolve) => {
          finish = resolve;
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
    fireEvent.click(await screen.findByRole("button", { name: "Speichern" }));
    await waitFor(() => expect(mocks.updateCareOffering).toHaveBeenCalled());
    expect(
      screen.getByRole("button", { name: "Dismiss panel" }),
    ).toBeDisabled();
    finish(offering());
    await waitFor(() =>
      expect(
        screen.queryByRole("button", { name: "Dismiss panel" }),
      ).not.toBeInTheDocument(),
    );
  });

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

  // #6044: the checkbox read „Für alle Klassenstufen“ while it was NOT
  // ticked for an unrestricted offering, and ticking it restricted the
  // offering. A school ticked it to mean „for everyone“. The label must say
  // what a tick does, in both states.
  it("names the availability checkbox after what a tick does", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings.mockResolvedValue([]);

    render(<CareOfferingsEditor />);
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Neues Betreuungsangebot",
      }),
    );

    const restrict = screen.getByRole("checkbox", {
      name: /^Nur für bestimmte Klassenstufen anbieten/,
    });
    expect(restrict).not.toBeChecked();
    expect(
      screen.getByText("Ohne Haken können alle Kinder das Angebot wählen."),
    ).toBeVisible();

    fireEvent.click(restrict);

    expect(
      screen.getByRole("checkbox", {
        name: /^Nur für bestimmte Klassenstufen anbieten/,
      }),
    ).toBeChecked();
    expect(
      screen.getByText("Bedingung 1: Wählen Sie mindestens eine Klassenstufe."),
    ).toBeVisible();
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
        name: "Neues Betreuungsangebot",
      }),
    );

    expect(
      screen.getByRole("checkbox", {
        name: /^Nur für bestimmte Klassenstufen anbieten/,
      }),
    ).not.toBeChecked();
    fireEvent.change(await waitForInputByName("name"), {
      target: { value: "Randstunde" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Mo" }));
    setPickupTime("Mo");
    fireEvent.click(
      screen.getByRole("checkbox", {
        name: /^Nur für bestimmte Klassenstufen anbieten/,
      }),
    );

    expect(
      screen.getByText("Bedingung 1: Wählen Sie mindestens eine Klassenstufe."),
    ).toBeVisible();
    const create = screen.getByRole("button", { name: "Erstellen" });
    expect(create).toHaveAttribute("aria-disabled", "true");
    expect(create).toHaveAccessibleDescription(
      "Vervollständigen Sie zuerst die markierten Bedingungen.",
    );
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

    // Der Hinweis steht am Feld und wird sofort vorgelesen (Bauart 2
    // Regel 5, #3113).
    expect(screen.getByRole("alert")).toHaveTextContent(
      /Regeltermin deckt die Angebotstage Di, Mi, Do, Fr nicht ab/,
    );
    expect(
      screen.getByText(
        "Der Regeltermin enthält Tage, die im Angebot nicht auswählbar sind.",
      ),
    ).toBeInTheDocument();
    const create = screen.getByRole("button", { name: "Erstellen" });
    expect(create).toHaveAttribute("aria-disabled", "true");
    expect(create).toHaveAccessibleDescription(
      "Wählen Sie einen passenden Regeltermin oder entfernen Sie ihn.",
    );

    for (const day of ["Di", "Mi", "Do", "Fr"]) {
      fireEvent.click(screen.getByRole("button", { name: day }));
    }

    expect(
      screen.queryByText(/Regeltermin deckt die Angebotstage/),
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
        name: /^Nur für bestimmte Klassenstufen anbieten/,
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

  // #4782: free places per offering at a glance.
  it("shows occupancy, free places and the full status in the list", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings.mockResolvedValue([
      offering({ id: "offer-1", name: "Regelbetreuung", capacity: 10 }),
      offering({ id: "offer-2", name: "Randstunde", capacity: 5 }),
      offering({ id: "offer-3", name: "Frühbetreuung", capacity: null }),
    ]);
    mocks.fetchCareOfferingBookingStats.mockResolvedValue({
      "offer-1": {
        offering_id: "offer-1",
        capacity: 10,
        booked: 7,
        grade_levels: {},
        unknown_grade_count: 0,
      },
      "offer-2": {
        offering_id: "offer-2",
        capacity: 5,
        booked: 5,
        grade_levels: {},
        unknown_grade_count: 0,
      },
      "offer-3": {
        offering_id: "offer-3",
        capacity: null,
        booked: 4,
        grade_levels: {},
        unknown_grade_count: 0,
      },
    });

    render(<CareOfferingsEditor />);

    const regular = within(
      (await screen.findByText("7 von 10")).closest("tr") as HTMLElement,
    );
    expect(screen.getByRole("button", { name: /^Belegt/ })).toBeInTheDocument();
    expect(regular.getByText("Regelbetreuung")).toBeInTheDocument();
    expect(regular.getByText("3 frei")).toBeInTheDocument();
    expect(regular.queryByText("Voll")).not.toBeInTheDocument();

    const full = within(
      screen.getByText("5 von 5").closest("tr") as HTMLElement,
    );
    expect(full.getByText("Randstunde")).toBeInTheDocument();
    expect(full.getByText("Keine frei")).toBeInTheDocument();
    expect(full.getByText("Voll")).toBeInTheDocument();

    // Without a capacity limit only the occupancy.
    const unlimited = within(
      screen.getByText("Frühbetreuung").closest("tr") as HTMLElement,
    );
    expect(unlimited.getByText("4")).toBeInTheDocument();
    expect(unlimited.getByText("Unbegrenzt")).toBeInTheDocument();

    // The capacity tile labels the free places as active offers: 3 + 0.
    expect(screen.getByText("· 3 frei (aktive Angebote)")).toBeInTheDocument();
    expect(
      screen.getByText("· 3 frei (aktive Angebote)").parentElement,
    ).toHaveTextContent("15 · 3 frei (aktive Angebote)");
  });

  it("shows a zero capacity instead of treating it as unlimited", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings.mockResolvedValue([
      offering({ id: "offer-1", capacity: 0 }),
    ]);
    mocks.fetchCareOfferingBookingStats.mockResolvedValue({
      "offer-1": {
        offering_id: "offer-1",
        capacity: 0,
        booked: 0,
        grade_levels: {},
        unknown_grade_count: 0,
      },
    });

    render(<CareOfferingsEditor />);

    expect(await screen.findByText("0 von 0")).toBeInTheDocument();
    const freePlaces = screen.getByText("· 0 frei (aktive Angebote)");
    expect(freePlaces.parentElement).toHaveTextContent(
      "0 · 0 frei (aktive Angebote)",
    );
  });

  it("hides the occupancy when it fails to load and keeps the list usable", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings.mockResolvedValue([
      offering({ id: "offer-1", name: "Regelbetreuung", capacity: 10 }),
    ]);
    mocks.fetchCareOfferingBookingStats.mockRejectedValue(
      new Error("stats down"),
    );

    render(<CareOfferingsEditor />);

    expect(await screen.findByText("Regelbetreuung")).toBeInTheDocument();
    await waitFor(() =>
      expect(mocks.fetchCareOfferingBookingStats).toHaveBeenCalled(),
    );
    await waitFor(() =>
      expect(
        screen.queryByRole("button", { name: /^Belegt/ }),
      ).not.toBeInTheDocument(),
    );
    expect(screen.getByRole("button", { name: /^Preis/ })).toBeInTheDocument();
    expect(screen.getByText("10 Plätze")).toBeInTheDocument();
    expect(screen.queryByText(/frei$/)).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Neues Betreuungsangebot" }),
    ).toBeEnabled();
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
        name: /^Nur für bestimmte Klassenstufen anbieten/,
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
        name: "Neues Betreuungsangebot",
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
    expect(save).toHaveAttribute("aria-disabled", "true");
    expect(save).toHaveAccessibleDescription(
      "Wählen Sie einen passenden Regeltermin oder entfernen Sie ihn.",
    );

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
      screen.getByText(/muss den ganzen Betreuungszeitraum/),
    ).toBeInTheDocument();
    const save = screen.getByRole("button", { name: "Speichern" });
    expect(save).toHaveAttribute("aria-disabled", "true");
    expect(save).toHaveAccessibleDescription(
      "Wählen Sie einen passenden Regeltermin oder entfernen Sie ihn.",
    );
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
    mocks.updateCareOffering.mockRejectedValue(
      new ApiError(technicalError, 500, { code: "general.server" }),
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
    await waitFor(() =>
      expect(screen.getByLabelText("Regeltermin")).toHaveTextContent(
        "Lernzeit",
      ),
    );
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    const saveError = await screen.findByText(
      catalogText("general.server", "das Betreuungsangebot"),
    );
    expect(saveError).toBeVisible();
    // Der Speicherfehler steht im Alert oben im Panel, nicht im Toast
    // (Bauart 2 Regel 5, #3113).
    expect(saveError.closest('[role="alert"]')).not.toBeNull();
    expect(screen.queryByText(technicalError)).not.toBeInTheDocument();
    expect(mocks.toast.error).not.toHaveBeenCalled();
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
    // Das Bearbeiten-Formular steht im SlideOver neben der Liste; die
    // Überschrift trägt jetzt der Panelkopf, nicht mehr das Formular.
    const form = document
      .querySelector<HTMLInputElement>('input[name="name"]')
      ?.closest("form");
    expect(form).toBeTruthy();
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
    // Das Bearbeiten-Formular steht im SlideOver neben der Liste; die
    // Überschrift trägt jetzt der Panelkopf, nicht mehr das Formular.
    const form = document
      .querySelector<HTMLInputElement>('input[name="name"]')
      ?.closest("form");
    expect(form).toBeTruthy();
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
      failedCatalogRequest.reject(
        new ApiError("Ferienkatalog nicht erreichbar", 503, {
          code: "general.unavailable",
        }),
      );
      await failedCatalogRequest.promise.catch(() => undefined);
      await new Promise((resolve) => setTimeout(resolve, 0));
    });

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Liste der Betreuungsangebote"),
      ),
    ).toBeVisible();
    expect(
      screen.queryByText("Ferienkatalog nicht erreichbar"),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Regelbetreuung")).not.toBeInTheDocument();
    expect(
      screen.queryByText("Noch kein Betreuungsangebot angelegt"),
    ).not.toBeInTheDocument();

    targetPhaseFails = false;
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
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

    mocks.listPhases.mockRejectedValueOnce(
      new ApiError("Phasen kaputt", 500, { code: "general.server" }),
    );
    render(<CareOfferingsEditor />);
    expect(await screen.findByRole("alert")).toHaveTextContent(
      catalogText("general.server", "die Liste der Betreuungsangebote"),
    );
    expect(screen.queryByText("Phasen kaputt")).not.toBeInTheDocument();
    // Kein Leerzustand hinter dem Ladefehler.
    expect(
      screen.queryByText("Erst eine Anmeldephase anlegen"),
    ).not.toBeInTheDocument();
  });
  // #2517: Der Seitenkopf bekommt nach einem Ladefehler keine "0 Angebote".
  it("reports an unavailable summary, not zero, when the catalog fails", async () => {
    mocks.listPhases.mockRejectedValueOnce(
      new ApiError("Phasen kaputt", 500, { code: "general.server" }),
    );
    const onSummaryChange = vi.fn();
    render(<CareOfferingsEditor onSummaryChange={onSummaryChange} />);

    expect(await screen.findByRole("alert")).toHaveTextContent(
      catalogText("general.server", "die Liste der Betreuungsangebote"),
    );
    await waitFor(() =>
      expect(onSummaryChange).toHaveBeenLastCalledWith("unavailable"),
    );
    expect(onSummaryChange).not.toHaveBeenCalledWith({ total: 0, active: 0 });
  });
  it("keeps a failed offering delete in the open dialog (#2515)", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings.mockResolvedValue([offering()]);
    mocks.deleteCareOffering.mockRejectedValue(
      new ApiError("in use", 409, { code: "general.business_rejection" }),
    );

    render(<CareOfferingsEditor />);

    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Regelbetreuung",
      }),
    );
    fireEvent.click(await screen.findByRole("menuitem", { name: "Löschen" }));
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Ja, löschen" }),
    );
    fireEvent.click(
      await within(dialog).findByRole("button", { name: "Endgültig löschen" }),
    );

    expect(
      await within(dialog).findByText(
        catalogText("general.business_rejection", "das Betreuungsangebot"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("in use")).not.toBeInTheDocument();
    expect(mocks.toast.error).not.toHaveBeenCalled();
  });

  it.each(["phases", "offerings"])(
    "loads the page anew when the post-write %s read is refused",
    async (source) => {
      resetPageReloads();
      mocks.listPhases.mockResolvedValue([phase()]);
      mocks.listCareOfferings.mockReset().mockResolvedValue([offering()]);
      mocks.deleteCareOffering.mockResolvedValue(undefined);
      render(<CareOfferingsEditor />);
      fireEvent.click(
        await screen.findByRole("button", {
          name: "Aktionen für Regelbetreuung",
        }),
      );
      fireEvent.click(await screen.findByRole("menuitem", { name: "Löschen" }));
      const dialog = await screen.findByRole("dialog");
      fireEvent.click(
        within(dialog).getByRole("button", { name: "Ja, löschen" }),
      );
      (source === "phases"
        ? mocks.listPhases
        : mocks.listCareOfferings
      ).mockRejectedValueOnce(new ApiError("refused", 403));
      fireEvent.click(
        await within(dialog).findByRole("button", {
          name: "Endgültig löschen",
        }),
      );
      // #6801: the page loads anew; its first load shows the refusal.
      await expectPageReloadedAfterRefusal();
      expect(mocks.deleteCareOffering).toHaveBeenCalledTimes(1);
    },
  );

  // #6652: the deletion holds once the write is through; a failed reload keeps
  // the confirmed catalog and says the change is saved.
  it("keeps the catalog after a deletion whose reload fails", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings
      .mockReset()
      .mockResolvedValueOnce([offering()])
      .mockRejectedValueOnce(
        new ApiError("down", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValue([]);
    mocks.deleteCareOffering.mockResolvedValue(undefined);

    render(<CareOfferingsEditor />);
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Regelbetreuung",
      }),
    );
    fireEvent.click(await screen.findByRole("menuitem", { name: "Löschen" }));
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Ja, löschen" }),
    );
    fireEvent.click(
      await within(dialog).findByRole("button", { name: "Endgültig löschen" }),
    );

    expect(
      await screen.findByText(
        `${catalogText("general.unavailable", "die Anzeige der Betreuungsangebote")} ${SAVED_BUT_STALE_SUFFIX}`,
      ),
    ).toBeInTheDocument();
    // Anker: der bestätigte Katalog bleibt stehen.
    expect(screen.getByText("Regelbetreuung")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() =>
      expect(mocks.listCareOfferings).toHaveBeenCalledTimes(3),
    );
    expect(mocks.deleteCareOffering).toHaveBeenCalledTimes(1);
  });
});
