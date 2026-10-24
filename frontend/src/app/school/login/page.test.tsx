import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

const { mockLogin, mockSearchParamsGet } = vi.hoisted(() => ({
  mockLogin: vi.fn(),
  mockSearchParamsGet: vi.fn((_key: string): string | null => null),
}));

vi.mock("next-auth/react", () => ({
  signIn: vi.fn(),
  signOut: vi.fn(),
  useSession: () => ({ data: null, status: "unauthenticated" }),
}));

const stableSearchParams = { get: mockSearchParamsGet };
vi.mock("next/navigation", () => ({
  useSearchParams: () => stableSearchParams,
}));

vi.mock("~/lib/mfa-api", () => ({
  login: (...args: unknown[]) => mockLogin(...args),
}));

vi.mock("~/lib/school-url", () => ({
  schoolPath: (path: string) => path,
}));

vi.mock("~/components/ui/password-reset-modal", () => ({
  PasswordResetModal: () => null,
}));

vi.mock("~/components/ui/alert", () => ({
  Alert: ({ type, message }: { type: string; message: string }) => (
    <div data-testid={`alert-${type}`}>{message}</div>
  ),
}));

import SchoolLoginPage from "./page";

function submit() {
  fireEvent.change(screen.getByTestId("input-email"), {
    target: { value: "lehrkraft@example.com" },
  });
  fireEvent.change(screen.getByTestId("input-password"), {
    target: { value: "secret" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Anmelden" }));
}

// The school login used to guess its messages from the backend text
// ("locked", "invalid", "code request"). It now shows the catalog text of
// the code the backend sends (ADR 0006).
describe("SchoolLoginPage error path", () => {
  beforeEach(() => {
    mockLogin.mockReset();
    mockSearchParamsGet.mockReturnValue(null);
  });

  // The backend answers a wrong password with 401 identity.invalid_credentials.
  // On the error path a 401 means "session expired" and leaves for the login
  // screen; the login itself keeps its refusals in the form instead.
  it("keeps a refused password in the form with the catalog text and does not redirect", async () => {
    const assign = vi
      .spyOn(window.location, "assign")
      .mockImplementation(() => undefined);
    mockLogin.mockRejectedValue(
      new ApiError("invalid username or password", 401, {
        code: "identity.invalid_credentials",
      }),
    );
    render(<SchoolLoginPage />);
    submit();

    expect(
      await screen.findByText(
        catalogText("identity.invalid_credentials", "die Anmeldung"),
      ),
    ).toBeInTheDocument();
    expect(screen.getByTestId("input-email")).toBeInTheDocument();
    expect(assign).not.toHaveBeenCalled();
    assign.mockRestore();
  });

  it.each([
    ["identity.session_account_inactive", 401],
    ["identity.mfa_blocked", 429],
  ] as const)("shows the catalog text of %s", async (code, status) => {
    const assign = vi
      .spyOn(window.location, "assign")
      .mockImplementation(() => undefined);
    mockLogin.mockRejectedValue(
      new ApiError("diagnostic text that is never shown", status, { code }),
    );
    render(<SchoolLoginPage />);
    submit();

    expect(
      await screen.findByText(catalogText(code, "die Anmeldung")),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/diagnostic text that is never shown/),
    ).not.toBeInTheDocument();
    expect(assign).not.toHaveBeenCalled();
    assign.mockRestore();
  });

  it("explains an account without a school portal role", async () => {
    mockLogin.mockRejectedValue(
      new ApiError("no school portal role", 403, {
        code: "school.no_school_portal_role",
      }),
    );
    render(<SchoolLoginPage />);
    submit();

    expect(
      await screen.findByText(/keinen Zugang zum Schul-Portal/),
    ).toBeInTheDocument();
  });

  it("shows an expired session as a notice", async () => {
    mockSearchParamsGet.mockImplementation((key) =>
      key === "error" ? "SessionExpired" : null,
    );
    render(<SchoolLoginPage />);

    await waitFor(() =>
      expect(screen.getByTestId("alert-info")).toHaveTextContent(
        "Ihre Sitzung ist abgelaufen",
      ),
    );
  });
});
