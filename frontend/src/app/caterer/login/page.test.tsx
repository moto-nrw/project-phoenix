import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { mockLogin, mockSignIn } = vi.hoisted(() => ({
  mockLogin: vi.fn(),
  mockSignIn: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  signIn: (...args: unknown[]) => mockSignIn(...args),
  signOut: vi.fn(),
  useSession: () => ({ data: null, status: "unauthenticated" }),
}));

const stableSearchParams = { get: () => null };
vi.mock("next/navigation", () => ({
  useSearchParams: () => stableSearchParams,
}));

vi.mock("~/lib/caterer-api", () => ({
  loginCaterer: (...args: unknown[]) => mockLogin(...args),
}));

vi.mock("~/lib/caterer-url", () => ({
  catererPath: (path: string) => path,
}));

import CatererLoginPage from "./page";

function submitCredentials() {
  fireEvent.change(screen.getByLabelText("E-Mail-Adresse"), {
    target: { value: "kueche@example.test" },
  });
  fireEvent.change(screen.getByLabelText("Passwort"), {
    target: { value: "secret" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Anmelden" }));
}

describe("CatererLoginPage (#4497)", () => {
  const fetchMock = vi.fn<typeof fetch>();

  beforeEach(() => {
    window.history.replaceState({}, "", "/login");
    mockLogin.mockReset();
    mockSignIn.mockReset();
    fetchMock.mockReset();
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("signs in without a code while two-factor sign-in is off", async () => {
    mockLogin.mockResolvedValue({ status: "authenticated" });
    render(<CatererLoginPage />);

    submitCredentials();

    // The BFF installed the session; the browser only moves on (#5005).
    await waitFor(() => expect(window.location.pathname).toBe("/caterer"));
    expect(mockSignIn).not.toHaveBeenCalled();
  });

  it("asks for the mailed code and can remember the device", async () => {
    mockLogin.mockResolvedValue({
      status: "mfa_required",
      challengeToken: "challenge",
      maskedEmail: "k***@example.test",
      trustedDeviceEnabled: true,
      trustedDeviceDays: 90,
    });
    fetchMock.mockResolvedValue(Response.json({ status: "authenticated" }));
    render(<CatererLoginPage />);

    submitCredentials();

    expect(
      await screen.findByRole("heading", { name: "Code-Eingabe" }),
    ).toBeInTheDocument();
    expect(screen.getByText("k***@example.test")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("checkbox", { name: /90 Tage merken/ }));
    const inputs = (await screen.findAllByRole("textbox")).slice(0, 6);
    await act(async () => {
      fireEvent.change(inputs[0]!, { target: { value: "123456" } });
    });

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe("/api/caterer/auth/mfa/verify");
    expect(JSON.parse(init?.body as string)).toEqual({
      challenge_token: "challenge",
      code: "123456",
      remember_device: true,
    });
    // The BFF installed the session; the browser only moves on (#5005).
    await waitFor(() => expect(window.location.pathname).toBe("/caterer"));
    expect(mockSignIn).not.toHaveBeenCalled();
  });
});
