import { describe, it, expect, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { SidebarToggle } from "./sidebar-toggle";

// Die Testumgebung simuliert einen Desktop-Viewport (vitest.config.ts),
// daher ist der Default ohne gespeicherte Wahl "ausgeklappt".
describe("SidebarToggle", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it("collapses the sidebar and persists the choice", () => {
    render(<SidebarToggle />);

    fireEvent.click(
      screen.getByRole("button", { name: "Seitenleiste einklappen" }),
    );

    expect(localStorage.getItem("sidebar-collapsed")).toBe("true");
    expect(
      screen.getByRole("button", { name: "Seitenleiste ausklappen" }),
    ).toBeInTheDocument();
  });

  it("expands again from a stored collapsed state", () => {
    localStorage.setItem("sidebar-collapsed", "true");

    render(<SidebarToggle />);

    fireEvent.click(
      screen.getByRole("button", { name: "Seitenleiste ausklappen" }),
    );

    expect(localStorage.getItem("sidebar-collapsed")).toBe("false");
    expect(
      screen.getByRole("button", { name: "Seitenleiste einklappen" }),
    ).toBeInTheDocument();
  });

  it("is hidden below lg where the sidebar does not exist", () => {
    render(<SidebarToggle />);

    expect(
      screen.getByRole("button", { name: "Seitenleiste einklappen" }),
    ).toHaveClass("hidden", "lg:inline-flex");
  });

  it("toggles via Ctrl+B / Cmd+B", () => {
    render(<SidebarToggle />);

    fireEvent.keyDown(globalThis.window, { key: "b", ctrlKey: true });
    expect(localStorage.getItem("sidebar-collapsed")).toBe("true");

    fireEvent.keyDown(globalThis.window, { key: "b", metaKey: true });
    expect(localStorage.getItem("sidebar-collapsed")).toBe("false");
  });

  it("leaves Ctrl+B alone while typing in an editable field", () => {
    render(
      <div>
        <SidebarToggle />
        <input aria-label="Testfeld" />
      </div>,
    );

    // In Eingabefeldern und Editoren bedeutet die Kombination „fett".
    fireEvent.keyDown(screen.getByLabelText("Testfeld"), {
      key: "b",
      ctrlKey: true,
    });

    expect(localStorage.getItem("sidebar-collapsed")).toBeNull();
  });

  it("ignores Ctrl+B with additional modifiers", () => {
    render(<SidebarToggle />);

    fireEvent.keyDown(globalThis.window, {
      key: "b",
      ctrlKey: true,
      shiftKey: true,
    });

    expect(localStorage.getItem("sidebar-collapsed")).toBeNull();
  });
});
