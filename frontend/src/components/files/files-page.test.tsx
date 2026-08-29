import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { FileFolder, FolderFiles, FolderOverview } from "~/lib/files-api";
import { FilesPage } from "./files-page";

const mutate = vi.hoisted(() => vi.fn());
const useSWRAuth = vi.hoisted(() => vi.fn());
const useSession = vi.hoisted(() => vi.fn());

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

  render(<FilesPage />);
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
    expect(
      screen.getByRole("link", { name: "Berechtigung ändern" }),
    ).toHaveAttribute(
      "href",
      "/settings?tab=operations&highlight=files.staff_upload_enabled",
    );
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
