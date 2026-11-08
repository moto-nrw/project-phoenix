import {
  fireEvent,
  render as renderComponent,
  screen,
  waitFor,
} from "@testing-library/react";
import type { ReactElement } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";
import type { CareOffering } from "~/lib/care-offering-api";
import type { AdminRequestChild } from "~/lib/enrollment-admin-api";
import type { OfferingAdjustmentSnapshot } from "~/lib/offering-adjustment-history";
import { useCareOfferingsEnabled } from "~/lib/tenant-context";

// Errors and success messages run through the real display path (#2515).
const render = (ui: ReactElement) =>
  renderComponent(ui, { wrapper: ToastProvider });

const mocks = vi.hoisted(() => ({
  listCareOfferings: vi.fn(),
  listAdminChildOfferingAdjustments: vi.fn(),
  updateAdminChildOfferings: vi.fn(),
  fetchCareOfferingBookingStats: vi.fn(),
}));

// Die Auswirkung auf den Betreuungsplan (#5102) liest die Berechtigungen.
vi.mock("next-auth/react", async (importOriginal) => ({
  ...(await importOriginal<typeof import("next-auth/react")>()),
  useSession: () => ({
    data: { user: { id: "1", token: "t", permissions: ["schedules:manage"] } },
    status: "authenticated",
  }),
}));

vi.mock("~/lib/care-offering-api", () => ({
  listCareOfferings: mocks.listCareOfferings,
}));

vi.mock("~/lib/care-offering-booking-stats", async (importOriginal) => {
  const actual = (await importOriginal()) as Record<string, unknown>;
  return {
    ...actual,
    fetchCareOfferingBookingStats: mocks.fetchCareOfferingBookingStats,
  };
});

vi.mock("~/lib/enrollment-admin-api", async (importOriginal) => {
  const actual = (await importOriginal()) as Record<string, unknown>;
  return {
    ...actual,
    listAdminChildOfferingAdjustments: mocks.listAdminChildOfferingAdjustments,
    updateAdminChildOfferings: mocks.updateAdminChildOfferings,
  };
});

import { ChildOfferingAdjustment } from "./child-offering-adjustment";

function catalogOffering(overrides: Partial<CareOffering> = {}): CareOffering {
  return {
    id: "offering-1",
    phase_id: "phase-1",
    name: "Ganztag",
    days_of_week_mode: "fixed",
    available_days: ["mon"],
    includes_holiday_care: false,
    includes_lunch: false,
    is_active: true,
    is_required: false,
    sort_order: 1,
    created_at: "2026-06-18T11:15:00Z",
    updated_at: "2026-06-18T11:15:00Z",
    ...overrides,
  };
}

function adjustmentChild(
  overrides: Partial<AdminRequestChild> = {},
): AdminRequestChild {
  return {
    id: "child-1",
    first_name: "Lina",
    last_name: "Kind",
    date_of_birth: "2018-01-01",
    status: "approved",
    activation_mode: "scheduled",
    ...overrides,
  };
}

function renderAdjustment(child: AdminRequestChild = adjustmentChild()) {
  return render(
    <ChildOfferingAdjustment
      requestId="request-1"
      phaseId="phase-1"
      onSaved={vi.fn()}
      child={child}
    />,
  );
}

beforeEach(() => {
  mocks.listCareOfferings.mockReset();
  mocks.listAdminChildOfferingAdjustments.mockReset();
  mocks.updateAdminChildOfferings.mockReset();
  mocks.listAdminChildOfferingAdjustments.mockResolvedValue([]);
  mocks.fetchCareOfferingBookingStats.mockReset();
  mocks.fetchCareOfferingBookingStats.mockResolvedValue({});
  vi.mocked(useCareOfferingsEnabled).mockReturnValue(true);
});

describe("ChildOfferingAdjustment", () => {
  // Bauart 2 Regel 3 (#3119): bearbeitet wird am Objekt. „Bearbeiten"
  // schaltet die Fläche in den Bearbeiten-Zustand, kein Dialog, kein
  // Scroll-Lock; „Abbrechen" schaltet zurück und zeigt den Einstieg wieder.
  it("switches the panel into an in-place edit state and back", async () => {
    mocks.listCareOfferings.mockResolvedValue([catalogOffering()]);
    renderAdjustment(adjustmentChild());

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));

    expect(
      await screen.findByRole("checkbox", { name: /Ganztag/ }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(document.documentElement.style.overflow).not.toBe("hidden");
    expect(
      screen.queryByRole("button", { name: "Bearbeiten" }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Nachbearbeitung")).toBeVisible();
    expect(screen.getByLabelText("Begründung")).toBeVisible();
    expect(screen.getByRole("button", { name: "Speichern" })).toBeVisible();

    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));

    expect(
      screen.queryByRole("checkbox", { name: /Ganztag/ }),
    ).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Begründung")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Bearbeiten" })).toBeVisible();
    expect(mocks.updateAdminChildOfferings).not.toHaveBeenCalled();
  });

  it("toggles a parent-choice day in place and saves the picked days", async () => {
    mocks.listCareOfferings.mockResolvedValue([
      catalogOffering({
        id: "offering-1",
        name: "Ganztag",
        days_of_week_mode: "parent_choice",
        available_days: ["mon", "tue"],
      }),
    ]);
    mocks.updateAdminChildOfferings.mockResolvedValue({});
    renderAdjustment(
      adjustmentChild({
        offerings: [
          {
            offering_id: "offering-1",
            offering_name: "Ganztag",
            days_of_week_mode: "parent_choice",
            selected_days: ["mon"],
            manual_selected_days: ["mon"],
            available_days: ["mon", "tue"],
          },
        ],
      }),
    );

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    const tuesday = await screen.findByRole("button", {
      name: "Di",
      pressed: false,
    });
    expect(screen.getByRole("button", { name: "Mo" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    fireEvent.click(tuesday);
    expect(tuesday).toHaveAttribute("aria-pressed", "true");
    fireEvent.change(screen.getByLabelText("Begründung"), {
      target: { value: "Dienstag ergänzt" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(mocks.updateAdminChildOfferings).toHaveBeenCalledWith(
        "request-1",
        "child-1",
        expect.objectContaining({
          reason: "Dienstag ergänzt",
          offerings: [
            { offering_id: "offering-1", selected_days: ["mon", "tue"] },
          ],
        }),
      );
    });
  });

  it("excludes deselected manual days from chained automatic bookings", async () => {
    mocks.listCareOfferings.mockResolvedValue([
      catalogOffering({ id: "a", name: "Ganztag" }),
      catalogOffering({
        id: "b",
        name: "Randstunde",
        days_of_week_mode: "parent_choice",
        available_days: ["mon", "tue"],
        auto_add_trigger_offering_ids: ["a"],
      }),
      catalogOffering({
        id: "c",
        name: "Folgeangebot",
        days_of_week_mode: "parent_choice",
        available_days: ["mon", "tue"],
        auto_add_trigger_offering_ids: ["b"],
      }),
    ]);
    mocks.updateAdminChildOfferings.mockResolvedValue({});
    renderAdjustment(
      adjustmentChild({
        offerings: [
          {
            offering_id: "a",
            offering_name: "Ganztag",
            days_of_week_mode: "fixed",
            available_days: ["mon"],
          },
          {
            offering_id: "b",
            offering_name: "Randstunde",
            days_of_week_mode: "parent_choice",
            available_days: ["mon", "tue"],
            selected_days: ["tue"],
            manual_selected_days: ["tue"],
          },
        ],
      }),
    );

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    const intermediate = await screen.findByRole("checkbox", {
      name: /Randstunde/,
    });
    const downstream = screen.getByRole("checkbox", { name: /Folgeangebot/ });
    expect(intermediate).toBeChecked();
    expect(downstream).toHaveAccessibleName(
      "Folgeangebot automatisch mitgebucht: Mo, Di",
    );

    fireEvent.click(intermediate);
    expect(downstream).toHaveAccessibleName(
      "Folgeangebot automatisch mitgebucht: Mo",
    );

    fireEvent.click(intermediate);
    expect(downstream).toHaveAccessibleName(
      "Folgeangebot automatisch mitgebucht: Mo, Di",
    );
    fireEvent.click(intermediate);
    expect(downstream).toHaveAccessibleName(
      "Folgeangebot automatisch mitgebucht: Mo",
    );
    fireEvent.change(screen.getByLabelText("Begründung"), {
      target: { value: "Randstunde abgewählt" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(mocks.updateAdminChildOfferings).toHaveBeenCalledWith(
        "request-1",
        "child-1",
        expect.objectContaining({ offerings: [{ offering_id: "a" }] }),
      );
    });
  });

  // #5102: nach dem Speichern steht da, was im Betreuungsplan passiert ist,
  // auch welcher von Hand gepflegte Regeltermin das Kind weiter einplant.
  it("shows the effect on the Betreuungsplan after saving", async () => {
    mocks.listCareOfferings.mockResolvedValue([
      catalogOffering({
        id: "offering-1",
        name: "Kurzbetreuung",
        days_of_week_mode: "parent_choice",
        available_days: ["mon", "fri"],
      }),
    ]);
    mocks.updateAdminChildOfferings.mockResolvedValue({
      roster_impact: {
        effective_from: "2026-09-14",
        adjusted: [
          {
            template_id: "7",
            name: "Kurzbetreuung Block",
            removed_weekdays: [5],
            added_weekdays: [],
          },
        ],
        not_adjusted: [
          {
            template_id: "9",
            name: "RDST 1b",
            weekdays: [5],
            via_target: true,
          },
        ],
      },
    });
    renderAdjustment(
      adjustmentChild({
        created_student_id: "42",
        offerings: [
          {
            offering_id: "offering-1",
            offering_name: "Kurzbetreuung",
            days_of_week_mode: "parent_choice",
            selected_days: ["mon", "fri"],
            manual_selected_days: ["mon", "fri"],
            available_days: ["mon", "fri"],
          },
        ],
      }),
    );

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    fireEvent.click(
      await screen.findByRole("button", { name: "Fr", pressed: true }),
    );
    fireEvent.change(screen.getByLabelText("Begründung"), {
      target: { value: "Freitag abgemeldet" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        "Auswirkung auf den Betreuungsplan ab 14.09.2026",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText(/freitags entfernt/)).toBeInTheDocument();
    expect(screen.getByText("RDST 1b")).toBeInTheDocument();
    expect(screen.getByText(/Kind bleibt eingeplant/)).toBeInTheDocument();
  });

  // Bauart 2 Regel 5: der Fehler steht oben im Bearbeiten-Bereich, die
  // Fläche bleibt im Bearbeiten-Zustand, damit die Eingabe nicht verloren geht.
  it("keeps the edit state and shows a failed save in the alert", async () => {
    mocks.listCareOfferings.mockResolvedValue([catalogOffering()]);
    mocks.updateAdminChildOfferings.mockRejectedValue(
      new ApiError("save failed", 500, { code: "general.server" }),
    );
    renderAdjustment(adjustmentChild());

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    await screen.findByRole("checkbox", { name: /Ganztag/ });
    fireEvent.change(screen.getByLabelText("Begründung"), {
      target: { value: "Testgrund" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      catalogText("general.server", "die Angebotsänderung"),
    );
    expect(screen.getByLabelText("Begründung")).toHaveValue("Testgrund");
    expect(screen.getByRole("button", { name: "Speichern" })).toBeEnabled();
    expect(
      screen.queryByRole("button", { name: "Bearbeiten" }),
    ).not.toBeInTheDocument();
  });

  it("requires a reason before saving and says so in place", async () => {
    mocks.listCareOfferings.mockResolvedValue([catalogOffering()]);
    renderAdjustment(adjustmentChild());

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    await screen.findByRole("checkbox", { name: /Ganztag/ });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText("Bitte tragen Sie eine Begründung ein."),
    ).toBeVisible();
    // Das Feld selbst ist markiert und nennt, was fehlt.
    expect(screen.getByLabelText("Begründung")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
    expect(screen.getByText("Die Begründung fehlt.")).toBeVisible();
    expect(mocks.updateAdminChildOfferings).not.toHaveBeenCalled();
  });

  // #2774 (kein Dialog über einem Dialog) gilt weiter, nur andersherum: die
  // Rückfrage ist jetzt der einzige Dialog und liegt über dem
  // Bearbeiten-Zustand. „Zurück" lässt Auswahl und Begründung stehen.
  it("returns from the withdrawal confirmation into the untouched edit state", async () => {
    mocks.listCareOfferings.mockResolvedValue([
      catalogOffering({ counts_as_care: true }),
    ]);
    mocks.updateAdminChildOfferings.mockRejectedValueOnce(
      new ApiError("confirmation required", 409, {
        code: "enrollment.complete_withdrawal_confirmation_required",
      }),
    );
    renderAdjustment(
      adjustmentChild({
        offerings: [
          {
            offering_id: "offering-1",
            offering_name: "Ganztag",
            days_of_week_mode: "fixed",
            selected_days: ["mon"],
            manual_selected_days: ["mon"],
            available_days: ["mon"],
          },
        ],
      }),
    );

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Alle Betreuungstage entfernen",
      }),
    );
    fireEvent.change(screen.getByLabelText("Begründung"), {
      target: { value: "Kein Betreuungsbedarf mehr" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    const dialog = await screen.findByRole("dialog", {
      name: "Alle Betreuungstage entfernen?",
    });
    // Nur ein Dialog: der Bearbeiten-Zustand darunter ist keine zweite Ebene.
    expect(screen.getAllByRole("dialog")).toHaveLength(1);
    fireEvent.click(screen.getByRole("button", { name: "Zurück" }));

    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    expect(screen.getByLabelText("Begründung")).toHaveValue(
      "Kein Betreuungsbedarf mehr",
    );
    expect(screen.getByRole("checkbox", { name: /Ganztag/ })).not.toBeChecked();
    expect(screen.getByRole("button", { name: "Speichern" })).toBeVisible();
    expect(mocks.updateAdminChildOfferings).toHaveBeenCalledOnce();
  });

  it("requires the exact confirmation before removing the final care day", async () => {
    mocks.listCareOfferings.mockResolvedValue([
      catalogOffering({ counts_as_care: true }),
    ]);
    mocks.updateAdminChildOfferings
      .mockRejectedValueOnce(
        new ApiError("confirmation required", 409, {
          code: "enrollment.complete_withdrawal_confirmation_required",
        }),
      )
      .mockResolvedValueOnce({});

    renderAdjustment(
      adjustmentChild({
        offerings: [
          {
            offering_id: "offering-1",
            offering_name: "Ganztag",
            days_of_week_mode: "fixed",
            selected_days: ["mon"],
            manual_selected_days: ["mon"],
            available_days: ["mon"],
          },
        ],
      }),
    );

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Alle Betreuungstage entfernen",
      }),
    );
    fireEvent.change(screen.getByLabelText("Begründung"), {
      target: { value: "Kein Betreuungsbedarf mehr" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByRole("heading", {
        name: "Alle Betreuungstage entfernen?",
      }),
    ).toBeVisible();
    expect(
      screen.getByText(
        "Danach ist für Lina kein Betreuungstag mehr gebucht. Die Abmeldung muss anschließend abgeschlossen werden.",
      ),
    ).toBeVisible();
    expect(mocks.updateAdminChildOfferings).toHaveBeenNthCalledWith(
      1,
      "request-1",
      "child-1",
      expect.objectContaining({
        offerings: [],
        completeWithdrawalConfirmed: false,
      }),
    );

    fireEvent.click(screen.getByRole("button", { name: "Änderung speichern" }));
    await waitFor(() => {
      expect(mocks.updateAdminChildOfferings).toHaveBeenNthCalledWith(
        2,
        "request-1",
        "child-1",
        expect.objectContaining({ completeWithdrawalConfirmed: true }),
      );
    });
    expect(await screen.findByText(/Abmeldung noch abschließen/)).toBeVisible();
  });

  it("hides mutation controls while retaining adjustment history when disabled", async () => {
    vi.mocked(useCareOfferingsEnabled).mockReturnValue(false);
    mocks.listAdminChildOfferingAdjustments.mockResolvedValue([
      {
        id: "adjustment-1",
        request_id: "request-1",
        request_child_id: "child-1",
        student_id: "student-1",
        actor_account_id: "account-1",
        actor_role: "admin",
        actor_name_snapshot: "Ada Admin",
        reason: "Historische Korrektur",
        before: [],
        after: [],
        changed_at: "2026-01-02T00:00:00Z",
      },
    ]);

    render(
      <ChildOfferingAdjustment
        requestId="request-1"
        phaseId="phase-1"
        onSaved={vi.fn()}
        child={{
          id: "child-1",
          first_name: "Lina",
          last_name: "Kind",
          date_of_birth: "2018-01-01",
          status: "approved",
          activation_mode: "scheduled",
        }}
      />,
    );

    expect(
      await screen.findByText(/Historische Korrektur/, { selector: "li" }),
    ).toBeVisible();
    expect(screen.getByText("Änderungshistorie")).toBeVisible();
    expect(
      screen.queryByRole("button", { name: "Bearbeiten" }),
    ).not.toBeInTheDocument();
    expect(mocks.listCareOfferings).not.toHaveBeenCalled();
  });

  it("preserves selected source-phase offerings that are absent from the current catalog", async () => {
    mocks.listCareOfferings.mockResolvedValue([
      {
        id: "current-1",
        phase_id: "phase-1",
        name: "Aktuelles Angebot",
        days_of_week_mode: "parent_choice",
        available_days: ["mon", "tue"],
        includes_holiday_care: false,
        includes_lunch: false,
        is_active: true,
        is_required: false,
        sort_order: 1,
        created_at: "2026-06-18T11:15:00Z",
        updated_at: "2026-06-18T11:15:00Z",
      },
    ]);
    mocks.updateAdminChildOfferings.mockResolvedValue({});

    render(
      <ChildOfferingAdjustment
        requestId="request-1"
        phaseId="phase-1"
        onSaved={vi.fn()}
        child={{
          id: "child-1",
          first_name: "Lina",
          last_name: "Kind",
          date_of_birth: "2018-01-01",
          status: "approved",
          activation_mode: "scheduled",
          offerings: [
            {
              offering_id: "source-1",
              offering_name: "Quellphase",
              days_of_week_mode: "parent_choice",
              selected_days: ["mon"],
              manual_selected_days: ["mon"],
              available_days: ["mon", "tue"],
            },
          ],
        }}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    await screen.findByText("Aktuelles Angebot");
    fireEvent.change(screen.getByLabelText("Begründung"), {
      target: { value: "Rollover unverändert gespeichert" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(mocks.updateAdminChildOfferings).toHaveBeenCalledWith(
        "request-1",
        "child-1",
        expect.objectContaining({
          offerings: [{ offering_id: "source-1", selected_days: ["mon"] }],
        }),
      );
    });
  });

  // Bestandsschutz (#2186): a rule tightened after the fact does NOT revoke a
  // booking the child already holds. Until #2186 the editor dropped it from
  // the selection and the very next save deleted it — silently, in the flow
  // an admin opens to FIX the support case. The offering is now shown greyed
  // with the reason and stays in the payload unless explicitly unchecked.
  it("keeps a booked offering that is unavailable for the child's grade", async () => {
    mocks.listCareOfferings.mockResolvedValue([
      {
        id: "grade-1-only",
        phase_id: "phase-1",
        name: "Randstunde",
        days_of_week_mode: "fixed",
        available_days: ["mon"],
        includes_holiday_care: false,
        includes_lunch: false,
        is_active: true,
        is_required: false,
        sort_order: 1,
        availability_rule: {
          match: "all",
          conditions: [{ source: "grade_level", operator: "in", value: [1] }],
        },
        created_at: "2026-06-18T11:15:00Z",
        updated_at: "2026-06-18T11:15:00Z",
      },
    ]);
    mocks.updateAdminChildOfferings.mockResolvedValue({});

    render(
      <ChildOfferingAdjustment
        requestId="request-1"
        phaseId="phase-1"
        onSaved={vi.fn()}
        child={{
          id: "child-1",
          first_name: "Lina",
          last_name: "Kind",
          date_of_birth: "2018-01-01",
          target_grade_level: 3,
          status: "approved",
          activation_mode: "scheduled",
          offerings: [
            {
              offering_id: "grade-1-only",
              offering_name: "Randstunde",
              days_of_week_mode: "fixed",
              selected_days: ["mon"],
              manual_selected_days: ["mon"],
              available_days: ["mon"],
            },
          ],
        }}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    await waitFor(() => expect(mocks.listCareOfferings).toHaveBeenCalled());
    // Visible with the reason instead of silently missing.
    expect(await screen.findByText("Randstunde")).toBeInTheDocument();
    expect(
      screen.getByText("Nicht wählbar: nur für Klasse 1 (Kind: Klasse 3)"),
    ).toBeInTheDocument();
    expect(screen.getByText("bereits gebucht")).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Begründung"), {
      target: { value: "Unverändert gespeichert" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(mocks.updateAdminChildOfferings).toHaveBeenCalledWith(
        "request-1",
        "child-1",
        expect.objectContaining({
          offerings: [{ offering_id: "grade-1-only" }],
        }),
      );
    });
  });

  it("lets the admin remove a booked but no longer available offering", async () => {
    mocks.listCareOfferings.mockResolvedValue([
      catalogOffering({
        id: "grade-1-only",
        name: "Randstunde",
        availability_rule: {
          match: "all",
          conditions: [{ source: "grade_level", operator: "in", value: [1] }],
        },
      }),
    ]);
    mocks.updateAdminChildOfferings.mockResolvedValue({});
    renderAdjustment(
      adjustmentChild({
        target_grade_level: 3,
        offerings: [
          {
            offering_id: "grade-1-only",
            offering_name: "Randstunde",
            days_of_week_mode: "fixed",
            selected_days: ["mon"],
            manual_selected_days: ["mon"],
            available_days: ["mon"],
          },
        ],
      }),
    );

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    const checkbox = await screen.findByRole("checkbox", {
      name: "Randstunde bleibt gebucht",
    });
    expect(checkbox).toBeChecked();
    fireEvent.click(checkbox);
    fireEvent.change(screen.getByLabelText("Begründung"), {
      target: { value: "Nicht für Klassenstufe 3 verfügbar" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(mocks.updateAdminChildOfferings).toHaveBeenCalledWith(
        "request-1",
        "child-1",
        expect.objectContaining({ offerings: [] }),
      );
    });
  });

  it("shows occupancy per offering so a full one is visible before saving", async () => {
    mocks.listCareOfferings.mockResolvedValue([
      catalogOffering({ id: "offering-1", name: "Ganztag" }),
    ]);
    mocks.fetchCareOfferingBookingStats.mockResolvedValue({
      "offering-1": {
        offering_id: "offering-1",
        capacity: 20,
        booked: 20,
        grade_levels: { "1": 20 },
        unknown_grade_count: 0,
      },
    });
    renderAdjustment();

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    expect(
      await screen.findByText("Ausgebucht (20 von 20 Plätzen belegt)"),
    ).toBeInTheDocument();
    expect(mocks.fetchCareOfferingBookingStats).toHaveBeenCalledWith("phase-1");
  });

  it("stays usable when occupancy cannot be loaded", async () => {
    mocks.listCareOfferings.mockResolvedValue([
      catalogOffering({ id: "offering-1", name: "Ganztag" }),
    ]);
    mocks.fetchCareOfferingBookingStats.mockRejectedValue(
      new Error("stats down"),
    );
    renderAdjustment();

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    expect(
      await screen.findByRole("checkbox", { name: /Ganztag/ }),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Plätzen belegt/)).not.toBeInTheDocument();
  });

  // #2186 review: the kit Checkbox renders an sr-only input behind a
  // pointer-events-none visual, so without a real <label> a mouse click on
  // the box did nothing. Driving the visible control, not the input, is the
  // point of this test.
  it("removes a grandfathered booking when the visible checkbox is clicked", async () => {
    mocks.listCareOfferings.mockResolvedValue([
      catalogOffering({
        id: "grade-1-only",
        name: "Randstunde",
        availability_rule: {
          match: "all",
          conditions: [{ source: "grade_level", operator: "in", value: [1] }],
        },
      }),
    ]);
    mocks.updateAdminChildOfferings.mockResolvedValue({});
    renderAdjustment(
      adjustmentChild({
        target_grade_level: 3,
        offerings: [
          {
            offering_id: "grade-1-only",
            offering_name: "Randstunde",
            days_of_week_mode: "fixed",
            selected_days: ["mon"],
            manual_selected_days: ["mon"],
            available_days: ["mon"],
          },
        ],
      }),
    );

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    const checkbox = await screen.findByRole("checkbox", {
      name: "Randstunde bleibt gebucht",
    });
    // The label must be wired to the input, or a mouse user cannot toggle it.
    const label = checkbox.closest("label");
    expect(label).not.toBeNull();
    expect(label).toHaveAttribute("for", checkbox.id);
    expect(checkbox.id).not.toBe("");

    fireEvent.click(label!);
    expect(checkbox).not.toBeChecked();
  });

  // #2186 review: an automatic booking has no checkbox to tick, so it is
  // absent from `selected`. Rendering the row from `selected` alone showed a
  // blocked automatic booking as empty and disabled — i.e. as NOT booked —
  // although the backend re-derives it from its trigger on every save.
  it("shows a blocked automatic booking as held and not removable", async () => {
    mocks.listCareOfferings.mockResolvedValue([
      catalogOffering({ id: "trigger", name: "Ganztag" }),
      catalogOffering({
        id: "grade-1-only",
        name: "Randstunde",
        days_of_week_mode: "parent_choice",
        available_days: ["mon", "tue"],
        auto_add_trigger_offering_ids: ["trigger"],
        availability_rule: {
          match: "all",
          conditions: [{ source: "grade_level", operator: "in", value: [1] }],
        },
      }),
    ]);
    renderAdjustment(
      adjustmentChild({
        target_grade_level: 3,
        offerings: [
          {
            offering_id: "trigger",
            offering_name: "Ganztag",
            days_of_week_mode: "fixed",
            selected_days: ["mon"],
            manual_selected_days: ["mon"],
            available_days: ["mon"],
          },
          {
            offering_id: "grade-1-only",
            offering_name: "Randstunde",
            days_of_week_mode: "parent_choice",
            selected_days: ["mon"],
            manual_selected_days: [],
            automatic_selected_days: ["mon"],
            available_days: ["mon", "tue"],
          },
        ],
      }),
    );

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    const checkbox = await screen.findByRole("checkbox", {
      name: "Randstunde bleibt gebucht",
    });
    expect(checkbox).toBeChecked();
    // Derived days disappear with their trigger, never by unticking here.
    expect(checkbox).toBeDisabled();
    expect(screen.getByText("automatisch mitgebucht: Mo")).toBeInTheDocument();
  });

  it("never lets an unbooked blocked offering be added", async () => {
    mocks.listCareOfferings.mockResolvedValue([
      catalogOffering({
        id: "grade-1-only",
        name: "Randstunde",
        availability_rule: {
          match: "all",
          conditions: [{ source: "grade_level", operator: "in", value: [1] }],
        },
      }),
    ]);
    renderAdjustment(adjustmentChild({ target_grade_level: 3 }));

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    const checkbox = await screen.findByRole("checkbox", {
      name: "Randstunde bleibt gebucht",
    });
    expect(checkbox).not.toBeChecked();
    expect(checkbox).toBeDisabled();
    expect(screen.queryByText("bereits gebucht")).not.toBeInTheDocument();
  });

  // Same Bestandsschutz rule as above, across the cached-catalog reopen path
  // (which rebuilds the selection from scratch).
  it("keeps a booked unavailable offering after reopening the cached catalog", async () => {
    mocks.listCareOfferings.mockResolvedValue([
      catalogOffering({
        id: "grade-1-only",
        name: "Randstunde",
        availability_rule: {
          match: "all",
          conditions: [{ source: "grade_level", operator: "in", value: [1] }],
        },
      }),
    ]);
    mocks.updateAdminChildOfferings.mockResolvedValue({});
    renderAdjustment(
      adjustmentChild({
        target_grade_level: 3,
        offerings: [
          {
            offering_id: "grade-1-only",
            offering_name: "Randstunde",
            days_of_week_mode: "fixed",
            selected_days: ["mon"],
            manual_selected_days: ["mon"],
            available_days: ["mon"],
          },
        ],
      }),
    );

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    await waitFor(() => expect(mocks.listCareOfferings).toHaveBeenCalledOnce());
    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));
    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    fireEvent.change(screen.getByLabelText("Begründung"), {
      target: { value: "Nicht mehr verfügbar" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(mocks.updateAdminChildOfferings).toHaveBeenCalledWith(
        "request-1",
        "child-1",
        expect.objectContaining({
          offerings: [{ offering_id: "grade-1-only" }],
        }),
      );
    });
    expect(mocks.listCareOfferings).toHaveBeenCalledOnce();
  });

  it("reseeds an active required offering after reopening the cached catalog", async () => {
    mocks.listCareOfferings.mockResolvedValue([
      catalogOffering({ name: "Verpflichtender Ganztag", is_required: true }),
    ]);
    renderAdjustment();

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    const checkbox = await screen.findByRole("checkbox", {
      name: /Verpflichtender Ganztag/,
    });
    expect(checkbox).toBeChecked();
    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));
    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));

    expect(
      screen.getByRole("checkbox", { name: /Verpflichtender Ganztag/ }),
    ).toBeChecked();
    expect(mocks.listCareOfferings).toHaveBeenCalledOnce();
  });

  it("does not newly select an inactive required offering", async () => {
    mocks.listCareOfferings.mockResolvedValue([
      catalogOffering({
        name: "Inaktiver Pflichtplatz",
        is_active: false,
        is_required: true,
      }),
    ]);
    renderAdjustment();

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    const checkbox = await screen.findByRole("checkbox", {
      name: /Inaktiver Pflichtplatz/,
    });

    expect(checkbox).toBeDisabled();
    expect(checkbox).not.toBeChecked();
  });

  it("retains an inactive required offering that was already selected", async () => {
    mocks.listCareOfferings.mockResolvedValue([
      catalogOffering({
        name: "Bestehender Pflichtplatz",
        is_active: false,
        is_required: true,
      }),
    ]);
    renderAdjustment(
      adjustmentChild({
        offerings: [
          {
            offering_id: "offering-1",
            offering_name: "Bestehender Pflichtplatz",
            days_of_week_mode: "fixed",
            selected_days: ["mon"],
            manual_selected_days: ["mon"],
            available_days: ["mon"],
          },
        ],
      }),
    );

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    const checkbox = await screen.findByRole("checkbox", {
      name: /Bestehender Pflichtplatz/,
    });

    expect(checkbox).toBeDisabled();
    expect(checkbox).toBeChecked();
  });

  it("blocks saving when the care-offering catalog failed to load", async () => {
    mocks.listCareOfferings.mockRejectedValue(
      new ApiError("catalog failed", 503, { code: "general.unavailable" }),
    );

    render(
      <ChildOfferingAdjustment
        requestId="request-1"
        phaseId="phase-1"
        onSaved={vi.fn()}
        child={{
          id: "child-1",
          first_name: "Lina",
          last_name: "Kind",
          date_of_birth: "2018-01-01",
          status: "approved",
          activation_mode: "scheduled",
          offerings: [
            {
              offering_id: "offering-1",
              offering_name: "Ganztag",
              days_of_week_mode: "parent_choice",
              selected_days: ["mon"],
              manual_selected_days: ["mon"],
              available_days: ["mon", "tue"],
            },
          ],
        }}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      catalogText("general.unavailable", "die Angebotsliste"),
    );
    const save = screen.getByRole("button", { name: "Speichern" });
    expect(save).toBeDisabled();

    fireEvent.click(save);
    await waitFor(() => {
      expect(mocks.updateAdminChildOfferings).not.toHaveBeenCalled();
    });
  });

  // #2185 ships bookings that only start later in their own array so staff
  // can SEE them. They must not seed the editor, which replaces the CURRENT
  // selection — otherwise an untouched save pulls a future booking into the
  // present. A client that predates the field never sees them at all.
  it("keeps a scheduled future booking out of the correction payload", async () => {
    mocks.listCareOfferings.mockResolvedValue([
      {
        id: "offering-now",
        phase_id: "phase-1",
        name: "Ganztag",
        days_of_week_mode: "fixed",
        available_days: ["mon"],
        includes_holiday_care: false,
        includes_lunch: false,
        is_active: true,
        is_required: false,
        sort_order: 1,
        created_at: "2026-06-18T11:15:00Z",
        updated_at: "2026-06-18T11:15:00Z",
      },
      {
        id: "offering-later",
        phase_id: "phase-1",
        name: "Ganztag mit Mittagessen",
        days_of_week_mode: "fixed",
        available_days: ["mon"],
        includes_holiday_care: false,
        includes_lunch: true,
        is_active: true,
        is_required: false,
        sort_order: 2,
        created_at: "2026-06-18T11:15:00Z",
        updated_at: "2026-06-18T11:15:00Z",
      },
    ]);
    mocks.updateAdminChildOfferings.mockResolvedValue({});

    renderAdjustment(
      adjustmentChild({
        offerings: [
          {
            offering_id: "offering-now",
            offering_name: "Ganztag",
            days_of_week_mode: "fixed",
            available_days: ["mon"],
          },
        ],
        upcoming_offerings: [
          {
            offering_id: "offering-later",
            offering_name: "Ganztag mit Mittagessen",
            days_of_week_mode: "fixed",
            available_days: ["mon"],
            valid_from: "2026-09-01",
            starts_later: true,
          },
        ],
      }),
    );

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    await waitFor(() => expect(mocks.listCareOfferings).toHaveBeenCalled());
    fireEvent.change(screen.getByLabelText("Begründung"), {
      target: { value: "Unverändert gespeichert" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(mocks.updateAdminChildOfferings).toHaveBeenCalledWith(
        "request-1",
        "child-1",
        expect.objectContaining({
          offerings: [
            { offering_id: "offering-now", selected_days: undefined },
          ],
        }),
      );
    });
  });

  // A failed offerings lookup is indistinguishable from "booked nothing" in
  // the payload alone. Correcting on top of that empty state replaces the
  // family's real bookings with nothing, so the entry point has to disappear.
  it("blocks corrections when the bookings could not be loaded", () => {
    renderAdjustment(adjustmentChild({ offerings_unavailable: true }));

    expect(
      screen.queryByRole("button", { name: "Bearbeiten" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText(/konnten nicht geladen werden/),
    ).toBeInTheDocument();
  });

  // Data-loss regression: before the phase's service start EVERY stored row
  // is flagged starts_later, while the write path still treats those rows as
  // the current selection. Seeding from !starts_later emptied the editor, and
  // an untouched save deleted the family's bookings and their chosen days.
  it("seeds from the current selection even when every booking starts later", async () => {
    mocks.listCareOfferings.mockResolvedValue([
      catalogOffering({
        id: "offering-now",
        name: "Ganztag",
        days_of_week_mode: "parent_choice",
        available_days: ["mon", "tue", "wed"],
      }),
    ]);
    mocks.updateAdminChildOfferings.mockResolvedValue({});

    renderAdjustment(
      adjustmentChild({
        offerings: [
          {
            offering_id: "offering-now",
            offering_name: "Ganztag",
            days_of_week_mode: "parent_choice",
            selected_days: ["mon", "wed"],
            manual_selected_days: ["mon", "wed"],
            available_days: ["mon", "tue", "wed"],
            // The phase has not started yet: not in effect, but on file.
            valid_from: "2026-10-06",
            starts_later: true,
          },
        ],
      }),
    );

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    await waitFor(() => expect(mocks.listCareOfferings).toHaveBeenCalled());
    fireEvent.change(screen.getByLabelText("Begründung"), {
      target: { value: "Unverändert gespeichert" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(mocks.updateAdminChildOfferings).toHaveBeenCalledWith(
        "request-1",
        "child-1",
        expect.objectContaining({
          offerings: [
            { offering_id: "offering-now", selected_days: ["mon", "wed"] },
          ],
        }),
      );
    });
  });
});

describe("ChildOfferingAdjustment history", () => {
  const snapshot = (
    id: string,
    name: string,
    selectedDays: string[],
  ): OfferingAdjustmentSnapshot => ({
    offering_id: id,
    offering_name: name,
    days_of_week_mode: "parent_choice",
    available_days: ["mon", "tue", "wed", "thu", "fri"],
    selected_days: selectedDays,
  });
  const randstunde = snapshot("1", "Randstunde", ["tue", "wed", "thu", "fri"]);
  const ganztag1430 = (selectedDays: string[]) =>
    snapshot("2", "Ganztagsbetreuung bis 14.30 Uhr", selectedDays);
  const ganztag16 = (selectedDays: string[]) =>
    snapshot("3", "Ganztagsbetreuung bis 16 Uhr", selectedDays);

  it("renders each change as its own line in the history", async () => {
    mocks.listAdminChildOfferingAdjustments.mockResolvedValue([
      {
        id: "adjustment-1",
        request_id: "request-1",
        request_child_id: "child-1",
        student_id: "student-1",
        actor_account_id: "account-1",
        actor_role: "admin",
        actor_name_snapshot: "Ada Admin",
        reason: "Mittwoch länger",
        before: [
          ganztag1430(["mon", "wed"]),
          { ...randstunde, selected_days: ["tue", "thu"] },
        ],
        after: [ganztag1430(["mon"]), ganztag16(["wed"])],
        changed_at: "2026-09-24T13:59:00Z",
      },
    ]);
    vi.mocked(useCareOfferingsEnabled).mockReturnValue(false);

    renderAdjustment();

    const change = await screen.findByText(
      "vorher Ganztagsbetreuung bis 14.30 Uhr, nachher Ganztagsbetreuung bis 16 Uhr",
    );
    expect(change.closest("li")).toHaveTextContent(
      /^Mi:? ?vorher Ganztagsbetreuung bis 14\.30 Uhr/,
    );
    expect(screen.getByText("Randstunde abgemeldet")).toBeVisible();
    expect(screen.queryByText(/Vorher:/)).not.toBeInTheDocument();
  });
});
