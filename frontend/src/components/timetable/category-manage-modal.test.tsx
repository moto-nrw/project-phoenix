import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { CategoryManageModal } from "./category-manage-modal";
import type { ActivityCategory } from "~/lib/activity-helpers";

const {
  mockGetManagedCategories,
  mockCreateCategory,
  mockUpdateCategory,
  mockArchiveCategory,
  mockRestoreCategory,
} = vi.hoisted(() => ({
  mockGetManagedCategories: vi.fn(),
  mockCreateCategory: vi.fn(),
  mockUpdateCategory: vi.fn(),
  mockArchiveCategory: vi.fn(),
  mockRestoreCategory: vi.fn(),
}));

vi.mock("~/lib/category-api", () => ({
  CategoryApiError: class CategoryApiError extends Error {
    status = 409;
    detail = "";
  },
  categoryService: {
    getManagedCategories: mockGetManagedCategories,
    createCategory: mockCreateCategory,
    updateCategory: mockUpdateCategory,
    archiveCategory: mockArchiveCategory,
    restoreCategory: mockRestoreCategory,
  },
}));

function makeCategory(
  overrides: Partial<ActivityCategory> & Pick<ActivityCategory, "id" | "name">,
): ActivityCategory {
  return {
    created_at: new Date("2026-08-01T10:00:00Z"),
    updated_at: new Date("2026-08-01T10:00:00Z"),
    isSystem: false,
    ...overrides,
  };
}

const active = makeCategory({ id: "1", name: "Sport", usageCount: 2 });
const archived = makeCategory({
  id: "2",
  name: "Kochen",
  usageCount: 0,
  archivedAt: "2026-08-01T10:00:00Z",
});

describe("CategoryManageModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetManagedCategories.mockResolvedValue([active, archived]);
  });

  it("lists active and archived categories with usage", async () => {
    render(
      <CategoryManageModal isOpen onClose={vi.fn()} onChanged={vi.fn()} />,
    );

    expect(await screen.findByText("Sport")).toBeInTheDocument();
    expect(screen.getByText("Archiviert")).toBeInTheDocument();
    expect(screen.getByText("Kochen")).toBeInTheDocument();
    expect(
      screen.getByText("In 2 Terminen/Aktivitäten verwendet"),
    ).toBeInTheDocument();
    // The manage-screen client is what opts into archived rows and usage
    // counts; without it nothing could restore an archived category.
    expect(mockGetManagedCategories).toHaveBeenCalled();
  });

  it("opens straight in the create form when asked", async () => {
    render(
      <CategoryManageModal
        isOpen
        initialView="create"
        onClose={vi.fn()}
        onChanged={vi.fn()}
      />,
    );

    expect(await screen.findByText("Neue Kategorie")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Anlegen und auswählen" }),
    ).toBeInTheDocument();
  });

  it("reports the created category so the form can select it", async () => {
    const created = makeCategory({ id: "9", name: "Essen" });
    mockCreateCategory.mockResolvedValue(created);
    const onChanged = vi.fn();

    render(
      <CategoryManageModal
        isOpen
        initialView="create"
        onClose={vi.fn()}
        onChanged={onChanged}
      />,
    );

    fireEvent.change(await screen.findByLabelText(/Name/), {
      target: { value: "  Essen  " },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Anlegen und auswählen" }),
    );

    await waitFor(() => {
      expect(mockCreateCategory).toHaveBeenCalledWith(
        expect.objectContaining({ name: "Essen" }),
      );
    });
    expect(onChanged).toHaveBeenCalledWith(created);
  });

  it("refuses to save an empty name without calling the API", async () => {
    render(
      <CategoryManageModal
        isOpen
        initialView="create"
        onClose={vi.fn()}
        onChanged={vi.fn()}
      />,
    );

    fireEvent.change(await screen.findByLabelText(/Name/), {
      target: { value: "   " },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Anlegen und auswählen" }),
    );

    expect(
      await screen.findByText("Name ist erforderlich"),
    ).toBeInTheDocument();
    expect(mockCreateCategory).not.toHaveBeenCalled();
  });

  it("surfaces a backend name conflict", async () => {
    mockCreateCategory.mockRejectedValue(
      new Error("Eine Kategorie mit diesem Namen existiert bereits"),
    );

    render(
      <CategoryManageModal
        isOpen
        initialView="create"
        onClose={vi.fn()}
        onChanged={vi.fn()}
      />,
    );

    fireEvent.change(await screen.findByLabelText(/Name/), {
      target: { value: "Sport" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Anlegen und auswählen" }),
    );

    expect(
      await screen.findByText(
        "Eine Kategorie mit diesem Namen existiert bereits",
      ),
    ).toBeInTheDocument();
  });

  it("archives only after confirmation", async () => {
    mockArchiveCategory.mockResolvedValue(undefined);
    render(
      <CategoryManageModal isOpen onClose={vi.fn()} onChanged={vi.fn()} />,
    );

    fireEvent.click(await screen.findByRole("button", { name: "Archivieren" }));
    expect(mockArchiveCategory).not.toHaveBeenCalled();

    // The confirmation dialog repeats the label on its confirm button.
    const confirmButtons = await screen.findAllByRole("button", {
      name: "Archivieren",
    });
    fireEvent.click(confirmButtons[confirmButtons.length - 1]!);

    await waitFor(() => {
      expect(mockArchiveCategory).toHaveBeenCalledWith("1");
    });
  });

  it("restores an archived category", async () => {
    mockRestoreCategory.mockResolvedValue(archived);
    const onChanged = vi.fn();
    render(
      <CategoryManageModal isOpen onClose={vi.fn()} onChanged={onChanged} />,
    );

    fireEvent.click(
      await screen.findByRole("button", { name: /Wiederherstellen/ }),
    );

    await waitFor(() => {
      expect(mockRestoreCategory).toHaveBeenCalledWith("2");
    });
    // No `created` argument — nothing to auto-select on a restore.
    expect(onChanged).toHaveBeenCalledWith();
  });

  it("shows a load error instead of an empty list", async () => {
    mockGetManagedCategories.mockRejectedValue(new Error("boom"));
    render(
      <CategoryManageModal isOpen onClose={vi.fn()} onChanged={vi.fn()} />,
    );

    expect(
      await screen.findByText(/Kategorien konnten nicht geladen werden/),
    ).toBeInTheDocument();
  });
});
