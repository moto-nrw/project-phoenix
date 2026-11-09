import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
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
import {
  DESKTOP_WIDTH,
  PHONE_WIDTH,
  setViewportWidth,
} from "~/test/helpers/viewport";

// DataTable renders only the layout for the window width (#6220).
afterEach(() => setViewportWidth(DESKTOP_WIDTH));

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
    templateVersion: 0,
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

/** The desktop table row of a family, or its repeated phone list item. */
function row(name: string, tag = "tr"): HTMLElement {
  const cell = screen
    .getAllByText(name)
    .find((element) => element.closest(tag));
  return cell!.closest<HTMLElement>(tag)!;
}

/** The content of a blob behind an object URL, always typed as a PDF. */
async function pdfAt(url: string): Promise<string | undefined> {
  const blob = urls.get(url);
  expect(blob?.type).toBe("application/pdf");
  return blob?.text();
}

/** What the preview frame of the letter currently shows. */
async function shownLetter(title: string): Promise<string | undefined> {
  const frame = await screen.findByTitle(title);
  return pdfAt((frame.getAttribute("src") ?? "").replace(/#.*$/, ""));
}

describe("BillingRunOverviewCard letter preview (#5692)", () => {
  it.each(["tr", "li"])(
    "keeps clicks on the %s row menu separate from the letter preview",
    async (tag) => {
      // The li rows are the phone list, in the document only at phone
      // width (#6220).
      if (tag === "li") setViewportWidth(PHONE_WIDTH);
      renderCard();
      const documentRow = row("Muster, Eva", tag);
      const menu = within(documentRow).getByRole("button", {
        name: "Aktionen für die Rechnung an Eva Muster",
      });

      fireEvent.click(menu);
      expect(
        await screen.findByRole("menuitem", {
          name: "Aufschlüsselung ansehen",
        }),
      ).toBeInTheDocument();
      // Anker: das Menü der geladenen Rechnungszeile ist offen.
      expect(menu).toHaveAttribute("aria-expanded", "true");
      expect(api.downloadBillingLetter).not.toHaveBeenCalled();

      fireEvent.click(
        screen.getByRole("menuitem", { name: "Aufschlüsselung ansehen" }),
      );
      expect(
        await screen.findByRole("dialog", { name: "Rechnung an Eva Muster" }),
      ).toBeInTheDocument();
      expect(api.downloadBillingLetter).not.toHaveBeenCalled();
    },
  );

  it.each(["tr", "li"])(
    "does not open a letter when a portaled cancellation action in a %s row is clicked",
    async (tag) => {
      // The li rows are the phone list, in the document only at phone
      // width (#6220).
      if (tag === "li") setViewportWidth(PHONE_WIDTH);
      const onCancel = vi.fn();
      render(
        <ToastProvider>
          <BillingRunOverviewCard run={RUN} onCancel={onCancel} />
        </ToastProvider>,
      );
      const documentRow = row("Muster, Eva", tag);
      const menu = within(documentRow).getByRole("button", {
        name: "Aktionen für die Rechnung an Eva Muster",
      });
      fireEvent.click(menu);
      const cancel = await screen.findByRole("menuitem", {
        name: "Rechnung stornieren",
      });
      expect(cancel).toBeInTheDocument();
      fireEvent.click(cancel);
      expect(onCancel).toHaveBeenCalledWith(RUN.documents[0]);
      // Anker: die Rechnungszeile und ihre Storno-Aktion sind geladen.
      expect(documentRow).toBeInTheDocument();
      expect(api.downloadBillingLetter).not.toHaveBeenCalled();
    },
  );

  it.each(["credit", "cancellation"] as const)(
    "returns focus to the %s document rather than another invoice for the same recipient",
    async (kind) => {
      const original = RUN.documents[0]!;
      const correction: BillingDocument = {
        ...original,
        id: "53",
        kind,
        number: "FE-0003",
        referenceNumber: original.number,
        totalCents: -original.totalCents,
      };
      api.downloadBillingLetter.mockResolvedValue(letter(correction.number));
      renderCard({ ...RUN, documents: [original, correction] });
      const documentRow = row("FE-0003");
      const menu = within(documentRow).getByRole("button", {
        name: "Aktionen für die Rechnung an Eva Muster",
      });
      // happy-dom has no layout; only the desktop trigger is visible here.
      vi.spyOn(HTMLElement.prototype, "getClientRects").mockImplementation(
        function (this: HTMLElement) {
          return Object.assign(this.closest("tr") ? [new DOMRect()] : [], {
            item: () => null,
          });
        },
      );
      fireEvent.click(documentRow);
      expect(
        await screen.findByTitle(
          `${kind === "credit" ? "Gutschrift" : "Storno"} FE-0003`,
        ),
      ).toBeInTheDocument();
      fireEvent.click(
        screen.getByRole("button", { name: "Schreiben schließen" }),
      );
      await waitFor(() => expect(menu).toHaveFocus());
    },
  );

  it("shows the letter of each chosen invoice under its own number", async () => {
    renderCard();

    expect(screen.getByText("Noch kein Schreiben gewählt")).toBeInTheDocument();
    // No letter is read before a row is chosen: every read is audited.
    expect(api.downloadBillingLetter).not.toHaveBeenCalled();

    fireEvent.click(row("Muster, Eva"));
    const first = screen.getByRole("region", {
      name: "Schreiben: Rechnung FE-0001",
    });
    expect(within(first).getByText("Eva Muster")).toBeInTheDocument();
    expect(await shownLetter("Rechnung FE-0001")).toBe("%PDF FE-0001");
    expect(row("Muster, Eva")).toHaveAttribute("aria-current", "true");
    expect(api.downloadBillingLetter).toHaveBeenLastCalledWith("9", "51");

    fireEvent.click(row("Beispiel, Jonas"));
    expect(
      screen.getByRole("region", { name: "Schreiben: Rechnung FE-0002" }),
    ).toBeInTheDocument();
    expect(await shownLetter("Rechnung FE-0002")).toBe("%PDF FE-0002");
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
    expect(await shownLetter("Rechnung FE-0002")).toBe("%PDF FE-0002");

    await act(async () => {
      finishFirst(LETTERS["51"]!);
      await Promise.resolve();
    });
    expect(await shownLetter("Rechnung FE-0002")).toBe("%PDF FE-0002");
    expect(screen.queryByTitle("Rechnung FE-0001")).not.toBeInTheDocument();
  });

  it("downloads and opens exactly the shown letter", async () => {
    renderCard();
    fireEvent.click(row("Beispiel, Jonas"));
    await shownLetter("Rechnung FE-0002");

    fireEvent.click(
      screen.getByRole("button", { name: "Rechnung FE-0002 herunterladen" }),
    );
    expect(downloadBlob).toHaveBeenCalledTimes(1);
    const [saved, filename] = downloadBlob.mock.calls[0] as [Blob, string];
    expect(filename).toBe("bestaetigung-fe-0002.pdf");
    expect(await saved.text()).toBe("%PDF FE-0002");
    // The download takes the file already loaded, without a second read.
    expect(api.downloadBillingLetter).toHaveBeenCalledTimes(1);

    const tab = screen.getByRole("link", {
      name: "Rechnung FE-0002 in neuem Tab öffnen",
    });
    expect(tab).toHaveAttribute("target", "_blank");
    expect(await pdfAt(tab.getAttribute("href") ?? "")).toBe("%PDF FE-0002");
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

    // The reader continues at the letter, and returns to the row menu.
    expect(
      screen.getByRole("heading", { name: "Rechnung FE-0001" }),
    ).toHaveFocus();
    fireEvent.click(
      screen.getByRole("button", { name: "Schreiben schließen" }),
    );
    expect(screen.getByText("Noch kein Schreiben gewählt")).toBeInTheDocument();
    expect(row("Muster, Eva")).not.toHaveAttribute("aria-current");
    await waitFor(() => expect(menu).toHaveFocus());
  });

  it("shows a response of another type only as a PDF", async () => {
    api.downloadBillingLetter.mockResolvedValue({
      blob: new Blob(["<script>alert(1)</script>"], { type: "text/html" }),
      filename: "bestaetigung-fe-0001.pdf",
    });
    renderCard();
    fireEvent.click(row("Muster, Eva"));

    expect(await shownLetter("Rechnung FE-0001")).toBe(
      "<script>alert(1)</script>",
    );
  });

  it("names a letter that fails to load and loads it again", async () => {
    api.downloadBillingLetter.mockRejectedValueOnce(
      new ApiError("Request failed (500)", 500),
    );
    renderCard();
    fireEvent.click(row("Muster, Eva"));

    fireEvent.click(await screen.findByRole("button", { name: "Wiederholen" }));
    expect(await shownLetter("Rechnung FE-0001")).toBe("%PDF FE-0001");
    expect(api.downloadBillingLetter).toHaveBeenCalledTimes(2);
  });

  // Before the release one switch shows every breakdown in place: checking
  // the run needs no menu per row.
  it("shows the breakdowns in place before the release", () => {
    renderCard({
      ...RUN,
      status: "calculated",
      releasedAt: null,
      letters: null,
      documents: [
        {
          ...invoice("51", "", "Eva", "Muster", 11250),
          children: [
            {
              childId: "c-51",
              firstName: "Lia",
              lastName: "Muster",
              totalCents: 11250,
              corrections: [],
              carriedCorrections: [],
              positions: [],
            },
          ],
        },
      ],
    });

    const toggle = screen.getByRole("button", {
      name: "Aufschlüsselung zeigen",
    });
    fireEvent.click(toggle);

    expect(toggle).toHaveAttribute("aria-expanded", "true");
    expect(toggle).toHaveTextContent("Aufschlüsselung ausblenden");
    expect(screen.getByText("Lia Muster")).toBeInTheDocument();
    // One child: its sum is the invoice amount, no second total line.
    expect(screen.getByText("Summe")).toBeInTheDocument();
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
