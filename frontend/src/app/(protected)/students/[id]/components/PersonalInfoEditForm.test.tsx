import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { PersonalInfoEditForm } from "./PersonalInfoEditForm";

const baseStudent = {
  first_name: "Max",
  second_name: "Mustermann",
  school_class: "3a",
  birthday: "2015-06-15",
  buskind: true,
  pickup_status: "Wird abgeholt",
  health_info: "Nussallergie",
  supervisor_notes: "Wichtig",
  extra_info: "Elternnotiz",
  sick: false,
};

describe("PersonalInfoEditForm", () => {
  it("renders title", () => {
    render(
      <PersonalInfoEditForm
        editedStudent={baseStudent}
        onStudentChange={vi.fn()}
        onSave={vi.fn()}
        onCancel={vi.fn()}
      />,
    );

    expect(screen.getByText("Persönliche Informationen")).toBeInTheDocument();
  });

  it("renders all form fields", () => {
    render(
      <PersonalInfoEditForm
        editedStudent={baseStudent}
        onStudentChange={vi.fn()}
        onSave={vi.fn()}
        onCancel={vi.fn()}
      />,
    );

    expect(screen.getByText("Vorname")).toBeInTheDocument();
    expect(screen.getByText("Nachname")).toBeInTheDocument();
    expect(screen.getByText("Klasse")).toBeInTheDocument();
    expect(screen.getByText("Geburtsdatum")).toBeInTheDocument();
    expect(screen.getByText("Buskind")).toBeInTheDocument();
    expect(screen.getByText("Abholstatus")).toBeInTheDocument();
  });

  it("renders save and cancel buttons", () => {
    render(
      <PersonalInfoEditForm
        editedStudent={baseStudent}
        onStudentChange={vi.fn()}
        onSave={vi.fn()}
        onCancel={vi.fn()}
      />,
    );

    expect(screen.getByText("Speichern")).toBeInTheDocument();
    expect(screen.getByText("Abbrechen")).toBeInTheDocument();
  });

  it("calls onSave when save button is clicked", () => {
    const onSave = vi.fn();
    render(
      <PersonalInfoEditForm
        editedStudent={baseStudent}
        onStudentChange={vi.fn()}
        onSave={onSave}
        onCancel={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByText("Speichern"));
    expect(onSave).toHaveBeenCalledTimes(1);
  });

  it("calls onCancel when cancel button is clicked", () => {
    const onCancel = vi.fn();
    render(
      <PersonalInfoEditForm
        editedStudent={baseStudent}
        onStudentChange={vi.fn()}
        onSave={vi.fn()}
        onCancel={onCancel}
      />,
    );

    fireEvent.click(screen.getByText("Abbrechen"));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it("calls onStudentChange when text input changes", () => {
    const onStudentChange = vi.fn();
    render(
      <PersonalInfoEditForm
        editedStudent={baseStudent}
        onStudentChange={onStudentChange}
        onSave={vi.fn()}
        onCancel={vi.fn()}
      />,
    );

    const inputs = screen.getAllByRole("textbox");
    fireEvent.change(inputs[0]!, { target: { value: "Moritz" } });

    expect(onStudentChange).toHaveBeenCalled();
  });

  it("renders sick toggle", () => {
    render(
      <PersonalInfoEditForm
        editedStudent={baseStudent}
        onStudentChange={vi.fn()}
        onSave={vi.fn()}
        onCancel={vi.fn()}
      />,
    );

    expect(screen.getByText("Kind krankmelden")).toBeInTheDocument();
    expect(screen.getByRole("switch")).toBeInTheDocument();
  });

  it("toggles sick state when switch is clicked", () => {
    const onStudentChange = vi.fn();
    render(
      <PersonalInfoEditForm
        editedStudent={baseStudent}
        onStudentChange={onStudentChange}
        onSave={vi.fn()}
        onCancel={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("switch"));
    expect(onStudentChange).toHaveBeenCalledWith(
      expect.objectContaining({ sick: true }),
    );
  });
});
