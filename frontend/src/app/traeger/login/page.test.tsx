import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

const { mockLogin, mockSignIn, mockSearchParamsGet } = vi.hoisted(() => ({
  mockLogin: vi.fn(),
  mockSignIn: vi.fn(),
  mockSearchParamsGet: vi.fn((_key: string): string | null => null),
}));

vi.mock("next-auth/react", () => ({
  signIn: (...args: unknown[]) => mockSignIn(...args),
  signOut: vi.fn(),
  useSession: () => ({ data: null, status: "unauthenticated" }),
}));

const stableSearchParams = { get: mockSearchParamsGet };
vi.mock("next/navigation", () => ({
  useSearchParams: () => stableSearchParams,
}));

vi.mock("~/lib/traeger-api", () => ({
  traegerLogin: (...args: unknown[]) => mockLogin(...args),
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
    });
    fetchMock.mockResolvedValue(jsonResponse({ status: "authenticated" }));
    mockSignIn.mockResolvedValue({ error: undefined });
    render(<TraegerLoginPage />);

    submitCredentials();

    expect(
      await screen.findByRole("heading", { name: "Code-Eingabe" }),
    ).toBeInTheDocument();
    expect(screen.getByText("b***@traeger.example")).toBeInTheDocument();
    expect(
      screen.queryByRole("checkbox", { name: /Tage merken/ }),
    ).not.toBeInTheDocument();
    await enterCode("123456");

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
