import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import TraegerNewExportRecipientPage from "./page";

// A new recipient (#6911, ADR 0221) needs a name and goes back to the
// overview, where the Träger assigns its files.

const api = vi.hoisted(() => ({ createExportRecipient: vi.fn() }));
vi.mock("~/lib/traeger-export-handovers-api", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  ...api,
}));
vi.mock("~/lib/traeger-url", () => ({ traegerPath: (path: string) => path }));
const router = vi.hoisted(() => ({ push: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => router }));

beforeEach(() => {
  vi.resetAllMocks();
  api.createExportRecipient.mockResolvedValue({ id: "9" });
});

function renderPage() {
  render(
    <ToastProvider>
      <TraegerNewExportRecipientPage />
    </ToastProvider>,
  );
}

it("asks for a name before it adds anything", async () => {
  renderPage();
  fireEvent.click(screen.getByRole("button", { name: "Empfänger anlegen" }));
  expect(
    await screen.findByText("Bitte geben Sie dem Empfänger einen Namen."),
  ).toBeInTheDocument();
  expect(api.createExportRecipient).not.toHaveBeenCalled();
});

it("adds the recipient and returns to the overview", async () => {
  renderPage();
  fireEvent.change(screen.getByLabelText(/Name/), {
    target: { value: " Steuerbüro " },
  });
  fireEvent.change(screen.getByLabelText("Adresse"), {
    target: { value: "sftp.steuerbuero-beispiel.de" },
  });
  fireEvent.change(screen.getByLabelText("Passwort"), {
    target: { value: "geheim" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Empfänger anlegen" }));
  await waitFor(() =>
    expect(api.createExportRecipient).toHaveBeenCalledWith(
      expect.objectContaining({
        name: "Steuerbüro",
        enabled: true,
        host: "sftp.steuerbuero-beispiel.de",
        port: 22,
        password: "geheim",
      }),
    ),
  );
  await waitFor(() =>
    expect(router.push).toHaveBeenCalledWith(
      "/traeger/abrechnung/exporte/empfaenger",
    ),
  );
});
