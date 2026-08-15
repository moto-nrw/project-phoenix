import { fireEvent, render, screen } from "@testing-library/react";
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
      "Kinder",
      "Nachrichten",
      "Kalender",
      "Aus der OGS",
      "Essensplan",
      "Benachrichtigungen",
      "Neue Anmeldung",
    ]) {
      expect(screen.getByText(label)).toBeVisible();
    }

    const targets = container.querySelectorAll("[data-parent-nav-item]");
    expect(targets.length).toBeGreaterThanOrEqual(8);
    for (const target of targets) {
      expect(target.querySelector("svg")).toBeInTheDocument();
    }
  });

  it("marks the current target as active and no other", () => {
    pathnameMock.mockReturnValue("/parents/children");
    const { container } = renderSidebar();

    const active = container.querySelectorAll('[data-active="true"]');
    expect(active).toHaveLength(1);
    expect(active[0]).toHaveAttribute("data-parent-nav-item", "children");
  });

  it("is not in the document below 1024px", () => {
    mockedUseMediaQuery.mockReturnValue(true);
    renderSidebar();

    expect(screen.queryByRole("navigation")).not.toBeInTheDocument();
    expect(screen.queryByText("Start")).not.toBeInTheDocument();
  });

  it("pins account and sign-out to the bottom", () => {
    renderSidebar();

    const account = screen.getByRole("link", { name: /Einstellungen/ });
    expect(account).toHaveAttribute("href", "/parents/settings");

    fireEvent.click(screen.getByRole("button", { name: "Abmelden" }));
    expect(logoutMock).toHaveBeenCalledTimes(1);
  });

  it("hides Aus der OGS and Essensplan when no school offers them", () => {
    renderSidebar({ gates: { news: false, mealPlan: false } });

    expect(screen.queryByText("Aus der OGS")).not.toBeInTheDocument();
    expect(screen.queryByText("Essensplan")).not.toBeInTheDocument();
  });

  it("shows the unread messages count next to Nachrichten", () => {
    const { container } = renderSidebar({ badges: { messages: 7, news: 0 } });

    expect(
      container.querySelector('[data-parent-nav-item="messages"]'),
    ).toHaveTextContent("7");
  });
});
