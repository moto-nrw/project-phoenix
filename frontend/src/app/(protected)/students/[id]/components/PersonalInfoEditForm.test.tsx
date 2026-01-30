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

  describe("Date Input Edge Cases", () => {
    it("handles birthday with ISO timestamp (splits on T)", () => {
      const studentWithTimestamp = {
        ...baseStudent,
        birthday: "2015-06-15T00:00:00Z",
      };

      const { container } = render(
        <PersonalInfoEditForm
          editedStudent={studentWithTimestamp}
          onStudentChange={vi.fn()}
          onSave={vi.fn()}
          onCancel={vi.fn()}
        />,
      );

      const dateInput = container.querySelector('input[type="date"]')!;
      expect((dateInput as HTMLInputElement).value).toBe("2015-06-15");
    });

    it("handles undefined birthday", () => {
      const studentNoBirthday = {
        ...baseStudent,
        birthday: undefined,
      };

      const { container } = render(
        <PersonalInfoEditForm
          editedStudent={studentNoBirthday}
          onStudentChange={vi.fn()}
          onSave={vi.fn()}
          onCancel={vi.fn()}
        />,
      );

      const dateInput = container.querySelector('input[type="date"]')!;
      expect((dateInput as HTMLInputElement).value).toBe("");
    });

    it("calls onStudentChange when birthday changes", () => {
      const onStudentChange = vi.fn();
      const { container } = render(
        <PersonalInfoEditForm
          editedStudent={baseStudent}
          onStudentChange={onStudentChange}
          onSave={vi.fn()}
          onCancel={vi.fn()}
        />,
      );

      const dateInput = container.querySelector('input[type="date"]')!;
      fireEvent.change(dateInput, { target: { value: "2016-03-20" } });

      expect(onStudentChange).toHaveBeenCalledWith(
        expect.objectContaining({ birthday: "2016-03-20" }),
      );
    });
  });

  describe("Select Input - Buskind", () => {
    it("displays buskind as true", () => {
      render(
        <PersonalInfoEditForm
          editedStudent={baseStudent}
          onStudentChange={vi.fn()}
          onSave={vi.fn()}
          onCancel={vi.fn()}
        />,
      );

      const buskindSelect = screen.getAllByRole(
        "combobox",
      )[0] as HTMLSelectElement;
      expect(buskindSelect.value).toBe("true");
    });

    it("displays buskind as false", () => {
      const studentNotBuskind = { ...baseStudent, buskind: false };

      render(
        <PersonalInfoEditForm
          editedStudent={studentNotBuskind}
          onStudentChange={vi.fn()}
          onSave={vi.fn()}
          onCancel={vi.fn()}
        />,
      );

      const buskindSelect = screen.getAllByRole(
        "combobox",
      )[0] as HTMLSelectElement;
      expect(buskindSelect.value).toBe("false");
    });

    it("calls onStudentChange when buskind changes to true", () => {
      const onStudentChange = vi.fn();
      const studentNotBuskind = { ...baseStudent, buskind: false };

      render(
        <PersonalInfoEditForm
          editedStudent={studentNotBuskind}
          onStudentChange={onStudentChange}
          onSave={vi.fn()}
          onCancel={vi.fn()}
        />,
      );

      const buskindSelect = screen.getAllByRole("combobox")[0]!;
      fireEvent.change(buskindSelect, { target: { value: "true" } });

      expect(onStudentChange).toHaveBeenCalledWith(
        expect.objectContaining({ buskind: true }),
      );
    });

    it("calls onStudentChange when buskind changes to false", () => {
      const onStudentChange = vi.fn();

      render(
        <PersonalInfoEditForm
          editedStudent={baseStudent}
          onStudentChange={onStudentChange}
          onSave={vi.fn()}
          onCancel={vi.fn()}
        />,
      );

      const buskindSelect = screen.getAllByRole("combobox")[0]!;
      fireEvent.change(buskindSelect, { target: { value: "false" } });

      expect(onStudentChange).toHaveBeenCalledWith(
        expect.objectContaining({ buskind: false }),
      );
    });
  });

  describe("Select Input - Pickup Status", () => {
    it("handles undefined pickup status", () => {
      const studentNoPickup = { ...baseStudent, pickup_status: undefined };

      render(
        <PersonalInfoEditForm
          editedStudent={studentNoPickup}
          onStudentChange={vi.fn()}
          onSave={vi.fn()}
          onCancel={vi.fn()}
        />,
      );

      const pickupSelect = screen.getAllByRole(
        "combobox",
      )[1] as HTMLSelectElement;
      expect(pickupSelect.value).toBe("");
    });

    it("calls onStudentChange with undefined when pickup status cleared", () => {
      const onStudentChange = vi.fn();

      render(
        <PersonalInfoEditForm
          editedStudent={baseStudent}
          onStudentChange={onStudentChange}
          onSave={vi.fn()}
          onCancel={vi.fn()}
        />,
      );

      const pickupSelect = screen.getAllByRole("combobox")[1]!;
      fireEvent.change(pickupSelect, { target: { value: "" } });

      expect(onStudentChange).toHaveBeenCalledWith(
        expect.objectContaining({ pickup_status: undefined }),
      );
    });

    it("calls onStudentChange with value when pickup status selected", () => {
      const onStudentChange = vi.fn();
      const studentNoPickup = { ...baseStudent, pickup_status: undefined };

      render(
        <PersonalInfoEditForm
          editedStudent={studentNoPickup}
          onStudentChange={onStudentChange}
          onSave={vi.fn()}
          onCancel={vi.fn()}
        />,
      );

      const pickupSelect = screen.getAllByRole("combobox")[1]!;
      fireEvent.change(pickupSelect, {
        target: { value: "Geht alleine nach Hause" },
      });

      expect(onStudentChange).toHaveBeenCalledWith(
        expect.objectContaining({ pickup_status: "Geht alleine nach Hause" }),
      );
    });
  });

  describe("Textarea Inputs - Undefined Handling", () => {
    it("handles undefined health_info", () => {
      const studentNoHealth = { ...baseStudent, health_info: undefined };

      render(
        <PersonalInfoEditForm
          editedStudent={studentNoHealth}
          onStudentChange={vi.fn()}
          onSave={vi.fn()}
          onCancel={vi.fn()}
        />,
      );

      const textareas = screen.getAllByRole("textbox");
      const healthTextarea = textareas[3] as HTMLTextAreaElement;
      expect(healthTextarea.value).toBe("");
      expect(healthTextarea.placeholder).toBe(
        "Allergien, Medikamente, wichtige medizinische Informationen",
      );
    });

    it("handles undefined supervisor_notes", () => {
      const studentNoNotes = { ...baseStudent, supervisor_notes: undefined };

      render(
        <PersonalInfoEditForm
          editedStudent={studentNoNotes}
          onStudentChange={vi.fn()}
          onSave={vi.fn()}
          onCancel={vi.fn()}
        />,
      );

      const textareas = screen.getAllByRole("textbox");
      const notesTextarea = textareas[4] as HTMLTextAreaElement;
      expect(notesTextarea.value).toBe("");
    });

    it("handles undefined extra_info", () => {
      const studentNoExtra = { ...baseStudent, extra_info: undefined };

      render(
        <PersonalInfoEditForm
          editedStudent={studentNoExtra}
          onStudentChange={vi.fn()}
          onSave={vi.fn()}
          onCancel={vi.fn()}
        />,
      );

      const textareas = screen.getAllByRole("textbox");
      const extraTextarea = textareas[5] as HTMLTextAreaElement;
      expect(extraTextarea.value).toBe("");
    });

    it("calls onStudentChange when health_info changes", () => {
      const onStudentChange = vi.fn();

      render(
        <PersonalInfoEditForm
          editedStudent={baseStudent}
          onStudentChange={onStudentChange}
          onSave={vi.fn()}
          onCancel={vi.fn()}
        />,
      );

      const healthTextarea = screen.getByDisplayValue("Nussallergie");
      fireEvent.change(healthTextarea, {
        target: { value: "Laktoseintoleranz" },
      });

      expect(onStudentChange).toHaveBeenCalledWith(
        expect.objectContaining({ health_info: "Laktoseintoleranz" }),
      );
    });

    it("calls onStudentChange when supervisor_notes changes", () => {
      const onStudentChange = vi.fn();

      render(
        <PersonalInfoEditForm
          editedStudent={baseStudent}
          onStudentChange={onStudentChange}
          onSave={vi.fn()}
          onCancel={vi.fn()}
        />,
      );

      const notesTextarea = screen.getByDisplayValue("Wichtig");
      fireEvent.change(notesTextarea, { target: { value: "Sehr wichtig" } });

      expect(onStudentChange).toHaveBeenCalledWith(
        expect.objectContaining({ supervisor_notes: "Sehr wichtig" }),
      );
    });

    it("calls onStudentChange when extra_info changes", () => {
      const onStudentChange = vi.fn();

      render(
        <PersonalInfoEditForm
          editedStudent={baseStudent}
          onStudentChange={onStudentChange}
          onSave={vi.fn()}
          onCancel={vi.fn()}
        />,
      );

      const extraTextarea = screen.getByDisplayValue("Elternnotiz");
      fireEvent.change(extraTextarea, {
        target: { value: "Neue Elternnotiz" },
      });

      expect(onStudentChange).toHaveBeenCalledWith(
        expect.objectContaining({ extra_info: "Neue Elternnotiz" }),
      );
    });
  });

  describe("Sick Toggle Conditional Rendering", () => {
    it("toggles sick state from true to false", () => {
      const onStudentChange = vi.fn();
      const sickStudent = { ...baseStudent, sick: true };

      render(
        <PersonalInfoEditForm
          editedStudent={sickStudent}
          onStudentChange={onStudentChange}
          onSave={vi.fn()}
          onCancel={vi.fn()}
        />,
      );

      fireEvent.click(screen.getByRole("switch"));
      expect(onStudentChange).toHaveBeenCalledWith(
        expect.objectContaining({ sick: false }),
      );
    });

    it("handles undefined sick status (defaults to false)", () => {
      const studentNoSick = { ...baseStudent, sick: undefined };

      render(
        <PersonalInfoEditForm
          editedStudent={studentNoSick}
          onStudentChange={vi.fn()}
          onSave={vi.fn()}
          onCancel={vi.fn()}
        />,
      );

      const sickSwitch = screen.getByRole("switch");
      expect(sickSwitch).toHaveAttribute("aria-checked", "false");
    });

    it("displays amber styling when sick is true", () => {
      const sickStudent = { ...baseStudent, sick: true };

      render(
        <PersonalInfoEditForm
          editedStudent={sickStudent}
          onStudentChange={vi.fn()}
          onSave={vi.fn()}
          onCancel={vi.fn()}
        />,
      );

      const sickSwitch = screen.getByRole("switch");
      expect(sickSwitch).toHaveClass("bg-amber-500");
    });

    it("displays gray styling when sick is false", () => {
      render(
        <PersonalInfoEditForm
          editedStudent={baseStudent}
          onStudentChange={vi.fn()}
          onSave={vi.fn()}
          onCancel={vi.fn()}
        />,
      );

      const sickSwitch = screen.getByRole("switch");
      expect(sickSwitch).toHaveClass("bg-gray-300");
    });
  });
});
