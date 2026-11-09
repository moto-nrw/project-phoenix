import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type {
  CareNoticeLetters,
  CareNoticeMailing,
} from "~/lib/traeger-care-notice-letters-api";
import type { CareNoticeRunDetail } from "~/lib/traeger-care-notices-api";
import { usePhoneViewport } from "~/test/helpers/viewport";

const { notices, letters } = vi.hoisted(() => ({
  notices: { fetchCareNoticeRun: vi.fn() },
  letters: {
    fetchCareNoticeLetters: vi.fn(),
    createCareNoticeLetters: vi.fn(),
    mailCareNoticeLetters: vi.fn(),
    mailCareNoticeLetter: vi.fn(),
    downloadCareNoticePDF: vi.fn(),
  },
}));
vi.mock("~/lib/traeger-care-notices-api", async (original) => ({
  ...(await original<typeof import("~/lib/traeger-care-notices-api")>()),
  ...notices,
}));
vi.mock("~/lib/traeger-care-notice-letters-api", async (original) => ({
  ...(await original<typeof import("~/lib/traeger-care-notice-letters-api")>()),
  ...letters,
}));
vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "9" }),
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => "/abrechnung/jahresbescheide/9/versand",
}));
vi.mock("~/lib/traeger-url", () => ({ traegerPath: (path: string) => path }));

import TraegerCareNoticeDeliveryPage from "./page";

const RUN = {
  id: "9",
  year: 2030,
  periodFrom: "2030-09-01",
  periodUntil: "2030-12-31",
  status: "released",
  schoolIds: ["3"],
  noticeDate: "2030-09-24",
  issuedOn: "2030-09-24",
  noticeCount: 1,
  totalCents: 31000,
  notices: [],
  previewCount: 1,
  previewCents: 31000,
} as unknown as CareNoticeRunDetail;

const MAILING: CareNoticeMailing = {
  noticeId: "51",
  contractId: "21",
  kind: "initial",
  number: "MB-2030-0001",
  amendsNumber: "",
  amendedByNumber: "",
  childFirstName: "Mia",
  childLastName: "Beispiel",
  schoolName: "Grundschule Ispringen",
  recipientFirstName: "Eva",
  recipientLastName: "Beispiel",
  recipientEmail: "eva@example.org",
  totalCents: 31000,
  hasLetter: false,
  sha256: "",
  inInbox: true,
  status: "not_sent",
  sends: 0,
  createdAt: null,
  requestedAt: null,
  sentAt: null,
};

function state(mailing: CareNoticeMailing): CareNoticeLetters {
  return {
    notices: [mailing],
    summary: {
      notices: 1,
      letters: mailing.hasLetter ? 1 : 0,
      sent: 0,
      queued: 0,
      failed: 0,
      notSent: mailing.hasLetter ? 1 : 0,
      withoutEmail: 0,
      withoutInbox: 0,
    },
  };
}

function renderPage() {
  return render(
    <ToastProvider>
      <TraegerCareNoticeDeliveryPage />
    </ToastProvider>,
  );
}

describe("TraegerCareNoticeDeliveryPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    notices.fetchCareNoticeRun.mockResolvedValue(RUN);
  });

  it.each([
    { name: "queued", counts: { letters: 2, queued: 1 } },
    { name: "failed", counts: { letters: 2, failed: 1 } },
    { name: "missing original", counts: { letters: 1 } },
  ])("does not report completion with a $name notice", async ({ counts }) => {
    const data = state({
      ...MAILING,
      hasLetter: true,
      status: "sent",
      sends: 1,
    });
    letters.fetchCareNoticeLetters.mockResolvedValue({
      ...data,
      summary: { ...data.summary, notices: 2, sent: 1, notSent: 0, ...counts },
    });
    renderPage();

    // Anker: der geladene Versandstand enthält einen gesendeten Bescheid.
    expect(
      await screen.findByText(/1 von .*Bescheid.*gesendet/),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/Alle Bescheide mit E-Mail-Adresse sind gesendet/),
    ).not.toBeInTheDocument();
    if ("failed" in counts) {
      expect(screen.getByText(/erneut.*oder drucken/i)).toBeInTheDocument();
    }
  });

  it("reports completion when only families without email remain", async () => {
    const data = state({
      ...MAILING,
      hasLetter: true,
      status: "sent",
      sends: 1,
    });
    letters.fetchCareNoticeLetters.mockResolvedValue({
      ...data,
      summary: {
        ...data.summary,
        notices: 2,
        letters: 2,
        sent: 1,
        notSent: 1,
        withoutEmail: 1,
      },
    });
    renderPage();
    expect(
      await screen.findByText(
        /Alle Bescheide mit E-Mail-Adresse sind gesendet/,
      ),
    ).toBeInTheDocument();
  });

  describe("phone actions", () => {
    usePhoneViewport();

    it.each([
      { status: "not_sent", sends: 0, action: "Senden" },
      { status: "failed", sends: 1, action: "Erneut senden" },
    ] as const)(
      "opens the original and sends a $status notice",
      async ({ status, sends, action }) => {
        letters.fetchCareNoticeLetters.mockResolvedValue(
          state({ ...MAILING, hasLetter: true, status, sends }),
        );
        letters.downloadCareNoticePDF.mockResolvedValue({
          blob: new Blob(["PDF"], { type: "application/pdf" }),
          filename: "bescheid.pdf",
        });
        letters.mailCareNoticeLetter.mockResolvedValue(undefined);
        renderPage();

        fireEvent.click(
          await screen.findByRole("button", {
            name: "Aktionen für den Bescheid von Mia Beispiel",
          }),
        );
        fireEvent.click(
          await screen.findByRole("menuitem", { name: "Original ansehen" }),
        );
        expect(
          await screen.findByRole("heading", { name: "Bescheid MB-2030-0001" }),
        ).toBeInTheDocument();
        await waitFor(() =>
          expect(letters.downloadCareNoticePDF).toHaveBeenCalledWith("9", "21"),
        );
        fireEvent.click(screen.getByRole("button", { name: "Schließen" }));
        fireEvent.click(
          await screen.findByRole("button", {
            name: "Aktionen für den Bescheid von Mia Beispiel",
          }),
        );
        fireEvent.click(
          await screen.findByRole("menuitem", {
            name: new RegExp(`^${action}`),
          }),
        );
        await waitFor(() =>
          expect(letters.mailCareNoticeLetter).toHaveBeenCalledWith("9", "21"),
        );
      },
    );
  });

  it("stores the originals first and sends nothing with it", async () => {
    letters.fetchCareNoticeLetters.mockResolvedValue(state(MAILING));
    letters.createCareNoticeLetters.mockResolvedValue(
      state({ ...MAILING, hasLetter: true, sha256: "ab" }),
    );
    renderPage();

    fireEvent.click(
      await screen.findByRole("button", { name: "1 Original speichern" }),
    );
    await waitFor(() =>
      expect(letters.createCareNoticeLetters).toHaveBeenCalledWith("9"),
    );
    expect(letters.mailCareNoticeLetters).not.toHaveBeenCalled();
    expect(
      await screen.findByRole("button", { name: "An alle senden" }),
    ).toBeEnabled();
    expect(screen.getByText("Keine förmliche Zustellung")).toBeInTheDocument();
  });

  it("sends to every family only after the confirmation", async () => {
    letters.fetchCareNoticeLetters.mockResolvedValue(
      state({ ...MAILING, hasLetter: true }),
    );
    letters.mailCareNoticeLetters.mockResolvedValue({
      queued: 1,
      already: 0,
      withoutEmail: 0,
      recipientChanged: 0,
    });
    renderPage();

    fireEvent.click(
      await screen.findByRole("button", { name: "An alle senden" }),
    );
    expect(letters.mailCareNoticeLetters).not.toHaveBeenCalled();
    fireEvent.click(
      await screen.findByRole("button", { name: "1 Bescheid senden" }),
    );
    await waitFor(() =>
      expect(letters.mailCareNoticeLetters).toHaveBeenCalledWith("9"),
    );
    expect(
      await screen.findByText("1 Bescheid wird jetzt gesendet."),
    ).toBeInTheDocument();
  });
});
