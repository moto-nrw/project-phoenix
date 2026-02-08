import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { StatusDropdown } from "./status-dropdown";

describe("StatusDropdown", () => {
  const mockOnChange = vi.fn();
  const mockOnOpenChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders current status", () => {
    render(<StatusDropdown value="open" onChange={mockOnChange} />);

    expect(screen.getByText("Offen")).toBeInTheDocument();
  });

  it("renders planned status", () => {
    render(<StatusDropdown value="planned" onChange={mockOnChange} />);

    expect(screen.getByText("Geplant")).toBeInTheDocument();
  });

  it("opens dropdown on button click", async () => {
    render(<StatusDropdown value="open" onChange={mockOnChange} />);

    const button = screen.getByRole("button");
    fireEvent.click(button);

    await waitFor(() => {
      expect(screen.getByRole("listbox")).toBeInTheDocument();
    });
  });

  it("calls onChange when selecting a status", async () => {
    render(<StatusDropdown value="open" onChange={mockOnChange} />);

    const button = screen.getByRole("button");
    fireEvent.click(button);

    await waitFor(() => {
      const listbox = screen.getByRole("listbox");
      const buttons = listbox.querySelectorAll("button");
      const plannedButton = Array.from(buttons).find((btn) =>
        btn.textContent?.includes("Geplant"),
      );
      fireEvent.click(plannedButton!);
    });

    expect(mockOnChange).toHaveBeenCalledWith("planned");
  });

  it("closes dropdown after selecting", async () => {
    render(<StatusDropdown value="open" onChange={mockOnChange} />);

    const button = screen.getByRole("button");
    fireEvent.click(button);

    await waitFor(() => {
      const listbox = screen.getByRole("listbox");
      const buttons = listbox.querySelectorAll("button");
      const plannedButton = Array.from(buttons).find((btn) =>
        btn.textContent?.includes("Geplant"),
      );
      fireEvent.click(plannedButton!);
    });

    await waitFor(() => {
      expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
    });
  });

  it("disables button when disabled prop is true", () => {
    render(<StatusDropdown value="open" onChange={mockOnChange} disabled />);

    const button = screen.getByRole("button");
    expect(button).toBeDisabled();
  });

  it("does not open when disabled", async () => {
    render(<StatusDropdown value="open" onChange={mockOnChange} disabled />);

    const button = screen.getByRole("button");
    fireEvent.click(button);

    // Wait a bit to ensure no dropdown appears
    await new Promise((resolve) => setTimeout(resolve, 100));
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
  });

  it("calls onOpenChange when opening dropdown", async () => {
    render(
      <StatusDropdown
        value="open"
        onChange={mockOnChange}
        onOpenChange={mockOnOpenChange}
      />,
    );

    const button = screen.getByRole("button");
    fireEvent.click(button);

    await waitFor(() => {
      expect(mockOnOpenChange).toHaveBeenCalledWith(true);
    });
  });

  it("applies small size classes", () => {
    render(<StatusDropdown value="open" onChange={mockOnChange} size="sm" />);

    const button = screen.getByRole("button");
    expect(button.className).toContain("px-2.5");
  });

  it("applies medium size classes", () => {
    render(<StatusDropdown value="open" onChange={mockOnChange} size="md" />);

    const button = screen.getByRole("button");
    expect(button.className).toContain("px-3");
  });

  it("stops event propagation on button click", () => {
    const parentClick = vi.fn();
    render(
      <div onClick={parentClick}>
        <StatusDropdown value="open" onChange={mockOnChange} />
      </div>,
    );

    const button = screen.getByRole("button");
    fireEvent.click(button);

    expect(parentClick).not.toHaveBeenCalled();
  });
});
