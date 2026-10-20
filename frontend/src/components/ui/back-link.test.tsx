import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { BackAction, BackLink } from "./back-link";

describe("BackLink", () => {
  it("is a link to the given path, named after its target", () => {
    render(
      <BackLink href="/school/nachrichten" label="Zurück zu Nachrichten" />,
    );

    expect(
      screen.getByRole("link", { name: "Zurück zu Nachrichten" }),
    ).toHaveAttribute("href", "/school/nachrichten");
  });

  // Der alte Zurück-Knopf verschwand ab `md` bzw. `lg` (#4364).
  it("stays visible on every width and hides its arrow from screen readers", () => {
    render(<BackLink href="/rooms" label="Zurück zu den Räumen" />);

    const link = screen.getByRole("link");
    expect(link.className).not.toMatch(/(^|\s)(sm|md|lg|xl):hidden/);
    expect(link.querySelector("svg")).toHaveAttribute("aria-hidden", "true");
  });
});

describe("BackAction", () => {
  it("looks like BackLink and runs its handler", () => {
    const onClick = vi.fn();
    render(<BackAction onClick={onClick} label="Zurück zur Übersicht" />);
    render(<BackLink href="/rooms" label="Zurück zu den Räumen" />);

    const action = screen.getByRole("button", { name: "Zurück zur Übersicht" });
    expect(action).toHaveAttribute("type", "button");
    expect(action.className).toBe(screen.getByRole("link").className);

    fireEvent.click(action);
    expect(onClick).toHaveBeenCalledOnce();
  });

  it("does nothing while disabled", () => {
    const onClick = vi.fn();
    render(
      <BackAction onClick={onClick} label="Zurück zur Übersicht" disabled />,
    );

    fireEvent.click(screen.getByRole("button"));
    expect(onClick).not.toHaveBeenCalled();
  });
});
