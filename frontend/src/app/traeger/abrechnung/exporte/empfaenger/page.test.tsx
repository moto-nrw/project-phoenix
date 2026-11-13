import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type {
  ExportRecipient,
  ExportRecipientsPage,
} from "~/lib/traeger-export-handovers-api";
import TraegerExportRecipientsPage from "./page";

// The recipients of the finance exports (#6911, ADR 0221): each file goes to
// exactly one recipient or stays a download, the assignment is saved as a
// whole, every recipient shows its state, and the handovers name their
// recipient.

const api = vi.hoisted(() => ({
  fetchExportRecipients: vi.fn(),
  assignExportFormats: vi.fn(),
}));
vi.mock("~/lib/traeger-export-handovers-api", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  ...api,
}));
vi.mock("~/lib/traeger-url", () => ({ traegerPath: (path: string) => path }));

const KAEMMEREI: ExportRecipient = {
  id: "7",
  name: "Kämmerei",
  enabled: true,
  ready: true,
  host: "dateien.kaemmerei-beispiel.de",
  port: 22,
  username: "moto",
  remoteDirectory: "/eingang",
  hostKeyFingerprint: "SHA256:AbCdEfGhIjKlMnOpQrStUvWxYz0123456789+/AbCdE",
  passwordSet: true,
  missingFields: [],
  formats: ["sollstellung", "pain008"],
  lastHandover: null,
  updatedAt: "2026-11-12T08:00:00Z",
};

const STEUERBUERO: ExportRecipient = {
  ...KAEMMEREI,
  id: "8",
  name: "Steuerbüro",
  host: "sftp.steuerbuero-beispiel.de",
  formats: [],
  lastHandover: {
    id: "3",
    createdAt: "2026-11-12T09:00:00Z",
    format: "datev",
    fileName: "EXTF_Buchungsstapel_4.csv",
    byteSize: 400,
    targetHost: "sftp.steuerbuero-beispiel.de",
    targetDirectory: "/eingang",
    recipientId: "8",
    recipientName: "Steuerbüro",
    state: "failed",
    reason: "connection_failed",
  },
};

const PAGE: ExportRecipientsPage = {
  recipients: [KAEMMEREI, STEUERBUERO],
  assignments: [
    { format: "sollstellung", recipientId: "7" },
    { format: "datev", recipientId: null },
    { format: "pain008", recipientId: "7" },
  ],
  handovers: [
    STEUERBUERO.lastHandover!,
    {
      ...STEUERBUERO.lastHandover!,
      id: "2",
      fileName: "sollstellung-110.csv",
      format: "sollstellung",
      recipientId: null,
      recipientName: "Alte Kasse",
      state: "transferred",
      reason: "",
    },
  ],
};

beforeEach(() => {
  vi.resetAllMocks();
  api.fetchExportRecipients.mockResolvedValue(PAGE);
  api.assignExportFormats.mockResolvedValue({
    ...PAGE,
    assignments: [
      { format: "sollstellung", recipientId: "7" },
      { format: "datev", recipientId: "8" },
      { format: "pain008", recipientId: "7" },
    ],
  });
});

function renderPage() {
  render(
    <ToastProvider>
      <TraegerExportRecipientsPage />
    </ToastProvider>,
  );
}

it("sends a file that has no recipient to one, as a whole", async () => {
  renderPage();
  const card = (await screen.findByText("Welche Datei geht wohin")).closest(
    "section",
  )!;
  const save = within(card).getByRole("button", { name: "Speichern" });
  expect(save).toBeDisabled();
  fireEvent.click(within(card).getByRole("combobox", { name: "DATEV-Datei" }));
  fireEvent.click(await screen.findByRole("option", { name: "Steuerbüro" }));
  fireEvent.click(save);
  await waitFor(() =>
    expect(api.assignExportFormats).toHaveBeenCalledWith([
      { format: "sollstellung", recipientId: "7" },
      { format: "datev", recipientId: "8" },
      { format: "pain008", recipientId: "7" },
    ]),
  );
  expect(
    await screen.findByText("Die Zuordnung ist gespeichert."),
  ).toBeInTheDocument();
});

it("offers the download for every file and leaves a file without recipient to it", async () => {
  renderPage();
  const card = (await screen.findByText("Welche Datei geht wohin")).closest(
    "section",
  )!;
  expect(
    within(card).getByRole("combobox", { name: "DATEV-Datei" }),
  ).toHaveTextContent("Nur herunterladen");
  expect(
    within(card).getByRole("combobox", { name: "Sollstellung" }),
  ).toHaveTextContent("Kämmerei");
});

it("shows every recipient with its state and the files it receives", async () => {
  renderPage();
  const kaemmerei = await screen.findByRole("link", { name: /Kämmerei/ });
  expect(kaemmerei).toHaveAttribute(
    "href",
    "/traeger/abrechnung/exporte/empfaenger/7",
  );
  expect(
    within(kaemmerei).getByText("Bekommt: Sollstellung und Lastschriftdatei"),
  ).toBeInTheDocument();
  expect(within(kaemmerei).getByText("Bereit")).toBeInTheDocument();
  const steuerbuero = screen.getByRole("link", { name: /Steuerbüro/ });
  expect(
    within(steuerbuero).getByText("Letzte Übergabe nicht angekommen"),
  ).toBeInTheDocument();
  expect(
    within(steuerbuero).getByText("Bekommt noch keine Datei"),
  ).toBeInTheDocument();
});

it("names the recipient of every handover, also a removed one", async () => {
  renderPage();
  expect(
    (await screen.findAllByText("sollstellung-110.csv")).length,
  ).toBeGreaterThan(0);
  expect(screen.getAllByText("Alte Kasse").length).toBeGreaterThan(0);
  expect(screen.getAllByText("(entfernt)").length).toBeGreaterThan(0);
});
