import { act, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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

describe("disabledReason on kit buttons", () => {
  afterEach(releaseFakeTimers);

  const REASON = "Wählen Sie zuerst mindestens ein Kind aus.";

  it("keeps a blocked button focusable and describes it with the reason", () => {
    render(
      <Button type="button" tooltip={HINT} disabled disabledReason={REASON}>
        Speichern
      </Button>,
    );
    const button = screen.getByRole("button", { name: "Speichern" });

    expect(button).toHaveAttribute("aria-disabled", "true");
    expect(button).not.toHaveAttribute("disabled");
    expect(button).toHaveAccessibleDescription(REASON);
  });

  it("shows the reason on mouse hover and on keyboard focus", () => {
    vi.useFakeTimers();
    render(
      <Button type="button" tooltip={HINT} disabled disabledReason={REASON}>
        Speichern
      </Button>,
    );
    const button = screen.getByRole("button", { name: "Speichern" });

    hoverWithMouse(button);
    expect(bubble()).toHaveTextContent(REASON);
    fireEvent.pointerLeave(button, { pointerType: "mouse" });
    act(() => {
      vi.advanceTimersByTime(ACTION_HINT_DELAY_MS);
    });
    // Anker: die Blase stand beim Hover (Prüfung oben).
    expect(bubble()).toBeNull();

    fireEvent.keyDown(document, { key: "Tab" });
    act(() => {
      button.focus();
    });
    expect(bubble()).toHaveTextContent(REASON);
  });

  it("shows the reason on a tap and closes on a second tap or a tap elsewhere", () => {
    render(
      <>
        <Button type="button" disabled disabledReason={REASON}>
          Speichern
        </Button>
        <p>Daneben</p>
      </>,
    );
    const button = screen.getByRole("button", { name: "Speichern" });

    fireEvent.pointerDown(button, { pointerType: "touch" });
    fireEvent.pointerLeave(button, { pointerType: "touch" });
    expect(bubble()).toHaveTextContent(REASON);
    const shown = bubble();
    if (!shown) throw new Error("bubble did not open");
    fireEvent.pointerDown(shown, { pointerType: "touch" });
    fireEvent.pointerLeave(shown, { pointerType: "touch" });
    expect(bubble()).toHaveTextContent(REASON);
    fireEvent.pointerDown(button, { pointerType: "touch" });
    // Anker: der erste Tipp hat die Blase geöffnet (Prüfung oben).
    expect(bubble()).toBeNull();

    fireEvent.pointerDown(button, { pointerType: "touch" });
    expect(bubble()).toHaveTextContent(REASON);
    fireEvent.pointerDown(screen.getByText("Daneben"), {
      pointerType: "touch",
    });
    expect(bubble()).toBeNull();
  });

  it("shows the reason at once when a mouse clicks the blocked button", () => {
    render(
      <Button type="button" disabled disabledReason={REASON}>
        Speichern
      </Button>,
    );
    const button = screen.getByRole("button", { name: "Speichern" });

    fireEvent.pointerEnter(button, { pointerType: "mouse" });
    fireEvent.pointerDown(button, { pointerType: "mouse" });

    expect(bubble()).toHaveTextContent(REASON);
  });

  it("runs neither onClick, key handlers nor submit on click or Enter", async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();
    const onKeyDown = vi.fn();
    const onKeyPress = vi.fn();
    const onSubmit = vi.fn((event: React.FormEvent) => event.preventDefault());
    function Form({ blocked }: Readonly<{ blocked: boolean }>) {
      return (
        <form onSubmit={onSubmit}>
          <input aria-label="Name" />
          <Button
            onClick={onClick}
            onKeyDown={onKeyDown}
            onKeyPress={onKeyPress}
            disabled={blocked}
            disabledReason={REASON}
          >
            Speichern
          </Button>
        </form>
      );
    }
    async function pressEverything() {
      await user.click(screen.getByRole("button", { name: "Speichern" }));
      screen.getByRole("button", { name: "Speichern" }).focus();
      await user.keyboard("{Enter}");
      fireEvent.keyPress(screen.getByRole("button", { name: "Speichern" }), {
        key: "Enter",
      });
      await user.click(screen.getByRole("textbox", { name: "Name" }));
      await user.keyboard("{Enter}");
    }
    const { rerender } = render(<Form blocked />);

    await pressEverything();
    expect(onClick).not.toHaveBeenCalled();
    expect(onKeyDown).not.toHaveBeenCalled();
    expect(onKeyPress).not.toHaveBeenCalled();
    expect(onSubmit).not.toHaveBeenCalled();

    // Gegenprobe: derselbe Weg löst am freien Knopf alles aus.
    rerender(<Form blocked={false} />);
    await pressEverything();
    expect(onClick).toHaveBeenCalled();
    expect(onKeyDown).toHaveBeenCalled();
    expect(onKeyPress).toHaveBeenCalled();
    expect(onSubmit).toHaveBeenCalled();
  });

  it.each([
    { name: "Button", link: false, role: "button" },
    { name: "ButtonLink", link: true, role: "link" },
  ])(
    "stops activation from reaching a clickable parent for $name",
    ({ link, role }) => {
      const onParentClick = vi.fn();
      const onParentPointerDown = vi.fn();
      const onParentKeyDown = vi.fn();
      const onParentKeyPress = vi.fn();
      const onControlClickCapture = vi.fn();
      render(
        <div
          role="button"
          tabIndex={0}
          aria-label="Klickbare Zeile"
          onClick={onParentClick}
          onPointerDown={onParentPointerDown}
          onKeyDown={onParentKeyDown}
          onKeyPress={onParentKeyPress}
        >
          {link ? (
            <ButtonLink
              href="/help"
              disabled
              disabledReason={REASON}
              onClickCapture={onControlClickCapture}
            >
              Weiter
            </ButtonLink>
          ) : (
            <Button
              type="button"
              disabled
              disabledReason={REASON}
              onClickCapture={onControlClickCapture}
            >
              Weiter
            </Button>
          )}
        </div>,
      );
      const control = screen.getByRole(role, { name: "Weiter" });

      expect(control).toHaveAccessibleDescription(REASON);
      fireEvent.pointerDown(control, { pointerType: "mouse" });
      fireEvent.click(control);
      fireEvent.keyDown(control, { key: "Enter" });
      fireEvent.keyPress(control, { key: "Enter" });

      expect(onControlClickCapture).not.toHaveBeenCalled();
      expect(onParentClick).not.toHaveBeenCalled();
      expect(onParentPointerDown).not.toHaveBeenCalled();
      expect(onParentKeyDown).not.toHaveBeenCalled();
      expect(onParentKeyPress).not.toHaveBeenCalled();
    },
  );

  it("gives a blocked ButtonLink no target to navigate to", () => {
    render(
      <>
        <ButtonLink href="/help" disabled disabledReason={REASON}>
          Hilfe
        </ButtonLink>
        <ButtonLink href="/start" tooltip="Öffnet die Startseite.">
          Start
        </ButtonLink>
      </>,
    );
    const link = screen.getByRole("link", { name: "Hilfe" });

    // Gegenprobe: ein freier ButtonLink trägt sein Ziel.
    expect(screen.getByRole("link", { name: "Start" })).toHaveAttribute(
      "href",
      "/start",
    );
    expect(link).not.toHaveAttribute("href");
    expect(link).toHaveAttribute("aria-disabled", "true");
    expect(link).toHaveAttribute("tabindex", "0");
    expect(link).toHaveAccessibleDescription(REASON);
  });

  it("keeps the native lock while loading", () => {
    render(
      <Button
        type="button"
        tooltip={HINT}
        isLoading
        loadingText="Speichert …"
        disabled
        disabledReason={REASON}
      >
        Speichern
      </Button>,
    );
    const button = screen.getByRole("button", { name: "Speichert …" });

    expect(button).toBeDisabled();
    expect(button).not.toHaveAttribute("aria-disabled");
    expect(button).toHaveAccessibleDescription(HINT);
  });

  it("stays natively disabled without a reason", () => {
    render(
      <Button type="button" disabled>
        Speichern
      </Button>,
    );
    const button = screen.getByRole("button", { name: "Speichern" });

    expect(button).toBeDisabled();
    expect(button).not.toHaveAttribute("aria-disabled");
  });
});
