import {
  render,
  screen,
  fireEvent,
  waitFor,
  cleanup,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import StaffImportPage from "./page";

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

// Mock the roles service used to populate the available-roles hint. getList
// resolves with two roles; getRoleDisplayName is mocked as identity.
vi.mock("~/lib/database/service-factory", () => ({
  createCrudService: () => ({
    getList: vi.fn(() =>
      Promise.resolve({
        data: [
          { id: "1", name: "user" },
          { id: "2", name: "admin" },
        ],
      }),
    ),
  }),
}));
vi.mock("~/components/database/configs/roles.config", () => ({
  rolesConfig: {},
}));
vi.mock("~/lib/auth-helpers", () => ({
  getRoleDisplayName: (name: string) => name,
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
    <button type="button" onClick={onClick} {...props}>
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

// Mock import components (staff page uses UploadSection + StatsCards)
vi.mock("~/components/import/upload-section", () => ({
  UploadSection: ({
    onFileSelect,
    isDragging,
    isLoading,
    onDragEnter,
    onDragLeave,
    onDragOver,
    onDrop,
  }: {
    onFileSelect: (file: File) => void;
    isDragging: boolean;
    isLoading: boolean;
    onDragEnter: (e: React.DragEvent) => void;
    onDragLeave: (e: React.DragEvent) => void;
    onDragOver: (e: React.DragEvent) => void;
    onDrop: (e: React.DragEvent) => void;
  }) => (
    <div
      data-testid="upload-section"
      onDragEnter={onDragEnter}
      onDragLeave={onDragLeave}
      onDragOver={onDragOver}
      onDrop={onDrop}
    >
      <span data-testid="is-dragging">{isDragging.toString()}</span>
      <span data-testid="is-loading">{isLoading.toString()}</span>
      <button
        type="button"
        data-testid="file-select-trigger"
        onClick={() =>
          onFileSelect(new File(["test"], "test.csv", { type: "text/csv" }))
        }
      >
        Select File
      </button>
    </div>
  ),
}));

vi.mock("~/components/import/stats-cards", () => ({
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
}));

interface StaffRowData {
  first_name: string;
  last_name: string;
  email: string;
  role_name: string;
  position?: string;
}

function previewRow(
  data: StaffRowData,
  errors: { code: string; severity: string; message: string; field: string }[],
) {
  return {
    RowNumber: 1,
    Data: data,
    Errors: errors,
    Timestamp: new Date().toISOString(),
  };
}

describe("StaffImportPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
  });

  afterEach(() => {
    cleanup();
  });

  it("renders the instruction section", () => {
    render(<StaffImportPage />);

    expect(screen.getByText("Import-Anleitung")).toBeInTheDocument();
    expect(
      screen.getByText(/Füllen Sie die Datei mit Ihren Mitarbeiterdaten aus/),
    ).toBeInTheDocument();
  });

  it("renders the template download section with a single format label", () => {
    render(<StaffImportPage />);

    expect(
      screen.getByText("Schritt 1: Vorlage herunterladen"),
    ).toBeInTheDocument();
    // Regression: the alignment spacer must not duplicate this label.
    expect(screen.getByText("Format wählen")).toBeInTheDocument();
    expect(screen.getByText("Vorlage herunterladen")).toBeInTheDocument();
  });

  it("renders the upload section", () => {
    render(<StaffImportPage />);
    expect(screen.getByTestId("upload-section")).toBeInTheDocument();
  });

  it("loads and displays the available roles", async () => {
    render(<StaffImportPage />);

    await waitFor(() => {
      expect(screen.getByText("user")).toBeInTheDocument();
      expect(screen.getByText("admin")).toBeInTheDocument();
    });
  });

  it("allows selecting template format", () => {
    render(<StaffImportPage />);

    const select = screen.getByRole("combobox");
    expect(select).toHaveTextContent("Excel (.xlsx)");
    fireEvent.click(select);
    fireEvent.click(
      screen.getByRole("option", { name: "CSV (Komma-getrennt)" }),
    );
    expect(select).toHaveTextContent("CSV (Komma-getrennt)");
  });

  it("calls the teacher template endpoint on download", async () => {
    const mockBlob = new Blob(["test"], { type: "text/csv" });
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      blob: () => Promise.resolve(mockBlob),
    });
    global.URL.createObjectURL = vi.fn(() => "blob:test-url");
    global.URL.revokeObjectURL = vi.fn();

    render(<StaffImportPage />);

    fireEvent.click(screen.getByText("Vorlage herunterladen"));

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/import/teachers/template?format=xlsx",
        expect.objectContaining({
          headers: { Authorization: "Bearer test-token" },
        }),
      );
    });
  });

  it("handles download error gracefully", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: false,
    });

    render(<StaffImportPage />);
    fireEvent.click(screen.getByText("Vorlage herunterladen"));

    await waitFor(() => {
      expect(screen.getByTestId("alert-error")).toBeInTheDocument();
    });
  });

  it("posts to the teacher preview endpoint on file upload", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      json: () =>
        Promise.resolve({
          data: {
            TotalRows: 2,
            CreatedCount: 2,
            UpdatedCount: 0,
            ErrorCount: 0,
            Errors: [],
          },
        }),
    });

    render(<StaffImportPage />);
    fireEvent.click(screen.getByTestId("file-select-trigger"));

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/import/teachers/preview",
        expect.objectContaining({ method: "POST" }),
      );
    });
  });

  it("displays error when preview fails", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: false,
      json: () => Promise.resolve({ message: "API Error" }),
    });

    render(<StaffImportPage />);
    fireEvent.click(screen.getByTestId("file-select-trigger"));

    await waitFor(() => {
      expect(screen.getByTestId("alert-error")).toBeInTheDocument();
      expect(screen.getByText("API Error")).toBeInTheDocument();
    });
  });

  it("allows closing the error alert", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: false,
      json: () => Promise.resolve({ message: "Test Error" }),
    });

    render(<StaffImportPage />);
    fireEvent.click(screen.getByTestId("file-select-trigger"));

    await waitFor(() => {
      expect(screen.getByTestId("alert-error")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByLabelText("Fehler schließen"));
    expect(screen.queryByTestId("alert-error")).not.toBeInTheDocument();
  });

  it("shows a summary row when the preview has no issues", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      json: () =>
        Promise.resolve({
          data: {
            TotalRows: 5,
            CreatedCount: 5,
            UpdatedCount: 0,
            ErrorCount: 0,
            Errors: [],
          },
        }),
    });

    render(<StaffImportPage />);
    fireEvent.click(screen.getByTestId("file-select-trigger"));

    await waitFor(() => {
      expect(
        screen.getByText("5 Mitarbeiter bereit zum Import"),
      ).toBeInTheDocument();
    });
  });

  it("shows the invite button label with the new count", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      json: () =>
        Promise.resolve({
          data: {
            TotalRows: 4,
            CreatedCount: 4,
            UpdatedCount: 0,
            ErrorCount: 0,
            Errors: [],
          },
        }),
    });

    render(<StaffImportPage />);
    fireEvent.click(screen.getByTestId("file-select-trigger"));

    await waitFor(() => {
      expect(screen.getByText("4 Mitarbeiter einladen")).toBeInTheDocument();
    });
  });

  it("renders a preview row with name, email and role", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      json: () =>
        Promise.resolve({
          data: {
            TotalRows: 1,
            CreatedCount: 1,
            UpdatedCount: 0,
            ErrorCount: 0,
            WarningCount: 0,
            Errors: [
              previewRow(
                {
                  first_name: "Anna",
                  last_name: "Lehmann",
                  email: "anna@example.com",
                  role_name: "user",
                  position: "Klassenlehrerin",
                },
                [
                  {
                    code: "x",
                    severity: "warning",
                    message: "Hinweis",
                    field: "role",
                  },
                ],
              ),
            ],
          },
        }),
    });

    render(<StaffImportPage />);
    fireEvent.click(screen.getByTestId("file-select-trigger"));

    await waitFor(() => {
      expect(screen.getByText("Anna Lehmann")).toBeInTheDocument();
      expect(screen.getByText("anna@example.com")).toBeInTheDocument();
    });
  });

  it("disables the invite button when there are errors", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      json: () =>
        Promise.resolve({
          data: {
            TotalRows: 1,
            CreatedCount: 0,
            UpdatedCount: 0,
            ErrorCount: 1,
            Errors: [
              previewRow(
                {
                  first_name: "Bernd",
                  last_name: "Falsch",
                  email: "bernd@example.com",
                  role_name: "GibtsNicht",
                },
                [
                  {
                    code: "role_not_found",
                    severity: "error",
                    message: "Rolle nicht gefunden",
                    field: "role",
                  },
                ],
              ),
            ],
          },
        }),
    });

    render(<StaffImportPage />);
    fireEvent.click(screen.getByTestId("file-select-trigger"));

    await waitFor(() => {
      const inviteButton = screen.getByText("0 Mitarbeiter einladen");
      expect(inviteButton).toBeDisabled();
    });
  });

  it("resets the form on cancel", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      json: () =>
        Promise.resolve({
          data: {
            TotalRows: 3,
            CreatedCount: 3,
            UpdatedCount: 0,
            ErrorCount: 0,
            Errors: [],
          },
        }),
    });

    render(<StaffImportPage />);
    fireEvent.click(screen.getByTestId("file-select-trigger"));

    await waitFor(() => {
      expect(screen.getByTestId("stats-cards")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Abbrechen"));

    await waitFor(() => {
      expect(screen.queryByTestId("stats-cards")).not.toBeInTheDocument();
    });
  });

  it("invites staff and shows a success toast", async () => {
    (global.fetch as ReturnType<typeof vi.fn>)
      .mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            data: {
              TotalRows: 3,
              CreatedCount: 3,
              UpdatedCount: 0,
              ErrorCount: 0,
              Errors: [],
            },
          }),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            data: {
              TotalRows: 3,
              CreatedCount: 3,
              UpdatedCount: 0,
              ErrorCount: 0,
              Errors: [],
            },
          }),
      });

    render(<StaffImportPage />);
    fireEvent.click(screen.getByTestId("file-select-trigger"));

    await waitFor(() => {
      expect(screen.getByText("3 Mitarbeiter einladen")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("3 Mitarbeiter einladen"));

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/import/teachers/import",
        expect.objectContaining({ method: "POST" }),
      );
      expect(mockToast.success).toHaveBeenCalledWith(
        "3 Mitarbeiter eingeladen",
      );
    });
  });

  it("shows a warning toast on partial import", async () => {
    (global.fetch as ReturnType<typeof vi.fn>)
      .mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            data: {
              TotalRows: 3,
              CreatedCount: 3,
              UpdatedCount: 0,
              ErrorCount: 0,
              Errors: [],
            },
          }),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            data: {
              TotalRows: 3,
              CreatedCount: 2,
              UpdatedCount: 0,
              ErrorCount: 1,
              Errors: [
                previewRow(
                  {
                    first_name: "Max",
                    last_name: "Müller",
                    email: "max@example.com",
                    role_name: "user",
                  },
                  [
                    {
                      code: "already_exists",
                      severity: "error",
                      message: "Existiert",
                      field: "email",
                    },
                  ],
                ),
              ],
            },
          }),
      });

    render(<StaffImportPage />);
    fireEvent.click(screen.getByTestId("file-select-trigger"));

    await waitFor(() => {
      expect(screen.getByText("3 Mitarbeiter einladen")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("3 Mitarbeiter einladen"));

    await waitFor(() => {
      expect(mockToast.warning).toHaveBeenCalledWith(
        "2 eingeladen, 1 übersprungen",
      );
      expect(mockToast.success).not.toHaveBeenCalled();
    });
  });

  it("shows an error when the import call fails", async () => {
    (global.fetch as ReturnType<typeof vi.fn>)
      .mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            data: {
              TotalRows: 2,
              CreatedCount: 2,
              UpdatedCount: 0,
              ErrorCount: 0,
              Errors: [],
            },
          }),
      })
      .mockResolvedValueOnce({
        ok: false,
        json: () => Promise.resolve({ message: "Import fehlgeschlagen" }),
      });

    render(<StaffImportPage />);
    fireEvent.click(screen.getByTestId("file-select-trigger"));

    await waitFor(() => {
      expect(screen.getByText("2 Mitarbeiter einladen")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("2 Mitarbeiter einladen"));

    await waitFor(() => {
      expect(screen.getByText("Import fehlgeschlagen")).toBeInTheDocument();
    });
  });

  it("toggles isDragging on dragEnter/dragLeave", () => {
    render(<StaffImportPage />);

    const uploadSection = screen.getByTestId("upload-section");
    expect(screen.getByTestId("is-dragging")).toHaveTextContent("false");

    fireEvent.dragEnter(uploadSection);
    expect(screen.getByTestId("is-dragging")).toHaveTextContent("true");

    fireEvent.dragLeave(uploadSection);
    expect(screen.getByTestId("is-dragging")).toHaveTextContent("false");
  });

  it("handles a valid file drop", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      json: () =>
        Promise.resolve({
          data: {
            TotalRows: 1,
            CreatedCount: 1,
            UpdatedCount: 0,
            ErrorCount: 0,
            Errors: [],
          },
        }),
    });

    render(<StaffImportPage />);

    const uploadSection = screen.getByTestId("upload-section");
    const csvFile = new File(["test"], "staff.csv", { type: "text/csv" });
    const dropEvent = new Event("drop", { bubbles: true });
    Object.defineProperty(dropEvent, "preventDefault", { value: vi.fn() });
    Object.defineProperty(dropEvent, "stopPropagation", { value: vi.fn() });
    Object.defineProperty(dropEvent, "dataTransfer", {
      value: { files: [csvFile] },
    });
    uploadSection.dispatchEvent(dropEvent);

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/import/teachers/preview",
        expect.objectContaining({ method: "POST" }),
      );
    });
  });

  it("rejects an invalid file drop", async () => {
    render(<StaffImportPage />);

    const uploadSection = screen.getByTestId("upload-section");
    const invalidFile = new File(["test"], "image.png", { type: "image/png" });
    const dropEvent = new Event("drop", { bubbles: true });
    Object.defineProperty(dropEvent, "preventDefault", { value: vi.fn() });
    Object.defineProperty(dropEvent, "stopPropagation", { value: vi.fn() });
    Object.defineProperty(dropEvent, "dataTransfer", {
      value: { files: [invalidFile] },
    });
    uploadSection.dispatchEvent(dropEvent);

    await waitFor(() => {
      expect(
        screen.getByText(
          "Bitte nur CSV- oder Excel-Dateien (.csv, .xlsx) hochladen",
        ),
      ).toBeInTheDocument();
    });
  });

  it("shows loading state while the session loads", async () => {
    const useSession = await import("next-auth/react");
    vi.mocked(useSession.useSession).mockReturnValueOnce({
      data: null,
      status: "loading",
      update: vi.fn(),
    });

    render(<StaffImportPage />);
    expect(screen.getByTestId("loading")).toBeInTheDocument();
  });

  it("handles a missing token gracefully", async () => {
    const useSession = await import("next-auth/react");
    vi.mocked(useSession.useSession).mockReturnValueOnce({
      data: { user: { token: undefined } },
      status: "authenticated",
      update: vi.fn(),
    } as unknown as ReturnType<typeof useSession.useSession>);

    render(<StaffImportPage />);
    fireEvent.click(screen.getByTestId("file-select-trigger"));

    await waitFor(() => {
      expect(screen.getByText("Keine Authentifizierung")).toBeInTheDocument();
    });
  });
});
