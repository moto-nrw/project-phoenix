import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { UploadSection } from "./upload-section";

describe("UploadSection", () => {
  const defaultProps = {
    isDragging: false,
    isLoading: false,
    uploadedFile: null,
    onDragEnter: vi.fn(),
    onDragLeave: vi.fn(),
    onDragOver: vi.fn(),
    onDrop: vi.fn(),
    onFileSelect: vi.fn(),
  };

  it("renders upload section header", () => {
    render(<UploadSection {...defaultProps} />);

    expect(
      screen.getByText("Schritt 2: CSV- oder Excel-Datei hochladen"),
    ).toBeInTheDocument();
  });

  it("displays default drop zone text", () => {
    render(<UploadSection {...defaultProps} />);

    expect(screen.getByText("Datei hierher ziehen")).toBeInTheDocument();
    expect(screen.getByText("oder")).toBeInTheDocument();
    expect(screen.getByText("Datei auswählen")).toBeInTheDocument();
  });

  it("displays dragging text when isDragging is true", () => {
    render(<UploadSection {...defaultProps} isDragging={true} />);

    expect(screen.getByText("Datei hier ablegen...")).toBeInTheDocument();
  });

  it("shows loading state when isLoading is true", () => {
    render(<UploadSection {...defaultProps} isLoading={true} />);

    expect(screen.getByText("Datei wird analysiert...")).toBeInTheDocument();
    expect(screen.queryByText("Datei hierher ziehen")).not.toBeInTheDocument();
  });

  it("displays uploaded file name when file is provided", () => {
    const file = new File(["content"], "students.csv", { type: "text/csv" });
    render(<UploadSection {...defaultProps} uploadedFile={file} />);

    expect(screen.getByText("students.csv")).toBeInTheDocument();
  });

  it("calls onDragEnter when dragging over", () => {
    const onDragEnter = vi.fn();
    render(<UploadSection {...defaultProps} onDragEnter={onDragEnter} />);

    const dropZone = screen.getByRole("group");
    fireEvent.dragEnter(dropZone);

    expect(onDragEnter).toHaveBeenCalled();
  });

  it("calls onDragLeave when leaving drag zone", () => {
    const onDragLeave = vi.fn();
    render(<UploadSection {...defaultProps} onDragLeave={onDragLeave} />);

    const dropZone = screen.getByRole("group");
    fireEvent.dragLeave(dropZone);

    expect(onDragLeave).toHaveBeenCalled();
  });

  it("calls onDragOver when dragging over zone", () => {
    const onDragOver = vi.fn();
    render(<UploadSection {...defaultProps} onDragOver={onDragOver} />);

    const dropZone = screen.getByRole("group");
    fireEvent.dragOver(dropZone);

    expect(onDragOver).toHaveBeenCalled();
  });

  it("calls onDrop when file is dropped", () => {
    const onDrop = vi.fn();
    render(<UploadSection {...defaultProps} onDrop={onDrop} />);

    const dropZone = screen.getByRole("group");
    fireEvent.drop(dropZone);

    expect(onDrop).toHaveBeenCalled();
  });

  it("has accessible file input", () => {
    render(<UploadSection {...defaultProps} />);

    expect(screen.getByLabelText("Datei auswählen")).toBeInTheDocument();
  });

  it("accepts csv and xlsx files", () => {
    render(<UploadSection {...defaultProps} />);

    const fileInput = screen.getByLabelText("Datei auswählen");
    expect(fileInput).toHaveAttribute("accept", ".csv,.xlsx");
  });

  it("calls onFileSelect when file is selected via input", () => {
    const onFileSelect = vi.fn();
    render(<UploadSection {...defaultProps} onFileSelect={onFileSelect} />);

    const fileInput = screen.getByLabelText("Datei auswählen");
    const file = new File(["content"], "test.csv", { type: "text/csv" });

    Object.defineProperty(fileInput, "files", {
      value: [file],
    });

    fireEvent.change(fileInput);

    expect(onFileSelect).toHaveBeenCalledWith(file);
  });

  it("has screen reader legend for accessibility", () => {
    render(<UploadSection {...defaultProps} />);

    expect(
      screen.getByText("Datei-Upload-Bereich für Drag-and-Drop"),
    ).toBeInTheDocument();
  });

  it("has accessible upload button", () => {
    render(<UploadSection {...defaultProps} />);

    expect(
      screen.getByLabelText(
        "Datei hochladen - ziehen Sie eine Datei hierher oder klicken Sie zum Auswählen",
      ),
    ).toBeInTheDocument();
  });

  it("displays loading spinner when loading", () => {
    const { container } = render(
      <UploadSection {...defaultProps} isLoading={true} />,
    );

    const spinner = container.querySelector(".animate-spin");
    expect(spinner).toBeInTheDocument();
  });
});
