import { act, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { Textarea } from "./textarea";

describe("Textarea label association", () => {
  it("uses an explicit id for the built-in label", () => {
    render(<Textarea id="answer" label="Antwort" />);

    expect(screen.getByLabelText("Antwort")).toHaveAttribute("id", "answer");
  });

  it("falls back to name when id is omitted", () => {
    render(<Textarea name="reason" label="Begründung" />);

    expect(screen.getByLabelText("Begründung")).toHaveAttribute("id", "reason");
  });

  it("keeps name for form submission when id differs", () => {
    render(<Textarea id="answer" name="response" label="Antwort" />);

    expect(screen.getByLabelText("Antwort")).toHaveAttribute(
      "name",
      "response",
    );
  });
});

describe("Textarea hint", () => {
  it("describes the field with the hint before the error", () => {
    render(
      <Textarea
        id="note"
        label="Vermerk"
        hint="Steht auf dem Beitrag."
        error="Bitte geben Sie den Vermerk an."
      />,
    );

    const field = screen.getByLabelText("Vermerk");
    expect(field).toHaveAccessibleDescription(
      "Steht auf dem Beitrag. Bitte geben Sie den Vermerk an.",
    );
    const hint = screen.getByText("Steht auf dem Beitrag.");
    const error = screen.getByText("Bitte geben Sie den Vermerk an.");
    expect(
      hint.compareDocumentPosition(error) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });
});

describe("Textarea without id or name", () => {
  it("still names the field with its label", () => {
    render(<Textarea label="Notiz" hint="Sehen nur Sie." />);

    expect(
      screen.getByRole("textbox", { name: "Notiz" }),
    ).toHaveAccessibleDescription("Sehen nur Sie.");
  });
});

describe("Textarea autoGrow", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  function mockContentHeight(height: number) {
    return vi
      .spyOn(HTMLElement.prototype, "scrollHeight", "get")
      .mockReturnValue(height);
  }

  it("takes the height of its content", () => {
    mockContentHeight(64);

    render(<Textarea aria-label="Notiz" autoGrow value="a" readOnly />);

    const field = screen.getByRole("textbox", { name: "Notiz" });
    expect(field.style.height).toBe("64px");
    expect(field.style.overflowY).toBe("hidden");
  });

  it("follows the value and scrolls inside past the cap", () => {
    const contentHeight = mockContentHeight(64);
    const { rerender } = render(
      <Textarea aria-label="Notiz" autoGrow value="a" readOnly />,
    );

    contentHeight.mockReturnValue(400);
    rerender(<Textarea aria-label="Notiz" autoGrow value="b" readOnly />);

    const field = screen.getByRole("textbox", { name: "Notiz" });
    expect(field.style.height).toBe("160px");
    expect(field.style.overflowY).toBe("auto");
  });

  it("re-measures when the visible area changes", () => {
    const original = window.visualViewport;
    let visibleHeight = 280;
    const viewport = new EventTarget();
    Object.defineProperty(viewport, "height", { get: () => visibleHeight });
    Object.defineProperty(window, "visualViewport", {
      value: viewport,
      configurable: true,
    });
    mockContentHeight(400);

    render(<Textarea aria-label="Notiz" autoGrow value="a" readOnly />);
    const field = screen.getByRole("textbox", { name: "Notiz" });
    expect(field.style.height).toBe("93px");

    visibleHeight = 640;
    act(() => {
      viewport.dispatchEvent(new Event("resize"));
    });
    expect(field.style.height).toBe("160px");

    Object.defineProperty(window, "visualViewport", {
      value: original,
      configurable: true,
    });
  });

  it("leaves the height alone without the prop", () => {
    mockContentHeight(64);

    render(<Textarea aria-label="Notiz" value="a" readOnly />);

    expect(screen.getByRole("textbox", { name: "Notiz" }).style.height).toBe(
      "",
    );
  });

  it("hands the caller its ref", () => {
    const ref: { current: HTMLTextAreaElement | null } = { current: null };

    render(<Textarea aria-label="Notiz" autoGrow ref={ref} />);

    expect(ref.current).toBe(screen.getByRole("textbox", { name: "Notiz" }));
  });
});
