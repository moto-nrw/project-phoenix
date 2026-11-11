// #6593: Solange eine Anmeldephase gespeichert oder eine Anschlussphase
// erstellt wird, lässt sich das Panel nicht schließen; eine späte Antwort
// schließt kein Panel, das inzwischen eine andere Phase zeigt.
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
const { createRollover, fetchRolloverPreview } = vi.hoisted(() => ({
  createRollover: vi.fn(),
  fetchRolloverPreview: vi.fn(),
}));

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
  createPhase: mocks.createPhase,
  listPhaseExpiryWarnings: mocks.listPhaseExpiryWarnings,
  listPhases: mocks.listPhases,
  updatePhase: mocks.updatePhase,
  createRollover,
  fetchRolloverPreview,
}));
vi.mock("~/lib/enrollment-form-schema-api", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  listSchemas: mocks.listSchemas,
}));
vi.mock("~/lib/calendar-period-api", () => ({
  calendarPeriodService: { list: mocks.listCalendarPeriods },
}));

import { ApiError } from "~/lib/api-error";
import type { Phase, RolloverResult } from "~/lib/enrollment-phase-api";
import { catalogText } from "~/test/error-catalog-text";
import {
  inputByName,
  phase,
  resetEditorMocks,
  schema,
} from "~/test/enrollment-editors";
import { PhasesEditor } from "./phases-editor";

const EDIT_TITLE = "Anmeldephase bearbeiten";
const ROLLOVER_TITLE = "Anmeldephase übertragen";
const FIRST = phase();
const SECOND = phase({ id: "11", name: "Ferien 2027" });

function rolloverResult(name: string): RolloverResult {
  return {
    phase: { ...FIRST, id: "99", name },
    summary: {
      source_child_count: 2,
      rolled_count: 2,
      review_count: 0,
      review_by_reason: {},
      request_count: 0,
      enqueued_emails: 0,
      skipped_empty_email: 0,
    },
  };
}

async function startEditSave() {
  const answer = deferred<Phase>();
  mocks.updatePhase.mockReturnValue(answer.promise);
  const view = render(<PhasesEditor />);
  fireEvent.click(
    await screen.findByRole("button", {
      name: "Aktionen für Schuljahr 2026/27",
    }),
  );
  fireEvent.click(await screen.findByRole("menuitem", { name: "Bearbeiten" }));
  fireEvent.change(inputByName("name"), {
    target: { value: "Schuljahr 2026/27 neu" },
  });
  fireEvent.click(await screen.findByRole("button", { name: "Speichern" }));
  await screen.findByRole("button", { name: "Speichert…" });
  return { ...view, answer };
}

describe("PhasesEditor: Speichern sperrt das Panel (#6593)", () => {
  beforeEach(() => {
    resetEditorMocks(mocks);
    mocks.listPhases.mockResolvedValue([FIRST, SECOND]);
    mocks.listSchemas.mockResolvedValue([schema()]);
    fetchRolloverPreview.mockReset();
    fetchRolloverPreview.mockResolvedValue({
      carry_candidate_count: 2,
      carried_count: 2,
      review_count: 0,
      review_by_reason: {},
      excluded_count: 0,
      excluded_by_status: {},
      request_count: 0,
    });
    createRollover.mockReset();
  });

  it("locks the panel while saving a phase and unlocks it with the error after a failure", async () => {
    const { answer } = await startEditSave();

    expectSlideOverSaveLocked(EDIT_TITLE, ["Abbrechen", "Speichert…"]);

    await act(async () => {
      answer.reject(new ApiError("kaputt", 500, { code: "general.server" }));
    });

    expect(
      await screen.findByText(
        catalogText("general.server", "die Anmeldephase"),
      ),
    ).toBeVisible();
    expectSlideOverUnlocked(EDIT_TITLE);
    expect(inputByName("name")).toHaveValue("Schuljahr 2026/27 neu");
  });

  it("closes the panel after the phase is saved", async () => {
    const { answer } = await startEditSave();

    await act(async () => {
      answer.resolve(phase({ name: "Schuljahr 2026/27 neu" }));
    });

    expect(mocks.toast.success).toHaveBeenCalledWith(
      "Die Anmeldephase „Schuljahr 2026/27 neu“ ist gespeichert.",
    );
    // Anker: die Liste steht wieder ohne Panel.
    expect(
      await screen.findByRole("button", {
        name: "Aktionen für Schuljahr 2026/27",
      }),
    ).toBeEnabled();
    expect(screen.queryByRole("heading", { name: EDIT_TITLE })).toBeNull();
  });

  it("locks the panel while the rollover is created", async () => {
    const answer = deferred<RolloverResult>();
    createRollover.mockReturnValue(answer.promise);
    mocks.searchParams.set("rollover", FIRST.id);
    render(<PhasesEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Anschlussphase erstellen" }),
    );
    await screen.findByRole("button", { name: "Wird erstellt…" });

    expectSlideOverSaveLocked(ROLLOVER_TITLE, ["Abbrechen", "Wird erstellt…"]);

    await act(async () => {
      answer.reject(new ApiError("kaputt", 500, { code: "general.server" }));
    });

    expect(
      await screen.findByText(
        catalogText("general.server", "die Anschlussphase"),
      ),
    ).toBeVisible();
    expectSlideOverUnlocked(ROLLOVER_TITLE);
  });

  it("keeps the rollover of another phase open when a late answer arrives", async () => {
    const answer = deferred<RolloverResult>();
    createRollover.mockReturnValue(answer.promise);
    mocks.searchParams.set("rollover", FIRST.id);
    const { rerender } = render(<PhasesEditor />);
    fireEvent.click(
      await screen.findByRole("button", { name: "Anschlussphase erstellen" }),
    );
    await screen.findByRole("button", { name: "Wird erstellt…" });

    // A link (?rollover=…) switches the open panel to another phase.
    mocks.searchParams.set("rollover", SECOND.id);
    rerender(<PhasesEditor />);
    expect(
      await screen.findByRole("heading", {
        name: `Anschlussphase für „${SECOND.name}“ erstellen`,
      }),
    ).toBeInTheDocument();

    await act(async () => {
      answer.resolve(rolloverResult("Schuljahr 2027/28"));
    });

    // The rollover exists: it is reported and the list refreshed …
    expect(mocks.toast.success).toHaveBeenCalledWith(
      "Die Anschlussphase „Schuljahr 2027/28“ ist angelegt (2 übernommen).",
    );
    // … but the panel of the other phase stays open.
    expectSlideOverUnlocked(ROLLOVER_TITLE);
    expect(
      screen.getByRole("heading", {
        name: `Anschlussphase für „${SECOND.name}“ erstellen`,
      }),
    ).toBeInTheDocument();
  });
});
