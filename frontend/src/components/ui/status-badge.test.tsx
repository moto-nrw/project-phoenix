import { render, screen } from "@testing-library/react";
import { RefreshCw } from "lucide-react";
import { CheckCircleIcon } from "@phosphor-icons/react/ssr";
import { describe, expect, it } from "vitest";

import { StatusBadge } from "./status-badge";

describe("StatusBadge", () => {
  it("renders the label", () => {
    render(<StatusBadge label="Bestätigt" tone="green" />);
    expect(screen.getByText("Bestätigt")).toBeInTheDocument();
  });

  it("applies the tone colors via inline styles", () => {
    render(<StatusBadge label="Abgelehnt" tone="red" />);
    const label = screen.getByText("Abgelehnt");
    const pill = label.closest("span[class*='rounded-full']");
    expect(pill).not.toBeNull();
    expect((pill as HTMLElement).style.color).toBe("#B91C1C");
  });

  it("renders the label as the pill's only content, without a decorative dot (#2476)", () => {
    render(<StatusBadge label="Warteliste" tone="orange" />);
    const pill = screen.getByText("Warteliste");
    expect(pill.children).toHaveLength(0);
    expect(pill.textContent).toBe("Warteliste");
    // No spacer left behind where the dot used to sit.
    expect(pill.className).not.toMatch(/\bgap-/);
  });

  it("passes the title through to the pill element", () => {
    render(
      <StatusBadge label="Feiertag" tone="gray" title="Tag der Einheit" />,
    );
    expect(screen.getByText("Feiertag")).toHaveAttribute(
      "title",
      "Tag der Einheit",
    );
  });

  it("supports an icon and additional accessible context", () => {
    render(
      <StatusBadge
        label="Automatisch"
        tone="green"
        icon={RefreshCw}
        accessibleLabel="Teilnehmerpflege: "
        compact
      />,
    );

    expect(screen.getByText("Automatisch")).toHaveTextContent(
      "Teilnehmerpflege: Automatisch",
    );
  });

  // #5584: Entschuldigt und Klassenfahrt behalten ihre eigene Farbe als Pille.
  it.each([
    ["purple", "Entschuldigt", "#6B21A8"],
    ["cyan", "Klassenfahrt", "#155E75"],
  ] as const)("renders the %s tone in its own colour", (tone, label, text) => {
    render(<StatusBadge label={label} tone={tone} />);
    expect(screen.getByText(label).style.color).toBe(text);
  });

  it("renders the outline tone with a frame and no tint", () => {
    render(<StatusBadge label="Entwurf" tone="outline" />);
    const pill = screen.getByText("Entwurf");
    expect(pill.style.backgroundColor).toBe("");
    expect(pill.className).toMatch(/\bborder\b/);
  });

  it("renders size xs in 12 px without capitals", () => {
    render(<StatusBadge label="Ausflug" tone="cyan" size="xs" />);
    const pill = screen.getByText("Ausflug");
    expect(pill.className).toMatch(/\btext-xs\b/);
    expect(pill.className).not.toMatch(/\buppercase\b/);
    expect(pill.className).toMatch(/\bpx-1\.5\b/);
  });

  it("accepts a rendered icon element as well as a Lucide component", () => {
    render(
      <StatusBadge
        label="Erledigt"
        tone="green"
        icon={<CheckCircleIcon data-testid="phosphor" />}
      />,
    );
    const pill = screen.getByText("Erledigt");
    const icon = screen.getByTestId("phosphor");
    expect(pill).toContainElement(icon);
    expect(icon.closest("[aria-hidden='true']")).not.toBeNull();
  });

  it("renders any other icon node as hidden decoration instead of crashing", () => {
    render(
      <StatusBadge
        label="Geprüft"
        tone="blue"
        icon={[<CheckCircleIcon key="a" data-testid="list-icon" />]}
      />,
    );
    expect(screen.getByText("Geprüft")).toContainElement(
      screen.getByTestId("list-icon"),
    );
  });

  it("shortens a long label and keeps it whole in the tooltip", () => {
    const label = "Kostenübernahme: Fachdienst Jugend des Landkreises";
    render(<StatusBadge label={label} tone="blue" truncate />);
    const text = screen.getByText(label);
    expect(text).toHaveClass("truncate");
    expect(text.parentElement).toHaveAttribute("title", label);
  });

  it("lets a long label break instead of overflowing", () => {
    render(<StatusBadge label="Teilweise gutgeschrieben" tone="blue" wrap />);
    const badge = screen.getByText("Teilweise gutgeschrieben");
    expect(badge).toHaveClass("whitespace-normal");
    expect(badge).not.toHaveClass("whitespace-nowrap");
  });
});
