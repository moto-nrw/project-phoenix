import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { ExportRecipient } from "~/lib/traeger-export-handovers-api";
import TraegerExportRecipientPage from "./page";

// One recipient of the finance exports (#6824, ADR 0218; #6911, ADR 0221):
// the password is never shown and only sent when typed, it never follows a
// changed destination, the removal asks first and names the files that go
// back to the download, and every handover of this recipient is listed.

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
const router = vi.hoisted(() => ({ push: vi.fn() }));
vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "7" }),
  useRouter: () => router,
}));

const READY: ExportRecipient = {
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
        recipientId: "7",
        recipientName: "Kämmerei",
        state: "failed",
        reason: "authentication_rejected",
      },
    ],
  });
  api.saveExportRecipient.mockImplementation(async (_id, draft) => ({
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
  expect(
    screen.getByText(
      "Sollstellung und Lastschriftdatei gehen an dateien.kaemmerei-beispiel.de, Ordner /eingang.",
    ),
  ).toBeInTheDocument();
  expect(api.fetchExportRecipient).toHaveBeenCalledWith("7");
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
  expect(api.saveExportRecipient.mock.calls[0]![0]).toBe("7");
  expect(api.saveExportRecipient.mock.calls[0]![1]).toMatchObject({
    name: "Kämmerei",
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
      "7",
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
      formats: [],
    },
    handovers: [],
  });
  renderPage();
  expect(await screen.findByText("Es fehlen noch Angaben")).toBeInTheDocument();
  expect(
    screen.getByText(
      /Es fehlen: Adresse, Benutzername, Passwort, Zielordner und Fingerabdruck\./,
    ),
  ).toBeInTheDocument();
  expect(screen.getByText("Noch keine Übergabe.")).toBeInTheDocument();
});

it("removes the recipient only after the question and names what goes back to the download", async () => {
  api.deleteExportRecipient.mockResolvedValue({
    recipients: [],
    assignments: [],
    handovers: [],
  });
  renderPage();
  fireEvent.click(
    await screen.findByRole("button", { name: "Empfänger entfernen" }),
  );
  expect(
    await screen.findByText(
      /Sollstellung und Lastschriftdatei können Sie danach nur herunterladen/,
    ),
  ).toBeInTheDocument();
  expect(api.deleteExportRecipient).not.toHaveBeenCalled();
  fireEvent.click(await screen.findByRole("button", { name: "Entfernen" }));
  fireEvent.click(await screen.findByRole("button", { name: "Ja, entfernen" }));
  await waitFor(() =>
    expect(api.deleteExportRecipient).toHaveBeenCalledWith("7"),
  );
  await waitFor(() =>
    expect(router.push).toHaveBeenCalledWith(
      "/traeger/abrechnung/exporte/empfaenger",
    ),
  );
});

it("keeps a failed last handover in view until the next one", async () => {
  api.fetchExportRecipient.mockResolvedValue({
    recipient: {
      ...READY,
      lastHandover: {
        id: "1",
        createdAt: "2026-11-12T08:30:00Z",
        format: "sollstellung",
        fileName: "sollstellung-110.csv",
        byteSize: 120,
        targetHost: "dateien.kaemmerei-beispiel.de",
        targetDirectory: "/eingang",
        recipientId: "7",
        recipientName: "Kämmerei",
        state: "failed",
        reason: "authentication_rejected",
      },
    },
    handovers: [],
  });
  renderPage();
  expect(
    await screen.findByText(
      "Die letzte Übergabe ist nicht angekommen: sollstellung-110.csv",
    ),
  ).toBeInTheDocument();
  expect(
    screen.queryByText("Der Empfänger ist bereit"),
  ).not.toBeInTheDocument();
});
