import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { DetailPanel, type DetailTab } from "./detail-panel";

describe("DetailPanel", () => {
  function tabs(): DetailTab[] {
    return [
      { id: "one", label: "One", content: <div>Content One</div> },
      { id: "two", label: "Two", content: <div>Content Two</div> },
      {
        id: "three",
        label: "Three",
        content: <div>Three</div>,
        disabled: true,
      },
    ];
  }

  it("renders header and all tab triggers", () => {
    render(
      <DetailPanel
        header={<h1>Header</h1>}
        tabs={tabs()}
        activeTab="one"
        onTabChange={vi.fn()}
      />,
    );

    expect(screen.getByText("Header")).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "One" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Two" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Three" })).toBeInTheDocument();
  });

  it("shows content of the active tab", () => {
    render(
      <DetailPanel
        header={null}
        tabs={tabs()}
        activeTab="two"
        onTabChange={vi.fn()}
      />,
    );

    expect(screen.getByText("Content Two")).toBeInTheDocument();
  });

  it("calls onTabChange when a tab is activated", () => {
    const onTabChange = vi.fn();
    render(
      <DetailPanel
        header={null}
        tabs={tabs()}
        activeTab="one"
        onTabChange={onTabChange}
      />,
    );

    const tab = screen.getByRole("tab", { name: "Two" });
    fireEvent.pointerDown(tab, { button: 0, pointerType: "mouse" });
    fireEvent.mouseDown(tab, { button: 0 });
    fireEvent.click(tab);
    fireEvent.keyDown(tab, { key: "Enter" });

    expect(onTabChange).toHaveBeenCalledWith("two");
  });

  it("disables tabs marked disabled", () => {
    render(
      <DetailPanel
        header={null}
        tabs={tabs()}
        activeTab="one"
        onTabChange={vi.fn()}
      />,
    );

    expect(screen.getByRole("tab", { name: "Three" })).toBeDisabled();
  });

  it("applies custom className to root", () => {
    const { container } = render(
      <DetailPanel
        header={null}
        tabs={tabs()}
        activeTab="one"
        onTabChange={vi.fn()}
        className="custom"
      />,
    );
    expect(container.firstChild).toHaveClass("custom");
  });
});
