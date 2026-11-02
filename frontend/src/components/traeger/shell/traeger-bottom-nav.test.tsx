import { fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { _resetTraegerHostCache } from "~/lib/traeger-url";

const { mockPathname, sessionUser } = vi.hoisted(() => ({
  mockPathname: vi.fn(() => "/"),
  sessionUser: { permissions: [] as string[] },
}));

vi.mock("next/navigation", async (importOriginal) => ({
  ...(await importOriginal<typeof import("next/navigation")>()),
  usePathname: () => mockPathname(),
  useRouter: () => ({ push: vi.fn(), prefetch: vi.fn() }),
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: { user: sessionUser }, status: "authenticated" }),
}));

import { TraegerBottomNav } from "./traeger-bottom-nav";

const ALL_RIGHTS = [
  "traeger_overview:read",
  "traeger_enrollment:manage",
  "traeger_billing:manage",
  "traeger_accounts:manage",
  "traeger_access_log:read",
];

function bar() {
  return within(screen.getByRole("navigation", { name: "Hauptnavigation" }));
}

describe("TraegerBottomNav", () => {
  beforeEach(() => {
    mockPathname.mockReturnValue("/");
    vi.stubEnv("NEXT_PUBLIC_TRAEGER_HOSTNAME", window.location.host);
    _resetTraegerHostCache();
  });

  afterEach(() => {
    vi.unstubAllEnvs();
    _resetTraegerHostCache();
    sessionUser.permissions = [];
  });

  it("shows every page in the bar while they fit", () => {
    sessionUser.permissions = ["traeger_overview:read"];
    render(<TraegerBottomNav />);

    for (const name of ["Schulen", "Briefkopf", "Einstellungen", "Hilfe"]) {
      expect(bar().getByRole("link", { name })).toBeInTheDocument();
    }
    expect(bar().getByRole("link", { name: "Schulen" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(bar().queryByRole("button", { name: "Mehr" })).toBeNull();
  });

  it("moves the pages that do not fit behind Mehr", async () => {
    sessionUser.permissions = ALL_RIGHTS;
    mockPathname.mockReturnValue("/briefkopf");
    render(<TraegerBottomNav />);

    expect(bar().getAllByRole("link")).toHaveLength(4);
    expect(bar().queryByRole("link", { name: "Briefkopf" })).toBeNull();
    const more = bar().getByRole("button", { name: "Mehr" });
    // A page behind Mehr is open: Mehr carries the mark.
    expect(more).toHaveAttribute("data-active", "true");

    fireEvent.click(more);
    const drawer = within(await screen.findByRole("dialog"));
    expect(drawer.getByRole("link", { name: "Briefkopf" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(drawer.getByRole("link", { name: "Hilfe" })).toHaveAttribute(
      "href",
      "/help?role=traeger",
    );
  });
});
