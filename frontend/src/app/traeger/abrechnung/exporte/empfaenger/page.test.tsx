import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { ExportRecipient } from "~/lib/traeger-export-handovers-api";
import TraegerExportRecipientPage from "./page";

// The recipient of the finance exports (#6824, ADR 0218): the password is
// never shown and only sent when typed, it never follows a changed
// destination, the removal asks first, and every handover is listed.

const api = vi.hoisted(() => ({
  fetchExportRecipient: vi.fn(),
  saveExportRecipient: vi.fn(),
  deleteExportRecipient: vi.fn(),
}));
vi.mock("~/lib/traeger-export-handovers-api", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  ...api,
}));
vi.mock("~/lib/traeger-url", () => ({ traegerPath: (path: string) => path }));

const READY: ExportRecipient = {
  configured: true,
  enabled: true,
  ready: true,
  host: "dateien.kaemmerei-beispiel.de",
  port: 22,
  username: "moto",
  remoteDirectory: "/eingang",
  hostKeyFingerprint: "SHA256:AbCdEfGhIjKlMnOpQrStUvWxYz0123456789+/AbCdE",
  passwordSet: true,
  missingFields: [],
  updatedAt: "2026-11-12T08:00:00Z",
};

beforeEach(() => {
  vi.resetAllMocks();
  api.fetchExportRecipient.mockResolvedValue({
    recipient: READY,
    handovers: [
      {
        id: "1",
        createdAt: "2026-11-12T08:30:00Z",
        format: "sollstellung",
        fileName: "sollstellung-110.csv",
        byteSize: 120,
        targetHost: "dateien.kaemmerei-beispiel.de",
        targetDirectory: "/eingang",
        state: "failed",
        reason: "authentication_rejected",
      },
    ],
  });
  api.saveExportRecipient.mockImplementation(async (draft) => ({
    ...READY,
    ...draft,
  }));
});

function renderPage() {
  render(
    <ToastProvider>
      <TraegerExportRecipientPage />
    </ToastProvider>,
  );
}

it("shows the ready recipient and every handover without the password", async () => {
  renderPage();
  expect(
    await screen.findByText("Der Empfänger ist bereit"),
  ).toBeInTheDocument();
  expect(screen.getByLabelText("Passwort")).toHaveValue("");
  expect(screen.getAllByText("sollstellung-110.csv").length).toBeGreaterThan(0);
  expect(screen.getAllByText("Nicht angekommen").length).toBeGreaterThan(0);
  expect(
    screen.getAllByText(/Die Anmeldung wurde abgelehnt/).length,
  ).toBeGreaterThan(0);
});

it("keeps the stored password unless one is typed", async () => {
  renderPage();
  const directory = await screen.findByLabelText("Zielordner");
  fireEvent.change(directory, { target: { value: "/eingang/moto" } });
  fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
  await waitFor(() => expect(api.saveExportRecipient).toHaveBeenCalledTimes(1));
  expect(api.saveExportRecipient.mock.calls[0]![0]).toMatchObject({
    remoteDirectory: "/eingang/moto",
    password: undefined,
  });
});

it("asks for the password again when the destination changes", async () => {
  renderPage();
  fireEvent.change(await screen.findByLabelText("Adresse"), {
    target: { value: "andere.example.org" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
  expect(
    await screen.findByText(
      /moto schickt es nur an die Adresse, für die Sie es eingegeben haben/,
    ),
  ).toBeInTheDocument();
  expect(api.saveExportRecipient).not.toHaveBeenCalled();
  fireEvent.change(screen.getByLabelText("Passwort"), {
    target: { value: "neues-geheimnis" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
  await waitFor(() =>
    expect(api.saveExportRecipient).toHaveBeenCalledWith(
      expect.objectContaining({
        host: "andere.example.org",
        password: "neues-geheimnis",
      }),
    ),
  );
});

it("names what is missing while the recipient is not complete", async () => {
  api.fetchExportRecipient.mockResolvedValue({
    recipient: {
      ...READY,
      configured: false,
      ready: false,
      host: "",
      username: "",
      remoteDirectory: "",
      hostKeyFingerprint: "",
      passwordSet: false,
      missingFields: [
        "host",
        "username",
        "password",
        "remote_directory",
        "host_key_fingerprint",
      ],
      updatedAt: null,
    },
    handovers: [],
  });
  renderPage();
  expect(
    await screen.findByText("Tragen Sie den Empfänger ein"),
  ).toBeInTheDocument();
  expect(
    screen.getByText(
      /Es fehlen: Adresse, Benutzername, Passwort, Zielordner und Fingerabdruck\./,
    ),
  ).toBeInTheDocument();
  expect(screen.getByText("Noch keine Übergabe.")).toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "Empfänger entfernen" }),
  ).not.toBeInTheDocument();
});

it("removes the recipient only after the question", async () => {
  api.deleteExportRecipient.mockResolvedValue({
    ...READY,
    configured: false,
    ready: false,
    passwordSet: false,
  });
  renderPage();
  fireEvent.click(
    await screen.findByRole("button", { name: "Empfänger entfernen" }),
  );
  expect(api.deleteExportRecipient).not.toHaveBeenCalled();
  fireEvent.click(await screen.findByRole("button", { name: "Entfernen" }));
  fireEvent.click(await screen.findByRole("button", { name: "Ja, entfernen" }));
  await waitFor(() =>
    expect(api.deleteExportRecipient).toHaveBeenCalledTimes(1),
  );
});
