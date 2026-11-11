/**
 * Shared setup of the enrollment admin editor tests
 * (care-offerings-editor, phases-editor and enrollment-form-editor): the
 * mock functions their `vi.mock` factories return, their reset before each
 * test, the module stand-ins, fixtures and DOM helpers.
 *
 * The mocks are created inside each file's `vi.hoisted`:
 * `const mocks = await vi.hoisted(async () => (await import("~/test/enrollment-editors")).createEditorMocks());`
 */
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { expect, vi } from "vitest";
import type { CareOffering } from "~/lib/care-offering-api";
import type { CalendarPeriod } from "~/lib/calendar-period-helpers";
import type { FormSchema } from "~/lib/enrollment-form-schema-api";
import type { Phase } from "~/lib/enrollment-phase-api";

export function createEditorMocks() {
  return {
    createCareOffering: vi.fn(),
    createLateInvite: vi.fn(),
    createManualApprovedEnrollment: vi.fn(),
    createPhase: vi.fn(),
    createSchema: vi.fn(),
    cloneCareOffering: vi.fn(),
    deleteCareOffering: vi.fn(),
    deletePhase: vi.fn(),
    getPhaseDeleteImpact: vi.fn(),
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
    showActionError: vi.fn(),
    showActionMessage: vi.fn(),
  };
}

export type EditorMocks = ReturnType<typeof createEditorMocks>;

/**
 * Vaul (SlideOver) rendert in jsdom nichts: das Panel, in dem die
 * Anmeldephasen bearbeitet werden, bliebe im Test leer. Derselbe Ersatz wie
 * in components/ui/slide-over.test.tsx: die Struktur bleibt, nur die
 * Animationsschicht fällt weg.
 */
export async function vaulMock() {
  const React = await import("react");

  return {
    Drawer: {
      Root: ({
        children,
        open,
      }: {
        children: React.ReactNode;
        open?: boolean;
      }) => (open === false ? null : <div>{children}</div>),
      Portal: ({ children }: { children: React.ReactNode }) => <>{children}</>,
      Overlay: React.forwardRef<
        HTMLDivElement,
        React.HTMLAttributes<HTMLDivElement>
      >((props, ref) => <div ref={ref} {...props} />),
      Content: React.forwardRef<
        HTMLDivElement,
        React.HTMLAttributes<HTMLDivElement>
      >((props, ref) => <div ref={ref} {...props} />),
      Close: React.forwardRef<
        HTMLButtonElement,
        React.ButtonHTMLAttributes<HTMLButtonElement>
      >((props, ref) => <button ref={ref} {...props} />),
      Title: React.forwardRef<
        HTMLHeadingElement,
        React.HTMLAttributes<HTMLHeadingElement>
      >(({ children, ...props }, ref) => (
        <h2 ref={ref} {...props}>
          {children ?? "Titel"}
        </h2>
      )),
      Description: React.forwardRef<
        HTMLParagraphElement,
        React.HTMLAttributes<HTMLParagraphElement>
      >((props, ref) => <p ref={ref} {...props} />),
    },
  };
}

export function rolloverFormMock() {
  return {
    RolloverForm: ({
      onCancel,
      onSuccess,
    }: {
      onCancel: () => void;
      onSuccess: (
        result: {
          phase: { name: string };
          summary: {
            rolled_count: number;
            review_count: number;
            enqueued_emails: number;
          };
        },
        op: { isCurrent: () => boolean },
      ) => void;
    }) => (
      <div>
        <button type="button" onClick={onCancel}>
          Rollover abbrechen
        </button>
        <button
          type="button"
          onClick={() =>
            onSuccess(
              {
                phase: { name: "Schuljahr 2027/28" },
                summary: {
                  rolled_count: 2,
                  review_count: 1,
                  enqueued_emails: 3,
                },
              },
              // The real form reports its panel session; this one is current.
              { isCurrent: () => true },
            )
          }
        >
          Rollover fertig
        </button>
      </div>
    ),
  };
}

export function phase(overrides: Partial<Phase> = {}): Phase {
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

export function schema(overrides: Partial<FormSchema> = {}): FormSchema {
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

export function offering(overrides: Partial<CareOffering> = {}): CareOffering {
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

export function calendarPeriod(
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

export function inputByName(name: string): HTMLInputElement {
  return document.querySelector(`input[name="${name}"]`) as HTMLInputElement;
}

export function textareaByName(name: string): HTMLTextAreaElement {
  return document.querySelector(
    `textarea[name="${name}"]`,
  ) as HTMLTextAreaElement;
}

export async function waitForInputByName(
  name: string,
): Promise<HTMLInputElement> {
  await waitFor(() => expect(inputByName(name)).toBeInTheDocument());
  return inputByName(name);
}

export function setPickupTime(day: string, value = "14:30") {
  fireEvent.change(screen.getByLabelText(`${day} Gehzeit`), {
    target: { value },
  });
}

export async function chooseOption(
  label: string | RegExp,
  optionName: string | RegExp,
) {
  fireEvent.click(screen.getByLabelText(label));
  fireEvent.click(await screen.findByRole("option", { name: optionName }));
}

export function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

export function editStandardLegalBlock(index: number) {
  const displaySwitch = screen.getAllByRole("switch", {
    name: /in dieser Vorlage anzeigen/i,
  })[index]!;
  const actionMenu = within(displaySwitch.parentElement!).getByRole("button", {
    name: /^Aktionen für /,
  });

  fireEvent.click(actionMenu);
  fireEvent.click(
    screen.getByRole("menuitem", { name: "Abweichend bearbeiten" }),
  );
}

export function resetEditorMocks(mocks: EditorMocks) {
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
}
