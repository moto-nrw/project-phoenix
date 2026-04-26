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
