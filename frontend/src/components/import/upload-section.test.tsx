import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { UploadSection } from "./upload-section";

describe("UploadSection", () => {
  const defaultProps = {
    isLoading: false,
    uploadedFile: null,
    onFileSelect: vi.fn(),
  };

  it("renders upload section header", () => {
    render(<UploadSection {...defaultProps} />);

    expect(screen.getByText("Schritt 2: Datei hochladen")).toBeInTheDocument();
  });

  it("displays default drop zone text", () => {
    render(<UploadSection {...defaultProps} />);

    expect(screen.getByText("Datei hierher ziehen")).toBeInTheDocument();
    expect(screen.getByText("oder")).toBeInTheDocument();
    expect(screen.getByText("Datei auswählen")).toBeInTheDocument();
  });

  it("displays dragging text while a file is held over the zone", () => {
    render(<UploadSection {...defaultProps} />);

    fireEvent.dragEnter(screen.getByRole("group"));

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

  it("calls onFileSelect when a file is dropped", () => {
    const onFileSelect = vi.fn();
    render(<UploadSection {...defaultProps} onFileSelect={onFileSelect} />);
    const file = new File(["content"], "test.csv", { type: "text/csv" });

    fireEvent.drop(screen.getByRole("group"), {
      dataTransfer: { files: [file] },
    });

    expect(onFileSelect).toHaveBeenCalledWith(file);
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
      screen.getByText("Bereich zum Hochladen einer Datei"),
    ).toBeInTheDocument();
  });

  it("has a real button that opens the file dialog", () => {
    render(<UploadSection {...defaultProps} />);

    expect(
      screen.getByRole("button", { name: "Datei auswählen" }),
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
