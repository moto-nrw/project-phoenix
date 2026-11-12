import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { SepaDebitRun } from "~/lib/traeger-sepa-debits-api";
import { SepaDebitCard } from "./sepa-debit-card";

const api = vi.hoisted(() => ({
  fetchSepaDebitRun: vi.fn(),
  downloadSepaDebitFile: vi.fn(),
  submitSepaDebits: vi.fn(),
}));
vi.mock("~/lib/traeger-sepa-debits-api", async (importOriginal) => ({
  messageIdOfDebitFile: (
    await importOriginal<typeof import("~/lib/traeger-sepa-debits-api")>()
  ).messageIdOfDebitFile,
  ...api,
}));
vi.mock("~/lib/file-download", () => ({ downloadBlob: vi.fn() }));
vi.mock("~/lib/traeger-url", () => ({ traegerPath: (path: string) => path }));

const RUN: SepaDebitRun = {
  creditorId: "DE98ZZZ09999999999",
  creditorName: "Verein",
  creditorIbanMasked: "DE•• 1234",
  accountMissing: false,
  messageId: "overview-id",
  count: 1,
  totalCents: 1000,
  pastDue: 0,
  groups: [
    {
      sequenceType: "RCUR",
      collectOn: "2026-11-21",
      count: 1,
      totalCents: 1000,
    },
  ],
  debits: [],
  submitted: [],
  left: [],
  submittedAt: null,
};

beforeEach(() => {
  vi.resetAllMocks();
  api.fetchSepaDebitRun.mockResolvedValue(RUN);
  api.downloadSepaDebitFile.mockResolvedValue({
    blob: new Blob(["xml"]),
    filename: "downloaded-id.xml",
  });
  api.submitSepaDebits.mockResolvedValue({ ...RUN, count: 0 });
});

it("confirms the downloaded file rather than the stale overview", async () => {
  render(
    <ToastProvider>
      <SepaDebitCard kind="care-runs" runId="42" />
    </ToastProvider>,
  );
  fireEvent.click(
    await screen.findByRole("button", { name: "Datei herunterladen" }),
  );
  await waitFor(() => expect(api.downloadSepaDebitFile).toHaveBeenCalled());
  fireEvent.click(
    screen.getByRole("button", { name: "Bei der Bank eingereicht" }),
  );
  fireEvent.click(
    await screen.findByRole("button", { name: "Ja, eingereicht" }),
  );
  await waitFor(() =>
    expect(api.submitSepaDebits).toHaveBeenCalledWith(
      "care-runs",
      "42",
      "downloaded-id",
    ),
  );
});

it("requires a downloaded file before confirmation", async () => {
  render(
    <ToastProvider>
      <SepaDebitCard kind="care-runs" runId="42" />
    </ToastProvider>,
  );
  expect(
    await screen.findByRole("button", { name: "Bei der Bank eingereicht" }),
  ).toHaveAttribute("aria-disabled", "true");
});

it("confirms the message ID of a file named with its file number (#4693)", async () => {
  api.downloadSepaDebitFile.mockResolvedValue({
    blob: new Blob(["xml"]),
    filename: "lastschrift-112-MOTO-M3-20291104.xml",
  });
  render(
    <ToastProvider>
      <SepaDebitCard kind="care-runs" runId="42" />
    </ToastProvider>,
  );
  fireEvent.click(
    await screen.findByRole("button", { name: "Datei herunterladen" }),
  );
  await waitFor(() => expect(api.downloadSepaDebitFile).toHaveBeenCalled());
  fireEvent.click(
    screen.getByRole("button", { name: "Bei der Bank eingereicht" }),
  );
  fireEvent.click(
    await screen.findByRole("button", { name: "Ja, eingereicht" }),
  );
  await waitFor(() =>
    expect(api.submitSepaDebits).toHaveBeenCalledWith(
      "care-runs",
      "42",
      "MOTO-M3-20291104",
    ),
  );
});
