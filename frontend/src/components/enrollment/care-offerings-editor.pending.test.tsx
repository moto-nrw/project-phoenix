// #6593: Solange ein Betreuungsangebot gespeichert oder dupliziert wird,
// lässt sich das Panel nicht schließen; eine späte Antwort schließt nichts
// und zeigt keinen Fehler mehr.
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  deferred,
  expectSlideOverSaveLocked,
  expectSlideOverUnlocked,
} from "~/test/helpers/form-modal-pending";

vi.mock("vaul", () => import("~/test/mocks/vaul"));

// useTenantMutate scopes its keys to the signed-in account (#5006).
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "1" }, expires: "2099-01-01" },
    status: "authenticated",
  }),
}));

const mocks = await vi.hoisted(async () =>
  (await import("~/test/enrollment-editors")).createEditorMocks(),
);

vi.mock("~/components/ui/date-picker", async () =>
  (await import("~/test/mocks/date-picker")).datePickerModuleMock(),
);
vi.mock("~/components/ui/date-time-picker", async (importOriginal) => {
  const { dateTimePickerMock } = await import("~/test/mocks/date-time-picker");
  return { ...(await importOriginal<object>()), ...dateTimePickerMock() };
});
vi.mock("next/navigation", () => ({
  usePathname: () => "/demo/care-offerings",
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
vi.mock("~/lib/enrollment-phase-api", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  listPhases: mocks.listPhases,
}));
vi.mock("~/lib/care-offering-api", () => ({
  cloneCareOffering: mocks.cloneCareOffering,
  createCareOffering: mocks.createCareOffering,
  deleteCareOffering: mocks.deleteCareOffering,
  listCareOfferings: mocks.listCareOfferings,
  updateCareOffering: mocks.updateCareOffering,
  SELECTION_RULE_LABELS: {
    optional: "Optional (frei wählbar)",
    exactly_one: "Genau eines (Pflicht)",
    at_least_one: "Mindestens eines (Pflicht)",
    at_most_one: "Höchstens eines",
  },
}));
vi.mock("~/lib/care-offering-booking-stats", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  fetchCareOfferingBookingStats: mocks.fetchCareOfferingBookingStats,
}));
vi.mock("~/lib/timetable-api", () => ({
  timetableService: { getTemplates: mocks.getTemplates },
}));
vi.mock("~/lib/calendar-period-api", () => ({
  calendarPeriodService: { list: mocks.listCalendarPeriods },
}));

import { ApiError } from "~/lib/api-error";
import type { CareOffering } from "~/lib/care-offering-api";
import { catalogText } from "~/test/error-catalog-text";
import {
  inputByName,
  offering,
  phase,
  resetEditorMocks,
} from "~/test/enrollment-editors";
import { CareOfferingsEditor } from "./care-offerings-editor";

const EDIT_TITLE = "Betreuungsangebot bearbeiten";
const CLONE_TITLE = "Betreuungsangebot duplizieren";

async function openRowAction(action: "Bearbeiten" | "Duplizieren") {
  fireEvent.click(
    await screen.findByRole("button", { name: "Aktionen für Regelbetreuung" }),
  );
  fireEvent.click(await screen.findByRole("menuitem", { name: action }));
}

async function startEditSave() {
  const answer = deferred<CareOffering>();
  mocks.updateCareOffering.mockReturnValue(answer.promise);
  const view = render(<CareOfferingsEditor />);
  await openRowAction("Bearbeiten");
  fireEvent.change(inputByName("name"), {
    target: { value: "Regelbetreuung neu" },
  });
  fireEvent.click(await screen.findByRole("button", { name: "Speichern" }));
  await screen.findByRole("button", { name: "Speichert…" });
  return { ...view, answer };
}

describe("CareOfferingsEditor: Speichern sperrt das Panel (#6593)", () => {
  beforeEach(() => {
    resetEditorMocks(mocks);
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listCareOfferings.mockResolvedValue([offering()]);
  });

  it("locks the panel while saving and unlocks it with the error after a failure", async () => {
    const { answer } = await startEditSave();

    expectSlideOverSaveLocked(EDIT_TITLE, ["Abbrechen", "Speichert…"]);

    await act(async () => {
      answer.reject(new ApiError("kaputt", 500, { code: "general.server" }));
    });

    expect(
      await screen.findByText(
        catalogText("general.server", "das Betreuungsangebot"),
      ),
    ).toBeVisible();
    expectSlideOverUnlocked(EDIT_TITLE);
    expect(inputByName("name")).toHaveValue("Regelbetreuung neu");
  });

  it("closes the panel after the offering is saved", async () => {
    const { answer } = await startEditSave();

    await act(async () => {
      answer.resolve(offering({ name: "Regelbetreuung neu" }));
    });

    expect(mocks.toast.success).toHaveBeenCalledWith(
      "Das Betreuungsangebot „Regelbetreuung neu“ ist gespeichert.",
    );
    // Anker: die Liste steht wieder ohne Panel.
    expect(
      await screen.findByRole("button", {
        name: "Aktionen für Regelbetreuung",
      }),
    ).toBeEnabled();
    expect(screen.queryByRole("heading", { name: EDIT_TITLE })).toBeNull();
  });

  it("still reports a write that lands after unmount", async () => {
    const { answer, unmount } = await startEditSave();

    unmount();
    await act(async () => {
      answer.resolve(offering({ name: "Regelbetreuung neu" }));
    });

    expect(mocks.toast.success).toHaveBeenCalledOnce();
  });

  it("locks the panel while cloning and unlocks it after a failure", async () => {
    const answer = deferred<CareOffering>();
    mocks.cloneCareOffering.mockReturnValue(answer.promise);
    render(<CareOfferingsEditor />);
    await openRowAction("Duplizieren");
    fireEvent.click(await screen.findByRole("button", { name: "Duplizieren" }));
    await screen.findByRole("button", { name: "Dupliziert…" });

    expectSlideOverSaveLocked(CLONE_TITLE, ["Abbrechen", "Dupliziert…"]);

    await act(async () => {
      answer.reject(new ApiError("kaputt", 500, { code: "general.server" }));
    });

    expect(
      await screen.findByText(
        catalogText("general.server", "das Betreuungsangebot"),
      ),
    ).toBeVisible();
    expectSlideOverUnlocked(CLONE_TITLE);
  });
});
