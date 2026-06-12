import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { useRouter } from "next/navigation";
import { useLocale } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { writeLocaleCookie } from "~/i18n/locales";
import { useParentLocale } from "~/lib/parent-locale-context";
import { LanguageSwitcher } from "./language-switcher";

const refreshMock = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: vi.fn(() => ({ refresh: refreshMock })),
}));

vi.mock("next-intl", () => ({
  useLocale: vi.fn(() => "de"),
  useTranslations: vi.fn(
    () => (key: string) => (key === "label" ? "Language" : key),
  ),
}));

vi.mock("~/lib/parent-locale-context", () => ({
  useParentLocale: vi.fn(() => null),
}));

vi.mock("~/i18n/locales", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/i18n/locales")>();
  return { ...actual, writeLocaleCookie: vi.fn() };
});

const mockedUseRouter = vi.mocked(useRouter);
const mockedUseLocale = vi.mocked(useLocale);
const mockedUseParentLocale = vi.mocked(useParentLocale);
const mockedWriteLocaleCookie = vi.mocked(writeLocaleCookie);

beforeEach(() => {
  vi.clearAllMocks();
  refreshMock.mockClear();
  mockedUseRouter.mockReturnValue({
    refresh: refreshMock,
  } as unknown as ReturnType<typeof useRouter>);
  mockedUseLocale.mockReturnValue("de");
  mockedUseParentLocale.mockReturnValue(null);
});

describe("LanguageSwitcher", () => {
  it("uses the shared listbox dropdown instead of a native select", () => {
    const { container } = render(<LanguageSwitcher />);

    expect(container.querySelector("select")).not.toBeInTheDocument();
    expect(container.firstElementChild).toHaveClass("inline-block");
    expect(screen.getByRole("button", { name: "Language" })).toHaveTextContent(
      "Deutsch",
    );
    expect(screen.getByRole("button", { name: "Language" })).toHaveClass("h-9");
  });

  it("opens the locale menu and changes anonymous locale", () => {
    render(<LanguageSwitcher />);

    fireEvent.click(screen.getByRole("button", { name: "Language" }));
    fireEvent.click(screen.getByRole("option", { name: "English" }));

    expect(mockedWriteLocaleCookie).toHaveBeenCalledWith("en");
    expect(refreshMock).toHaveBeenCalledTimes(1);
  });

  it("delegates to the parent locale context when available", async () => {
    const setLocale = vi.fn().mockResolvedValue(undefined);
    mockedUseParentLocale.mockReturnValue({
      locale: "ru",
      setLocale,
      authenticated: true,
    });

    render(<LanguageSwitcher compact />);

    const trigger = screen.getByRole("button", { name: "Language" });
    expect(trigger).toHaveTextContent("Русский");
    expect(screen.getByText("Language")).toHaveClass("sr-only");

    fireEvent.click(trigger);
    fireEvent.click(screen.getByRole("option", { name: "Shqip" }));

    await waitFor(() => expect(setLocale).toHaveBeenCalledWith("sq"));
    expect(mockedWriteLocaleCookie).not.toHaveBeenCalled();
    expect(refreshMock).not.toHaveBeenCalled();
  });
});
