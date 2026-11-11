import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { releaseFakeTimers } from "~/test/clock";
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
  afterEach(releaseFakeTimers);

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
    // Anker: Knopf und Beschreibung sind oben geprüft.
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
    act(() => {
      vi.advanceTimersByTime(ACTION_HINT_DELAY_MS);
    });
    expect(bubble()).toBeNull();
  });

  it("stays open while the pointer moves onto the bubble", () => {
    vi.useFakeTimers();
    render(
      <Button type="button" tooltip={HINT}>
        Speichern
      </Button>,
    );
    const button = screen.getByRole("button", { name: "Speichern" });

    hoverWithMouse(button);
    const shown = bubble();
    if (!shown) throw new Error("bubble did not open");
    fireEvent.pointerLeave(button, { pointerType: "mouse" });
    fireEvent.pointerEnter(shown, { pointerType: "mouse" });
    act(() => {
      vi.advanceTimersByTime(ACTION_HINT_DELAY_MS);
    });
    expect(bubble()).toHaveTextContent(HINT);

    fireEvent.pointerLeave(shown, { pointerType: "mouse" });
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
    // Anker: der Knopf trägt den Hinweis, nur sichtbar wird er nicht.
    expect(button).toHaveAccessibleDescription(HINT);

    fireEvent.pointerEnter(button, { pointerType: "touch" });
    fireEvent.pointerDown(button, { pointerType: "touch" });
    act(() => {
      button.focus();
      vi.advanceTimersByTime(ACTION_HINT_DELAY_MS * 2);
    });

    expect(button).toHaveFocus();
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

    // Anker: die Blase stand eben noch (Prüfung oben).
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
    // Anker: die Blase steht vor der Escape-Taste.
    expect(bubble()).toHaveTextContent(HINT);
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
    // Anker: die Blase stand beim Fokus (Prüfung oben).
    expect(bubble()).toBeNull();
  });

  it("cancels a pending hover hint when focus moves to another control", () => {
    vi.useFakeTimers();
    render(
      <>
        <Button type="button" tooltip={HINT}>
          Speichern
        </Button>
        <Button type="button" tooltip="Öffnet die Hilfe.">
          Hilfe
        </Button>
      </>,
    );
    const save = screen.getByRole("button", { name: "Speichern" });
    const help = screen.getByRole("button", { name: "Hilfe" });

    fireEvent.pointerEnter(save, { pointerType: "mouse" });
    fireEvent.keyDown(document, { key: "Tab" });
    act(() => {
      help.focus();
    });
    expect(bubble()).toHaveTextContent("Öffnet die Hilfe.");

    act(() => {
      vi.advanceTimersByTime(ACTION_HINT_DELAY_MS);
    });
    expect(document.body.querySelectorAll("[data-action-hint]")).toHaveLength(
      1,
    );
    expect(bubble()).toHaveTextContent("Öffnet die Hilfe.");
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

    // Anker: der Fokus liegt auf dem Knopf.
    expect(button).toHaveFocus();
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
        aria-label="Profil ansehen: Mia Klein"
        tooltip="Profil ansehen"
      >
        ✎
      </Button>,
    );

    expect(
      screen.getByRole("button", { name: "Profil ansehen: Mia Klein" }),
    ).not.toHaveAttribute("aria-describedby");
  });

  it("renders a plain button without hint props", () => {
    render(<Button type="button">Speichern</Button>);

    expect(
      screen.getByRole("button", { name: "Speichern" }),
    ).not.toHaveAttribute("aria-describedby");
  });
});
