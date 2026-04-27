import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "./tabs";

function renderTabs(
  props: {
    variant?: "default" | "line";
    onValueChange?: (v: string) => void;
  } = {},
) {
  return render(
    <Tabs defaultValue="a" onValueChange={props.onValueChange}>
      <TabsList variant={props.variant}>
        <TabsTrigger value="a">One</TabsTrigger>
        <TabsTrigger value="b">Two</TabsTrigger>
      </TabsList>
      <TabsContent value="a">A-content</TabsContent>
      <TabsContent value="b">B-content</TabsContent>
    </Tabs>,
  );
}

describe("Tabs primitives", () => {
  it("renders default variant list", () => {
    renderTabs();
    const list = screen.getByRole("tablist");
    expect(list).toHaveAttribute("data-variant", "default");
    expect(list).toHaveClass("bg-muted");
  });

  it("renders line variant list", () => {
    renderTabs({ variant: "line" });
    const list = screen.getByRole("tablist");
    expect(list).toHaveAttribute("data-variant", "line");
    expect(list).toHaveClass("border-b");
  });

  it("switches active content via tab activation", () => {
    const onValueChange = vi.fn();
    renderTabs({ onValueChange });

    expect(screen.getByText("A-content")).toBeInTheDocument();

    const tabTwo = screen.getByRole("tab", { name: "Two" });
    fireEvent.pointerDown(tabTwo, { button: 0, pointerType: "mouse" });
    fireEvent.mouseDown(tabTwo, { button: 0 });
    fireEvent.click(tabTwo);
    fireEvent.keyDown(tabTwo, { key: "Enter" });

    expect(onValueChange).toHaveBeenCalledWith("b");
  });

  it("TabsList accepts extra className", () => {
    render(
      <Tabs defaultValue="a">
        <TabsList className="extra-class">
          <TabsTrigger value="a">A</TabsTrigger>
        </TabsList>
      </Tabs>,
    );
    expect(screen.getByRole("tablist")).toHaveClass("extra-class");
  });

  it("TabsTrigger accepts extra className", () => {
    render(
      <Tabs defaultValue="a">
        <TabsList>
          <TabsTrigger value="a" className="trigger-class">
            A
          </TabsTrigger>
        </TabsList>
      </Tabs>,
    );
    expect(screen.getByRole("tab")).toHaveClass("trigger-class");
  });

  it("TabsContent accepts extra className", () => {
    render(
      <Tabs defaultValue="a">
        <TabsList>
          <TabsTrigger value="a">A</TabsTrigger>
        </TabsList>
        <TabsContent value="a" className="content-class">
          Hello
        </TabsContent>
      </Tabs>,
    );
    expect(screen.getByRole("tabpanel")).toHaveClass("content-class");
  });
});
