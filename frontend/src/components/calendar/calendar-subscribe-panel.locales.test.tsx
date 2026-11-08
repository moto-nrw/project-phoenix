import { fireEvent, render as rtlRender, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import type { ReactElement, ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import enMessages from "~/i18n/messages/en.generated";

// The parent calendar is rendered with the active locale, not the global
// German translation mock used by most component tests.
vi.unmock("next-intl");

const { mockGetParentCalendarFeed } = vi.hoisted(() => ({
  mockGetParentCalendarFeed: vi.fn(),
}));

vi.mock("~/lib/personal-calendar-api", async () => {
  const actual = await vi.importActual<
    typeof import("~/lib/personal-calendar-api")
  >("~/lib/personal-calendar-api");
  return {
    ...actual,
    getParentCalendarFeed: mockGetParentCalendarFeed,
  };
});

import { ToastProvider } from "~/contexts/ToastContext";

import { CalendarSubscribePanel } from "./calendar-subscribe-panel";

function render(ui: ReactElement) {
  return rtlRender(ui, {
    wrapper: ({ children }: { children: ReactNode }) => (
      <NextIntlClientProvider
        locale="en"
        messages={enMessages}
        timeZone="Europe/Berlin"
      >
        <ToastProvider>{children}</ToastProvider>
      </NextIntlClientProvider>
    ),
  });
}

describe("CalendarSubscribePanel locales", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows an English loading label while fetching the parent subscription link", () => {
    mockGetParentCalendarFeed.mockReturnValue(new Promise(() => undefined));

    render(<CalendarSubscribePanel />);
    fireEvent.click(
      screen.getByRole("button", { name: "Show subscription link" }),
    );

    expect(screen.getByRole("button", { name: "Loading …" })).toBeDisabled();
  });
});
