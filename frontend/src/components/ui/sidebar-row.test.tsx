import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { settingsConcept } from "~/lib/concepts/settings";
import { PortalSidebarGroup, PortalSidebarLink } from "./portal-sidebar";
import { sidebarRowClasses } from "./sidebar-geometry";
import {
  SidebarRowButton,
  SidebarRowLink,
  SidebarSectionHeading,
} from "./sidebar-row";

const ICON = <svg data-testid="icon" aria-hidden="true" />;
const BADGE = {
  count: 3,
  tone: "orange",
  ariaLabel: "3 offene Anfragen",
} as const;

describe("SidebarRowLink (#5975)", () => {
  it("marks the current page and keeps the shared row grid", () => {
    render(<SidebarRowLink href="/a" icon={ICON} label="Räume" active />);

    const link = screen.getByRole("link", { name: "Räume" });
    expect(link).toHaveAttribute("aria-current", "page");
    expect(link).toHaveAttribute("data-active", "true");
    expect(link.className).toBe(sidebarRowClasses({ isActive: true }));
  });

  it("shows the count beside the label while expanded", () => {
    render(
      <SidebarRowLink
        href="/a"
        icon={ICON}
        label="Anfragen"
        active={false}
        badge={BADGE}
      />,
    );

    expect(screen.getAllByLabelText("3 offene Anfragen")).toHaveLength(1);
    expect(screen.getByText("Anfragen")).toBeInTheDocument();
  });

  it("names the row in the rail and hides the fading count from screen readers", () => {
    render(
      <SidebarRowLink
        href="/a"
        icon={ICON}
        label="Abrechnung"
        collapsedLabel="Abrechnung, nicht gebucht"
        active={false}
        badge={BADGE}
        collapsed
        labelsMounted={false}
        labelsVisible={false}
      />,
    );

    const link = screen.getByRole("link", {
      name: "Abrechnung, nicht gebucht",
    });
    expect(link).toHaveAttribute("title", "Abrechnung");
    expect(screen.queryByText("Abrechnung")).not.toBeInTheDocument();
    const railBadge = screen.getByLabelText("3 offene Anfragen");
    expect(railBadge.parentElement).toHaveAttribute("aria-hidden", "false");
  });

  it("opens targets outside the app in a new tab", () => {
    render(
      <SidebarRowLink
        href="/help"
        icon={ICON}
        label="Hilfe"
        active={false}
        newTab
      />,
    );

    expect(screen.getByRole("link", { name: "Hilfe" })).toHaveAttribute(
      "target",
      "_blank",
    );
  });
});

describe("SidebarRowButton", () => {
  it("is a plain button row, never a form submit", () => {
    render(<SidebarRowButton icon={ICON} label="Abmelden" />);

    expect(screen.getByRole("button", { name: "Abmelden" })).toHaveAttribute(
      "type",
      "button",
    );
  });
});

describe("Portal rows use the tenant geometry (#5975)", () => {
  it("renders the same 40px row on every desktop width", () => {
    render(
      <ul>
        <PortalSidebarLink
          href="/settings"
          label="Einstellungen"
          concept={settingsConcept}
          active={false}
        />
      </ul>,
    );

    const link = screen.getByRole("link", { name: "Einstellungen" });
    expect(link.className).toBe(sidebarRowClasses({ isActive: false }));
    expect(link.className).not.toMatch(/\b(lg|xl):/);
  });

  it("titles a portal group in sentence case, like the tenant sections", () => {
    render(
      <ul>
        <PortalSidebarGroup label="Abrechnung">
          <li>Läufe</li>
        </PortalSidebarGroup>
      </ul>,
    );

    const heading = screen.getByText("Abrechnung");
    expect(heading.className).not.toContain("uppercase");
    render(<SidebarSectionHeading>Bereiche</SidebarSectionHeading>);
    expect(heading.className).toBe(screen.getByText("Bereiche").className);
  });
});
