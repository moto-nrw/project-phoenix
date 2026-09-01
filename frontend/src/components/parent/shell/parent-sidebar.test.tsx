import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { UserIcon, UsersThreeIcon } from "@phosphor-icons/react";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import deMessages from "~/i18n/messages/de.json";
import { useMediaQuery } from "~/lib/hooks/use-media-query";
import { ParentSidebar } from "./parent-sidebar";

const pathnameMock = vi.fn(() => "/parents");
const logoutMock = vi.fn();

vi.mock("next/navigation", () => ({
  usePathname: () => pathnameMock(),
}));

vi.mock("~/lib/parent-url", () => ({
  parentPath: (path: string) => path,
}));

vi.mock("~/lib/hooks/use-media-query", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("~/lib/hooks/use-media-query")>();
  return { ...actual, useMediaQuery: vi.fn(() => false) };
});

vi.mock("~/lib/shell-auth-context", () => ({
  useShellAuth: () => ({
    mode: "parent",
    homeUrl: "/parents",
    profileUrl: "/parents/settings",
    logout: logoutMock,
  }),
}));

const mockedUseMediaQuery = vi.mocked(useMediaQuery);

function renderSidebar(
  overrides: Partial<React.ComponentProps<typeof ParentSidebar>> = {},
) {
  return render(
    <NextIntlClientProvider locale="de" messages={deMessages}>
      <ParentSidebar
        badges={{ messages: 0, news: 0 }}
        gates={{ news: true, mealPlan: true }}
        childCount={2}
        {...overrides}
      />
    </NextIntlClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  pathnameMock.mockReturnValue("/parents");
  mockedUseMediaQuery.mockReturnValue(false);
});

describe("ParentSidebar", () => {
  it("renders every target with an icon and a visible label", () => {
    const { container } = renderSidebar();

    for (const label of [
      "Start",
      "Meine Kinder",
      "Nachrichten",
      "Kalender",
      "Elternbriefe",
      "Mittagessen",
      "Neue Anmeldung",
      "Einstellungen",
      "Abmelden",
    ]) {
      expect(screen.getByText(label)).toBeVisible();
    }
    expect(screen.getAllByText("Einstellungen")).toHaveLength(1);

    const targets = container.querySelectorAll("[data-parent-nav-item]");
    expect(targets.length).toBeGreaterThanOrEqual(8);
    for (const target of targets) {
      expect(target.querySelector("svg")).toBeInTheDocument();
    }
  });

  it("uses the singular label for exactly one linked child", () => {
    pathnameMock.mockReturnValue("/parents/children");
    const { container } = renderSidebar({ childCount: 1 });

    expect(screen.getByRole("link", { name: "Mein Kind" })).toBeVisible();
    expect(screen.queryByText("Meine Kinder")).not.toBeInTheDocument();
    const expected = render(<UserIcon weight="duotone" />);
    expect(
      container.querySelector('[data-parent-nav-item="children"] svg')
        ?.innerHTML,
    ).toBe(expected.container.querySelector("svg")?.innerHTML);
  });

  it("marks the current target as active and no other", () => {
    pathnameMock.mockReturnValue("/parents/children");
    const { container } = renderSidebar();

    const active = container.querySelectorAll('[data-active="true"]');
    expect(active).toHaveLength(1);
    expect(active[0]).toHaveAttribute("data-parent-nav-item", "children");
    const expected = render(<UsersThreeIcon weight="duotone" />);
    expect(active[0]?.querySelector("svg")?.innerHTML).toBe(
      expected.container.querySelector("svg")?.innerHTML,
    );
  });

  it("marks the pinned settings target as active", () => {
    pathnameMock.mockReturnValue("/settings");
    const { container } = renderSidebar();

    const active = container.querySelectorAll('[data-active="true"]');
    expect(active).toHaveLength(1);
    expect(active[0]).toHaveAttribute("data-parent-nav-item", "settings");
  });

  it("pins enrollment with settings and logout instead of the main navigation", () => {
    renderSidebar();

    const mainNav = screen.getByRole("navigation", { name: "Hauptnavigation" });
    const accountNav = screen.getByRole("navigation", {
      name: "Kontonavigation",
    });

    expect(
      within(mainNav).queryByText("Neue Anmeldung"),
    ).not.toBeInTheDocument();
    expect(within(accountNav).getByText("Einstellungen")).toBeVisible();
    expect(within(accountNav).getByText("Neue Anmeldung")).toBeVisible();
    expect(within(accountNav).getByText("Abmelden")).toBeVisible();
  });

  it("uses CSS to hide itself below 1024px without a hydration swap", () => {
    mockedUseMediaQuery.mockReturnValue(true);
    const { container } = renderSidebar();

    expect(container.querySelector("aside")).toHaveClass("hidden", "lg:block");
    expect(
      screen.getByRole("navigation", { name: "Hauptnavigation" }),
    ).toBeInTheDocument();
  });

  it("asks for confirmation before signing out from the pinned navigation", async () => {
    renderSidebar();

    expect(screen.getByRole("link", { name: "Einstellungen" })).toHaveAttribute(
      "href",
      "/parents/settings",
    );

    fireEvent.click(screen.getByRole("button", { name: "Abmelden" }));

    const dialog = screen.getByRole("dialog");
    expect(
      within(dialog).getByText(
        "Möchten Sie sich wirklich von Ihrem Konto abmelden?",
      ),
    ).toBeVisible();
    expect(logoutMock).not.toHaveBeenCalled();

    fireEvent.click(within(dialog).getByRole("button", { name: "Abmelden" }));
    await waitFor(() => expect(logoutMock).toHaveBeenCalledOnce());
  });

  it("hides Elternbriefe and Essensplan when no school offers them", () => {
    renderSidebar({ gates: { news: false, mealPlan: false } });

    expect(screen.queryByText("Elternbriefe")).not.toBeInTheDocument();
    expect(screen.queryByText("Essensplan")).not.toBeInTheDocument();
  });

  it("shows the unread messages count next to Nachrichten", () => {
    const { container } = renderSidebar({ badges: { messages: 7, news: 0 } });

    expect(
      container.querySelector('[data-parent-nav-item="messages"]'),
    ).toHaveTextContent("7");
  });
});
