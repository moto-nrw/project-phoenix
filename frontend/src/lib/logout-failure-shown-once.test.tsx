import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { LogoutModal } from "~/components/ui/logout-modal";
import {
  ParentShellProvider,
  TeacherShellProvider,
} from "./shell-auth-context";

const { mockSignOut } = vi.hoisted(() => ({ mockSignOut: vi.fn() }));

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: {
      user: { id: "7", name: "User", email: "u@example.invalid", roles: [] },
    },
    status: "authenticated",
    update: vi.fn(),
  }),
  signOut: mockSignOut,
}));
vi.mock("~/lib/profile-context", () => ({
  useProfile: () => ({ profile: null }),
}));
vi.mock("~/lib/push-api", () => ({ unsubscribePushSilently: vi.fn() }));

beforeEach(() => {
  vi.stubGlobal(
    "fetch",
    vi.fn(async () =>
      Response.json(
        {
          status: 503,
          detail: "logout could not revoke the session",
          code: "general.unavailable",
        },
        { status: 503 },
      ),
    ),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});

// #5194: a failed revocation keeps the session and is shown
// exactly once, by the shell, with a retry; the confirmation only closes.
it.each([
  ["OGS", TeacherShellProvider],
  ["Eltern", ParentShellProvider],
] as const)(
  "%s: a failed logout is shown once with a retry",
  async (_portal, Provider) => {
    const onClose = vi.fn();
    render(
      <ToastProvider>
        <Provider>
          <LogoutModal isOpen onClose={onClose} />
        </Provider>
      </ToastProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Abmelden" }));

    const alert = await screen.findByRole("alert");
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 50));
    });
    expect(screen.getAllByRole("alert")).toEqual([alert]);
    expect(alert).toHaveTextContent("Abmeldung");
    expect(
      screen.getByRole("button", { name: "Wiederholen" }),
    ).toBeInTheDocument();
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(mockSignOut).not.toHaveBeenCalled();

    // "Wiederholen" runs the logout again: the revocation is requested anew.
    const revocations = () =>
      vi
        .mocked(fetch)
        .mock.calls.filter(([url]) => String(url).endsWith("/auth/logout"))
        .length;
    const before = revocations();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() => expect(revocations()).toBe(before + 1));
  },
);
