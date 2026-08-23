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
import { ParentBottomNav } from "./parent-bottom-nav";

const pathnameMock = vi.fn(() => "/parents");
const logoutMock = vi.fn();

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
    logout: logoutMock,
  }),
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
        childCount={2}
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
  it("renders the four daily targets plus Mehr as named controls", () => {
    renderNav();

    expect(screen.getByRole("link", { name: "Start" })).toBeVisible();
    expect(screen.getByRole("link", { name: "Meine Kinder" })).toBeVisible();
    expect(screen.getByRole("link", { name: "Nachrichten" })).toBeVisible();
    expect(screen.getByRole("link", { name: "Kalender" })).toBeVisible();
    expect(screen.getByRole("button", { name: "Mehr" })).toBeVisible();
  });

  it("uses the singular label for exactly one linked child", () => {
    pathnameMock.mockReturnValue("/parents/children");
    const { container } = renderNav({ childCount: 1 });

    expect(screen.getByRole("link", { name: "Mein Kind" })).toBeVisible();
    expect(
      screen.queryByRole("link", { name: "Meine Kinder" }),
    ).not.toBeInTheDocument();
    const expected = render(<UserIcon weight="duotone" />);
    expect(
      container.querySelector('[data-parent-nav-item="children"] svg')
        ?.innerHTML,
    ).toBe(expected.container.querySelector("svg")?.innerHTML);
  });

  it("gives every target an icon next to its label", () => {
    const { container } = renderNav();

    const targets = container.querySelectorAll("[data-parent-nav-item]");
    expect(targets).toHaveLength(5);
    for (const target of targets) {
      expect(target.querySelector("svg")).toBeInTheDocument();
    }
    const expected = render(<UsersThreeIcon weight="regular" />);
    expect(
      container.querySelector('[data-parent-nav-item="children"] svg')
        ?.innerHTML,
    ).toBe(expected.container.querySelector("svg")?.innerHTML);
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

  it("uses CSS to hide itself from 1024px without a hydration swap", () => {
    mockedUseMediaQuery.mockReturnValue(false);
    renderNav();

    expect(screen.getByRole("navigation")).toHaveClass("lg:hidden");
    expect(screen.getByText("Start")).toBeInTheDocument();
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

    fireEvent.click(screen.getByRole("button", { name: "Mehr" }));

    expect(screen.getByText("Elternbriefe")).toBeVisible();
    expect(screen.getByText("Essensplan")).toBeVisible();
    expect(screen.getByText("Einstellungen")).toBeVisible();
    expect(screen.getByText("Neue Anmeldung")).toBeVisible();
    expect(screen.getByText("Abmelden")).toBeVisible();
    expect(screen.queryByText("Sprache")).not.toBeInTheDocument();
  });

  it("separates account actions from the regular Mehr targets", () => {
    renderNav();

    fireEvent.click(screen.getByRole("button", { name: "Mehr" }));

    const accountGroup = document.querySelector(
      '[data-parent-nav-group="account"]',
    );
    expect(accountGroup).not.toBeNull();
    expect(accountGroup).toHaveClass("border-t", "pt-5");
    expect(accountGroup).toHaveTextContent("Einstellungen");
  });

  it("asks for confirmation before signing out from Mehr", async () => {
    renderNav();

    fireEvent.click(screen.getByRole("button", { name: "Mehr" }));
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
    renderNav({ gates: { news: false, mealPlan: false } });

    fireEvent.click(screen.getByRole("button", { name: "Mehr" }));

    expect(screen.queryByText("Elternbriefe")).not.toBeInTheDocument();
    expect(screen.queryByText("Essensplan")).not.toBeInTheDocument();
    expect(screen.getByText("Einstellungen")).toBeVisible();
  });
  it("uses the development drawer row density and gives targets an arrow", () => {
    renderNav();

    fireEvent.click(screen.getByRole("button", { name: "Mehr" }));

    const newsItem = document.querySelector('[data-parent-nav-item="news"]');
    expect(newsItem).not.toBeNull();
    expect(newsItem!.className).toContain("px-4");
    expect(newsItem!.className).toContain("py-3");
    expect(newsItem!.querySelector("svg")).not.toBeNull();
  });

  // Teilmenge von #2326: Produktfeedback gehört nicht in eine Eltern-App.
  it("bietet kein Produktfeedback mehr an", () => {
    renderNav();

    fireEvent.click(screen.getByRole("button", { name: "Mehr" }));

    expect(screen.queryByText(/Feedback/)).not.toBeInTheDocument();
    expect(
      document.querySelector('[data-parent-nav-item="feedback"]'),
    ).toBeNull();
  });
});
