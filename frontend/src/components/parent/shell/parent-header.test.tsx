import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import deMessages from "~/i18n/messages/de.json";
import { ParentHeader } from "./parent-header";

const pathnameMock = vi.fn(() => "/parents");

vi.mock("next/navigation", () => ({
  usePathname: () => pathnameMock(),
}));

vi.mock("next/image", () => ({
  default: ({ alt, ...props }: React.ImgHTMLAttributes<HTMLImageElement>) => (
    // eslint-disable-next-line @next/next/no-img-element
    <img alt={alt} {...props} />
  ),
}));

vi.mock("~/lib/parent-url", () => ({
  parentPath: (path: string) => path,
}));

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

function renderHeader() {
  return render(
    <NextIntlClientProvider locale="de" messages={deMessages}>
      <ParentHeader />
    </NextIntlClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  pathnameMock.mockReturnValue("/parents");
});

describe("ParentHeader", () => {
  it("shows the moto wordmark linking home", () => {
    renderHeader();

    const home = screen.getByRole("link", { name: "moto" });
    expect(home).toHaveAttribute("href", "/parents");
    expect(home.querySelector("img")).toHaveAttribute(
      "src",
      "/moto-logo-wordmark.webp",
    );
  });

  it("names the current page next to the wordmark", () => {
    pathnameMock.mockReturnValue("/parents/messages");
    renderHeader();

    expect(screen.getByText("Nachrichten")).toBeVisible();
  });

  it("offers the language switch and the account", () => {
    renderHeader();

    expect(screen.getByTestId("language-switcher")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Einstellungen" })).toHaveAttribute(
      "href",
      "/parents/settings",
    );
  });

  // Die Kopfzeile sagt, wo man ist; die Ueberschrift der Seite gehoert der
  // Seite. Zwei h1 waeren fuer die Sprachausgabe zwei Titel, deshalb traegt
  // die Kopfzeile gar keine Ueberschriftsebene.
  it("carries no heading and no eyebrow", () => {
    pathnameMock.mockReturnValue("/parents/calendar");
    const { container } = renderHeader();

    expect(container.querySelectorAll("h1, h2, h3")).toHaveLength(0);
    expect(screen.getByText("Kalender")).toBeVisible();
    expect(container.querySelector(".moto-eyebrow")).not.toBeInTheDocument();
  });
});
