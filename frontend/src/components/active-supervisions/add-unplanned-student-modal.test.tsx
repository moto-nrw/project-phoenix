import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  AddUnplannedStudentModal,
  type AddUnplannedStudentModalProps,
} from "./add-unplanned-student-modal";
import type { Student } from "~/lib/student-helpers";

const mocks = vi.hoisted(() => ({ fetchStudents: vi.fn() }));

vi.mock("~/lib/student-api", () => ({ fetchStudents: mocks.fetchStudents }));

function student(id: string, name: string, schoolClass = "2a"): Student {
  return {
    id,
    name,
    school_class: schoolClass,
    current_location: "home",
  } as Student;
}

function Harness(
  props: Partial<AddUnplannedStudentModalProps> & { readonly open?: boolean },
) {
  const [search, setSearch] = useState("");
  const [open, setOpen] = useState(props.open ?? true);
  return (
    <>
      <button type="button" onClick={() => setOpen(true)}>
        Öffnen
      </button>
      <AddUnplannedStudentModal
        isOpen={open}
        instanceId="instance-1"
        inBlockStudentIds={new Set()}
        isAddingStudent={false}
        search={search}
        error={null}
        onAdd={vi.fn().mockResolvedValue(true)}
        onClose={() => {
          setSearch("");
          setOpen(false);
        }}
        onSearchChange={setSearch}
        {...props}
      />
    </>
  );
}

const names = () =>
  screen
    .getAllByRole("radio")
    .map((radio) => radio.closest("label")?.textContent);

describe("AddUnplannedStudentModal", () => {
  beforeEach(() => {
    mocks.fetchStudents.mockReset();
  });

  it("lists every child alphabetically on open, before anything is typed", async () => {
    mocks.fetchStudents
      .mockResolvedValueOnce({
        students: [student("3", "Zoe Weber"), student("1", "Ömer Kaya")],
        pagination: {
          current_page: 1,
          page_size: 100,
          total_pages: 2,
          total_records: 3,
        },
      })
      .mockResolvedValueOnce({
        students: [student("2", "Anna Beier", "3b")],
        pagination: {
          current_page: 2,
          page_size: 100,
          total_pages: 2,
          total_records: 3,
        },
      });

    render(<Harness />);

    expect(await screen.findByText("3 Kinder")).toBeInTheDocument();
    expect(
      screen.getByRole("searchbox", { name: "Kind ungeplant suchen" }),
    ).toHaveValue("");
    expect(names()).toEqual([
      expect.stringContaining("Anna Beier"),
      expect.stringContaining("Ömer Kaya"),
      expect.stringContaining("Zoe Weber"),
    ]);
    expect(mocks.fetchStudents).toHaveBeenCalledTimes(2);
    expect(mocks.fetchStudents).toHaveBeenLastCalledWith({
      page: 2,
      page_size: 100,
    });
  });

  it("filters the list from the first typed character without a new request", async () => {
    mocks.fetchStudents.mockResolvedValue({
      students: [
        student("1", "Marie Beier"),
        student("2", "Ben Neu"),
        student("3", "Lara Marie Brüggemann"),
      ],
    });
    render(<Harness />);
    await screen.findByText("3 Kinder");

    fireEvent.change(
      screen.getByRole("searchbox", { name: "Kind ungeplant suchen" }),
      { target: { value: "M" } },
    );

    expect(screen.getByText("2 Kinder")).toBeInTheDocument();
    expect(screen.queryByRole("radio", { name: /Ben Neu/ })).toBeNull();

    fireEvent.change(
      screen.getByRole("searchbox", { name: "Kind ungeplant suchen" }),
      { target: { value: "xyz" } },
    );
    expect(
      screen.getByText("Kein Kind mit diesem Namen gefunden."),
    ).toBeInTheDocument();
    expect(mocks.fetchStudents).toHaveBeenCalledTimes(1);
  });

  it("marks children already in the block and keeps them out of the selection", async () => {
    mocks.fetchStudents.mockResolvedValue({
      students: [student("1", "Marie Beier"), student("2", "Ben Neu")],
    });
    const onAdd = vi.fn().mockResolvedValue(true);
    render(<Harness inBlockStudentIds={new Set(["1"])} onAdd={onAdd} />);

    const inBlock = await screen.findByRole("radio", { name: /Marie Beier/ });
    expect(inBlock).toBeDisabled();
    expect(inBlock.closest("label")).toHaveTextContent("schon dabei");

    // Ben Neu ist das einzige wählbare Kind: der schnelle Weg bleibt.
    expect(screen.getByRole("radio", { name: /Ben Neu/ })).toBeChecked();
    fireEvent.click(screen.getByRole("button", { name: "Hinzufügen" }));
    await waitFor(() => expect(onAdd).toHaveBeenCalledWith("2"));
  });

  it("requires a choice among several children and clears it when the dialog closes", async () => {
    mocks.fetchStudents.mockResolvedValue({
      students: [student("1", "Marie Beier"), student("2", "Marie Garschagen")],
    });
    render(<Harness />);

    const beier = await screen.findByRole("radio", { name: /Marie Beier/ });
    const addButton = screen.getByRole("button", { name: "Hinzufügen" });
    expect(addButton).toBeDisabled();
    expect(
      screen.getByText("Bitte ein Kind aus der Liste antippen."),
    ).toBeInTheDocument();

    fireEvent.click(beier);
    expect(beier).toBeChecked();
    expect(addButton).toBeEnabled();
    expect(
      screen.getByRole("searchbox", { name: "Kind ungeplant suchen" }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));
    await waitFor(() =>
      expect(
        screen.queryByRole("searchbox", { name: "Kind ungeplant suchen" }),
      ).not.toBeInTheDocument(),
    );

    fireEvent.click(screen.getByRole("button", { name: "Öffnen" }));
    expect(
      await screen.findByRole("radio", { name: /Marie Beier/ }),
    ).not.toBeChecked();
    expect(screen.getByRole("button", { name: "Hinzufügen" })).toBeDisabled();
  });

  it("shows a load failure in the dialog", async () => {
    mocks.fetchStudents.mockRejectedValue(new Error("offline"));
    render(<Harness />);

    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(screen.queryByRole("radio")).toBeNull();
    expect(screen.getByRole("button", { name: "Hinzufügen" })).toBeDisabled();
  });
});
