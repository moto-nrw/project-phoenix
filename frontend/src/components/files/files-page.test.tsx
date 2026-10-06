import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactElement } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type { FileFolder, FolderFiles, FolderOverview } from "~/lib/files-api";
import { catalogText } from "~/test/error-catalog-text";
import { FilesPage } from "./files-page";

const mutate = vi.hoisted(() => vi.fn());
const useSWRAuth = vi.hoisted(() => vi.fn());
const useSession = vi.hoisted(() => vi.fn());
const filesApi = vi.hoisted(() => ({
  deleteFile: vi.fn(),
  deleteFolder: vi.fn(),
  upload: vi.fn(),
}));

vi.mock("~/lib/files-api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/files-api")>();
  return {
    ...actual,
    filesService: {
      ...actual.filesService,
      deleteFile: filesApi.deleteFile,
      deleteFolder: filesApi.deleteFolder,
      upload: filesApi.upload,
      downloadUrl: () => "/download",
      viewUrl: () => "/view",
    },
  };
});

function renderWithToasts(ui: ReactElement) {
  return render(<ToastProvider>{ui}</ToastProvider>);
}

vi.mock("~/lib/swr", () => ({ useSWRAuth }));
vi.mock("next-auth/react", () => ({ useSession }));

const folder: FileFolder = {
  id: "1",
  name: "Konzeption",
  visibility: "all_staff",
  fileCount: 0,
  roleIds: [],
  accountIds: [],
  createdAt: "2026-08-29T10:00:00Z",
};

function renderPage(staffUploadEnabled: boolean) {
  const overview: FolderOverview = {
    folders: [folder],
    canManage: true,
    canUpload: true,
    staffUploadEnabled,
    usedBytes: 0,
    maxBytes: 1024 * 1024 * 1024,
  };
  const folderFiles: FolderFiles = { folder, files: [] };

  useSWRAuth.mockImplementation((key: string) => ({
    data: key === "files-folders" ? overview : folderFiles,
    error: undefined,
    isLoading: false,
    mutate,
  }));

  renderWithToasts(<FilesPage />);
}

describe("FilesPage upload permission summary", () => {
  beforeEach(() => {
    mutate.mockReset();
    useSWRAuth.mockReset();
    useSession.mockReturnValue({
      data: {
        user: { permissions: ["config:read", "config:update"] },
      },
    });
  });

  it("says that only the leadership uploads when team uploads are off", () => {
    renderPage(false);

    expect(screen.getByText("Dateien hochladen")).toBeInTheDocument();
    expect(screen.getByText("Nur Leitung")).toBeInTheDocument();
    expect(screen.queryByText("Nein (Einstellungen)")).not.toBeInTheDocument();
  });

  it("says that leadership and team upload when team uploads are on", () => {
    renderPage(true);

    expect(screen.getByText("Leitung und Team")).toBeInTheDocument();
    const settingsLink = screen.getByRole("link", {
      name: "Berechtigung ändern",
    });
    expect(settingsLink).toHaveAttribute(
      "href",
      "/settings?tab=operations&highlight=files.staff_upload_enabled",
    );
    expect(settingsLink).toHaveClass("text-xs", "underline");
  });

  it("does not link to settings without permission to change them", () => {
    useSession.mockReturnValue({
      data: { user: { permissions: ["files:manage"] } },
    });

    renderPage(false);

    expect(
      screen.queryByRole("link", { name: "Berechtigung ändern" }),
    ).not.toBeInTheDocument();
  });
});

// #2517: Fehler über den gemeinsamen Anzeigeweg.
describe("FilesPage errors", () => {
  beforeEach(() => {
    mutate.mockReset();
    useSWRAuth.mockReset();
    filesApi.deleteFile.mockReset();
    filesApi.deleteFolder.mockReset();
    filesApi.upload.mockReset();
    useSession.mockReturnValue({ data: { user: { permissions: [] } } });
  });

  it("shows a failed load with the catalog text and no empty state", async () => {
    useSWRAuth.mockImplementation(() => ({
      data: undefined,
      error: new ApiError("boom", 503, { code: "general.unavailable" }),
      isLoading: false,
      mutate,
    }));

    renderWithToasts(<FilesPage />);

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Dateiablage"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Noch keine Ordner")).not.toBeInTheDocument();
    expect(screen.queryByText(/boom/)).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(mutate).toHaveBeenCalled();
  });

  it("keeps a failed file delete in the dialog with the catalog text", async () => {
    const file = {
      id: "9",
      folderId: "1",
      filename: "Konzept.pdf",
      sizeBytes: 100,
      contentType: "application/pdf",
      uploadedAt: "2026-09-01T10:00:00Z",
      uploadedBy: "Anna",
      canDelete: true,
    };
    useSWRAuth.mockImplementation((key: string) => ({
      data:
        key === "files-folders"
          ? {
              folders: [{ ...folder, fileCount: 1 }],
              canManage: true,
              canUpload: true,
              staffUploadEnabled: true,
              usedBytes: 0,
              maxBytes: 0,
            }
          : { folder, files: [file] },
      error: undefined,
      isLoading: false,
      mutate,
    }));
    filesApi.deleteFile.mockRejectedValueOnce(
      new ApiError("file storage action not permitted", 403, {
        code: "general.permission",
      }),
    );

    renderWithToasts(<FilesPage />);

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Konzept.pdf" }),
    );
    fireEvent.click(await screen.findByRole("menuitem", { name: "Löschen" }));
    // Zwei Schritte: erst „Ja, löschen“, dann „Endgültig löschen“.
    fireEvent.click(await screen.findByRole("button", { name: "Ja, löschen" }));
    fireEvent.click(screen.getByRole("button", { name: "Endgültig löschen" }));
    await waitFor(() =>
      expect(filesApi.deleteFile).toHaveBeenCalledWith("1", "9"),
    );

    expect(
      await screen.findByText(
        catalogText("general.permission", "das Löschen der Datei"),
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(
      screen.queryByText(/file storage action not permitted/),
    ).not.toBeInTheDocument();
  });
});
