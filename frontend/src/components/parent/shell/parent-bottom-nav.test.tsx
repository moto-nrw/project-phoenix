import { fireEvent, render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import deMessages from "~/i18n/messages/de.json";
import { useMediaQuery } from "~/lib/hooks/use-media-query";
import { ParentBottomNav } from "./parent-bottom-nav";

const pathnameMock = vi.fn(() => "/parents");

vi.mock("next/navigation", () => ({
  usePathname: () => pathnameMock(),
}));

// parentPath() reads NEXT_PUBLIC_PARENTS_HOSTNAME and throws when it is unset.
vi.mock("~/lib/parent-url", () => ({
  parentPath: (path: string) => path,
}));

vi.mock("~/lib/hooks/use-media-query", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("~/lib/hooks/use-media-query")>();
  return { ...actual, useMediaQuery: vi.fn(() => true) };
});

vi.mock("~/lib/shell-auth-context", () => ({
  useShellAuth: () => ({
    mode: "parent",
    homeUrl: "/parents",
    profileUrl: "/parents/settings",
    logout: vi.fn(),
  }),
}));

vi.mock("~/components/parent/language-switcher", () => ({
  LanguageSwitcher: () => <div data-testid="language-switcher" />,
}));

const mockedUseMediaQuery = vi.mocked(useMediaQuery);

function renderNav(
  overrides: Partial<React.ComponentProps<typeof ParentBottomNav>> = {},
) {
  return render(
    <NextIntlClientProvider locale="de" messages={deMessages}>
      <ParentBottomNav
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
  mockedUseMediaQuery.mockReturnValue(true);
});

describe("ParentBottomNav", () => {
  it("renders the four daily targets plus Mehr, each with a visible label", () => {
    renderNav();

    for (const label of [
      "Start",
      "Kinder",
      "Nachrichten",
      "Kalender",
      "Mehr",
    ]) {
      expect(screen.getByText(label)).toBeVisible();
    }
  });

  it("gives every target an icon next to its label", () => {
    const { container } = renderNav();

    const targets = container.querySelectorAll("[data-parent-nav-item]");
    expect(targets).toHaveLength(5);
    for (const target of targets) {
      expect(target.querySelector("svg")).toBeInTheDocument();
    }
  });

  it("marks the current target as active and no other", () => {
    pathnameMock.mockReturnValue("/parents/messages");
    const { container } = renderNav();

    const active = container.querySelectorAll('[data-active="true"]');
    expect(active).toHaveLength(1);
    expect(active[0]).toHaveAttribute("data-parent-nav-item", "messages");
  });

  it("treats the portal root as the Start target", () => {
    pathnameMock.mockReturnValue("/");
    const { container } = renderNav();

    expect(
      container.querySelector('[data-parent-nav-item="start"]'),
    ).toHaveAttribute("data-active", "true");
  });

  it("is not in the document from 1024px upwards", () => {
    mockedUseMediaQuery.mockReturnValue(false);
    renderNav();

    expect(screen.queryByRole("navigation")).not.toBeInTheDocument();
    expect(screen.queryByText("Start")).not.toBeInTheDocument();
  });

  it("shows the unread messages count on the Nachrichten target", () => {
    const { container } = renderNav({ badges: { messages: 3, news: 0 } });

    const messages = container.querySelector(
      '[data-parent-nav-item="messages"]',
    );
    expect(messages).toHaveTextContent("3");
  });

  it("adds the unread news count onto Mehr so a notice stays visible", () => {
    const { container } = renderNav({ badges: { messages: 0, news: 2 } });

    const more = container.querySelector('[data-parent-nav-item="more"]');
    expect(more).toHaveTextContent("2");
  });

  it("opens the Mehr sheet with the secondary targets", () => {
    renderNav();

    fireEvent.click(screen.getByText("Mehr"));

    expect(screen.getByText("Neuigkeiten")).toBeVisible();
    expect(screen.getByText("Essensplan")).toBeVisible();
    expect(screen.getByText("Benachrichtigungen")).toBeVisible();
    expect(screen.getByText("Neue Anmeldung")).toBeVisible();
    expect(screen.getByText("Abmelden")).toBeVisible();
  });

  it("hides Neuigkeiten and Essensplan when no school offers them", () => {
    renderNav({ gates: { news: false, mealPlan: false } });

    fireEvent.click(screen.getByText("Mehr"));

    expect(screen.queryByText("Neuigkeiten")).not.toBeInTheDocument();
    expect(screen.queryByText("Essensplan")).not.toBeInTheDocument();
    expect(screen.getByText("Benachrichtigungen")).toBeVisible();
  });
  it("haelt die Eintraege des Mehr-Sheets auf 56 px und gibt Zielen einen Pfeil", () => {
    renderNav();

    fireEvent.click(screen.getByText("Mehr"));

    const newsItem = document.querySelector('[data-parent-nav-item="news"]');
    expect(newsItem).not.toBeNull();
    expect(newsItem!.className).toContain("min-h-14");
    expect(newsItem!.querySelector("svg")).not.toBeNull();
  });

  // Teilmenge von #2326: Produktfeedback gehoert nicht in eine Eltern-App.
  it("bietet kein Produktfeedback mehr an", () => {
    renderNav();

    fireEvent.click(screen.getByText("Mehr"));

    expect(screen.queryByText(/Feedback/)).not.toBeInTheDocument();
    expect(
      document.querySelector('[data-parent-nav-item="feedback"]'),
    ).toBeNull();
  });
});
