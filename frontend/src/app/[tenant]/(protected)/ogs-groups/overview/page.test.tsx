import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { NavigationEducationalGroup } from "~/lib/usercontext-helpers";
import GroupOverviewPage from "./page";

const { supervision, replace } = vi.hoisted(() => ({
  supervision: {
    isLoadingGroups: false,
    groupsLoadFailed: false,
    groups: [] as NavigationEducationalGroup[],
    refresh: vi.fn(),
  },
  replace: vi.fn(),
}));

vi.mock("~/lib/supervision-context", () => ({
  useSupervision: () => supervision,
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: vi.fn(), replace, back: vi.fn() }),
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenantSafe: () => null,
  useTenantSlugSafe: () => null,
  useTenantRoutingModeSafe: () => "subdomain",
}));

vi.mock("~/components/auth/role-guard", () => ({
  RoleGuard: ({ children }: { children: ReactNode }) => children,
}));

vi.mock("~/components/tenant/open-care-mode-guard", () => ({
  OpenCareModeGuard: ({ children }: { children: ReactNode }) => children,
}));

const mondgruppe: NavigationEducationalGroup = {
  id: "1",
  name: "Mondgruppe",
  is_personal: true,
  room_name: "Raum 101",
  leaders: ["Anna Berg", "Carla Dorn"],
};
const sonnengruppe: NavigationEducationalGroup = {
  id: "2",
  name: "Sonnengruppe",
  is_personal: true,
  via_substitution: true,
  room_name: "Raum 102",
};
const ahorn: NavigationEducationalGroup = {
  id: "3",
  name: "Ahorngruppe",
  is_personal: false,
  leaders: ["Bernd Ernst"],
};

function tileOf(name: string): HTMLElement {
  const link = screen.getByRole("link", { name: new RegExp(name) });
  return link;
}

describe("GroupOverviewPage (#4351)", () => {
  beforeEach(() => {
    supervision.isLoadingGroups = false;
    supervision.groupsLoadFailed = false;
    supervision.groups = [ahorn, mondgruppe, sonnengruppe];
    supervision.refresh.mockReset();
    replace.mockReset();
  });

  it("lists every visible group with the own groups first", () => {
    render(<GroupOverviewPage />);

    expect(
      screen.getByRole("heading", { level: 1, name: "Gruppen" }),
    ).toBeInTheDocument();
    const names = screen
      .getAllByRole("heading", { level: 2 })
      .map((heading) => heading.textContent);
    expect(names).toEqual(["Mondgruppe", "Sonnengruppe", "Ahorngruppe"]);
    expect(tileOf("Mondgruppe")).toHaveAttribute("href", "/ogs-groups?group=1");
    expect(tileOf("Ahorngruppe")).toHaveAttribute(
      "href",
      "/ogs-groups?group=3",
    );
  });

  it("marks the own group and a substitution, and nothing on a foreign group", () => {
    render(<GroupOverviewPage />);

    expect(
      within(tileOf("Mondgruppe")).getByText("Meine Gruppe"),
    ).toBeVisible();
    expect(
      within(tileOf("Sonnengruppe")).getByText("Vertretung"),
    ).toBeVisible();
    const foreign = tileOf("Ahorngruppe");
    expect(within(foreign).getByText("Ahorngruppe")).toBeVisible();
    expect(within(foreign).queryByText("Meine Gruppe")).toBeNull();
    expect(within(foreign).queryByText("Vertretung")).toBeNull();
  });

  it("names the Gruppenleitung only where one is assigned", () => {
    render(<GroupOverviewPage />);

    expect(
      within(tileOf("Mondgruppe")).getByText(
        "Gruppenleitung: Anna Berg, Carla Dorn",
      ),
    ).toBeVisible();
    expect(
      within(tileOf("Ahorngruppe")).getByText("Gruppenleitung: Bernd Ernst"),
    ).toBeVisible();
    const unled = tileOf("Sonnengruppe");
    expect(within(unled).getByText("Raum 102")).toBeVisible();
    expect(within(unled).queryByText(/Gruppenleitung/)).toBeNull();
  });

  it("opens the only visible group directly", () => {
    supervision.groups = [mondgruppe];
    render(<GroupOverviewPage />);

    expect(replace).toHaveBeenCalledWith("/ogs-groups?group=1");
  });

  it("stays on the selection while the groups load", () => {
    supervision.isLoadingGroups = true;
    supervision.groups = [];
    render(<GroupOverviewPage />);

    expect(
      screen.getByRole("heading", { level: 1, name: "Gruppen" }),
    ).toBeInTheDocument();
    expect(replace).not.toHaveBeenCalled();
  });

  it("says who assigns a group when there is none", () => {
    supervision.groups = [];
    render(<GroupOverviewPage />);

    expect(screen.getByText("Keine Gruppe sichtbar")).toBeInTheDocument();
    expect(replace).not.toHaveBeenCalled();
  });

  // Ein gescheiterter Abruf darf nicht wie „keine Gruppe“ aussehen.
  it("reports a failed load with a retry instead of the empty state", () => {
    supervision.groups = [];
    supervision.groupsLoadFailed = true;
    render(<GroupOverviewPage />);

    expect(
      screen.getByText("Die Gruppen konnten nicht geladen werden."),
    ).toBeInTheDocument();
    expect(screen.queryByText("Keine Gruppe sichtbar")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Erneut versuchen" }));
    expect(supervision.refresh).toHaveBeenCalledWith({
      force: true,
      groupsOnly: true,
    });
  });
});
