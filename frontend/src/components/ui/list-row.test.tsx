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

  it("does not fire when disabled", () => {
    const onClick = vi.fn();
    render(
      <ListRow onClick={onClick} disabled>
        Mia
      </ListRow>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Mia" }));

    expect(onClick).not.toHaveBeenCalled();
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
      <FramedList aria-label="Klassen">
        <li>1a</li>
        <li>1b</li>
      </FramedList>,
    );

    const list = screen.getByRole("list", { name: "Klassen" });
    expect(list).toContainElement(screen.getByText("1a"));
    expect(screen.getAllByRole("listitem")).toHaveLength(2);
  });

  it("keeps the header outside the list", () => {
    render(
      <FramedList aria-label="Kinder" header={<span>Alle auswählen</span>}>
        <li>Mia</li>
      </FramedList>,
    );

    expect(screen.getByText("Alle auswählen")).toBeInTheDocument();
    expect(screen.getByRole("list", { name: "Kinder" })).not.toContainElement(
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
