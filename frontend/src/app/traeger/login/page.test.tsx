import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { hydrateRoot } from "react-dom/client";
import { renderToString } from "react-dom/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

const {
  mockLogin,
  mockSignIn,
  mockSignOut,
  mockSession,
  mockSearchParamsGet,
  mockEndTraegerSession,
} = vi.hoisted(() => ({
  mockLogin: vi.fn(),
  mockSignIn: vi.fn(),
  mockSignOut: vi.fn(),
  mockSession: {
    current: { data: null, status: "unauthenticated" } as {
      data: unknown;
      status: string;
    },
  },
  mockSearchParamsGet: vi.fn((_key: string): string | null => null),
  mockEndTraegerSession: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  signIn: (...args: unknown[]) => mockSignIn(...args),
  signOut: (...args: unknown[]) => mockSignOut(...args),
  useSession: () => mockSession.current,
}));

const stableSearchParams = { get: mockSearchParamsGet };
vi.mock("next/navigation", () => ({
  useSearchParams: () => stableSearchParams,
}));

vi.mock("~/lib/traeger-api", () => ({
  traegerLogin: (...args: unknown[]) => mockLogin(...args),
}));

vi.mock("~/lib/shell-auth-context", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/shell-auth-context")>()),
  endTraegerSession: () => mockEndTraegerSession() as Promise<void>,
}));

vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path,
}));

import TraegerLoginPage from "./page";

function submitCredentials() {
  fireEvent.change(screen.getByTestId("input-email"), {
    target: { value: "buero@traeger.example" },
  });
  fireEvent.change(screen.getByTestId("input-password"), {
    target: { value: "secret" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Anmelden" }));
}

function jsonResponse(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}

async function enterCode(code: string) {
  const inputs = (await screen.findAllByRole("textbox")).slice(0, 6);
  await act(async () => {
    fireEvent.change(inputs[0]!, { target: { value: code } });
  });
}

describe("TraegerLoginPage", () => {
  const fetchMock = vi.fn<typeof fetch>();
  let navigate: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    navigate = vi
      .spyOn(window.location, "href", "set")
      .mockImplementation(() => undefined);
    mockLogin.mockReset();
    mockSignIn.mockReset();
    mockSignOut.mockReset().mockResolvedValue(undefined);
    mockEndTraegerSession.mockReset().mockResolvedValue(undefined);
    mockSession.current = { data: null, status: "unauthenticated" };
    mockSearchParamsGet.mockReturnValue(null);
    fetchMock.mockReset();
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("asks for the mailed code and redeems it for the resolved Träger", async () => {
    mockLogin.mockResolvedValue({
      status: "mfa_required",
      challengeToken: "challenge",
      maskedEmail: "b***@traeger.example",
      organizationId: "7",
      trustedDeviceEnabled: true,
      trustedDeviceDays: 90,
    });
    fetchMock.mockResolvedValue(jsonResponse({ status: "authenticated" }));
    mockSignIn.mockResolvedValue({ error: undefined });
    render(<TraegerLoginPage />);

    submitCredentials();

    expect(
      await screen.findByRole("heading", { name: "Code-Eingabe" }),
    ).toBeInTheDocument();
    expect(screen.getByText("b***@traeger.example")).toBeInTheDocument();
    // Since #6955 the Träger-Portal remembers a device like the other
    // portals, and only on the button that says so.
    expect(
      screen.getByRole("button", {
        name: /^Ja, merken und anmelden/,
      }),
    ).toBeInTheDocument();
    await enterCode("123456");
    expect(fetchMock).not.toHaveBeenCalled();
    fireEvent.click(
      screen.getByRole("button", { name: /^Nein, nur anmelden/ }),
    );

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe("/api/traeger/auth/mfa/verify");
    expect(JSON.parse(init?.body as string)).toEqual({
      challenge_token: "challenge",
      code: "123456",
      remember_device: false,
      organization_id: 7,
    });
    await waitFor(() => expect(navigate).toHaveBeenCalledWith("/traeger"));
  });

  it("asks for the code until the sign-in completes, also after going back from the code step (#6955)", async () => {
    sessionStorage.setItem("moto.traeger.sign-in-with-code", "1");
    mockLogin.mockResolvedValue({
      status: "mfa_required",
      challengeToken: "challenge",
      maskedEmail: "b***@traeger.example",
      organizationId: "7",
      trustedDeviceEnabled: true,
      trustedDeviceDays: 90,
    });
    fetchMock.mockResolvedValue(jsonResponse({ status: "authenticated" }));
    mockSignIn.mockResolvedValue({ error: undefined });
    render(<TraegerLoginPage />);

    expect(
      screen.getByText(
        "Die Schule verlangt eine Anmeldung mit Code. Geben Sie nach dem Passwort den Code aus Ihrer E-Mail ein.",
      ),
    ).toBeInTheDocument();
    submitCredentials();
    await screen.findByRole("heading", { name: "Code-Eingabe" });
    expect(mockLogin).toHaveBeenLastCalledWith(
      expect.objectContaining({ ignoreTrustedDevice: true }),
    );
    expect(sessionStorage.getItem("moto.traeger.sign-in-with-code")).toBe("1");

    // Back to the password: the next attempt still ignores the device.
    fireEvent.click(screen.getByRole("button", { name: /Zurück/ }));
    submitCredentials();
    await screen.findByRole("heading", { name: "Code-Eingabe" });
    expect(mockLogin).toHaveBeenCalledTimes(2);
    expect(mockLogin).toHaveBeenLastCalledWith(
      expect.objectContaining({ ignoreTrustedDevice: true }),
    );

    await enterCode("123456");
    fireEvent.click(
      screen.getByRole("button", { name: /^Nein, nur anmelden/ }),
    );
    await waitFor(() => expect(navigate).toHaveBeenCalledWith("/traeger"));
    expect(sessionStorage.getItem("moto.traeger.sign-in-with-code")).toBeNull();
  });

  it("shows the code hint after hydrating server markup and sends the flag (#6955)", async () => {
    // The page as the server renders it, then hydrated in the browser: the
    // request must survive the server render and show up after hydration.
    sessionStorage.setItem("moto.traeger.sign-in-with-code", "1");
    mockLogin.mockResolvedValue({
      status: "mfa_required",
      challengeToken: "challenge",
      maskedEmail: "b***@traeger.example",
      organizationId: "7",
      trustedDeviceEnabled: true,
      trustedDeviceDays: 90,
    });
    const container = document.createElement("div");
    document.body.append(container);
    container.innerHTML = renderToString(<TraegerLoginPage />);
    const recoverableErrors = vi.fn();
    const root = await act(async () =>
      hydrateRoot(container, <TraegerLoginPage />, {
        onRecoverableError: recoverableErrors,
      }),
    );

    expect(
      await screen.findByText(
        "Die Schule verlangt eine Anmeldung mit Code. Geben Sie nach dem Passwort den Code aus Ihrer E-Mail ein.",
      ),
    ).toBeInTheDocument();
    expect(recoverableErrors).not.toHaveBeenCalled();
    expect(sessionStorage.getItem("moto.traeger.sign-in-with-code")).toBe("1");
    submitCredentials();
    await screen.findByRole("heading", { name: "Code-Eingabe" });
    expect(mockLogin).toHaveBeenCalledWith(
      expect.objectContaining({ ignoreTrustedDevice: true }),
    );
    expect(sessionStorage.getItem("moto.traeger.sign-in-with-code")).toBe("1");
    act(() => root.unmount());
    container.remove();
    sessionStorage.removeItem("moto.traeger.sign-in-with-code");
  });

  it("keeps the code request while the person still chooses the Träger (#6955)", async () => {
    sessionStorage.setItem("moto.traeger.sign-in-with-code", "1");
    mockLogin.mockResolvedValueOnce({
      status: "organization_selection_required",
      organizations: [
        { id: "7", name: "Träger Nord" },
        { id: "8", name: "Träger Süd" },
      ],
    });
    render(<TraegerLoginPage />);

    submitCredentials();
    await waitFor(() => expect(mockLogin).toHaveBeenCalledTimes(1));
    expect(mockLogin).toHaveBeenLastCalledWith(
      expect.objectContaining({ ignoreTrustedDevice: true }),
    );
    expect(sessionStorage.getItem("moto.traeger.sign-in-with-code")).toBe("1");
    sessionStorage.removeItem("moto.traeger.sign-in-with-code");
  });

  it("takes the code request from a school tab's link and forgets it in the address (#6955)", () => {
    mockSearchParamsGet.mockImplementation((key: string) =>
      key === "mit-code" ? "1" : null,
    );
    globalThis.history.replaceState(null, "", "/login?mit-code=1");
    render(<TraegerLoginPage />);

    expect(
      screen.getByText(
        "Die Schule verlangt eine Anmeldung mit Code. Geben Sie nach dem Passwort den Code aus Ihrer E-Mail ein.",
      ),
    ).toBeInTheDocument();
    expect(sessionStorage.getItem("moto.traeger.sign-in-with-code")).toBe("1");
    expect(globalThis.location.search).toBe("");
    sessionStorage.removeItem("moto.traeger.sign-in-with-code");
  });

  it("never signs out on a visit; the person ends the session with the button (#6955)", async () => {
    mockSearchParamsGet.mockImplementation((key: string) =>
      key === "mit-code" ? "1" : null,
    );
    mockSession.current = {
      data: { user: { scope: "org", token: "access-token" } },
      status: "authenticated",
    };
    let finish: () => void = () => undefined;
    mockEndTraegerSession.mockReturnValue(
      new Promise<void>((resolve) => {
        finish = resolve;
      }),
    );
    render(<TraegerLoginPage />);

    const end = await screen.findByRole("button", {
      name: "Abmelden und mit Code anmelden",
    });
    expect(
      screen.getByText(
        "Die Schule verlangt eine Anmeldung mit Code. Melden Sie sich dafür zuerst ab.",
      ),
    ).toBeInTheDocument();
    expect(mockSignOut).not.toHaveBeenCalled();
    expect(mockEndTraegerSession).not.toHaveBeenCalled();
    expect(navigate).not.toHaveBeenCalled();
    expect(sessionStorage.getItem("moto.traeger.sign-in-with-code")).toBe("1");

    fireEvent.click(end);
    // The portal's logout: revoke on the server, clear the account's data,
    // hold late requests (shell-auth-context.test.tsx, endTraegerSession).
    expect(mockEndTraegerSession).toHaveBeenCalledTimes(1);
    expect(mockSignOut).not.toHaveBeenCalled();
    await act(async () => finish());
    expect(sessionStorage.getItem("moto.traeger.sign-in-with-code")).toBe("1");
    sessionStorage.removeItem("moto.traeger.sign-in-with-code");
  });

  it("shows why the session stays when the sign-out fails (#6955)", async () => {
    sessionStorage.setItem("moto.traeger.sign-in-with-code", "1");
    mockSession.current = {
      data: { user: { scope: "org", token: "access-token" } },
      status: "authenticated",
    };
    mockEndTraegerSession.mockRejectedValue(
      new ApiError("logout failed", 500, { code: "general.server" }),
    );
    render(<TraegerLoginPage />);

    fireEvent.click(
      await screen.findByRole("button", {
        name: "Abmelden und mit Code anmelden",
      }),
    );
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Abmelden und mit Code anmelden" }),
    ).toBeEnabled();
    sessionStorage.removeItem("moto.traeger.sign-in-with-code");
  });

  it("sets up the second factor before the first Träger session", async () => {
    mockLogin.mockResolvedValue({
      status: "mfa_enrollment_required",
      enrollmentToken: "enrollment",
      maskedEmail: "b***@traeger.example",
      organizationId: "7",
    });
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ challenge_token: "challenge" }))
      .mockResolvedValueOnce(jsonResponse({ status: "authenticated" }));
    mockSignIn.mockResolvedValue({ error: undefined });
    render(<TraegerLoginPage />);

    submitCredentials();

    expect(
      await screen.findByText(
        "Kein Zugriff auf Ihre E-Mails? Wenden Sie sich an das moto-Team. Im Träger-Portal ist der Code immer Pflicht.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Code an meine E-Mail senden" }),
    );
    await enterCode("654321");

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
    const [startUrl, startInit] = fetchMock.mock.calls[0]!;
    expect(startUrl).toBe("/api/traeger/auth/mfa/enroll/start");
    const startHeaders = (startInit?.headers ?? {}) as Record<string, string>;
    expect(startHeaders.Authorization).toBe("Bearer enrollment");
    const [confirmUrl, confirmInit] = fetchMock.mock.calls[1]!;
    expect(confirmUrl).toBe("/api/traeger/auth/mfa/enroll/confirm");
    expect(JSON.parse(confirmInit?.body as string)).toEqual({
      code: "654321",
      remember_device: false,
      challenge_token: "challenge",
      organization_id: 7,
    });
  });

  it("opens the BFF-created Träger session without browser token seeding", async () => {
    mockLogin.mockResolvedValue({
      status: "authenticated",
    });
    mockSignIn.mockResolvedValue({ error: undefined });
    render(<TraegerLoginPage />);

    expect(
      screen.getByRole("heading", { name: "Willkommen im Träger-Portal" }),
    ).toBeInTheDocument();
    submitCredentials();

    await waitFor(() => expect(navigate).toHaveBeenCalledWith("/traeger"));
    expect(mockSignIn).not.toHaveBeenCalled();
    expect(mockLogin).toHaveBeenCalledWith({
      email: "buero@traeger.example",
      password: "secret",
      organizationId: undefined,
    });
  });

  it("asks which Träger to open and resubmits with the choice", async () => {
    mockLogin
      .mockResolvedValueOnce({
        status: "organization_selection_required",
        organizations: [
          { id: "7", name: "Träger Nord" },
          { id: "9", name: "Träger Süd" },
        ],
      })
      .mockResolvedValueOnce({
        status: "authenticated",
      });
    mockSignIn.mockResolvedValue({ error: undefined });
    render(<TraegerLoginPage />);

    submitCredentials();

    const south = await screen.findByLabelText("Träger Süd");
    expect(screen.getByLabelText("Träger Nord")).toBeChecked();
    fireEvent.click(south);
    fireEvent.click(screen.getByRole("button", { name: "Träger öffnen" }));

    await waitFor(() =>
      expect(mockLogin).toHaveBeenLastCalledWith({
        email: "buero@traeger.example",
        password: "secret",
        organizationId: "9",
      }),
    );
    await waitFor(() => expect(navigate).toHaveBeenCalledWith("/traeger"));
  });

  it("explains a missing Träger-Portal access with the catalog text", async () => {
    mockLogin.mockRejectedValue(
      new ApiError("account has no Träger membership", 403, {
        code: "traeger.no_membership",
      }),
    );
    render(<TraegerLoginPage />);

    submitCredentials();

    expect(
      await screen.findByText(
        catalogText("traeger.no_membership", "die Anmeldung"),
      ),
    ).toBeInTheDocument();
    expect(mockSignIn).not.toHaveBeenCalled();
  });

  // #5149: "Passwort vergessen?" opens the reset dialog of the other
  // logins, which asks the Träger route for the link.
  it("requests a reset link through the Träger route", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse({
        message: "If the email exists, a password reset link has been sent",
      }),
    );
    render(<TraegerLoginPage />);

    fireEvent.click(
      screen.getByRole("button", { name: "Passwort vergessen?" }),
    );
    const dialog = await screen.findByRole("dialog");
    fireEvent.change(dialog.querySelector("#reset-email")!, {
      target: { value: "buero@traeger.example" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Link senden" }));

    expect(await screen.findByText("E-Mail versendet!")).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/traeger/auth/password-reset",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ email: "buero@traeger.example" }),
      }),
    );
    expect(mockLogin).not.toHaveBeenCalled();
  });

  it("opens the reset dialog when a reset link without token leads here", async () => {
    mockSearchParamsGet.mockImplementation((key: string) =>
      key === "passwort-vergessen" ? "1" : null,
    );
    render(<TraegerLoginPage />);

    const dialog = await screen.findByRole("dialog");
    expect(dialog.querySelector("#reset-email")).not.toBeNull();
  });

  // An empty field is named before anything is sent.
  it("names the empty fields without sending the login", async () => {
    render(<TraegerLoginPage />);

    fireEvent.click(screen.getByRole("button", { name: "Anmelden" }));

    expect(
      await screen.findByText(
        "Bitte geben Sie E-Mail-Adresse und Passwort ein.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Bitte geben Sie Ihre E-Mail-Adresse ein."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Bitte geben Sie Ihr Passwort ein."),
    ).toBeInTheDocument();
    expect(mockLogin).not.toHaveBeenCalled();

    fireEvent.change(screen.getByTestId("input-email"), {
      target: { value: "buero@traeger.example" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Anmelden" }));
    expect(
      (await screen.findAllByText("Bitte geben Sie Ihr Passwort ein."))[0],
    ).toBeInTheDocument();
    expect(mockLogin).not.toHaveBeenCalled();
  });

  it("keeps a refused password in the form instead of redirecting", async () => {
    const assign = vi
      .spyOn(window.location, "assign")
      .mockImplementation(() => undefined);
    mockLogin.mockRejectedValue(
      new ApiError("invalid username or password", 401, {
        code: "identity.invalid_credentials",
      }),
    );
    render(<TraegerLoginPage />);

    submitCredentials();

    expect(
      await screen.findByText(
        catalogText("identity.invalid_credentials", "die Anmeldung"),
      ),
    ).toBeInTheDocument();
    expect(assign).not.toHaveBeenCalled();
    expect(mockSignIn).not.toHaveBeenCalled();
    assign.mockRestore();
  });
});
