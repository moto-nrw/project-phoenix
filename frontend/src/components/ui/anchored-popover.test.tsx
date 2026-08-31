import { fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { AnchoredPopover } from "./anchored-popover";

function TestPopover({
  scoped = false,
  preferredWidth,
  align,
}: Readonly<{
  scoped?: boolean;
  preferredWidth?: number;
  align?: "start" | "end";
}>) {
  const [open, setOpen] = useState(false);
  const popover = (
    <AnchoredPopover
      open={open}
      onOpenChange={setOpen}
      ariaLabel="Testauswahl"
      preferredWidth={preferredWidth}
      align={align}
      renderTrigger={({ ref, toggle }) => (
        <button ref={ref} type="button" onClick={toggle}>
          Öffnen
        </button>
      )}
    >
      {() => <div>Inhalt</div>}
    </AnchoredPopover>
  );

  return scoped ? (
    <div data-date-picker-focus-trap="true">{popover}</div>
  ) : (
    popover
  );
}

describe("AnchoredPopover", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("uses container-relative positioning inside a scoped portal", () => {
    vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockReturnValue({
      left: 300,
      right: 700,
      top: 0,
      bottom: 800,
      width: 400,
      height: 800,
      x: 300,
      y: 0,
      toJSON: () => undefined,
    });

    render(<TestPopover scoped />);
    fireEvent.click(screen.getByRole("button", { name: "Öffnen" }));

    expect(screen.getByRole("dialog", { name: "Testauswahl" })).toHaveStyle({
      position: "absolute",
    });
  });

  it("keeps viewport positioning outside a scoped portal", () => {
    render(<TestPopover />);
    fireEvent.click(screen.getByRole("button", { name: "Öffnen" }));

    expect(screen.getByRole("dialog", { name: "Testauswahl" })).toHaveStyle({
      position: "fixed",
    });
  });

  it("uses a preferred panel width independently from the trigger", () => {
    render(<TestPopover preferredWidth={380} />);
    fireEvent.click(screen.getByRole("button", { name: "Öffnen" }));

    expect(screen.getByRole("dialog", { name: "Testauswahl" })).toHaveStyle({
      width: "380px",
    });
  });

  it("right-aligns an end-aligned panel with its trigger", () => {
    vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockReturnValue({
      left: 1800,
      right: 1840,
      top: 100,
      bottom: 140,
      width: 40,
      height: 40,
      x: 1800,
      y: 100,
      toJSON: () => undefined,
    });

    render(<TestPopover preferredWidth={288} align="end" />);
    fireEvent.click(screen.getByRole("button", { name: "Öffnen" }));

    expect(screen.getByRole("dialog", { name: "Testauswahl" })).toHaveStyle({
      left: "1552px",
    });
  });
});
