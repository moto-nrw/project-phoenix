import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { SettingsSections } from "~/components/settings/settings-nav-items";
import { settingsConcept } from "~/lib/concepts/settings";
import { MobileBottomNav } from "./mobile-bottom-nav";
import {
  StaffSettingsSidebarNav,
  staffSettingsReturnPathKey,
} from "./staff-settings-nav";

const nav = vi.hoisted(() => ({ search: "" }));
const tenant = vi.hoisted(() => ({ slug: null as string | null }));
const sections = vi.hoisted(() => ({
  value: null as SettingsSections | null,
}));

vi.mock("next/navigation", () => ({
  usePathname: () => "/settings",
  useSearchParams: () => new URLSearchParams(nav.search),
}));

vi.mock("~/lib/tenant-path", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/tenant-path")>()),
  useTenantAwarePath: () => (path: string) => path,
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => tenant.slug,
  useTenantRoutingModeSafe: () => "subdomain",
}));

vi.mock("~/components/settings/settings-nav-items", async (importOriginal) => ({
  ...(await importOriginal<
    typeof import("~/components/settings/settings-nav-items")
  >()),
  useSettingsSections: () => sections.value,
}));

function sectionList(count: number): SettingsSections {
  return {
    tabs: Array.from({ length: count }, (_, index) => ({
      id: `settings-s${index}`,
      label: `Bereich ${index}`,
      icon: settingsConcept,
    })),
    highlightTabId: null,
    highlightNeedsNfc: false,
  };
}

describe("staff settings navigation", () => {
  beforeEach(() => {
    sessionStorage.clear();
    nav.search = "";
    tenant.slug = null;
    sections.value = sectionList(2);
  });

  it("lists the back link and marks the section from the address", () => {
    nav.search = "tab=s1";
    render(
      <StaffSettingsSidebarNav collapsed={false} labelsMounted labelsVisible />,
    );

    expect(
      screen.getByRole("link", { name: "Zurück zur App" }),
    ).toHaveAttribute("href", "/");
    const active = screen.getByRole("link", { name: "Bereich 1" });
    expect(active).toHaveAttribute("aria-current", "page");
    expect(active).toHaveAttribute("href", "/settings?tab=s1");
    expect(screen.getByRole("link", { name: "Bereich 0" })).not.toHaveAttribute(
      "aria-current",
    );
  });

  it("names the rows of the collapsed rail", () => {
    render(
      <StaffSettingsSidebarNav
        collapsed
        labelsMounted={false}
        labelsVisible={false}
      />,
    );

    expect(screen.getByRole("link", { name: "Bereich 0" })).toHaveAttribute(
      "title",
      "Bereich 0",
    );
  });

  it("keeps the way back while the sections load", () => {
    sections.value = null;
    render(
      <StaffSettingsSidebarNav collapsed={false} labelsMounted labelsVisible />,
    );

    expect(
      screen.getByRole("link", { name: "Zurück zur App" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Bereich/ })).toBeNull();
  });

  it("keeps remembered routes separate for each tenant", () => {
    sessionStorage.setItem(
      staffSettingsReturnPathKey("schule-a"),
      "/schule-a/students",
    );
    sessionStorage.setItem(
      staffSettingsReturnPathKey("schule-b"),
      "/schule-b/rooms",
    );
    tenant.slug = "schule-b";

    render(
      <StaffSettingsSidebarNav collapsed={false} labelsMounted labelsVisible />,
    );

    expect(
      screen.getByRole("link", { name: "Zurück zur App" }),
    ).toHaveAttribute("href", "/schule-b/rooms");
  });

  it("follows on the phone, with the rest behind Mehr", () => {
    sections.value = sectionList(6);
    render(<MobileBottomNav />);

    expect(
      screen.getByRole("link", { name: "Zurück zur App" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Bereich 2" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Bereich 3" })).toBeNull();
    expect(screen.getByRole("button", { name: "Mehr" })).toBeInTheDocument();
  });
});
