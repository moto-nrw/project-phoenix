import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Tooltip } from "./tooltip";

describe("Tooltip", () => {
  it("links the bubble to the trigger via aria-describedby", () => {
    render(<Tooltip content="Geplant 4 h · Soll 8 h">7,5 h</Tooltip>);

    const bubble = screen.getByRole("tooltip");
    expect(bubble).toHaveTextContent("Geplant 4 h · Soll 8 h");

    const trigger = screen.getByText("7,5 h").closest("[aria-describedby]");
    expect(trigger).toHaveAttribute("aria-describedby", bubble.id);
  });

  it("keeps the trigger keyboard-focusable and dismisses on Escape", () => {
    render(<Tooltip content="Hinweis">Inhalt</Tooltip>);

    const trigger = screen
      .getByText("Inhalt")
      .closest<HTMLElement>("[tabindex]");

    if (!trigger) throw new Error("Tooltip trigger was not rendered");
    expect(trigger).toHaveAttribute("tabindex", "0");
    expect(trigger?.tagName).toBe("SPAN");
    trigger.focus();
    expect(trigger).toHaveFocus();
    fireEvent.keyDown(trigger, { key: "Escape" });
    expect(trigger).not.toHaveFocus();
  });
});
