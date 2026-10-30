import { render, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { useSession, router, schoolPortalLoginUrl } = vi.hoisted(() => ({
  useSession: vi.fn(),
  router: { replace: vi.fn() },
  schoolPortalLoginUrl: vi.fn(() => "https://schule.example.test/login"),
}));

vi.mock("next-auth/react", () => ({ useSession }));
vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div>Lädt</div>,
}));
vi.mock("~/lib/school-url", () => ({ schoolPortalLoginUrl }));
vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => router,
}));

import DashboardRedirectPage from "./page";

describe("DashboardRedirectPage", () => {
  let originalLocation: Location;

  beforeEach(() => {
    vi.clearAllMocks();
    originalLocation = window.location;
    Object.defineProperty(window, "location", {
      writable: true,
      value: { ...originalLocation, href: "" },
    });
    vi.mocked(useSession).mockReturnValue({
      data: { user: { roles: ["user"] } },
      status: "authenticated",
    } as ReturnType<typeof useSession>);
  });

  afterEach(() => {
    Object.defineProperty(window, "location", {
      writable: true,
      value: originalLocation,
    });
  });

  it("leitet Sitzungen im Mitarbeiter-Portal auf die Startseite", async () => {
    render(<DashboardRedirectPage />);

    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/home"));
    expect(schoolPortalLoginUrl).not.toHaveBeenCalled();
  });

  it("überschreibt eine neuere Navigation nicht nach einer Sitzungserneuerung", async () => {
    const { rerender } = render(<DashboardRedirectPage />);
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/home"));

    // Die alte Seite bleibt während des Ladens noch gemountet. Ein frisches
    // Sitzungsobjekt darf die inzwischen gestartete Tour-Navigation nicht
    // mit einer zweiten Weiterleitung zur Startseite verdrängen.
    vi.mocked(useSession).mockReturnValue({
      data: { user: { roles: ["user"] } },
      status: "authenticated",
    } as ReturnType<typeof useSession>);
    rerender(<DashboardRedirectPage />);

    expect(router.replace).toHaveBeenCalledTimes(1);
  });

  it("leitet nach dem Sitzungsende trotz begonnener Weiterleitung zur Anmeldung", async () => {
    const { rerender } = render(<DashboardRedirectPage />);
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/home"));

    vi.mocked(useSession).mockReturnValue({
      data: null,
      status: "unauthenticated",
    } as ReturnType<typeof useSession>);
    rerender(<DashboardRedirectPage />);

    expect(router.replace).toHaveBeenLastCalledWith("/");
  });

  it("leitet nicht angemeldete Personen zur Anmeldung", async () => {
    vi.mocked(useSession).mockReturnValue({
      data: null,
      status: "unauthenticated",
    } as ReturnType<typeof useSession>);

    render(<DashboardRedirectPage />);

    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/"));
    expect(schoolPortalLoginUrl).not.toHaveBeenCalled();
  });

  it("übergibt reine Lehrkraft-Sitzungen an moto schule", async () => {
    vi.mocked(useSession).mockReturnValue({
      data: { user: { roles: ["lehrkraft"] } },
      status: "authenticated",
    } as ReturnType<typeof useSession>);

    render(<DashboardRedirectPage />);

    await waitFor(() =>
      expect(window.location.href).toBe("https://schule.example.test/login"),
    );
    expect(router.replace).not.toHaveBeenCalled();
  });
});
