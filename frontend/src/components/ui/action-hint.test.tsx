import { act, fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ActionHint, ACTION_HINT_DELAY_MS } from "./action-hint";
import { Button, ButtonLink } from "./button";

const HINT = "Speichert die Änderungen für die ganze Schule.";

function bubble() {
  return document.body.querySelector<HTMLElement>("[data-action-hint]");
}

function hoverWithMouse(element: HTMLElement) {
  fireEvent.pointerEnter(element, { pointerType: "mouse" });
  act(() => {
    vi.advanceTimersByTime(ACTION_HINT_DELAY_MS);
  });
}

describe("ActionHint on kit buttons", () => {
  it("describes the button without changing its name and adds no wrapper", () => {
    const { container } = render(
      <div data-testid="row">
        <Button type="button" tooltip={HINT}>
          Speichern
        </Button>
      </div>,
    );

    const button = screen.getByRole("button", { name: "Speichern" });
    expect(button).toHaveAccessibleDescription(HINT);
    expect(button.parentElement).toBe(screen.getByTestId("row"));
    expect(container.querySelector("[data-action-hint]")).toBeNull();
    expect(bubble()).toBeNull();
  });

  it("shows the bubble after a short mouse hover and hides it on leave", () => {
    vi.useFakeTimers();
    render(
      <Button type="button" tooltip={HINT}>
        Speichern
      </Button>,
    );
    const button = screen.getByRole("button", { name: "Speichern" });

    fireEvent.pointerEnter(button, { pointerType: "mouse" });
    expect(bubble()).toBeNull();
    act(() => {
      vi.advanceTimersByTime(ACTION_HINT_DELAY_MS);
    });
    expect(bubble()).toHaveTextContent(HINT);
    expect(bubble()).toHaveAttribute("aria-hidden", "true");

    fireEvent.pointerLeave(button, { pointerType: "mouse" });
    expect(bubble()).toBeNull();
  });

  it("never shows on touch, where a tap runs the action at once", () => {
    vi.useFakeTimers();
    render(
      <Button type="button" tooltip={HINT}>
        Speichern
      </Button>,
    );
    const button = screen.getByRole("button", { name: "Speichern" });

    fireEvent.pointerEnter(button, { pointerType: "touch" });
    act(() => {
      vi.advanceTimersByTime(ACTION_HINT_DELAY_MS * 2);
    });
    fireEvent.focus(button);

    expect(bubble()).toBeNull();
  });

  it("hides when the button is pressed", () => {
    vi.useFakeTimers();
    render(
      <Button type="button" tooltip={HINT}>
        Speichern
      </Button>,
    );
    const button = screen.getByRole("button", { name: "Speichern" });

    hoverWithMouse(button);
    expect(bubble()).not.toBeNull();
    fireEvent.pointerDown(button, { pointerType: "mouse" });

    expect(bubble()).toBeNull();
  });

  it("closes on Escape without closing anything behind it", () => {
    vi.useFakeTimers();
    const outerEscape = vi.fn();
    window.addEventListener("keydown", outerEscape);
    render(
      <Button type="button" tooltip={HINT}>
        Speichern
      </Button>,
    );
    const button = screen.getByRole("button", { name: "Speichern" });

    hoverWithMouse(button);
    fireEvent.keyDown(document.body, { key: "Escape" });

    expect(bubble()).toBeNull();
    expect(outerEscape).not.toHaveBeenCalled();

    fireEvent.keyDown(document.body, { key: "Escape" });
    expect(outerEscape).toHaveBeenCalledTimes(1);
    window.removeEventListener("keydown", outerEscape);
  });

  it("shows at once on keyboard focus and hides on blur", () => {
    render(
      <Button type="button" tooltip={HINT}>
        Speichern
      </Button>,
    );
    const button = screen.getByRole("button", { name: "Speichern" });

    fireEvent.keyDown(document, { key: "Tab" });
    act(() => {
      button.focus();
    });
    expect(bubble()).toHaveTextContent(HINT);

    act(() => {
      button.blur();
    });
    expect(bubble()).toBeNull();
  });

  it("stays hidden when a click put the focus on the button", () => {
    render(
      <Button type="button" tooltip={HINT}>
        Speichern
      </Button>,
    );
    const button = screen.getByRole("button", { name: "Speichern" });

    fireEvent.pointerDown(document, { pointerType: "mouse" });
    act(() => {
      button.focus();
    });

    expect(bubble()).toBeNull();
  });

  it("keeps the description while the button shows its loading text", () => {
    render(
      <Button type="button" tooltip={HINT} isLoading loadingText="Speichert …">
        Speichern
      </Button>,
    );

    expect(
      screen.getByRole("button", { name: "Speichert …" }),
    ).toHaveAccessibleDescription(HINT);
  });

  it("works on ButtonLink and on any element styled as a button", () => {
    vi.useFakeTimers();
    render(
      <>
        <ButtonLink href="/help" tooltip="Öffnet die Hilfe.">
          Hilfe
        </ButtonLink>
        <ActionHint text="Öffnet das Formular in einem neuen Tab.">
          <a href="https://example.org/anmeldung">Öffnen</a>
        </ActionHint>
      </>,
    );

    const help = screen.getByRole("link", { name: "Hilfe" });
    const open = screen.getByRole("link", { name: "Öffnen" });
    expect(help).toHaveAccessibleDescription("Öffnet die Hilfe.");
    expect(open).toHaveAccessibleDescription(
      "Öffnet das Formular in einem neuen Tab.",
    );

    hoverWithMouse(open);
    expect(bubble()).toHaveTextContent(
      "Öffnet das Formular in einem neuen Tab.",
    );
  });

  it("does not repeat a description that only restates the name", () => {
    render(
      <Button
        type="button"
        size="icon"
        aria-label="Wert ändern"
        tooltip="Wert ändern"
      >
        ✎
      </Button>,
    );

    expect(
      screen.getByRole("button", { name: "Wert ändern" }),
    ).not.toHaveAttribute("aria-describedby");
  });

  it("renders a plain button without hint props", () => {
    render(<Button type="button">Speichern</Button>);

    expect(
      screen.getByRole("button", { name: "Speichern" }),
    ).not.toHaveAttribute("aria-describedby");
  });
});
