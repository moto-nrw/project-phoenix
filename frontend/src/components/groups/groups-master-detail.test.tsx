import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

/** Text of a FormErrorInput, as the shared kit components render it. */
function errorText(error: unknown): string | null {
  if (!error) return null;
  return typeof error === "string"
    ? error
    : (error as { message: string }).message;
}

interface MockErrorPath {
  error: unknown;
  show: (error: unknown, options: { object: string }) => unknown;
  clear: () => void;
}

vi.mock("~/components/ui/hooks/useIsMobile", () => ({
  useIsMobile: vi.fn(() => false),
}));

// Mirrors DatabaseForm in production: a rejected save goes to the owner's
// error path, whose text the form renders.
vi.mock("~/components/ui/database/database-form", () => ({
  DatabaseForm: ({
    onSubmit,
    onCancel,
    errorPath,
    errorObject,
  }: {
    onSubmit: (data: { name: string }) => Promise<void>;
    onCancel: () => void;
    errorPath: MockErrorPath;
    errorObject?: string;
  }) => (
    <div data-testid="database-form">
      {errorText(errorPath.error) ? (
        <span data-testid="form-error">{errorText(errorPath.error)}</span>
      ) : null}
      <button
        type="button"
        onClick={() =>
          void onSubmit({ name: "Updated Group" }).catch((err: unknown) => {
            void errorPath.show(err, { object: errorObject ?? "" });
          })
        }
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

  it("shows a failed save with the catalog text of the group in the form", async () => {
    onSaveGroup.mockRejectedValueOnce(
      new ApiError("conflict", 409, { code: "general.business_rejection" }),
    );
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

    fireEvent.click(screen.getByText("Save"));

    expect(await screen.findByTestId("form-error")).toHaveTextContent(
      catalogText("general.business_rejection", "die Gruppe"),
    );

    // Abbrechen setzt den Entwurf und den Fehler zurück.
    fireEvent.click(screen.getByText("Cancel"));
    expect(screen.queryByTestId("form-error")).not.toBeInTheDocument();
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
