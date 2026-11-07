import { fireEvent, render, screen } from "@testing-library/react";
import { useRef, useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { AnchoredPopover } from "./anchored-popover";
import { OverflowMenu } from "./page-header/OverflowMenu";

function TestPopover({
  scoped = false,
  preferredWidth,
  align,
  initialFocus = false,
}: Readonly<{
  scoped?: boolean;
  preferredWidth?: number;
  align?: "start" | "end";
  initialFocus?: boolean;
}>) {
  const [open, setOpen] = useState(false);
  const initialFocusRef = useRef<HTMLDivElement>(null);
  const popover = (
    <AnchoredPopover
      open={open}
      onOpenChange={setOpen}
      ariaLabel="Testauswahl"
      preferredWidth={preferredWidth}
      align={align}
      initialFocusRef={initialFocus ? initialFocusRef : undefined}
      renderTrigger={({ ref, toggle }) => (
        <button ref={ref} type="button" onClick={toggle}>
          Öffnen
        </button>
      )}
    >
      {() => (
        <div
          ref={initialFocus ? initialFocusRef : undefined}
          tabIndex={-1}
          data-testid={initialFocus ? "initial-focus" : undefined}
        >
          Inhalt
          {initialFocus && <a href="#ziel">Zum Ziel</a>}
        </div>
      )}
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

  it("keeps an overflow menu click inside the popover", () => {
    const onOpenChange = vi.fn();
    const onAction = vi.fn();
    render(
      <AnchoredPopover
        open
        onOpenChange={onOpenChange}
        ariaLabel="Testauswahl"
        renderTrigger={({ ref }) => (
          <button ref={ref} type="button">
            Öffnen
          </button>
        )}
      >
        {({ overflowMenuPortal }) => (
          <OverflowMenu
            items={[{ label: "Bearbeiten", onClick: onAction }]}
            portalOwnerId={overflowMenuPortal.ownerId}
            portalZIndex={overflowMenuPortal.zIndex}
          />
        )}
      </AnchoredPopover>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Weitere Aktionen" }));
    const action = screen.getByRole("menuitem", { name: "Bearbeiten" });
    expect(action.closest("[data-overflow-menu-owner]")).toHaveAttribute(
      "data-overflow-menu-owner",
      screen.getByRole("dialog", { name: "Testauswahl" }).id,
    );

    fireEvent.mouseDown(action);
    fireEvent.click(action);

    expect(onOpenChange).not.toHaveBeenCalled();
    expect(onAction).toHaveBeenCalledOnce();
  });

  it("uses a preferred panel width independently from the trigger", () => {
    render(<TestPopover preferredWidth={380} />);
    fireEvent.click(screen.getByRole("button", { name: "Öffnen" }));

    expect(screen.getByRole("dialog", { name: "Testauswahl" })).toHaveStyle({
      width: "380px",
    });
  });

  it("right-aligns an end-aligned panel with its trigger", () => {
    Object.defineProperty(window, "innerWidth", {
      configurable: true,
      value: 1920,
    });
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

  it("keeps focus in the popover when it is repositioned", () => {
    render(<TestPopover initialFocus />);
    fireEvent.click(screen.getByRole("button", { name: "Öffnen" }));

    expect(screen.getByTestId("initial-focus")).toHaveFocus();
    const link = screen.getByRole("link", { name: "Zum Ziel" });
    link.focus();
    fireEvent.resize(window);

    expect(link).toHaveFocus();
  });

  it("keeps header and footer outside the scrolling content", () => {
    function Slots() {
      const [open, setOpen] = useState(true);
      return (
        <AnchoredPopover
          open={open}
          onOpenChange={setOpen}
          ariaLabel="Mit Kopf"
          header={<p>Kopf</p>}
          footer={<a href="#alle">Alle ansehen</a>}
          renderTrigger={({ ref, toggle }) => (
            <button ref={ref} type="button" onClick={toggle}>
              Öffnen
            </button>
          )}
        >
          {() => <p>Inhalt</p>}
        </AnchoredPopover>
      );
    }
    render(<Slots />);
    fireEvent.click(screen.getByRole("button", { name: "Öffnen" }));
    fireEvent.click(screen.getByRole("button", { name: "Öffnen" }));

    const panel = screen.getByRole("dialog", { name: "Mit Kopf" });
    const body = screen.getByText("Inhalt").parentElement;
    expect(panel).toContainElement(screen.getByText("Kopf"));
    expect(body).not.toContainElement(screen.getByText("Kopf"));
    expect(body).not.toContainElement(screen.getByText("Alle ansehen"));
    expect(body).toHaveClass("overflow-y-auto");
  });

  function FocusPopover({
    swallowOutsideClick = false,
    onBehindClick,
  }: Readonly<{
    swallowOutsideClick?: boolean;
    onBehindClick?: () => void;
  }>) {
    const [open, setOpen] = useState(false);
    return (
      <>
        <AnchoredPopover
          open={open}
          onOpenChange={setOpen}
          ariaLabel="Mit Fokus"
          focusPanelOnOpen
          swallowOutsideClick={swallowOutsideClick}
          renderTrigger={({ ref, toggle }) => (
            <button ref={ref} type="button" onClick={toggle}>
              Öffnen
            </button>
          )}
        >
          {() => <button type="button">Im Rahmen</button>}
        </AnchoredPopover>
        <button type="button" onClick={onBehindClick}>
          Dahinter
        </button>
      </>
    );
  }

  it("takes the focus into the panel and returns it on Escape", () => {
    render(<FocusPopover />);
    const trigger = screen.getByRole("button", { name: "Öffnen" });
    fireEvent.click(trigger);

    const panel = screen.getByRole("dialog", { name: "Mit Fokus" });
    expect(panel).toHaveFocus();

    fireEvent.keyDown(panel, { key: "Escape" });

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
  });

  it("consumes the outside click that closes a swallowing popover", () => {
    const onBehindClick = vi.fn();
    render(<FocusPopover swallowOutsideClick onBehindClick={onBehindClick} />);
    fireEvent.click(screen.getByRole("button", { name: "Öffnen" }));
    const behind = screen.getByRole("button", { name: "Dahinter" });

    fireEvent.mouseDown(behind);
    expect(screen.getByRole("dialog", { name: "Mit Fokus" })).toBeInTheDocument();

    fireEvent.click(behind);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(onBehindClick).not.toHaveBeenCalled();
  });

  it("lets the outside click through by default", () => {
    const onBehindClick = vi.fn();
    render(<FocusPopover onBehindClick={onBehindClick} />);
    fireEvent.click(screen.getByRole("button", { name: "Öffnen" }));
    const behind = screen.getByRole("button", { name: "Dahinter" });

    fireEvent.mouseDown(behind);
    fireEvent.click(behind);

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(onBehindClick).toHaveBeenCalledOnce();
  });
});
