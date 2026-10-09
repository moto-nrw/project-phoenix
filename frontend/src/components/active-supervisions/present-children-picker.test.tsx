import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { PresentChildrenPicker } from "./present-children-picker";

const mocks = vi.hoisted(() => ({
  fetchStudents: vi.fn(),
}));

vi.mock("~/lib/student-api", () => ({
  fetchStudents: mocks.fetchStudents,
}));

vi.mock("~/components/ui/form-modal", () => ({
  FormModal: ({
    isOpen,
    children,
    footer,
    title,
  }: {
    isOpen: boolean;
    children: React.ReactNode;
    footer?: React.ReactNode;
    title: string;
  }) =>
    isOpen ? (
      <div data-testid="modal" data-title={title}>
        {children}
        {footer}
      </div>
    ) : null,
}));

// The shared test clock stands at 12:00 Berlin.
const presentStudents = [
  {
    id: 1,
    name: "Anna Abel",
    school_class: "2a",
    group_name: "Bären",
    current_location: "Anwesend - Turnhalle",
    pickup_time: "16:00",
  },
  {
    id: 2,
    name: "Ben Berg",
    school_class: "3b",
    current_location: "Anwesend",
    pickup_time: "11:45",
  },
  {
    id: 3,
    name: "Cem Celik",
    school_class: "1c",
    current_location: "Unterwegs",
  },
  {
    id: 4,
    name: "Dana Dorn",
    school_class: "4a",
    current_location: "Anwesend - Raum 4",
    pickup_time: "15:00",
  },
];

function renderPicker(
  overrides: Partial<React.ComponentProps<typeof PresentChildrenPicker>> = {},
) {
  const onAdd = overrides.onAdd ?? vi.fn().mockResolvedValue(true);
  const onClose = overrides.onClose ?? vi.fn();
  render(
    <PresentChildrenPicker
      isOpen
      instanceId="70"
      inBlockStudentIds={new Set(["4"])}
      isAdding={false}
      error={null}
      {...overrides}
      onAdd={onAdd}
      onClose={onClose}
    />,
  );
  return { onAdd, onClose };
}

describe("PresentChildrenPicker", () => {
  beforeEach(() => {
    mocks.fetchStudents.mockReset();
    mocks.fetchStudents.mockResolvedValue({ students: presentStudents });
  });

  it("loads the present children with today's Gehzeit", async () => {
    renderPicker();

    await screen.findByText("Anna Abel");
    expect(mocks.fetchStudents).toHaveBeenCalledWith({
      location_state: "present",
      include_pickup_times: true,
      page: 1,
      page_size: 1000,
    });
    expect(screen.getByText("geht 16:00")).toBeInTheDocument();
    expect(screen.getByText("ohne Gehzeit")).toBeInTheDocument();
    expect(screen.getByText("2a · Bären · Turnhalle")).toBeInTheDocument();
  });

  it("hides children who already left and children already in the block", async () => {
    renderPicker();

    await screen.findByText("Anna Abel");
    // Ben's Gehzeit 11:45 has passed; Dana is already in the block.
    expect(screen.queryByText("Ben Berg")).not.toBeInTheDocument();
    expect(screen.queryByText("Dana Dorn")).not.toBeInTheDocument();
    expect(screen.getByText("2 Kinder")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Alle anwesenden" }));

    expect(screen.getByText("Ben Berg")).toBeInTheDocument();
    expect(screen.queryByText("Dana Dorn")).not.toBeInTheDocument();
  });

  it("adds the selected children together", async () => {
    const { onAdd, onClose } = renderPicker();

    await screen.findByText("Anna Abel");
    const submit = screen.getByRole("button", { name: "Kinder hinzufügen" });
    expect(submit).toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "Alle auswählen" }));
    fireEvent.click(
      screen.getByRole("button", { name: "2 Kinder hinzufügen" }),
    );

    await waitFor(() => expect(onAdd).toHaveBeenCalledWith(["1", "3"]));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });

  it("narrows the list by name and keeps the selection", async () => {
    const { onAdd } = renderPicker();

    await screen.findByText("Anna Abel");
    fireEvent.click(screen.getByRole("checkbox", { name: /Anna Abel/ }));
    fireEvent.change(
      screen.getByRole("searchbox", { name: "Anwesende Kinder durchsuchen" }),
      { target: { value: "cem" } },
    );

    expect(screen.queryByText("Anna Abel")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("checkbox", { name: /Cem Celik/ }));
    fireEvent.click(
      screen.getByRole("button", { name: "2 Kinder hinzufügen" }),
    );

    await waitFor(() => expect(onAdd).toHaveBeenCalledWith(["1", "3"]));
  });

  it("keeps the dialog open when adding fails", async () => {
    const { onAdd, onClose } = renderPicker({
      onAdd: vi.fn().mockResolvedValue(false),
    });

    await screen.findByText("Anna Abel");
    fireEvent.click(screen.getByRole("checkbox", { name: /Anna Abel/ }));
    fireEvent.click(screen.getByRole("button", { name: "1 Kind hinzufügen" }));

    await waitFor(() => expect(onAdd).toHaveBeenCalled());
    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByRole("checkbox", { name: /Anna Abel/ })).toBeChecked();
  });

  it("says so when the list cannot be loaded", async () => {
    mocks.fetchStudents.mockRejectedValue(new Error("offline"));
    renderPicker();

    expect(
      await screen.findByText("Die Kinder konnten nicht geladen werden."),
    ).toBeInTheDocument();
  });

  it("points to all present children when nobody stays longer", async () => {
    mocks.fetchStudents.mockResolvedValue({
      students: [presentStudents[1]],
    });
    renderPicker();

    expect(
      await screen.findByText("Kein weiteres Kind bleibt noch länger."),
    ).toBeInTheDocument();
  });
});
