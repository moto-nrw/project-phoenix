// #6593: Der Start eines Jahreslaufs sperrt den Dialog, und eine späte
// Antwort schließt keinen neueren Dialog und navigiert nicht. Echte Seite,
// echter Dialog. Die Sitzung endet im echten Seitenbaum, wenn die Seite
// während des Starts verlassen wird (Zurück im Browser): der Dialog ist
// gesperrt, `data` fällt nie weg, also hängt nur das Verlassen der Seite den
// Dialog aus. Danach öffnet die Nutzerin die Seite und den Dialog neu.
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { CareNoticeRun } from "~/lib/traeger-care-notices-api";
import {
  deferred,
  expectDialogUnlocked,
  expectSaveLocksDialog,
} from "~/test/helpers/form-modal-pending";

const { api, tariffs, mockPush } = vi.hoisted(() => ({
  api: {
    fetchCareNoticeRuns: vi.fn(),
    startCareNoticeRun: vi.fn(),
  },
  tariffs: { fetchCareTariffs: vi.fn() },
  mockPush: vi.fn(),
}));
vi.mock("~/lib/traeger-care-notices-api", async (original) => ({
  ...(await original<typeof import("~/lib/traeger-care-notices-api")>()),
  ...api,
}));
vi.mock("~/lib/traeger-care-billing-api", async (original) => ({
  ...(await original<typeof import("~/lib/traeger-care-billing-api")>()),
  ...tariffs,
}));
vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path.replace(/^\/traeger/, "") || "/",
}));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: vi.fn() }),
  usePathname: () => "/abrechnung/jahresbescheide",
}));

import TraegerJahresbescheidePage from "./page";

const TITLE = "Jahreslauf starten";

const RUN: CareNoticeRun = {
  id: "12",
  year: 2027,
  periodFrom: "2027-01-01",
  periodUntil: "2027-12-01",
  status: "calculated",
  schoolIds: ["1"],
  noticeDate: null,
  issuedOn: null,
  noticeCount: null,
  totalCents: null,
};

function renderPage() {
  return render(
    <ToastProvider>
      <TraegerJahresbescheidePage />
    </ToastProvider>,
  );
}

/** Opens the start dialog of the loaded page. */
async function openDialog(): Promise<HTMLElement> {
  const head = screen.getByRole("button", { name: TITLE });
  await waitFor(() => expect(head).toBeEnabled());
  fireEvent.click(head);
  const dialog = await screen.findByRole("dialog", { name: TITLE });
  const start = await within(dialog).findByRole("button", {
    name: "Vorschau öffnen",
  });
  await waitFor(() => expect(start).toBeEnabled());
  return dialog;
}

/** Renders the page, opens the dialog and starts a run that waits. */
async function startRun() {
  const answer = deferred<CareNoticeRun>();
  api.startCareNoticeRun.mockReturnValueOnce(answer.promise);
  const view = renderPage();
  const dialog = await openDialog();
  fireEvent.click(
    within(dialog).getByRole("button", { name: "Vorschau öffnen" }),
  );
  await waitFor(() => expect(api.startCareNoticeRun).toHaveBeenCalledOnce());
  expectSaveLocksDialog(TITLE);
  return { ...view, answer };
}

describe("TraegerJahresbescheidePage start pending (#6593)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    api.fetchCareNoticeRuns.mockResolvedValue({
      runs: [],
      suggestedYear: 2027,
      suggestedMonth: "2027-01-01",
    });
    tariffs.fetchCareTariffs.mockResolvedValue({
      schools: [{ id: "1", name: "Grundschule Birkenfeld" }],
      modules: [],
      tariffs: [],
      billedUntil: {},
    });
  });

  it("closes the dialog and opens the started run", async () => {
    const { answer } = await startRun();

    await act(async () => answer.resolve(RUN));

    await waitFor(() =>
      expect(screen.queryByRole("dialog", { name: TITLE })).toBeNull(),
    );
    expect(mockPush).toHaveBeenCalledExactlyOnceWith(
      "/abrechnung/jahresbescheide/12",
    );
  });

  it("keeps the dialog open with the error after a refused start", async () => {
    const { answer } = await startRun();

    await act(async () => answer.reject(new Error("abgelehnt")));

    const dialog = expectDialogUnlocked(TITLE);
    expect(await within(dialog).findByRole("alert")).toBeInTheDocument();
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("neither closes the reopened dialog nor navigates on a late success", async () => {
    const { answer, unmount } = await startRun();
    unmount();
    renderPage();
    await openDialog();

    await act(async () => answer.resolve(RUN));

    // Anker: der neue Dialog steht offen und lässt sich bedienen.
    const dialog = expectDialogUnlocked(TITLE);
    expect(
      within(dialog).getByRole("button", { name: "Vorschau öffnen" }),
    ).toBeEnabled();
    expect(mockPush).not.toHaveBeenCalled();
    // The run was created all the same; the page started it only once.
    expect(api.startCareNoticeRun).toHaveBeenCalledOnce();
  });

  it("shows no late error in the reopened dialog", async () => {
    const { answer, unmount } = await startRun();
    unmount();
    renderPage();
    await openDialog();

    await act(async () => answer.reject(new Error("abgelehnt")));

    // Anker: der neue Dialog steht offen und lässt sich bedienen.
    const dialog = expectDialogUnlocked(TITLE);
    expect(
      within(dialog).getByRole("button", { name: "Vorschau öffnen" }),
    ).toBeEnabled();
    expect(within(dialog).queryByRole("alert")).toBeNull();
    expect(mockPush).not.toHaveBeenCalled();
  });
});
