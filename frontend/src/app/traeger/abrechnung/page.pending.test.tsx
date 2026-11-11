// #6593: Der Start eines Abrechnungslaufs (Ferien) sperrt den Dialog, und eine späte
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
import type { BillingRun } from "~/lib/traeger-billing-api";
import {
  deferred,
  expectDialogUnlocked,
  expectSaveLocksDialog,
} from "~/test/helpers/form-modal-pending";

const { api, mockPush } = vi.hoisted(() => ({
  api: {
    fetchBillingRuns: vi.fn(),
    fetchBillingPeriods: vi.fn(),
    startBillingRun: vi.fn(),
  },
  mockPush: vi.fn(),
}));
vi.mock("~/lib/traeger-billing-api", () => api);
vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path.replace(/^\/traeger/, "") || "/",
}));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: vi.fn() }),
  usePathname: () => "/abrechnung",
  useSearchParams: () => new URLSearchParams(),
}));

import TraegerAbrechnungPage from "./page";

const TITLE = "Lauf starten";

const RUN: BillingRun = {
  id: "12",
  holidayPeriodId: "5",
  periodName: "Winterferien 2027",
  serviceFrom: "2027-01-04",
  serviceUntil: "2027-01-15",
  status: "calculated",
  createdAt: "2026-09-09T10:00:00Z",
  releasedAt: null,
  issuedOn: null,
  documentCount: null,
  totalCents: null,
};

function renderPage() {
  return render(
    <ToastProvider>
      <TraegerAbrechnungPage />
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
    name: TITLE,
  });
  await waitFor(() => expect(start).toBeEnabled());
  return dialog;
}

/** Renders the page, opens the dialog and starts a run that waits. */
async function startRun() {
  const answer = deferred<BillingRun>();
  api.startBillingRun.mockReturnValueOnce(answer.promise);
  const view = renderPage();
  const dialog = await openDialog();
  fireEvent.click(within(dialog).getByRole("button", { name: TITLE }));
  await waitFor(() => expect(api.startBillingRun).toHaveBeenCalledOnce());
  expectSaveLocksDialog(TITLE);
  return { ...view, answer };
}

describe("TraegerAbrechnungPage start pending (#6593)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    api.fetchBillingRuns.mockResolvedValue([]);
    api.fetchBillingPeriods.mockResolvedValue([
      {
        id: "5",
        name: "Winterferien 2027",
        startsOn: "2027-01-04",
        endsOn: "2027-01-15",
      },
    ]);
  });

  it("closes the dialog and opens the started run", async () => {
    const { answer } = await startRun();

    await act(async () => answer.resolve(RUN));

    await waitFor(() =>
      expect(screen.queryByRole("dialog", { name: TITLE })).toBeNull(),
    );
    expect(mockPush).toHaveBeenCalledExactlyOnceWith("/abrechnung/12");
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
    expect(within(dialog).getByRole("button", { name: TITLE })).toBeEnabled();
    expect(mockPush).not.toHaveBeenCalled();
    // The run was created all the same; the page started it only once.
    expect(api.startBillingRun).toHaveBeenCalledOnce();
  });

  it("shows no late error in the reopened dialog", async () => {
    const { answer, unmount } = await startRun();
    unmount();
    renderPage();
    await openDialog();

    await act(async () => answer.reject(new Error("abgelehnt")));

    // Anker: der neue Dialog steht offen und lässt sich bedienen.
    const dialog = expectDialogUnlocked(TITLE);
    expect(within(dialog).getByRole("button", { name: TITLE })).toBeEnabled();
    expect(within(dialog).queryByRole("alert")).toBeNull();
    expect(mockPush).not.toHaveBeenCalled();
  });
});
