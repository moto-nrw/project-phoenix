import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { TenantPage } from "~/components/ui/tenant-page";
import { ParentPageHeader } from "~/components/parent/parent-page";
import { HELP_TOPICS, type HelpTopicId } from "~/lib/help-topics";
import { MobilePageHelpPortal } from "./mobile-page-help-portal";
import { MobilePageHelpLink } from "./mobile-page-help-link";

const state = vi.hoisted(() => ({
  pathname: "/testschule/students/search",
  mode: "teacher",
  roles: ["admin"],
  helpTopic: undefined as HelpTopicId | undefined,
}));

vi.mock("next/navigation", () => ({
  usePathname: () => state.pathname,
  useSearchParams: () => new URLSearchParams("filter=heute"),
}));
vi.mock("~/lib/shell-auth-context", () => ({
  useShellAuthSafe: () => ({ mode: state.mode }),
  useShellAuth: () => ({ mode: state.mode, user: { roles: state.roles } }),
}));
vi.mock("~/lib/breadcrumb-context", () => ({
  useBreadcrumb: () => ({ breadcrumb: { helpTopic: state.helpTopic } }),
}));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => "testschule",
  useTenantRoutingModeSafe: () => "path",
  useNFCEnabled: () => true,
  usePresenceMode: () => "detailed",
  useOpenCareGroupMode: () => false,
}));

beforeEach(() => {
  state.pathname = "/testschule/students/search";
  state.mode = "teacher";
  state.roles = ["admin"];
  state.helpTopic = undefined;
});

describe("mobile page help", () => {
  it("opens staff help from the page header, preserving filters and return path", async () => {
    render(
      <>
        <MobilePageHelpPortal />
        <TenantPage title="Kinder" />
      </>,
    );
    const link = await screen.findByRole("link", {
      name: "Hilfe zu dieser Seite",
    });
    expect(link.closest("header")).toContainElement(
      screen.getByRole("heading"),
    );
    expect(link.parentElement).toHaveClass("lg:hidden");
    const url = new URL(link.getAttribute("href")!, "https://moto.invalid");
    expect(url.pathname).toBe("/help/kindersuche");
    expect(url.searchParams.get("return_to")).toBe(
      "/testschule/students/search?filter=heute",
    );
    expect(url.searchParams.get("role")).toBe("lead");
    expect(link).not.toHaveAttribute("target");
  });

  it.each(["/parents/children/12", "/children/12"])(
    "opens parent help in the parent page header for %s",
    async (pathname) => {
      state.mode = "parent";
      state.roles = ["guardian"];
      state.pathname = pathname;
      render(
        <>
          <MobilePageHelpPortal />
          <ParentPageHeader title="Mein Kind" />
        </>,
      );
      const link = await screen.findByRole("link", {
        name: "Hilfe zu dieser Seite",
      });
      expect(link.closest("header")).toContainElement(
        screen.getByRole("heading"),
      );
      const url = new URL(link.getAttribute("href")!, "https://moto.invalid");
      expect(url.pathname).toBe("/help/mein-kind-und-den-heutigen-tag-ansehen");
      expect(url.searchParams.get("role")).toBe("parent");
      expect(link.parentElement).toHaveClass("lg:hidden");
    },
  );

  it("respects the staff page's explicit topic", () => {
    state.helpTopic = HELP_TOPICS.settings;
    render(<MobilePageHelpLink />);
    expect(screen.getByRole("link")).toHaveAttribute(
      "href",
      expect.stringContaining("/help/einstellungen-ueberblick?"),
    );
  });

  it("does not send caregivers to an article for school leads", () => {
    state.pathname = "/testschule/settings";
    state.roles = ["user"];
    render(<TenantPage title="Einstellungen" />);
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });

  it("does not replace a missing article with the help overview", () => {
    state.pathname = "/testschule/unmapped";
    render(<MobilePageHelpLink />);
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });

  it.each(["school", "operator"])(
    "leaves %s help to its own header",
    (mode) => {
      state.mode = mode;
      render(<MobilePageHelpLink />);
      expect(screen.queryByRole("link")).not.toBeInTheDocument();
    },
  );
});
