import "@testing-library/jest-dom/vitest";
import {
  cleanup,
  fireEvent,
  render as rtlRender,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import type { ReactElement } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  replace: vi.fn(),
  search: { value: "" },
  fetchSlotListOptions: vi.fn(),
  fetchSlotListPreview: vi.fn(),
  exportSlotList: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { token: "token" } },
    status: "authenticated",
  }),
}));

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  usePathname: () => "/lists",
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), prefetch: vi.fn() }),
  useSearchParams: () => new URLSearchParams(mocks.search.value),
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ replace: mocks.replace, push: vi.fn() }),
}));

vi.mock("~/lib/hooks/use-settings-schema", () => ({
  useSettingsSchema: () => ({ data: undefined }),
}));

vi.mock("~/lib/slot-lists-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/slot-lists-api")>()),
  fetchSlotListOptions: mocks.fetchSlotListOptions,
  fetchSlotListPreview: mocks.fetchSlotListPreview,
  exportSlotList: mocks.exportSlotList,
}));

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import {
  SlotListExportError,
  SlotListExportSupersededError,
  type SlotListOptionsResult,
  type SlotListResult,
} from "~/lib/slot-lists-api";
import { catalogText } from "~/test/error-catalog-text";

import SlotListsPage from "./page";

function render(ui: ReactElement) {
  return rtlRender(ui, { wrapper: ToastProvider });
}

const options: SlotListOptionsResult = {
  date: "2026-09-09",
  slots: [
    {
      instance_id: "11",
      title: "Fußball",
      time_range: "14:00-15:00",
      status: "planned",
      list_kind: "activity",
    },
  ],
  pickup_cohorts: [],
  list_kinds: [],
};

const preview: SlotListResult = {
  date: "2026-09-09",
  target: "slots",
  list_label: "Angebote",
  source: "planned",
  provenance: "Plan",
  slots: options.slots,
  groups: [],
  classes: [],
  counters: { planned: 1, present: 0, missing: 0, excused: 0, unplanned: 0 },
  rows: [
    {
      student_id: "5",
      name: "Mia Muster",
      school_class: "2a",
      group_name: "Bären",
      instance_id: "11",
      slot: "Fußball",
      planned: true,
      present: false,
      unplanned: false,
      excused: false,
      status_label: "Geplant",
    },
  ],
  signature: "sig-1",
};

const LIST_CHANGED_NOTICE =
  "Die Liste hat sich seit dem Laden geändert. Bitte prüfen Sie die Liste und exportieren Sie erneut.";
const SELECTION_CHANGED_NOTICE =
  "Die Auswahl hat sich geändert. Bitte prüfen Sie die Liste und exportieren Sie erneut.";
const SLOTS_CHANGED_NOTICE =
  "Die Angebote für diesen Tag haben sich geändert. Bitte prüfen Sie die Liste und exportieren Sie erneut.";
const PRINT_BLOCKED_NOTICE =
  "Das Druckfenster konnte nicht geöffnet werden. Bitte laden Sie die Liste als PDF herunter.";

async function openExportMenuAndPick(label: string) {
  fireEvent.click(screen.getByRole("button", { name: "Weitere Aktionen" }));
  fireEvent.click(await screen.findByRole("menuitem", { name: label }));
}

describe("SlotListsPage error paths (#2516)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.search.value = "";
    mocks.fetchSlotListOptions.mockResolvedValue(options);
    mocks.fetchSlotListPreview.mockResolvedValue(preview);
    mocks.exportSlotList.mockResolvedValue(undefined);
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it("shows a failed options load in place of the preview and retries it", async () => {
    mocks.fetchSlotListOptions.mockRejectedValueOnce(
      new ApiError("Failed to fetch", 503, { code: "general.unavailable" }),
    );

    render(<SlotListsPage />);

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Liste der Angebote"),
      ),
    ).toBeInTheDocument();
    // No empty list: the failure must not read as "no children today".
    expect(
      screen.queryByText("Keine Kinder in dieser Liste"),
    ).not.toBeInTheDocument();
    // Nor "no offers planned" for a day whose offers never loaded.
    expect(
      screen.queryByText("Für dieses Datum sind keine Angebote geplant."),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText("Die Angebote konnten nicht geladen werden."),
    ).toBeInTheDocument();
    expect(mocks.fetchSlotListPreview).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    expect(await screen.findByText("Mia Muster")).toBeInTheDocument();
    expect(mocks.fetchSlotListOptions).toHaveBeenCalledTimes(2);
    expect(
      screen.queryByText(
        catalogText("general.unavailable", "die Liste der Angebote"),
      ),
    ).not.toBeInTheDocument();
  });

  it("shows a failed preview with request ID and retry instead of an empty list", async () => {
    mocks.fetchSlotListPreview.mockRejectedValueOnce(
      new ApiError("boom", 500, {
        code: "general.server",
        instance: "req-preview",
      }),
    );

    render(<SlotListsPage />);

    expect(
      await screen.findByText(catalogText("general.server", "die Vorschau")),
    ).toBeInTheDocument();
    expect(screen.queryByText("boom")).not.toBeInTheDocument();
    expect(
      screen.queryByText("Keine Kinder in dieser Liste"),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Vorgangskennung kopieren" }),
    ).toHaveTextContent("req-preview");

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    expect(await screen.findByText("Mia Muster")).toBeInTheDocument();
    expect(mocks.fetchSlotListPreview).toHaveBeenCalledTimes(2);
  });

  it("reports a failed export as a toast whose retry exports again", async () => {
    mocks.exportSlotList.mockRejectedValueOnce(
      new SlotListExportError(
        new ApiError("render failed", 500, {
          code: "general.server",
          instance: "req-export",
        }),
      ),
    );

    render(<SlotListsPage />);
    await screen.findByText("Mia Muster");

    await openExportMenuAndPick("PDF herunterladen");

    const toast = await screen.findByText(
      catalogText("general.server", "die Tagesliste"),
    );
    expect(screen.queryByText("render failed")).not.toBeInTheDocument();
    expect(mocks.exportSlotList).toHaveBeenCalledTimes(1);

    const toastRoot = toast.closest("[role='alert'], [role='status']");
    const scope = toastRoot ? within(toastRoot as HTMLElement) : screen;
    fireEvent.click(scope.getByRole("button", { name: "Wiederholen" }));

    await waitFor(() => expect(mocks.exportSlotList).toHaveBeenCalledTimes(2));
    expect(mocks.exportSlotList).toHaveBeenLastCalledWith(
      expect.objectContaining({ date: "2026-09-09" }),
      "pdf",
      "download",
      null,
      "sig-1",
      expect.any(Function),
    );
  });

  it("asks for a re-check after the backend's 409 drift refusal and reloads the preview", async () => {
    mocks.exportSlotList.mockRejectedValueOnce(
      new SlotListExportError(
        new ApiError("Die Liste hat sich seit der Vorschau geändert.", 409),
      ),
    );

    render(<SlotListsPage />);
    await screen.findByText("Mia Muster");
    const previewCallsBefore = mocks.fetchSlotListPreview.mock.calls.length;

    await openExportMenuAndPick("Excel herunterladen");

    expect(await screen.findByText(LIST_CHANGED_NOTICE)).toBeInTheDocument();
    expect(
      screen.queryByText("Die Liste hat sich seit der Vorschau geändert."),
    ).not.toBeInTheDocument();
    await waitFor(() =>
      expect(mocks.fetchSlotListPreview.mock.calls.length).toBeGreaterThan(
        previewCallsBefore,
      ),
    );
  });

  it("reports a failed check before the export as a toast with retry", async () => {
    render(<SlotListsPage />);
    await screen.findByText("Mia Muster");
    mocks.fetchSlotListOptions.mockRejectedValueOnce(
      new ApiError("Failed to fetch", 503, { code: "general.unavailable" }),
    );

    await openExportMenuAndPick("PDF herunterladen");

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Tagesliste"),
      ),
    ).toBeInTheDocument();
    expect(mocks.exportSlotList).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    await waitFor(() => expect(mocks.exportSlotList).toHaveBeenCalledTimes(1));
  });

  it("stops the export and says so when the offers of the day changed", async () => {
    render(<SlotListsPage />);
    await screen.findByText("Mia Muster");
    mocks.fetchSlotListOptions.mockResolvedValueOnce({
      ...options,
      slots: [{ ...options.slots[0]!, title: "Basketball" }],
    });

    await openExportMenuAndPick("PDF herunterladen");

    expect(await screen.findByText(SLOTS_CHANGED_NOTICE)).toBeInTheDocument();
    expect(mocks.exportSlotList).not.toHaveBeenCalled();
  });

  it("shows the changed list and asks for a re-check before exporting it", async () => {
    render(<SlotListsPage />);
    await screen.findByText("Mia Muster");
    mocks.fetchSlotListPreview.mockResolvedValue({
      ...preview,
      rows: [{ ...preview.rows[0]!, name: "Ben Beispiel" }],
      signature: "sig-2",
    });

    await openExportMenuAndPick("PDF herunterladen");

    expect(await screen.findByText(LIST_CHANGED_NOTICE)).toBeInTheDocument();
    expect(await screen.findByText("Ben Beispiel")).toBeInTheDocument();
    expect(mocks.exportSlotList).not.toHaveBeenCalled();
  });

  it("explains an export dropped because the selection moved on", async () => {
    mocks.exportSlotList.mockRejectedValueOnce(
      new SlotListExportSupersededError(),
    );

    render(<SlotListsPage />);
    await screen.findByText("Mia Muster");

    await openExportMenuAndPick("PDF herunterladen");

    expect(
      await screen.findByText(SELECTION_CHANGED_NOTICE),
    ).toBeInTheDocument();
  });

  it("explains a blocked print window without starting the export", async () => {
    vi.spyOn(globalThis, "open").mockReturnValue(null);

    render(<SlotListsPage />);
    await screen.findByText("Mia Muster");
    const optionsCallsBefore = mocks.fetchSlotListOptions.mock.calls.length;

    await openExportMenuAndPick("Drucken");

    expect(await screen.findByText(PRINT_BLOCKED_NOTICE)).toBeInTheDocument();
    expect(mocks.fetchSlotListOptions).toHaveBeenCalledTimes(
      optionsCallsBefore,
    );
    expect(mocks.exportSlotList).not.toHaveBeenCalled();
  });

  it("rejects a corrupted filter link and offers to reset the filters", async () => {
    mocks.search.value = "datum=2026-09-09&gruppen=-1";

    render(<SlotListsPage />);

    expect(
      await screen.findByText(
        "Dieser Link enthält einen ungültigen Filter. Die Liste wird deshalb nicht angezeigt.",
      ),
    ).toBeInTheDocument();
    expect(mocks.fetchSlotListPreview).not.toHaveBeenCalled();
    expect(
      screen.queryByText("Keine Kinder in dieser Liste"),
    ).not.toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Filter zurücksetzen" }),
    );

    expect(mocks.replace).toHaveBeenCalledWith(
      expect.not.stringContaining("gruppen"),
    );
  });
});
