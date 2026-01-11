import {
  render,
  screen,
  fireEvent,
  waitFor,
  cleanup,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import StudentCSVImportPage from "./page";

// Mock next-auth/react
vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    data: { user: { token: "test-token" } },
    status: "authenticated",
  })),
}));

// Mock next/navigation
vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
}));

// Mock ToastContext
const mockToast = {
  success: vi.fn(),
  error: vi.fn(),
  warning: vi.fn(),
  info: vi.fn(),
};
vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => mockToast,
}));

// Mock ResponsiveLayout to simplify testing
vi.mock("~/components/dashboard", () => ({
  ResponsiveLayout: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="responsive-layout">{children}</div>
  ),
}));

// Mock Loading component
vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div data-testid="loading">Loading...</div>,
}));

// Mock Button component
vi.mock("~/components/ui/button", () => ({
  Button: ({
    children,
    onClick,
    ...props
  }: {
    children: React.ReactNode;
    onClick?: () => void;
  }) => (
    <button onClick={onClick} {...props}>
      {children}
    </button>
  ),
}));

// Mock Alert component
vi.mock("~/components/ui/alert", () => ({
  Alert: ({ message, type }: { message: string; type: string }) => (
    <div data-testid={`alert-${type}`}>{message}</div>
  ),
}));

// Mock import components
vi.mock("~/components/import", () => ({
  UploadSection: ({
    onFileSelect,
    isDragging,
    isLoading,
  }: {
    onFileSelect: (file: File) => void;
    isDragging: boolean;
    isLoading: boolean;
  }) => (
    <div data-testid="upload-section">
      <span data-testid="is-dragging">{isDragging.toString()}</span>
      <span data-testid="is-loading">{isLoading.toString()}</span>
      <button
        data-testid="file-select-trigger"
        onClick={() =>
          onFileSelect(new File(["test"], "test.csv", { type: "text/csv" }))
        }
      >
        Select File
      </button>
    </div>
  ),
  StatsCards: ({
    total,
    newCount,
    existing,
    errors,
  }: {
    total: number;
    newCount: number;
    existing: number;
    errors: number;
  }) => (
    <div data-testid="stats-cards">
      <span data-testid="stat-total">{total}</span>
      <span data-testid="stat-new">{newCount}</span>
      <span data-testid="stat-existing">{existing}</span>
      <span data-testid="stat-errors">{errors}</span>
    </div>
  ),
  StudentRowCard: ({
    student,
  }: {
    student: { first_name: string; last_name: string };
  }) => (
    <div data-testid="student-row">
      {student.first_name} {student.last_name}
    </div>
  ),
}));

describe("StudentCSVImportPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
  });

  afterEach(() => {
    cleanup();
  });

  it("renders the page with instruction section", () => {
    render(<StudentCSVImportPage />);

    expect(screen.getByText("CSV/Excel-Import Anleitung")).toBeInTheDocument();
    expect(
      screen.getByText(/Laden Sie die Vorlage herunter/),
    ).toBeInTheDocument();
  });

  it("renders the template download section", () => {
    render(<StudentCSVImportPage />);

    expect(
      screen.getByText("Schritt 1: Vorlage herunterladen"),
    ).toBeInTheDocument();
    expect(screen.getByText("Format wählen")).toBeInTheDocument();
    expect(screen.getByText("Vorlage herunterladen")).toBeInTheDocument();
  });

  it("renders the upload section", () => {
    render(<StudentCSVImportPage />);

    expect(screen.getByTestId("upload-section")).toBeInTheDocument();
  });

  it("allows selecting template format", () => {
    render(<StudentCSVImportPage />);

    const select = screen.getByRole("combobox");
    expect(select).toHaveValue("csv");

    fireEvent.change(select, { target: { value: "xlsx" } });
    expect(select).toHaveValue("xlsx");
  });

  it("calls API when download button is clicked", async () => {
    const mockBlob = new Blob(["test"], { type: "text/csv" });
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      blob: () => Promise.resolve(mockBlob),
    });

    // Mock URL methods (needed to prevent errors)
    global.URL.createObjectURL = vi.fn(() => "blob:test-url");
    global.URL.revokeObjectURL = vi.fn();

    render(<StudentCSVImportPage />);

    const downloadButton = screen.getByText("Vorlage herunterladen");
    fireEvent.click(downloadButton);

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/import/students/template?format=csv",
        expect.objectContaining({
          headers: { Authorization: "Bearer test-token" },
        }),
      );
    });
  });

  it("handles file upload and preview", async () => {
    const mockPreviewResponse = {
      data: {
        TotalRows: 5,
        CreatedCount: 3,
        UpdatedCount: 2,
        ErrorCount: 0,
        Errors: [],
      },
    };

    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockPreviewResponse),
    });

    render(<StudentCSVImportPage />);

    const fileSelectButton = screen.getByTestId("file-select-trigger");
    fireEvent.click(fileSelectButton);

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/import/students/preview",
        expect.objectContaining({
          method: "POST",
        }),
      );
    });
  });

  it("displays error when API call fails", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: false,
      json: () => Promise.resolve({ message: "API Error" }),
    });

    render(<StudentCSVImportPage />);

    const fileSelectButton = screen.getByTestId("file-select-trigger");
    fireEvent.click(fileSelectButton);

    await waitFor(() => {
      expect(screen.getByTestId("alert-error")).toBeInTheDocument();
      expect(screen.getByText("API Error")).toBeInTheDocument();
    });
  });

  it("allows closing error alert", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: false,
      json: () => Promise.resolve({ message: "Test Error" }),
    });

    render(<StudentCSVImportPage />);

    const fileSelectButton = screen.getByTestId("file-select-trigger");
    fireEvent.click(fileSelectButton);

    await waitFor(() => {
      expect(screen.getByTestId("alert-error")).toBeInTheDocument();
    });

    const closeButton = screen.getByLabelText("Fehler schließen");
    fireEvent.click(closeButton);

    expect(screen.queryByTestId("alert-error")).not.toBeInTheDocument();
  });

  it("shows loading state in upload section", () => {
    render(<StudentCSVImportPage />);

    // Initial state should not be loading
    expect(screen.getByTestId("is-loading")).toHaveTextContent("false");
  });

  it("shows preview section after successful file upload", async () => {
    const mockPreviewResponse = {
      data: {
        TotalRows: 3,
        CreatedCount: 2,
        UpdatedCount: 1,
        ErrorCount: 0,
        Errors: [
          {
            RowNumber: 1,
            Data: {
              first_name: "Max",
              last_name: "Mustermann",
              school_class: "1a",
              group_name: "Gruppe A",
              birthday: "2015-01-01",
              guardians: [],
            },
            Errors: [
              {
                code: "already_exists",
                severity: "error",
                message: "Exists",
                field: "",
              },
            ],
            Timestamp: new Date().toISOString(),
          },
        ],
      },
    };

    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockPreviewResponse),
    });

    render(<StudentCSVImportPage />);

    const fileSelectButton = screen.getByTestId("file-select-trigger");
    fireEvent.click(fileSelectButton);

    await waitFor(() => {
      expect(screen.getByTestId("stats-cards")).toBeInTheDocument();
      expect(screen.getByText("Abbrechen")).toBeInTheDocument();
    });
  });

  it("resets form when cancel button is clicked", async () => {
    const mockPreviewResponse = {
      data: {
        TotalRows: 3,
        CreatedCount: 2,
        UpdatedCount: 1,
        ErrorCount: 0,
        Errors: [
          {
            RowNumber: 1,
            Data: {
              first_name: "Test",
              last_name: "Student",
              school_class: "2b",
              group_name: "",
              birthday: "2014-05-01",
              guardians: [],
            },
            Errors: [
              {
                code: "already_exists",
                severity: "error",
                message: "Exists",
                field: "",
              },
            ],
            Timestamp: new Date().toISOString(),
          },
        ],
      },
    };

    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockPreviewResponse),
    });

    render(<StudentCSVImportPage />);

    const fileSelectButton = screen.getByTestId("file-select-trigger");
    fireEvent.click(fileSelectButton);

    await waitFor(() => {
      expect(screen.getByTestId("stats-cards")).toBeInTheDocument();
    });

    const cancelButton = screen.getByText("Abbrechen");
    fireEvent.click(cancelButton);

    await waitFor(() => {
      expect(screen.queryByTestId("stats-cards")).not.toBeInTheDocument();
    });
  });

  it("handles import with preview data containing warnings", async () => {
    const mockPreviewResponse = {
      data: {
        TotalRows: 2,
        CreatedCount: 1,
        UpdatedCount: 0,
        ErrorCount: 0,
        WarningCount: 1,
        Errors: [
          {
            RowNumber: 1,
            Data: {
              first_name: "Anna",
              last_name: "Schmidt",
              school_class: "3c",
              group_name: "Gruppe B",
              birthday: "2013-03-15",
              guardians: [
                {
                  first_name: "Maria",
                  last_name: "Schmidt",
                  email: "maria@test.com",
                  phone: "123",
                  relationship_type: "mother",
                  is_primary: true,
                },
              ],
            },
            Errors: [
              {
                code: "phone_format",
                severity: "warning",
                message: "Telefon ungültig",
                field: "phone",
              },
            ],
            Timestamp: new Date().toISOString(),
          },
        ],
      },
    };

    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockPreviewResponse),
    });

    render(<StudentCSVImportPage />);

    const fileSelectButton = screen.getByTestId("file-select-trigger");
    fireEvent.click(fileSelectButton);

    await waitFor(() => {
      expect(screen.getByTestId("stats-cards")).toBeInTheDocument();
      expect(screen.getByTestId("student-row")).toBeInTheDocument();
    });
  });

  it("handles import with real errors", async () => {
    const mockPreviewResponse = {
      data: {
        TotalRows: 1,
        CreatedCount: 0,
        UpdatedCount: 0,
        ErrorCount: 1,
        Errors: [
          {
            RowNumber: 1,
            Data: {
              first_name: "",
              last_name: "Müller",
              school_class: "1a",
              group_name: "",
              birthday: "",
              guardians: [],
            },
            Errors: [
              {
                code: "required",
                severity: "error",
                message: "Vorname erforderlich",
                field: "first_name",
              },
            ],
            Timestamp: new Date().toISOString(),
          },
        ],
      },
    };

    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockPreviewResponse),
    });

    render(<StudentCSVImportPage />);

    const fileSelectButton = screen.getByTestId("file-select-trigger");
    fireEvent.click(fileSelectButton);

    await waitFor(() => {
      expect(screen.getByTestId("stat-errors")).toHaveTextContent("1");
    });
  });

  it("shows summary row when no errors in preview", async () => {
    const mockPreviewResponse = {
      data: {
        TotalRows: 5,
        CreatedCount: 5,
        UpdatedCount: 0,
        ErrorCount: 0,
        Errors: [],
      },
    };

    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockPreviewResponse),
    });

    render(<StudentCSVImportPage />);

    const fileSelectButton = screen.getByTestId("file-select-trigger");
    fireEvent.click(fileSelectButton);

    await waitFor(() => {
      expect(screen.getByTestId("student-row")).toHaveTextContent(
        "5 Schüler bereit zum Import",
      );
    });
  });

  it("handles download error gracefully", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: false,
    });

    render(<StudentCSVImportPage />);

    const downloadButton = screen.getByText("Vorlage herunterladen");
    fireEvent.click(downloadButton);

    await waitFor(() => {
      expect(screen.getByTestId("alert-error")).toBeInTheDocument();
    });
  });

  it("displays correct import button text with stats", async () => {
    const mockPreviewResponse = {
      data: {
        TotalRows: 10,
        CreatedCount: 7,
        UpdatedCount: 3,
        ErrorCount: 0,
        Errors: [],
      },
    };

    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockPreviewResponse),
    });

    render(<StudentCSVImportPage />);

    const fileSelectButton = screen.getByTestId("file-select-trigger");
    fireEvent.click(fileSelectButton);

    await waitFor(() => {
      expect(screen.getByText("7 Schüler importieren")).toBeInTheDocument();
    });
  });

  it("handles successful import and shows toast", async () => {
    // First mock for preview
    const mockPreviewResponse = {
      data: {
        TotalRows: 3,
        CreatedCount: 3,
        UpdatedCount: 0,
        ErrorCount: 0,
        Errors: [],
      },
    };

    // Second mock for actual import
    const mockImportResponse = {
      data: {
        TotalRows: 3,
        CreatedCount: 3,
        UpdatedCount: 0,
        ErrorCount: 0,
        Errors: [],
      },
    };

    (global.fetch as ReturnType<typeof vi.fn>)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockPreviewResponse),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockImportResponse),
      });

    render(<StudentCSVImportPage />);

    // Upload file
    const fileSelectButton = screen.getByTestId("file-select-trigger");
    fireEvent.click(fileSelectButton);

    await waitFor(() => {
      expect(screen.getByText("3 Schüler importieren")).toBeInTheDocument();
    });

    // Click import button
    const importButton = screen.getByText("3 Schüler importieren");
    fireEvent.click(importButton);

    await waitFor(() => {
      expect(mockToast.success).toHaveBeenCalledWith(
        "3 Schüler importiert, 0 aktualisiert",
      );
    });
  });

  it("handles partial import success with errors and shows warning toast", async () => {
    // Preview shows all rows as valid (0 errors - button will be enabled)
    const mockPreviewResponse = {
      data: {
        TotalRows: 3,
        CreatedCount: 3,
        UpdatedCount: 0,
        ErrorCount: 0,
        Errors: [],
      },
    };

    // Import returns partial success - some rows failed during actual import
    // (e.g., race condition, database constraint violation during insert)
    const mockImportResponse = {
      data: {
        TotalRows: 3,
        CreatedCount: 2,
        UpdatedCount: 0,
        ErrorCount: 1,
        Errors: [
          {
            RowNumber: 3,
            Data: {
              first_name: "Max",
              last_name: "Müller",
              school_class: "1a",
              group_name: "",
              birthday: "2015-01-01",
              guardians: [],
            },
            Errors: [
              {
                code: "duplicate",
                severity: "error",
                message: "Schüler bereits vorhanden",
                field: "",
              },
            ],
            Timestamp: new Date().toISOString(),
          },
        ],
      },
    };

    (global.fetch as ReturnType<typeof vi.fn>)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockPreviewResponse),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockImportResponse),
      });

    render(<StudentCSVImportPage />);

    // Upload file
    const fileSelectButton = screen.getByTestId("file-select-trigger");
    fireEvent.click(fileSelectButton);

    await waitFor(() => {
      expect(screen.getByText("3 Schüler importieren")).toBeInTheDocument();
    });

    // Click import button (enabled because preview showed 0 errors)
    const importButton = screen.getByText("3 Schüler importieren");
    fireEvent.click(importButton);

    await waitFor(() => {
      // Should show warning toast, not success
      expect(mockToast.warning).toHaveBeenCalledWith(
        "2 Schüler importiert, 0 aktualisiert, 1 übersprungen",
      );
      // Should NOT have called success
      expect(mockToast.success).not.toHaveBeenCalled();
    });

    // Preview should still be visible (form not reset)
    await waitFor(() => {
      expect(screen.getByTestId("stats-cards")).toBeInTheDocument();
    });
  });

  it("handles import error", async () => {
    // First mock for preview
    const mockPreviewResponse = {
      data: {
        TotalRows: 2,
        CreatedCount: 2,
        UpdatedCount: 0,
        ErrorCount: 0,
        Errors: [],
      },
    };

    (global.fetch as ReturnType<typeof vi.fn>)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockPreviewResponse),
      })
      .mockResolvedValueOnce({
        ok: false,
        json: () => Promise.resolve({ message: "Import fehlgeschlagen" }),
      });

    render(<StudentCSVImportPage />);

    // Upload file
    const fileSelectButton = screen.getByTestId("file-select-trigger");
    fireEvent.click(fileSelectButton);

    await waitFor(() => {
      expect(screen.getByText("2 Schüler importieren")).toBeInTheDocument();
    });

    // Click import button
    const importButton = screen.getByText("2 Schüler importieren");
    fireEvent.click(importButton);

    await waitFor(() => {
      expect(screen.getByText("Import fehlgeschlagen")).toBeInTheDocument();
    });
  });

  it("disables import button when there are errors", async () => {
    const mockPreviewResponse = {
      data: {
        TotalRows: 1,
        CreatedCount: 0,
        UpdatedCount: 0,
        ErrorCount: 1,
        Errors: [
          {
            RowNumber: 1,
            Data: {
              first_name: "",
              last_name: "",
              school_class: "",
              group_name: "",
              birthday: "",
              guardians: [],
            },
            Errors: [
              {
                code: "required",
                severity: "error",
                message: "Required",
                field: "first_name",
              },
            ],
            Timestamp: new Date().toISOString(),
          },
        ],
      },
    };

    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockPreviewResponse),
    });

    render(<StudentCSVImportPage />);

    const fileSelectButton = screen.getByTestId("file-select-trigger");
    fireEvent.click(fileSelectButton);

    await waitFor(() => {
      const importButton = screen.getByText("0 Schüler importieren");
      expect(importButton).toBeDisabled();
    });
  });

  it("shows loading state when session is loading", async () => {
    // Override mock for this test
    const useSession = await import("next-auth/react");
    vi.mocked(useSession.useSession).mockReturnValueOnce({
      data: null,
      status: "loading",
      update: vi.fn(),
    });

    render(<StudentCSVImportPage />);

    expect(screen.getByTestId("loading")).toBeInTheDocument();
  });

  it("handles missing token gracefully", async () => {
    // Override mock for this test
    const useSession = await import("next-auth/react");
    vi.mocked(useSession.useSession).mockReturnValueOnce({
      data: { user: { token: undefined } },
      status: "authenticated",
      update: vi.fn(),
    } as unknown as ReturnType<typeof useSession.useSession>);

    render(<StudentCSVImportPage />);

    const fileSelectButton = screen.getByTestId("file-select-trigger");
    fireEvent.click(fileSelectButton);

    await waitFor(() => {
      expect(screen.getByText("Keine Authentifizierung")).toBeInTheDocument();
    });
  });
});
