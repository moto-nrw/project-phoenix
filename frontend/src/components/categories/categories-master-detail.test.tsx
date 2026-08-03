import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { CategoriesMasterDetail } from "./categories-master-detail";
import type { ActivityCategory } from "@/lib/activity-helpers";

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

function renderDetail(
  selected: ActivityCategory | null,
  handlers: Partial<{
    onSaveCategory: (data: Partial<ActivityCategory>) => Promise<void>;
    onArchiveClick: () => void;
    onRestore: () => Promise<void>;
  }> = {},
) {
  return render(
    <CategoriesMasterDetail
      categories={[active, archived]}
      selectedId={selected?.id ?? null}
      selectedCategory={selected}
      onSelect={vi.fn()}
      onSaveCategory={handlers.onSaveCategory ?? vi.fn(async () => undefined)}
      onArchiveClick={handlers.onArchiveClick ?? vi.fn()}
      onRestore={handlers.onRestore ?? vi.fn(async () => undefined)}
    />,
  );
}

describe("CategoriesMasterDetail", () => {
  it("splits the list into active and archived groups", () => {
    renderDetail(null);

    expect(screen.getByText("Aktiv")).toBeInTheDocument();
    expect(screen.getByText("Archiviert")).toBeInTheDocument();
  });

  it("labels usage per category", () => {
    renderDetail(null);

    expect(
      screen.getByText("In 2 Aktivitäten/Terminen verwendet"),
    ).toBeInTheDocument();
    expect(screen.getByText("Noch nicht verwendet")).toBeInTheDocument();
  });

  it("offers archiving for an active category", () => {
    const onArchiveClick = vi.fn();
    renderDetail(active, { onArchiveClick });

    fireEvent.click(screen.getByRole("button", { name: "Archivieren" }));
    expect(onArchiveClick).toHaveBeenCalled();
  });

  it("offers restoring instead of editing for an archived category", async () => {
    const onRestore = vi.fn(async () => undefined);
    renderDetail(archived, { onRestore });

    // No edit form for an archived category — it must be restored first.
    expect(
      screen.getByText(/Zum Bearbeiten\s+zuerst wiederherstellen/),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Wiederherstellen" }));
    await waitFor(() => {
      expect(onRestore).toHaveBeenCalled();
    });
  });

  it("renders the edit form for an active category", () => {
    renderDetail(active);

    expect(screen.getByText("Kategoriedetails")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Speichern" }),
    ).toBeInTheDocument();
  });
});
