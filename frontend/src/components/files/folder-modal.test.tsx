import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";
import { FolderModal } from "./folder-modal";

const filesApi = vi.hoisted(() => ({
  createFolder: vi.fn(),
  updateFolder: vi.fn(),
  listAudience: vi.fn(),
}));
const audienceState = vi.hoisted(() => ({
  error: undefined as unknown,
  mutate: vi.fn(),
}));

vi.mock("~/lib/files-api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/files-api")>();
  return {
    ...actual,
    filesService: { ...actual.filesService, ...filesApi },
  };
});

vi.mock("~/lib/swr", () => ({
  useSWRAuth: () => ({
    data: audienceState.error ? undefined : { roles: [], accounts: [] },
    error: audienceState.error,
    isLoading: false,
    mutate: audienceState.mutate,
  }),
}));

function renderModal() {
  const onSaved = vi.fn();
  const onClose = vi.fn();
  render(
    <ToastProvider>
      <FolderModal isOpen initial={null} onSaved={onSaved} onClose={onClose} />
    </ToastProvider>,
  );
  return { onSaved, onClose };
}

// #2517: Speicherfehler im Dialog, Prüfungen am Feld.
describe("FolderModal errors", () => {
  beforeEach(() => {
    filesApi.createFolder.mockReset();
    audienceState.error = undefined;
    audienceState.mutate.mockReset();
  });

  it("marks a missing name at the field before sending", async () => {
    renderModal();

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText("Bitte geben Sie einen Namen ein."),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Name")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
    expect(filesApi.createFolder).not.toHaveBeenCalled();
  });

  it("keeps a failed save in the dialog and retries with the current name", async () => {
    filesApi.createFolder
      .mockRejectedValueOnce(
        new ApiError("folder name already exists", 409, {
          code: "files.folder_name_taken",
        }),
      )
      .mockResolvedValueOnce({ id: "4" });
    const { onSaved, onClose } = renderModal();

    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: "Formulare" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        catalogText("files.folder_name_taken", "das Speichern des Ordners"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/already exists/)).not.toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: "Formulare 2026" },
    });
    // files.folder_name_taken is a business rejection: no retry, a new save.
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(filesApi.createFolder).toHaveBeenLastCalledWith(
        expect.objectContaining({ name: "Formulare 2026" }),
      ),
    );
    await waitFor(() => expect(onClose).toHaveBeenCalledOnce());
    expect(onSaved).toHaveBeenCalledOnce();
  });

  it("shows a failed audience load inside the share section", async () => {
    audienceState.error = new ApiError("down", 500, { code: "general.server" });
    renderModal();

    fireEvent.click(screen.getByRole("button", { name: "Ausgewählt" }));

    expect(
      await screen.findByText(
        catalogText("general.server", "die Liste der Rollen und Personen"),
      ),
    ).toBeInTheDocument();
  });
});
