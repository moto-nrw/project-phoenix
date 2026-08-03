import "@testing-library/jest-dom/vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import OpeningBalanceImportPage from "./page";

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    data: { user: { token: "test-token" } },
    status: "authenticated",
  })),
}));

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
}));

const mockToast = {
  success: vi.fn(),
  error: vi.fn(),
  warning: vi.fn(),
  info: vi.fn(),
};
vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => mockToast,
}));

vi.mock("~/lib/auth-utils", () => ({
  hasPermission: () => true,
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn() }),
}));

vi.mock("~/components/ui/date-picker", async (importOriginal) => {
  const { isoDatePickerMock } = await import("~/test/mocks/date-picker");
  return { ...(await importOriginal<object>()), ...isoDatePickerMock() };
});

vi.mock("~/components/import/upload-section", () => ({
  UploadSection: ({
    title,
    onFileSelect,
  }: {
    title?: string;
    onFileSelect: (file: File) => void;
  }) => (
    <div data-testid="upload-section">
      <span>{title}</span>
      <button
        type="button"
        data-testid="file-select-trigger"
        onClick={() =>
          onFileSelect(new File(["x"], "salden.csv", { type: "text/csv" }))
        }
      >
        Datei wählen
      </button>
    </div>
  ),
}));

function fillParams() {
  fireEvent.change(screen.getByLabelText("Stichtag"), {
    target: { value: "2026-08-01" },
  });
  fireEvent.change(screen.getByLabelText("Begründung"), {
    target: { value: "Übernahme aus Altsystem" },
  });
}

function previewResponse(overrides: Record<string, unknown> = {}) {
  return {
    ok: true,
    json: () =>
      Promise.resolve({
        data: {
          TotalRows: 3,
          CreatedCount: 2,
          UpdatedCount: 0,
          ErrorCount: 1,
          WarningCount: 0,
          DryRun: true,
          Errors: [
            {
              RowNumber: 4,
              Data: {
                personnel_number: "1003",
                first_name: "Clara",
                last_name: "Weber",
                hours_balance: "",
                vacation_entitled: "28",
                vacation_carryover: "0",
                vacation_remaining: "28",
              },
              Errors: [
                {
                  field: "vacation",
                  message: "Für diese Person besteht bereits eine Übernahme",
                  code: "already_exists",
                  severity: "error",
                },
              ],
              Timestamp: "2026-08-03T10:00:00Z",
            },
          ],
          ...overrides,
        },
      }),
  };
}

describe("OpeningBalanceImportPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
  });

  afterEach(() => {
    cleanup();
  });

  it("rendert die vier Schritte der Seite", () => {
    render(<OpeningBalanceImportPage />);

    expect(
      screen.getByRole("heading", { name: "Eröffnungssalden importieren" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Schritt 1: Vorlage herunterladen"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Schritt 2: Stichtag und Begründung"),
    ).toBeInTheDocument();
    expect(screen.getByText("Schritt 3: Datei hochladen")).toBeInTheDocument();
  });

  it("lädt die Vorlage über den Eröffnungssalden-Endpoint", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      blob: () => Promise.resolve(new Blob(["x"], { type: "text/csv" })),
    });
    global.URL.createObjectURL = vi.fn(() => "blob:test");
    global.URL.revokeObjectURL = vi.fn();

    render(<OpeningBalanceImportPage />);
    fireEvent.click(screen.getByText("Vorlage herunterladen"));

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/import/opening-balances/template?format=xlsx",
        expect.objectContaining({
          headers: { Authorization: "Bearer test-token" },
        }),
      );
    });
  });

  it("verlangt Stichtag und Begründung, bevor die Vorschau läuft", async () => {
    render(<OpeningBalanceImportPage />);
    fireEvent.click(screen.getByTestId("file-select-trigger"));

    await waitFor(() => {
      expect(
        screen.getByText(
          "Bitte zuerst Stichtag und Begründung angeben, dann wird die Vorschau erstellt.",
        ),
      ).toBeInTheDocument();
    });
    expect(global.fetch).not.toHaveBeenCalled();
  });

  it("schickt Stichtag und Begründung im FormData der Vorschau", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce(
      previewResponse(),
    );

    render(<OpeningBalanceImportPage />);
    fillParams();
    fireEvent.click(screen.getByTestId("file-select-trigger"));

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/import/opening-balances/preview",
        expect.objectContaining({ method: "POST" }),
      );
    });

    const call = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0] as [
      string,
      { body: FormData },
    ];
    expect(call[1].body.get("effective_date")).toBe("2026-08-01");
    expect(call[1].body.get("note")).toBe("Übernahme aus Altsystem");
    expect(call[1].body.get("file")).toBeInstanceOf(File);
  });

  it("zeigt Fehlerzeile und Sammelzeile für die übrigen Zeilen", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce(
      previewResponse(),
    );

    render(<OpeningBalanceImportPage />);
    fillParams();
    fireEvent.click(screen.getByTestId("file-select-trigger"));

    await waitFor(() => {
      expect(screen.getByText("Clara Weber")).toBeInTheDocument();
    });
    expect(
      screen.getByText("Für diese Person besteht bereits eine Übernahme"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("2 weitere Zeilen ohne Befund"),
    ).toBeInTheDocument();
    expect(screen.getByText("2 Übernahmen buchen")).toBeInTheDocument();
  });

  it("clears the previous preview while a newly selected file is being previewed", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce(
      previewResponse(),
    );
    render(<OpeningBalanceImportPage />);
    fillParams();
    fireEvent.click(screen.getByTestId("file-select-trigger"));

    await waitFor(() => {
      expect(screen.getByText("2 Übernahmen buchen")).toBeInTheDocument();
    });

    (global.fetch as ReturnType<typeof vi.fn>).mockImplementationOnce(
      () => new Promise(() => undefined),
    );
    fireEvent.click(screen.getByTestId("file-select-trigger"));

    expect(
      screen.queryByRole("button", { name: "2 Übernahmen buchen" }),
    ).not.toBeInTheDocument();
  });

  it("verwirft die Vorschau, wenn der Stichtag nachträglich geändert wird", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce(
      previewResponse(),
    );

    render(<OpeningBalanceImportPage />);
    fillParams();
    fireEvent.click(screen.getByTestId("file-select-trigger"));

    await waitFor(() => {
      expect(screen.getByText("2 Übernahmen buchen")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText("Stichtag"), {
      target: { value: "2026-07-31" },
    });

    await waitFor(() => {
      expect(screen.queryByText("2 Übernahmen buchen")).not.toBeInTheDocument();
    });
    expect(screen.getByText("Vorschau erneut erstellen")).toBeInTheDocument();
  });

  it("bucht den Import und meldet Teilerfolg als Warnung", async () => {
    (global.fetch as ReturnType<typeof vi.fn>)
      .mockResolvedValueOnce(previewResponse())
      .mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            data: {
              TotalRows: 3,
              CreatedCount: 2,
              UpdatedCount: 0,
              ErrorCount: 1,
              WarningCount: 0,
              DryRun: false,
              Errors: [],
            },
          }),
      });

    render(<OpeningBalanceImportPage />);
    fillParams();
    fireEvent.click(screen.getByTestId("file-select-trigger"));

    await waitFor(() => {
      expect(screen.getByText("2 Übernahmen buchen")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("2 Übernahmen buchen"));

    await waitFor(() => {
      expect(global.fetch).toHaveBeenLastCalledWith(
        "/api/import/opening-balances/import",
        expect.objectContaining({ method: "POST" }),
      );
      expect(mockToast.warning).toHaveBeenCalledWith(
        "2 übernommen, 1 übersprungen",
      );
    });
    expect(screen.getByText("Import abgeschlossen")).toBeInTheDocument();
  });
});
