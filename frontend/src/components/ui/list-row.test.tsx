import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { FramedList } from "./framed-list";
import { ListRow } from "./list-row";
import { ListRowLink } from "./list-row-link";
import { ListRowSkeleton } from "./skeleton";

describe("ListRow", () => {
  it("is a plain display row without a target", () => {
    render(<ListRow trailing={<span>12:00</span>}>Mia</ListRow>);

    expect(screen.getByText("Mia")).toBeInTheDocument();
    expect(screen.getByText("12:00")).toBeInTheDocument();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("is a link with href, and the trailing content belongs to it", () => {
    render(
      <ListRow href="/students/1" aria-label="Mia öffnen" trailing="12:00">
        Mia
      </ListRow>,
    );

    const link = screen.getByRole("link", { name: "Mia öffnen" });
    expect(link).toHaveAttribute("href", "/students/1");
    expect(link).toHaveTextContent("12:00");
  });

  it("is a button with onClick", () => {
    const onClick = vi.fn();
    render(<ListRow onClick={onClick}>Mia</ListRow>);

    fireEvent.click(screen.getByRole("button", { name: "Mia" }));

    expect(onClick).toHaveBeenCalledOnce();
  });
});

describe("ListRowSkeleton", () => {
  it("shows no text and stays out of the accessibility tree of its region", () => {
    const { container } = render(<ListRowSkeleton />);

    expect(container).toHaveTextContent("");
    expect(container.querySelectorAll(".animate-pulse")).toHaveLength(2);
  });
});

describe("FramedList", () => {
  it("renders its entries as a list", () => {
    render(
      <FramedList>
        <li>1a</li>
        <li>1b</li>
      </FramedList>,
    );

    const list = screen.getByRole("list");
    expect(list).toContainElement(screen.getByText("1a"));
    expect(screen.getAllByRole("listitem")).toHaveLength(2);
  });

  it("keeps the header outside the list", () => {
    render(
      <FramedList header={<span>Alle auswählen</span>}>
        <li>Mia</li>
      </FramedList>,
    );

    expect(screen.getByText("Alle auswählen")).toBeInTheDocument();
    expect(screen.getByRole("list")).not.toContainElement(
      screen.getByText("Alle auswählen"),
    );
  });
});

describe("ListRowLink", () => {
  it("links with title, subtitle and badge", () => {
    render(
      <ListRowLink
        href="/staff/1"
        title="Anna Beck"
        subtitle="Klasse 1a"
        badge={<span>Neu</span>}
      />,
    );

    const link = screen.getByRole("link", { name: /Anna Beck/ });
    expect(link).toHaveAttribute("href", "/staff/1");
    expect(link).toHaveTextContent("Klasse 1a");
    expect(link).toHaveTextContent("Neu");
  });

  it("is a button without href and can be disabled", () => {
    const onClick = vi.fn();
    render(<ListRowLink title="Verlauf" onClick={onClick} disabled />);

    const button = screen.getByRole("button", { name: "Verlauf" });
    expect(button).toBeDisabled();
    fireEvent.click(button);
    expect(onClick).not.toHaveBeenCalled();
  });

  it("hides the arrow from assistive technology and can omit it", () => {
    const { container, rerender } = render(
      <ListRowLink href="/a" title="Mit Pfeil" />,
    );
    expect(container.querySelector("svg")).toHaveAttribute(
      "aria-hidden",
      "true",
    );

    rerender(<ListRowLink href="/a" title="Ohne Pfeil" chevron={false} />);
    expect(container.querySelector("svg")).toBeNull();
  });
});

describe("ListRowLink states (#5593)", () => {
  it("marks the open selection with aria-current, green fill and a green arrow", () => {
    const { container, rerender } = render(
      <ListRowLink href="/database/groups/1" title="Bärengruppe" selected />,
    );
    const link = screen.getByRole("link", { name: "Bärengruppe" });
    expect(link).toHaveAttribute("aria-current", "true");
    expect(link.className).toContain("bg-moto-green-soft/60");
    expect(screen.getByText("Bärengruppe").className).toContain(
      "font-semibold",
    );
    expect(container.querySelector("svg")?.getAttribute("class")).toContain(
      "text-moto-green",
    );

    rerender(<ListRowLink href="/database/groups/1" title="Bärengruppe" />);
    expect(link).not.toHaveAttribute("aria-current");
    expect(link.className).not.toContain("bg-moto-green-soft");
  });

  it("sets aria-current on a button row too", () => {
    render(<ListRowLink onClick={() => {}} title="Raum 1" selected />);
    expect(screen.getByRole("button", { name: "Raum 1" })).toHaveAttribute(
      "aria-current",
      "true",
    );
  });

  it("draws the accent edge and tints only on request", () => {
    const { rerender } = render(
      <ListRowLink onClick={() => {}} title="Frühbetreuung" accent="#DC2626" />,
    );
    const row = screen.getByRole("button", { name: "Frühbetreuung" });
    expect(row.className).toContain("border-l-4");
    expect(row.style.borderLeftColor.toUpperCase()).toBe("#DC2626");
    expect(row.style.backgroundColor).toBe("");

    rerender(
      <ListRowLink
        onClick={() => {}}
        title="Frühbetreuung"
        accent="#DC2626"
        accentTint
      />,
    );
    // Dieselbe Tönung wie ein eigener Block im Plan: Hex mit Alpha 1A.
    expect(row.style.backgroundColor.toUpperCase()).toBe("#DC26261A");
  });

  it("has no edge without accent", () => {
    render(<ListRowLink onClick={() => {}} title="Ohne Kante" />);
    const row = screen.getByRole("button", { name: "Ohne Kante" });
    expect(row.className).not.toContain("border-l-4");
    expect(row.getAttribute("style")).toBeNull();
  });

  it("stacks overline, title and preview and stresses unread rows", () => {
    render(
      <ListRowLink
        href="/parents/messages/1"
        overline="Für Baran Amini"
        meta="08.11.2026, 02:35"
        metaLabel="8. November, 02:35"
        title="OGS-Team"
        subtitle="Bis morgen"
        badge={<span>2</span>}
        unread
      />,
    );
    const link = screen.getByRole("link", {
      name: /Für Baran Amini.*OGS-Team/,
    });
    expect(link).toHaveTextContent("Bis morgen");
    expect(screen.getByText("OGS-Team").className).toContain("font-bold");
    expect(screen.getByLabelText("8. November, 02:35").className).toContain(
      "text-moto-blue",
    );
    expect(screen.getByText("Bis morgen").className).toContain("text-gray-700");
    expect(link).toHaveTextContent("2");
  });

  it("keeps read stacked rows calm", () => {
    render(
      <ListRowLink
        href="/a"
        overline="Für Aida Amini"
        meta="gestern"
        title="OGS-Team"
        subtitle="Danke"
      />,
    );
    expect(screen.getByText("OGS-Team").className).toContain("font-semibold");
    expect(screen.getByText("gestern").className).toContain("text-gray-500");
    expect(screen.getByText("Danke").className).toContain("text-gray-500");
  });
});
