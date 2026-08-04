import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";

const { create, update, reorder, archive, restore } = vi.hoisted(() => ({
  create: vi.fn(),
  update: vi.fn(),
  reorder: vi.fn(),
  archive: vi.fn(),
  restore: vi.fn(),
}));

vi.mock("~/lib/planning-track-api", () => ({
  planningTrackService: { create, update, reorder, archive, restore },
}));

import { PlanningTrackSelect } from "./planning-track-select";

const tracks = [
  { id: "1", name: "Jahrgang 1", color: "#5080D8", sortOrder: 0 },
  { id: "2", name: "Jahrgang 2", color: "#83CD2D", sortOrder: 1 },
  {
    id: "3",
    name: "Archiv",
    color: "#F78C10",
    sortOrder: 2,
    archivedAt: "2026-08-03T12:00:00Z",
  },
];

function renderSelect() {
  const onChange = vi.fn();
  const onTracksChanged = vi.fn();
  render(
    <ToastProvider>
      <PlanningTrackSelect
        value=""
        tracks={tracks}
        onChange={onChange}
        onTracksChanged={onTracksChanged}
        canManage
      />
    </ToastProvider>,
  );
  return { onChange, onTracksChanged };
}

describe("PlanningTrackSelect", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    reorder.mockResolvedValue([tracks[1], tracks[0], tracks[2]]);
    archive.mockResolvedValue(undefined);
    restore.mockResolvedValue({ ...tracks[2], archivedAt: undefined });
  });

  it("creates the name entered by the user and selects the result", async () => {
    const created = {
      id: "4",
      name: "Nord",
      color: "#83CD2D",
      sortOrder: 2,
    };
    create.mockResolvedValue(created);
    const { onChange, onTracksChanged } = renderSelect();

    fireEvent.click(screen.getByRole("combobox", { name: "Planungsspur" }));
    fireEvent.change(
      screen.getByPlaceholderText("Planungsspur suchen oder anlegen …"),
      { target: { value: "Nord" } },
    );
    fireEvent.click(
      screen.getByRole("option", {
        name: "„Nord“ als Planungsspur anlegen",
      }),
    );

    expect(screen.getByLabelText("Name")).toHaveValue("Nord");
    fireEvent.change(screen.getByLabelText("Farbe"), {
      target: { value: "#83cd2d" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Anlegen" }));

    await waitFor(() =>
      expect(create).toHaveBeenCalledWith({
        name: "Nord",
        color: "#83CD2D",
        sort_order: 2,
      }),
    );
    expect(onChange).toHaveBeenCalledWith("4");
    expect(onTracksChanged).toHaveBeenCalledWith(created);
  });

  it("keeps the form open when creating a track fails", async () => {
    create.mockRejectedValue(
      new Error("Planungsspur konnte nicht angelegt werden"),
    );
    const { onChange, onTracksChanged } = renderSelect();

    fireEvent.click(screen.getByRole("combobox", { name: "Planungsspur" }));
    fireEvent.change(
      screen.getByPlaceholderText("Planungsspur suchen oder anlegen …"),
      { target: { value: "Nord" } },
    );
    fireEvent.click(
      screen.getByRole("option", {
        name: "„Nord“ als Planungsspur anlegen",
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Anlegen" }));

    expect(
      await screen.findByText("Planungsspur konnte nicht angelegt werden"),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Name")).toHaveValue("Nord");
    expect(onChange).not.toHaveBeenCalled();
    expect(onTracksChanged).not.toHaveBeenCalled();
  });

  it("manages ordering and archiving inside the same popover", async () => {
    const { onTracksChanged } = renderSelect();

    fireEvent.click(screen.getByRole("combobox", { name: "Planungsspur" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Planungsspuren verwalten" }),
    );

    expect(
      screen.getByRole("button", { name: "Jahrgang 1 nach unten" }),
    ).toBeEnabled();
    fireEvent.click(
      screen.getByRole("button", { name: "Jahrgang 1 nach unten" }),
    );
    await waitFor(() => expect(reorder).toHaveBeenCalledWith(["2", "1"]));

    fireEvent.click(
      screen.getByRole("button", { name: "Jahrgang 1 archivieren" }),
    );
    await waitFor(() => expect(archive).toHaveBeenCalledWith("1"));
    expect(onTracksChanged).toHaveBeenCalled();
    expect(
      screen.queryByRole("heading", { name: "Planungsspur archivieren?" }),
    ).not.toBeInTheDocument();
  });

  it("keeps archived tracks out of selection and restores them in management", async () => {
    renderSelect();

    fireEvent.click(screen.getByRole("combobox", { name: "Planungsspur" }));
    expect(
      screen.queryByRole("option", { name: /Archiv/ }),
    ).not.toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Planungsspuren verwalten" }),
    );
    fireEvent.click(screen.getByText("Archivierte Planungsspuren (1)"));
    fireEvent.click(screen.getByRole("button", { name: "Wiederherstellen" }));

    await waitFor(() => expect(restore).toHaveBeenCalledWith("3"));
  });

  it("closes only the popover on Escape and restores trigger focus", () => {
    renderSelect();
    const trigger = screen.getByRole("combobox", { name: "Planungsspur" });

    fireEvent.click(trigger);
    fireEvent.keyDown(
      screen.getByPlaceholderText("Planungsspur suchen oder anlegen …"),
      { key: "Escape" },
    );

    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(trigger).toHaveFocus();
    expect(
      screen.queryByPlaceholderText("Planungsspur suchen oder anlegen …"),
    ).not.toBeInTheDocument();
  });
});
