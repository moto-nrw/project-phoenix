import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { useShellAuth } from "~/lib/shell-auth-context";
import { _resetTraegerHostCache } from "~/lib/traeger-url";

// #6870: the header shows the name of the current Träger membership, not the
// e-mail address of the session; the page „Konto“ updates it in place.

const { mockUseSession, api } = vi.hoisted(() => ({
  mockUseSession: vi.fn(),
  api: {
    fetchTraegerOrganization: vi.fn(),
    fetchTraegerProfile: vi.fn(),
    changeTraegerProfileName: vi.fn(),
  },
}));

vi.mock("next-auth/react", () => ({
  useSession: () => mockUseSession(),
  signOut: vi.fn(),
}));
vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  usePathname: () => "/traeger/konto",
}));
vi.mock("~/lib/traeger-api", () => api);
// The real shell needs the whole portal; the header reads the same context.
vi.mock("~/components/traeger/traeger-shell", () => ({
  TraegerShell: ({ children }: { children: React.ReactNode }) => (
    <>
      <HeaderName />
      {children}
    </>
  ),
}));

function HeaderName() {
  const { user } = useShellAuth();
  return <p data-testid="header-name">{user?.name}</p>;
}

import { TraegerAuthGuard } from "./auth-guard";
import TraegerAccountPage from "./konto/page";

function sessionOf(id: string, orgId: number, email: string) {
  return {
    status: "authenticated",
    data: {
      user: { id, orgId, scope: "org", email, name: email, roles: [] },
    },
  };
}

function renderPortal(page: React.ReactNode = <p>Seite</p>) {
  return render(
    <ToastProvider>
      <TraegerAuthGuard>{page}</TraegerAuthGuard>
    </ToastProvider>,
  );
}

const headerName = () => screen.getByTestId("header-name");

describe("Träger header name (#6870)", () => {
  beforeEach(() => {
    for (const fn of Object.values(api)) fn.mockReset();
    api.fetchTraegerOrganization.mockResolvedValue({
      id: "7",
      name: "Träger",
      testSystem: false,
    });
    mockUseSession.mockReturnValue(sessionOf("4", 7, "kasse@traeger.example"));
    vi.stubEnv("NEXT_PUBLIC_TRAEGER_HOSTNAME", "traeger.localhost:3000");
    _resetTraegerHostCache();
  });

  afterEach(() => {
    vi.unstubAllEnvs();
    _resetTraegerHostCache();
  });

  it("shows the name of the membership instead of the e-mail address", async () => {
    api.fetchTraegerProfile.mockResolvedValue({
      accountId: "4",
      email: "kasse@traeger.example",
      name: "Karl Kasse",
    });
    renderPortal();

    await waitFor(() => expect(headerName()).toHaveTextContent("Karl Kasse"));
  });

  it("keeps the e-mail address for an account without a name", async () => {
    api.fetchTraegerProfile.mockResolvedValue({
      accountId: "4",
      email: "kasse@traeger.example",
      name: "",
    });
    renderPortal();

    await waitFor(() => expect(api.fetchTraegerProfile).toHaveBeenCalled());
    expect(headerName()).toHaveTextContent("kasse@traeger.example");
  });

  it("keeps the e-mail address when the profile cannot be read", async () => {
    api.fetchTraegerProfile.mockRejectedValue(new Error("offline"));
    renderPortal();

    await waitFor(() => expect(api.fetchTraegerProfile).toHaveBeenCalled());
    expect(headerName()).toHaveTextContent("kasse@traeger.example");
  });

  it("takes a name saved under „Konto“ without signing in again", async () => {
    api.fetchTraegerProfile.mockResolvedValue({
      accountId: "4",
      email: "kasse@traeger.example",
      name: "",
    });
    api.changeTraegerProfileName.mockResolvedValue({
      accountId: "4",
      email: "kasse@traeger.example",
      name: "Karl Kasse",
    });
    renderPortal(<TraegerAccountPage />);

    const field = await screen.findByRole("textbox", { name: /Ihr Name/ });
    expect(headerName()).toHaveTextContent("kasse@traeger.example");
    fireEvent.change(field, { target: { value: "Karl Kasse" } });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => expect(headerName()).toHaveTextContent("Karl Kasse"));
  });

  it("drops the name of the former membership after a change of Träger", async () => {
    api.fetchTraegerProfile.mockResolvedValueOnce({
      accountId: "4",
      email: "kasse@traeger.example",
      name: "Karl Kasse",
    });
    const view = renderPortal();
    await waitFor(() => expect(headerName()).toHaveTextContent("Karl Kasse"));

    // The other Träger answers later; until then its name is unknown.
    let answer: (value: unknown) => void = () => {};
    api.fetchTraegerProfile.mockReturnValueOnce(
      new Promise((resolve) => {
        answer = resolve;
      }),
    );
    mockUseSession.mockReturnValue(sessionOf("4", 8, "kasse@traeger.example"));
    view.rerender(
      <ToastProvider>
        <TraegerAuthGuard>
          <p>Seite</p>
        </TraegerAuthGuard>
      </ToastProvider>,
    );

    expect(headerName()).toHaveTextContent("kasse@traeger.example");
    await act(async () =>
      answer({
        accountId: "4",
        email: "kasse@traeger.example",
        name: "K. Kasse",
      }),
    );
    expect(headerName()).toHaveTextContent("K. Kasse");
  });
});
