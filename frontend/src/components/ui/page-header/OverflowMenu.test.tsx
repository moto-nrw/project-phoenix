import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";

import { OverflowMenu } from "./OverflowMenu";

describe("OverflowMenu", () => {
  it("renders nothing when items are empty", () => {
    const { container } = render(<OverflowMenu items={[]} />);
    expect(container.firstChild).toBeNull();
  });

  it("renders the kebab trigger with the default aria-label", () => {
    render(
      <OverflowMenu items={[{ label: "Export", onClick: () => undefined }]} />,
    );
    expect(
      screen.getByRole("button", { name: /Weitere Aktionen/i }),
    ).toBeInTheDocument();
  });

  it("uses the full-size trigger by default and a compact one for triggerSize='sm'", () => {
    const { rerender } = render(
      <OverflowMenu items={[{ label: "Export", onClick: () => undefined }]} />,
    );
    const defaultTrigger = screen.getByRole("button", {
      name: /Weitere Aktionen/i,
    });
    expect(defaultTrigger.className).toContain("size-9");
    expect(defaultTrigger.className).toContain("text-gray-600");

    rerender(
      <OverflowMenu
        triggerSize="sm"
        items={[{ label: "Export", onClick: () => undefined }]}
      />,
    );
    const smallTrigger = screen.getByRole("button", {
      name: /Weitere Aktionen/i,
    });
    expect(smallTrigger.className).toContain("size-6");
    expect(smallTrigger.className).toContain("text-gray-400");
    expect(smallTrigger.className).not.toContain("size-9");
  });

  it("opens the menu on trigger click and renders items", () => {
    render(
      <OverflowMenu
        items={[
          { label: "Export", onClick: () => undefined },
          { label: "Import", onClick: () => undefined },
        ]}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /Weitere Aktionen/i }));

    expect(screen.getByRole("menu")).toBeInTheDocument();
    expect(
      screen.getByRole("menuitem", { name: /Export/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("menuitem", { name: /Import/i }),
    ).toBeInTheDocument();
  });

  it("calls the item onClick and closes the menu", () => {
    const onClick = vi.fn();
    render(<OverflowMenu items={[{ label: "Export", onClick }]} />);

    fireEvent.click(screen.getByRole("button", { name: /Weitere Aktionen/i }));
    fireEvent.click(screen.getByRole("menuitem", { name: /Export/i }));

    expect(onClick).toHaveBeenCalledOnce();
    expect(screen.queryByRole("menu")).toBeNull();
  });

  it("renders a numeric badge next to the item label", () => {
    render(
      <OverflowMenu
        items={[
          { label: "Gruppe übergeben", badge: 3, onClick: () => undefined },
        ]}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /Weitere Aktionen/i }));
    const item = screen.getByRole("menuitem", { name: /Gruppe übergeben/i });
    expect(item.textContent).toContain("3");
  });

  it("respects the disabled flag", () => {
    const onClick = vi.fn();
    render(
      <OverflowMenu items={[{ label: "Export", onClick, disabled: true }]} />,
    );

    fireEvent.click(screen.getByRole("button", { name: /Weitere Aktionen/i }));
    const item = screen.getByRole("menuitem", { name: /Export/i });
    expect(item).toBeDisabled();

    fireEvent.click(item);
    expect(onClick).not.toHaveBeenCalled();
  });

  it("closes on Escape", () => {
    render(
      <OverflowMenu items={[{ label: "Export", onClick: () => undefined }]} />,
    );
    fireEvent.click(screen.getByRole("button", { name: /Weitere Aktionen/i }));
    expect(screen.getByRole("menu")).toBeInTheDocument();

    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("menu")).toBeNull();
  });

  it("invokes onClick on Enter keydown", () => {
    const onClick = vi.fn();
    render(<OverflowMenu items={[{ label: "Export", onClick }]} />);

    fireEvent.click(screen.getByRole("button", { name: /Weitere Aktionen/i }));
    const item = screen.getByRole("menuitem", { name: /Export/i });
    fireEvent.keyDown(item, { key: "Enter" });

    expect(onClick).toHaveBeenCalledOnce();
    expect(screen.queryByRole("menu")).toBeNull();
  });

  it("invokes onClick on Space keydown", () => {
    const onClick = vi.fn();
    render(<OverflowMenu items={[{ label: "Export", onClick }]} />);

    fireEvent.click(screen.getByRole("button", { name: /Weitere Aktionen/i }));
    const item = screen.getByRole("menuitem", { name: /Export/i });
    fireEvent.keyDown(item, { key: " " });

    expect(onClick).toHaveBeenCalledOnce();
  });

  it("ignores keydowns when the item is disabled", () => {
    const onClick = vi.fn();
    render(
      <OverflowMenu items={[{ label: "Export", onClick, disabled: true }]} />,
    );

    fireEvent.click(screen.getByRole("button", { name: /Weitere Aktionen/i }));
    const item = screen.getByRole("menuitem", { name: /Export/i });
    fireEvent.keyDown(item, { key: "Enter" });

    expect(onClick).not.toHaveBeenCalled();
  });

  it("ignores other keydowns (e.g. Tab)", () => {
    const onClick = vi.fn();
    render(<OverflowMenu items={[{ label: "Export", onClick }]} />);

    fireEvent.click(screen.getByRole("button", { name: /Weitere Aktionen/i }));
    const item = screen.getByRole("menuitem", { name: /Export/i });
    fireEvent.keyDown(item, { key: "Tab" });

    expect(onClick).not.toHaveBeenCalled();
    // Menu stays open — Tab doesn't activate.
    expect(screen.getByRole("menu")).toBeInTheDocument();
  });

  it("renders an icon next to the item when provided", () => {
    render(
      <OverflowMenu
        items={[
          {
            label: "Export",
            icon: <span data-testid="custom-icon">★</span>,
            onClick: () => undefined,
          },
        ]}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /Weitere Aktionen/i }));
    expect(screen.getByTestId("custom-icon")).toBeInTheDocument();
  });

  it("renders destructive items with a red text colour class", () => {
    render(
      <OverflowMenu
        items={[
          { label: "Löschen", destructive: true, onClick: () => undefined },
        ]}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /Weitere Aktionen/i }));
    const item = screen.getByRole("menuitem", { name: /Löschen/i });
    expect(item.className).toContain("text-moto-red");
  });

  it("anchors menu to the left when the trigger sits near the left edge", () => {
    // Force the trigger's bounding rect to report it sits near the left edge
    // (left < 220) so the alignment logic anchors with an inline `left` rather
    // than `right`. The portaled menu positions via inline fixed-position
    // styles, not `left-0`/`right-0` classes.
    const originalGetRect = window.HTMLElement.prototype.getBoundingClientRect;
    window.HTMLElement.prototype.getBoundingClientRect = () =>
      ({
        left: 10,
        right: 50,
        top: 0,
        bottom: 0,
        width: 40,
        height: 0,
        x: 10,
        y: 0,
        toJSON: () => ({}),
      }) as DOMRect;

    try {
      render(
        <OverflowMenu
          items={[{ label: "Export", onClick: () => undefined }]}
        />,
      );
      fireEvent.click(
        screen.getByRole("button", { name: /Weitere Aktionen/i }),
      );
      const menu = screen.getByRole("menu");
      expect(menu.style.left).toBe("10px");
      expect(menu.style.right).toBe("");
    } finally {
      window.HTMLElement.prototype.getBoundingClientRect = originalGetRect;
    }
  });

  it("anchors menu to the right when the trigger has room on its left", () => {
    // left >= 220 → the menu extends left from the trigger's right edge,
    // positioned with an inline `right` (viewport width minus rect.right) and
    // no inline `left`.
    const originalGetRect = window.HTMLElement.prototype.getBoundingClientRect;
    window.HTMLElement.prototype.getBoundingClientRect = () =>
      ({
        left: 600,
        right: 640,
        top: 0,
        bottom: 0,
        width: 40,
        height: 0,
        x: 600,
        y: 0,
        toJSON: () => ({}),
      }) as DOMRect;

    try {
      render(
        <OverflowMenu
          items={[{ label: "Export", onClick: () => undefined }]}
        />,
      );
      fireEvent.click(
        screen.getByRole("button", { name: /Weitere Aktionen/i }),
      );
      const menu = screen.getByRole("menu");
      expect(menu.style.right).toBe(`${window.innerWidth - 640}px`);
      expect(menu.style.left).toBe("");
    } finally {
      window.HTMLElement.prototype.getBoundingClientRect = originalGetRect;
    }
  });

  it("keeps the menu inside a narrow viewport when neither side has enough room", () => {
    const originalInnerWidth = window.innerWidth;
    const originalGetRect = window.HTMLElement.prototype.getBoundingClientRect;
    Object.defineProperty(window, "innerWidth", {
      configurable: true,
      value: 320,
    });
    window.HTMLElement.prototype.getBoundingClientRect = () =>
      ({
        left: 150,
        right: 190,
        top: 0,
        bottom: 0,
        width: 40,
        height: 0,
        x: 150,
        y: 0,
        toJSON: () => ({}),
      }) as DOMRect;

    try {
      render(
        <OverflowMenu
          items={[{ label: "Export", onClick: () => undefined }]}
        />,
      );
      fireEvent.click(
        screen.getByRole("button", { name: /Weitere Aktionen/i }),
      );
      const menu = screen.getByRole("menu");

      expect(menu.style.right).toBe(`${window.innerWidth - 220 - 8}px`);
      expect(menu.style.left).toBe("");
      expect(menu.style.minWidth).toBe("220px");
      expect(menu.style.maxWidth).toBe("304px");
    } finally {
      Object.defineProperty(window, "innerWidth", {
        configurable: true,
        value: originalInnerWidth,
      });
      window.HTMLElement.prototype.getBoundingClientRect = originalGetRect;
    }
  });

  it("flips the menu above the trigger when it sits near the viewport bottom", () => {
    // A trigger low in the viewport (little room below it) must anchor the menu
    // by `bottom` (above the trigger) instead of `top`, so it never drops below
    // the fold where the scroll listener would immediately close it.
    const triggerTop = window.innerHeight - 18;
    const triggerBottom = window.innerHeight - 8;
    const originalGetRect = window.HTMLElement.prototype.getBoundingClientRect;
    window.HTMLElement.prototype.getBoundingClientRect = () =>
      ({
        left: 600,
        right: 640,
        top: triggerTop,
        bottom: triggerBottom,
        width: 40,
        height: 10,
        x: 600,
        y: triggerTop,
        toJSON: () => ({}),
      }) as DOMRect;

    try {
      render(
        <OverflowMenu
          items={[{ label: "Export", onClick: () => undefined }]}
        />,
      );
      fireEvent.click(
        screen.getByRole("button", { name: /Weitere Aktionen/i }),
      );
      const menu = screen.getByRole("menu");
      // Anchored by bottom (above the trigger), never by top.
      expect(menu.style.bottom).toBe(
        `${window.innerHeight - triggerTop + 4}px`,
      );
      expect(menu.style.top).toBe("");
    } finally {
      window.HTMLElement.prototype.getBoundingClientRect = originalGetRect;
    }
  });

  it("bounds a tall menu to the viewport and keeps it open during internal scrolling", () => {
    const originalGetRect = window.HTMLElement.prototype.getBoundingClientRect;
    window.HTMLElement.prototype.getBoundingClientRect = () =>
      ({
        left: 600,
        right: 640,
        top: 200,
        bottom: 210,
        width: 40,
        height: 10,
        x: 600,
        y: 200,
        toJSON: () => ({}),
      }) as DOMRect;

    try {
      render(
        <OverflowMenu
          items={Array.from({ length: 30 }, (_, index) => ({
            label: `Aktion ${index + 1}`,
            onClick: () => undefined,
          }))}
        />,
      );
      fireEvent.click(
        screen.getByRole("button", { name: /Weitere Aktionen/i }),
      );
      const menu = screen.getByRole("menu");

      expect(menu).toHaveClass("overflow-y-auto");
      expect(Number.parseFloat(menu.style.maxHeight)).toBeGreaterThan(0);
      fireEvent.scroll(menu);
      expect(screen.getByRole("menu")).toBeInTheDocument();

      fireEvent.scroll(window);
      expect(screen.queryByRole("menu")).toBeNull();
    } finally {
      window.HTMLElement.prototype.getBoundingClientRect = originalGetRect;
    }
  });

  it("closes on outside click", () => {
    render(
      <div>
        <OverflowMenu items={[{ label: "Export", onClick: () => undefined }]} />
        <div data-testid="outside">outside</div>
      </div>,
    );

    fireEvent.click(screen.getByRole("button", { name: /Weitere Aktionen/i }));
    expect(screen.getByRole("menu")).toBeInTheDocument();

    fireEvent.mouseDown(screen.getByTestId("outside"));
    expect(screen.queryByRole("menu")).toBeNull();
  });
});
