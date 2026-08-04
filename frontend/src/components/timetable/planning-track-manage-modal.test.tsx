import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { list, create, update, reorder, archive, restore } = vi.hoisted(() => ({
  list: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  reorder: vi.fn(),
  archive: vi.fn(),
  restore: vi.fn(),
}));

vi.mock("~/lib/planning-track-api", () => ({
  planningTrackService: { list, create, update, reorder, archive, restore },
}));

import { PlanningTrackManageModal } from "./planning-track-manage-modal";

const tracks = [
  { id: "1", name: "Früh", color: "#5080D8", sortOrder: 0 },
  { id: "2", name: "Mittag", color: "#F78C10", sortOrder: 1 },
  {
    id: "3",
    name: "Alt",
    color: "#83CD2D",
    sortOrder: 2,
    archivedAt: "2026-08-03T12:00:00Z",
  },
];

describe("PlanningTrackManageModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    list.mockResolvedValue(tracks);
    reorder.mockResolvedValue([tracks[1], tracks[0], tracks[2]]);
    restore.mockResolvedValue({ ...tracks[2], archivedAt: undefined });
  });

  it("exposes named controls for ordering, editing, archiving, and restoring", async () => {
    render(
      <PlanningTrackManageModal isOpen onClose={vi.fn()} onChanged={vi.fn()} />,
    );

    expect(await screen.findByText("Früh")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Früh nach unten" }),
    ).toBeEnabled();
    expect(
      screen.getByRole("button", { name: "Früh bearbeiten" }),
    ).toBeEnabled();
    expect(screen.getAllByRole("button", { name: "Archivieren" })).toHaveLength(
      2,
    );
    expect(
      screen.getByRole("button", { name: "Neue Planungsspur" }),
    ).toHaveClass("px-4", "py-2", "text-sm");
    expect(screen.getByRole("button", { name: "Schließen" })).toHaveClass(
      "px-4",
      "py-2",
      "text-sm",
    );

    fireEvent.click(screen.getByRole("button", { name: "Früh nach unten" }));
    await waitFor(() => expect(reorder).toHaveBeenCalledWith(["2", "1"]));

    fireEvent.click(screen.getByRole("button", { name: "Wiederherstellen" }));
    await waitFor(() => expect(restore).toHaveBeenCalledWith("3"));
  });

  it("creates a track with normalized color and selects it in the caller", async () => {
    const created = {
      id: "4",
      name: "Nord",
      color: "#83CD2D",
      sortOrder: 2,
    };
    create.mockResolvedValue(created);
    const onChanged = vi.fn();
    render(
      <PlanningTrackManageModal
        isOpen
        initialView="create"
        onClose={vi.fn()}
        onChanged={onChanged}
      />,
    );

    await screen.findByRole("heading", { name: "Neue Planungsspur" });
    expect(screen.getByRole("button", { name: "Abbrechen" })).toHaveClass(
      "px-4",
      "py-2",
      "text-sm",
    );
    expect(screen.getByRole("button", { name: "Speichern" })).toHaveClass(
      "px-4",
      "py-2",
      "text-sm",
    );
    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: " Nord " },
    });
    fireEvent.change(screen.getByLabelText("Farbe"), {
      target: { value: "#83cd2d" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(create).toHaveBeenCalledWith({
        name: "Nord",
        color: "#83CD2D",
        sort_order: 2,
      }),
    );
    expect(onChanged).toHaveBeenCalledWith(created);
  });
});
