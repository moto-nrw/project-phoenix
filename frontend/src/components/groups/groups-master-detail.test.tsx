import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("~/components/ui/hooks/useIsMobile", () => ({
  useIsMobile: vi.fn(() => false),
}));

vi.mock("~/components/ui/database/database-form", () => ({
  DatabaseForm: ({
    onSubmit,
    onCancel,
  }: {
    onSubmit: (data: { name: string }) => Promise<void>;
    onCancel: () => void;
  }) => (
    <div data-testid="database-form">
      <button
        type="button"
        onClick={() => void onSubmit({ name: "Updated Group" })}
      >
        Save
      </button>
      <button type="button" onClick={onCancel}>
        Cancel
      </button>
    </div>
  ),
}));

class MockResizeObserver {
  observe = vi.fn();
  unobserve = vi.fn();
  disconnect = vi.fn();
}

vi.stubGlobal("ResizeObserver", MockResizeObserver);

import { GroupsMasterDetail } from "./groups-master-detail";
import type { Group } from "@/lib/group-helpers";

const baseGroup: Group = {
  id: "1",
  name: "Gruppe Rot",
  room_name: "Raum 101",
  student_count: 12,
  supervisors: [
    { id: "1", name: "Frau Müller" },
    { id: "2", name: "Herr Schmidt" },
  ],
};

const groupWithoutRoom: Group = {
  id: "2",
  name: "Gruppe Blau",
  student_count: 0,
};

describe("GroupsMasterDetail", () => {
  const onSelect = vi.fn();
  const onSaveGroup = vi.fn();
  const onDeleteClick = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows the empty detail state when nothing is selected", () => {
    render(
      <GroupsMasterDetail
        groups={[baseGroup]}
        selectedId={null}
        selectedGroup={null}
        onSelect={onSelect}
        onSaveGroup={onSaveGroup}
        onDeleteClick={onDeleteClick}
      />,
    );

    expect(screen.queryByTestId("database-form")).not.toBeInTheDocument();
    expect(screen.getByText("Gruppe Rot")).toBeInTheDocument();
  });

  it("renders the selected group detail form and save path", () => {
    render(
      <GroupsMasterDetail
        groups={[baseGroup]}
        selectedId="1"
        selectedGroup={baseGroup}
        onSelect={onSelect}
        onSaveGroup={onSaveGroup}
        onDeleteClick={onDeleteClick}
      />,
    );

    expect(screen.getAllByText("Gruppe Rot")).toHaveLength(2);
    expect(screen.getByTestId("database-form")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Save"));
    expect(onSaveGroup).toHaveBeenCalledWith({ name: "Updated Group" });
  });

  it("renders supervisors joined as the Gruppenleitung field", () => {
    render(
      <GroupsMasterDetail
        groups={[baseGroup]}
        selectedId="1"
        selectedGroup={baseGroup}
        onSelect={onSelect}
        onSaveGroup={onSaveGroup}
        onDeleteClick={onDeleteClick}
      />,
    );

    expect(screen.getByText("Gruppenleitung")).toBeInTheDocument();
    expect(screen.getByText("Frau Müller, Herr Schmidt")).toBeInTheDocument();
  });

  it("renders the placeholder when no supervisors are assigned", () => {
    render(
      <GroupsMasterDetail
        groups={[groupWithoutRoom]}
        selectedId="2"
        selectedGroup={groupWithoutRoom}
        onSelect={onSelect}
        onSaveGroup={onSaveGroup}
        onDeleteClick={onDeleteClick}
      />,
    );

    expect(
      screen.getByText("Keine Gruppenleitung zugewiesen"),
    ).toBeInTheDocument();
    expect(screen.getAllByText("Kein Gruppenraum")[0]).toBeInTheDocument();
  });

  it("passes delete clicks through", () => {
    render(
      <GroupsMasterDetail
        groups={[baseGroup]}
        selectedId="1"
        selectedGroup={baseGroup}
        onSelect={onSelect}
        onSaveGroup={onSaveGroup}
        onDeleteClick={onDeleteClick}
      />,
    );

    fireEvent.click(screen.getByText("Löschen"));
    expect(onDeleteClick).toHaveBeenCalled();
  });

  it("groups groups under a single 'Alle Gruppen' bucket and renders a Stammdaten tab", () => {
    render(
      <GroupsMasterDetail
        groups={[baseGroup, groupWithoutRoom]}
        selectedId="1"
        selectedGroup={baseGroup}
        onSelect={onSelect}
        onSaveGroup={onSaveGroup}
        onDeleteClick={onDeleteClick}
      />,
    );

    expect(screen.getByText("Alle Gruppen")).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Stammdaten" })).toBeInTheDocument();
  });
});
