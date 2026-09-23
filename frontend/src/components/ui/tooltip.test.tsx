import { fireEvent, render, screen } from "@testing-library/react";
import Link from "next/link";
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

  it("uses an interactive child as the only keyboard target", () => {
    render(
      <Tooltip content="Hilfe zu dieser Seite" asChild>
        <Link href="/help">Hilfe</Link>
      </Tooltip>,
    );

    const link = screen.getByRole("link", { name: "Hilfe" });
    const bubble = screen.getByRole("tooltip");

    expect(link).toHaveAttribute("aria-describedby", bubble.id);
    expect(link.parentElement).not.toHaveAttribute("tabindex");
  });

  it("stays dismissed on Escape while the pointer remains over the trigger", () => {
    render(
      <Tooltip content="Hilfe zu dieser Seite" asChild>
        <Link href="/help">Hilfe</Link>
      </Tooltip>,
    );

    const link = screen.getByRole("link", { name: "Hilfe" });
    const bubble = screen.getByRole("tooltip");
    const wrapper = link.parentElement;
    if (!wrapper) throw new Error("Tooltip wrapper was not rendered");

    fireEvent.pointerEnter(wrapper);
    link.focus();
    fireEvent.keyDown(link, { key: "Escape" });

    expect(bubble).toHaveStyle({ visibility: "hidden", opacity: 0 });
    expect(link).not.toHaveFocus();
  });
});
