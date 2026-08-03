import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import CategoriesPage from "./page";

const mockTenantMutate = vi.hoisted(() => vi.fn(() => Promise.resolve()));

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    data: { user: { id: "1", token: "test-token" }, expires: "2099-01-01" },
    status: "authenticated",
  })),
}));

let currentSearch = new URLSearchParams();
const mockReplace = vi.fn((url: string) => {
  const query = url.includes("?") ? (url.split("?")[1] ?? "") : "";
  currentSearch = new URLSearchParams(query);
});

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  useRouter: vi.fn(() => ({ push: vi.fn(), replace: mockReplace })),
  usePathname: vi.fn(() => "/tenant/database/categories"),
  useSearchParams: () => currentSearch,
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn(),
  mutate: vi.fn(),
  useTenantMutate: vi.fn(() => mockTenantMutate),
}));

const mockCreate = vi.fn();
const mockUpdate = vi.fn();
const mockDelete = vi.fn();
vi.mock("@/lib/database/service-factory", () => ({
  createCrudService: vi.fn(() => ({
    getList: vi.fn(),
    getOne: vi.fn(),
    create: mockCreate,
    update: mockUpdate,
    delete: mockDelete,
  })),
}));

vi.mock("~/components/ui/hooks/useIsMobile", () => ({
  useIsMobile: vi.fn(() => false),
}));

const mockToastSuccess = vi.fn();
const mockToastError = vi.fn();
vi.mock("~/contexts/ToastContext", () => ({
  useToast: vi.fn(() => ({
    success: mockToastSuccess,
    error: mockToastError,
  })),
}));

vi.mock("~/components/database/database-page-layout", () => ({
  DatabasePageLayout: ({
    children,
    loading,
  }: {
    children: ReactNode;
    loading: boolean;
  }) => (
    <div data-testid="database-layout" data-loading={loading}>
      {children}
    </div>
  ),
}));

vi.mock("~/components/ui/page-header/PageHeaderWithSearch", () => ({
  PageHeaderWithSearch: ({
    search,
    actionButton,
  }: {
    search: { value: string; onChange: (value: string) => void };
    actionButton?: ReactNode;
  }) => (
    <div data-testid="page-header">
      <input
        data-testid="search-input"
        value={search.value}
        onChange={(event) => search.onChange(event.target.value)}
      />
      {actionButton}
    </div>
  ),
}));

vi.mock("~/components/ui/database/database-form-modal", () => ({
  DatabaseFormModal: ({
    isOpen,
    onSubmit,
  }: {
    isOpen: boolean;
    onSubmit: (data: { name: string }) => Promise<void>;
  }) =>
    isOpen ? (
      <div data-testid="category-create-modal">
        <button
          type="button"
          data-testid="submit-create"
          onClick={() => void onSubmit({ name: "  Essen  " })}
        >
          Submit
        </button>
      </div>
    ) : null,
}));

vi.mock("@/components/categories/categories-master-detail", () => ({
  CategoriesMasterDetail: ({
    categories,
    selectedId,
    selectedCategory,
    onSelect,
    onSaveCategory,
    onArchiveClick,
    onRestore,
  }: {
    categories: Array<{ id: string; name: string; archivedAt?: string }>;
    selectedId: string | null;
    selectedCategory?: { name: string } | null;
    onSelect: (id: string | null) => void;
    onSaveCategory: (data: { name: string }) => Promise<void>;
    onArchiveClick: () => void;
    onRestore: () => Promise<void>;
  }) => (
    <div data-testid="categories-master-detail">
      {categories.map((category) => (
        <button
          type="button"
          key={category.id}
          data-testid={`category-row-${category.id}`}
          data-archived={category.archivedAt ? "true" : "false"}
          onClick={() => onSelect(category.id)}
        >
          {category.name}
        </button>
      ))}
      {selectedId ? (
        <div data-testid="category-detail-panel">
          <span data-testid="detail-category-name">
            {selectedCategory?.name ?? "unbekannt"}
          </span>
          <button
            type="button"
            data-testid="trigger-update"
            onClick={() => void onSaveCategory({ name: "Mittagessen" })}
          >
            Save
          </button>
          <button
            type="button"
            data-testid="trigger-archive"
            onClick={onArchiveClick}
          >
            Archive
          </button>
          <button
            type="button"
            data-testid="trigger-restore"
            onClick={() => void onRestore()}
          >
            Restore
          </button>
        </div>
      ) : null}
    </div>
  ),
}));

vi.mock("~/components/ui/modal", () => ({
  ConfirmationModal: ({
    isOpen,
    onConfirm,
  }: {
    isOpen: boolean;
    onConfirm?: () => void;
  }) =>
    isOpen ? (
      <div data-testid="confirmation-modal">
        <button type="button" data-testid="confirm-archive" onClick={onConfirm}>
          Confirm
        </button>
      </div>
    ) : null,
}));

import { useSWRAuth } from "~/lib/swr";

const mockCategories = [
  {
    id: "1",
    name: "Sport",
    usageCount: 2,
    isSystem: false,
    created_at: new Date(),
    updated_at: new Date(),
  },
  {
    id: "2",
    name: "Kochen",
    usageCount: 0,
    isSystem: false,
    archivedAt: "2026-08-01T10:00:00Z",
    created_at: new Date(),
    updated_at: new Date(),
  },
];

function selectCategory(id: string) {
  currentSearch = new URLSearchParams();
  currentSearch.set("category", id);
}

describe("CategoriesPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    currentSearch = new URLSearchParams();

    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockCategories,
      isLoading: false,
      error: null,
      isValidating: false,
      mutate: vi.fn(),
    } as ReturnType<typeof useSWRAuth>);
  });

  it("lists active and archived categories", async () => {
    render(<CategoriesPage />);

    await waitFor(() => {
      expect(screen.getByText("Sport")).toBeInTheDocument();
      expect(screen.getByText("Kochen")).toBeInTheDocument();
    });
    expect(screen.getByTestId("category-row-2")).toHaveAttribute(
      "data-archived",
      "true",
    );
  });

  it("filters by search term", async () => {
    render(<CategoriesPage />);

    fireEvent.change(screen.getByTestId("search-input"), {
      target: { value: "koch" },
    });

    await waitFor(() => {
      expect(screen.queryByText("Sport")).not.toBeInTheDocument();
      expect(screen.getByText("Kochen")).toBeInTheDocument();
    });
  });

  it("trims the name before creating and refreshes the list", async () => {
    mockCreate.mockResolvedValue({ id: "3", name: "Essen" });
    render(<CategoriesPage />);

    // DatabaseCreateAction renders both the desktop button and the mobile FAB.
    fireEvent.click(screen.getAllByLabelText("Kategorie erstellen")[0]!);
    fireEvent.click(await screen.findByTestId("submit-create"));

    await waitFor(() => {
      expect(mockCreate).toHaveBeenCalledWith(
        expect.objectContaining({ name: "Essen" }),
      );
    });
    expect(mockTenantMutate).toHaveBeenCalledWith("database-categories-list");
  });

  it("updates the selected category", async () => {
    selectCategory("1");
    mockUpdate.mockResolvedValue({ id: "1", name: "Mittagessen" });
    render(<CategoriesPage />);

    fireEvent.click(await screen.findByTestId("trigger-update"));

    await waitFor(() => {
      expect(mockUpdate).toHaveBeenCalledWith(
        "1",
        expect.objectContaining({ name: "Mittagessen" }),
      );
    });
    expect(mockToastSuccess).toHaveBeenCalled();
  });

  it("archives only after confirmation", async () => {
    selectCategory("1");
    mockDelete.mockResolvedValue(null);
    render(<CategoriesPage />);

    fireEvent.click(await screen.findByTestId("trigger-archive"));
    expect(mockDelete).not.toHaveBeenCalled();

    fireEvent.click(await screen.findByTestId("confirm-archive"));

    await waitFor(() => {
      expect(mockDelete).toHaveBeenCalledWith("1");
    });
    expect(mockToastSuccess).toHaveBeenCalledWith(
      'Kategorie "Sport" wurde archiviert.',
    );
  });

  it("surfaces an archive failure as a toast instead of a success", async () => {
    selectCategory("1");
    mockDelete.mockResolvedValue("Kategorie wird noch verwendet.");
    render(<CategoriesPage />);

    fireEvent.click(await screen.findByTestId("trigger-archive"));
    fireEvent.click(await screen.findByTestId("confirm-archive"));

    await waitFor(() => {
      expect(mockToastError).toHaveBeenCalledWith(
        "Kategorie wird noch verwendet.",
      );
    });
    expect(mockToastSuccess).not.toHaveBeenCalled();
  });

  it("restores an archived category via the restore endpoint", async () => {
    selectCategory("2");
    const fetchMock = vi
      .fn()
      .mockResolvedValue({ ok: true, text: () => Promise.resolve("") });
    vi.stubGlobal("fetch", fetchMock);

    render(<CategoriesPage />);

    fireEvent.click(await screen.findByTestId("trigger-restore"));

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/activities/categories/2/restore",
        expect.objectContaining({ method: "POST" }),
      );
    });
    expect(mockToastSuccess).toHaveBeenCalledWith(
      'Kategorie "Kochen" wurde wiederhergestellt.',
    );

    vi.unstubAllGlobals();
  });

  it("shows an error when the list fails to load", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error("boom"),
      isValidating: false,
      mutate: vi.fn(),
    } as ReturnType<typeof useSWRAuth>);

    render(<CategoriesPage />);

    expect(
      await screen.findByText(
        "Fehler beim Laden der Kategorien. Bitte versuche es später erneut.",
      ),
    ).toBeInTheDocument();
  });
});
