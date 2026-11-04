import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type {
  BillingDocument,
  BillingFile,
  BillingRunDetail,
} from "~/lib/traeger-billing-api";

const { api } = vi.hoisted(() => ({
  api: {
    downloadBillingLetter: vi.fn(),
    downloadBillingRunOverview: vi.fn(),
  },
}));
const { downloadBlob } = vi.hoisted(() => ({ downloadBlob: vi.fn() }));
vi.mock("~/lib/traeger-billing-api", () => api);
vi.mock("~/lib/file-download", () => ({ downloadBlob }));
vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path,
}));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => "/abrechnung/9",
}));

import { BillingRunOverviewCard } from "./billing-run-overview-card";

function invoice(
  id: string,
  number: string,
  firstName: string,
  lastName: string,
  totalCents: number,
): BillingDocument {
  return {
    id,
    kind: "invoice",
    referenceNumber: "",
    reason: "",
    history: [],
    replacedCredits: [],
    number,
    issuedOn: "2026-09-09",
    registrationId: `r-${id}`,
    recipientFirstName: firstName,
    recipientLastName: lastName,
    recipientEmail: "",
    debtorNumber: "",
    serviceFrom: "2027-01-04",
    serviceUntil: "2027-01-15",
    totalCents,
    children: [
      {
        childId: `c-${id}`,
        firstName: "Lia",
        lastName,
        totalCents,
        corrections: [],
        carriedCorrections: [],
        positions: [],
      },
    ],
  };
}

const RUN: BillingRunDetail = {
  id: "9",
  holidayPeriodId: "5",
  periodName: "Winterferien 2027",
  serviceFrom: "2027-01-04",
  serviceUntil: "2027-01-15",
  status: "released",
  createdAt: "2026-09-01T08:00:00Z",
  releasedAt: "2026-09-09T08:00:00Z",
  issuedOn: "2026-09-09",
  documentCount: 2,
  totalCents: 20000,
  previewCount: 2,
  previewCents: 20000,
  unbilled: 0,
  missingDebtorNumbers: 0,
  paymentTextMissingDebtorNumbers: 0,
  numberPattern: "FE-{NNNN}",
  numberRangeSet: true,
  letters: {
    paymentDueOn: "2026-09-23",
    notes: "",
    emergencyContacts: [],
    createdAt: "2026-09-09T10:00:00Z",
  },
  documents: [
    invoice("51", "FE-0001", "Eva", "Muster", 11250),
    invoice("52", "FE-0002", "Jonas", "Beispiel", 8750),
  ],
};

function letter(number: string): BillingFile {
  return {
    blob: new Blob([`%PDF ${number}`], { type: "application/pdf" }),
    filename: `bestaetigung-${number.toLowerCase()}.pdf`,
  };
}

const LETTERS: Record<string, BillingFile> = {
  "51": letter("FE-0001"),
  "52": letter("FE-0002"),
};

let urls: Map<string, Blob>;
let revoked: string[];

beforeEach(() => {
  vi.clearAllMocks();
  urls = new Map();
  revoked = [];
  let next = 0;
  vi.spyOn(URL, "createObjectURL").mockImplementation((blob) => {
    next += 1;
    const url = `blob:letter-${next}`;
    urls.set(url, blob as Blob);
    return url;
  });
  vi.spyOn(URL, "revokeObjectURL").mockImplementation((url) => {
    revoked.push(url);
  });
  api.downloadBillingLetter.mockImplementation(
    (_runId: string, documentId: string) =>
      Promise.resolve(LETTERS[documentId]),
  );
});

afterEach(() => {
  vi.restoreAllMocks();
});

function renderCard(run: BillingRunDetail = RUN) {
  return render(
    <ToastProvider>
      <BillingRunOverviewCard run={run} onDownloadLetter={vi.fn()} />
    </ToastProvider>,
  );
}

/** The desktop table row of a family; the phone list repeats it. */
function row(name: string): HTMLElement {
  const cell = screen
    .getAllByText(name)
    .find((element) => element.closest("tr"));
  return cell!.closest("tr")!;
}

/** The blob the preview frame of the letter currently shows. */
async function shownLetter(title: string): Promise<Blob | undefined> {
  const frame = await screen.findByTitle(title);
  const src = frame.getAttribute("src") ?? "";
  return urls.get(src.replace(/#.*$/, ""));
}

describe("BillingRunOverviewCard letter preview (#5692)", () => {
  it("shows the letter of each chosen invoice under its own number", async () => {
    renderCard();

    expect(screen.getByText("Noch kein Schreiben gewählt")).toBeInTheDocument();
    // No letter is read before a row is chosen: every read is audited.
    expect(api.downloadBillingLetter).not.toHaveBeenCalled();

    fireEvent.click(row("Muster, Eva"));
    const first = screen.getByRole("region", {
      name: "Schreiben zur Rechnung FE-0001",
    });
    expect(within(first).getByText("Eva Muster")).toBeInTheDocument();
    expect(await shownLetter("Rechnung FE-0001")).toBe(LETTERS["51"]!.blob);
    expect(row("Muster, Eva")).toHaveAttribute("aria-current", "true");
    expect(api.downloadBillingLetter).toHaveBeenLastCalledWith("9", "51");

    fireEvent.click(row("Beispiel, Jonas"));
    expect(
      screen.getByRole("region", { name: "Schreiben zur Rechnung FE-0002" }),
    ).toBeInTheDocument();
    expect(await shownLetter("Rechnung FE-0002")).toBe(LETTERS["52"]!.blob);
    expect(row("Beispiel, Jonas")).toHaveAttribute("aria-current", "true");
    expect(row("Muster, Eva")).not.toHaveAttribute("aria-current");
    expect(api.downloadBillingLetter).toHaveBeenLastCalledWith("9", "52");
  });

  it("never shows a late letter of the invoice chosen before", async () => {
    let finishFirst: (file: BillingFile) => void = () => {};
    api.downloadBillingLetter.mockImplementation(
      (_runId: string, documentId: string) =>
        documentId === "51"
          ? new Promise<BillingFile>((resolve) => {
              finishFirst = resolve;
            })
          : Promise.resolve(LETTERS[documentId]),
    );
    renderCard();

    fireEvent.click(row("Muster, Eva"));
    fireEvent.click(row("Beispiel, Jonas"));
    expect(await shownLetter("Rechnung FE-0002")).toBe(LETTERS["52"]!.blob);

    await act(async () => {
      finishFirst(LETTERS["51"]!);
      await Promise.resolve();
    });
    expect(await shownLetter("Rechnung FE-0002")).toBe(LETTERS["52"]!.blob);
    expect(screen.queryByTitle("Rechnung FE-0001")).not.toBeInTheDocument();
  });

  it("downloads and opens exactly the shown letter", async () => {
    renderCard();
    fireEvent.click(row("Beispiel, Jonas"));
    await shownLetter("Rechnung FE-0002");

    fireEvent.click(
      screen.getByRole("button", { name: "Rechnung FE-0002 herunterladen" }),
    );
    expect(downloadBlob).toHaveBeenCalledWith(
      LETTERS["52"]!.blob,
      "bestaetigung-fe-0002.pdf",
    );
    // The download takes the file already loaded, without a second read.
    expect(api.downloadBillingLetter).toHaveBeenCalledTimes(1);

    const tab = screen.getByRole("link", {
      name: "Rechnung FE-0002 in neuem Tab öffnen",
    });
    expect(tab).toHaveAttribute("target", "_blank");
    expect(urls.get(tab.getAttribute("href") ?? "")).toBe(LETTERS["52"]!.blob);
  });

  it("releases the object URL of a letter once another one shows", async () => {
    const { unmount } = renderCard();
    fireEvent.click(row("Muster, Eva"));
    const firstSrc = (await screen.findByTitle("Rechnung FE-0001"))
      .getAttribute("src")!
      .replace(/#.*$/, "");

    fireEvent.click(row("Beispiel, Jonas"));
    await shownLetter("Rechnung FE-0002");
    expect(revoked).toContain(firstSrc);

    const secondSrc = screen
      .getByTitle("Rechnung FE-0002")
      .getAttribute("src")!
      .replace(/#.*$/, "");
    unmount();
    expect(revoked).toContain(secondSrc);
  });

  it("opens the letter from the row menu and closes it again", async () => {
    renderCard();
    const [menu] = screen.getAllByRole("button", {
      name: "Aktionen für die Rechnung an Eva Muster",
    });
    fireEvent.click(menu!);
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Schreiben ansehen" }),
    );
    await shownLetter("Rechnung FE-0001");

    fireEvent.click(screen.getByRole("button", { name: "Vorschau schließen" }));
    expect(screen.getByText("Noch kein Schreiben gewählt")).toBeInTheDocument();
    expect(row("Muster, Eva")).not.toHaveAttribute("aria-current");
  });

  it("names a letter that fails to load and loads it again", async () => {
    api.downloadBillingLetter.mockRejectedValueOnce(
      new ApiError("Request failed (500)", 500),
    );
    renderCard();
    fireEvent.click(row("Muster, Eva"));

    fireEvent.click(await screen.findByRole("button", { name: "Wiederholen" }));
    expect(await shownLetter("Rechnung FE-0001")).toBe(LETTERS["51"]!.blob);
    expect(api.downloadBillingLetter).toHaveBeenCalledTimes(2);
  });

  it("offers no preview before the letters of the run exist", () => {
    renderCard({ ...RUN, letters: null });

    expect(
      screen.queryByText("Noch kein Schreiben gewählt"),
    ).not.toBeInTheDocument();
    fireEvent.click(row("Muster, Eva"));
    expect(api.downloadBillingLetter).not.toHaveBeenCalled();
    expect(row("Muster, Eva")).not.toHaveAttribute("aria-current");
    expect(screen.getByText("Rechnungen")).toBeInTheDocument();
  });
});
