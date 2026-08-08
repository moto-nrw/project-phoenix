import type { ReactNode } from "react";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const { mockListSystemRoles, mockCreateSchoolAccount, mockLoggerError } =
  vi.hoisted(() => ({
    mockListSystemRoles: vi.fn(),
    mockCreateSchoolAccount: vi.fn(),
    mockLoggerError: vi.fn(),
  }));

vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    isOpen,
    title,
    children,
    footer,
  }: {
    isOpen: boolean;
    title: string;
    children: ReactNode;
    footer?: ReactNode;
  }) =>
    isOpen ? (
      <div data-testid="modal">
        <h2>{title}</h2>
        {children}
        <div>{footer}</div>
      </div>
    ) : null,
}));

vi.mock("~/lib/operator/provisioning-api", () => ({
  operatorProvisioningService: {
    listSystemRoles: mockListSystemRoles,
    createSchoolAccount: mockCreateSchoolAccount,
  },
}));

vi.mock("~/lib/auth-helpers", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/auth-helpers")>();
  return {
    ...actual,
    getRoleDisplayName: (name: string) => name,
  };
});

vi.mock("~/lib/hooks/use-scroll-to-error", () => ({
  useScrollToError: () => vi.fn(),
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({
    error: mockLoggerError,
  }),
}));

import { CreateAccountModal } from "./create-account-modal";

function selectCustomOption(
  labelMatcher: RegExp | string,
  optionName: RegExp | string,
) {
  fireEvent.click(screen.getByLabelText(labelMatcher));
  fireEvent.click(screen.getByRole("option", { name: optionName }));
}

const defaultRoles = [
  { id: "1", name: "admin" },
  { id: "2", name: "user" },
  { id: "3", name: "teacher" },
  { id: "4", name: "guardian" },
  { id: "5", name: "lehrkraft" },
];

function renderModal(
  props: Partial<Parameters<typeof CreateAccountModal>[0]> = {},
) {
  const defaultProps = {
    isOpen: true,
    onClose: vi.fn(),
    schoolId: "10",
    schoolName: "Test Schule",
    onCreated: vi.fn(),
    ...props,
  };
  return {
    ...render(<CreateAccountModal {...defaultProps} />),
    ...defaultProps,
  };
}

async function fillForm(overrides: Record<string, string> = {}) {
  const values = {
    firstName: "Max",
    lastName: "Mustermann",
    email: "max@example.com",
    password: "Test1234!",
    confirmPassword: "Test1234!",
    ...overrides,
  };

  fireEvent.change(screen.getByLabelText(/Vorname/), {
    target: { value: values.firstName },
  });
  fireEvent.change(screen.getByLabelText(/Nachname/), {
    target: { value: values.lastName },
  });
  fireEvent.change(screen.getByLabelText(/E-Mail/), {
    target: { value: values.email },
  });

  // Use IDs to disambiguate password fields
  const passwordInput = document.getElementById("create-account-password")!;
  const confirmInput = document.getElementById(
    "create-account-confirm-password",
  )!;

  fireEvent.change(passwordInput, {
    target: { value: values.password },
  });
  fireEvent.change(confirmInput, {
    target: { value: values.confirmPassword },
  });

  // Select role
  await waitFor(() => {
    expect(screen.getByLabelText(/System-Rolle/)).not.toBeDisabled();
  });
  selectCustomOption(/System-Rolle/, "admin");
}

describe("CreateAccountModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockListSystemRoles.mockResolvedValue(defaultRoles);
  });

  it("does not render when isOpen is false", () => {
    renderModal({ isOpen: false });
    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("renders modal with school name in title when isOpen", () => {
    renderModal({ schoolName: "Grundschule Mitte" });
    expect(
      screen.getByText("Konto erstellen — Grundschule Mitte"),
    ).toBeInTheDocument();
  });

  it("loads roles on mount and renders role options", async () => {
    renderModal();

    await waitFor(() => {
      expect(mockListSystemRoles).toHaveBeenCalled();
    });

    await waitFor(() => {
      expect(screen.getByLabelText(/System-Rolle/)).not.toBeDisabled();
    });
    fireEvent.click(screen.getByLabelText(/System-Rolle/));

    expect(screen.getByRole("option", { name: "admin" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "user" })).toBeInTheDocument();
  });

  it("filters out unsupported legacy roles from options", async () => {
    renderModal();

    await waitFor(() => {
      expect(screen.getByLabelText(/System-Rolle/)).not.toBeDisabled();
    });
    fireEvent.click(screen.getByLabelText(/System-Rolle/));

    expect(screen.getByRole("option", { name: "admin" })).toBeInTheDocument();
    expect(
      screen.queryByRole("option", { name: "teacher" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("option", { name: "guardian" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("option", { name: "lehrkraft" }),
    ).not.toBeInTheDocument();
  });

  it("handles role loading failure gracefully", async () => {
    mockListSystemRoles.mockRejectedValue(new Error("network error"));
    renderModal();

    await waitFor(() => {
      expect(mockLoggerError).toHaveBeenCalledWith(
        "failed_to_load_roles",
        expect.objectContaining({ error: "network error" }),
      );
    });
  });

  it("renders all form fields", () => {
    renderModal();

    expect(screen.getByLabelText(/Vorname/)).toBeInTheDocument();
    expect(screen.getByLabelText(/Nachname/)).toBeInTheDocument();
    expect(screen.getByLabelText(/E-Mail/)).toBeInTheDocument();
    expect(screen.getByLabelText(/System-Rolle/)).toBeInTheDocument();
    expect(
      document.getElementById("create-account-password"),
    ).toBeInTheDocument();
    expect(
      document.getElementById("create-account-confirm-password"),
    ).toBeInTheDocument();
    expect(screen.getByLabelText(/Position/)).toBeInTheDocument();
  });

  it("renders caregiver helper copy with umlauts", async () => {
    renderModal();

    await waitFor(() => {
      expect(screen.getByLabelText(/System-Rolle/)).not.toBeDisabled();
    });
    selectCustomOption(/System-Rolle/, "admin");

    expect(screen.getByText("Auch als Betreuer einsetzen")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Vergibt zusätzlich die Betreuer-Rolle und legt das nötige Staff-/Teacher-Profil an.",
      ),
    ).toBeInTheDocument();
  });

  it("submit button is disabled when form is incomplete", () => {
    renderModal();

    const submitButton = screen.getByText("Konto erstellen");
    expect(submitButton).toBeDisabled();
  });

  it("shows password mismatch error", async () => {
    renderModal();

    await fillForm({
      password: "Test1234!",
      confirmPassword: "Different1!",
    });

    fireEvent.click(screen.getByText("Konto erstellen"));

    await waitFor(() => {
      expect(
        screen.getByText("Passwörter stimmen nicht überein."),
      ).toBeInTheDocument();
    });
  });

  it("creates account successfully and shows success state", async () => {
    mockCreateSchoolAccount.mockResolvedValue({
      id: "42",
      email: "max@example.com",
    });

    const { onCreated } = renderModal();

    await fillForm();
    fireEvent.click(screen.getByText("Konto erstellen"));

    await waitFor(() => {
      expect(screen.getByText("Konto erstellt")).toBeInTheDocument();
      expect(screen.getByText("max@example.com")).toBeInTheDocument();
    });

    expect(onCreated).toHaveBeenCalled();
    expect(mockCreateSchoolAccount).toHaveBeenCalledWith("10", {
      email: "max@example.com",
      first_name: "Max",
      last_name: "Mustermann",
      password: "Test1234!",
      confirm_password: "Test1234!",
      role_id: 1,
      position: undefined,
    });
  });

  it("handles API error on create and shows error message", async () => {
    mockCreateSchoolAccount.mockRejectedValue(
      new Error("E-Mail bereits vergeben"),
    );

    renderModal();

    await fillForm();
    fireEvent.click(screen.getByText("Konto erstellen"));

    await waitFor(() => {
      expect(screen.getByText("E-Mail bereits vergeben")).toBeInTheDocument();
      expect(mockLoggerError).toHaveBeenCalledWith(
        "account_create_failed",
        expect.objectContaining({ error: "E-Mail bereits vergeben" }),
      );
    });
  });

  it("calls onClose when Abbrechen button clicked", () => {
    const { onClose } = renderModal();

    fireEvent.click(screen.getByText("Abbrechen"));
    expect(onClose).toHaveBeenCalled();
  });

  it("calls onClose when Schließen button clicked on success", async () => {
    mockCreateSchoolAccount.mockResolvedValue({
      id: "42",
      email: "max@example.com",
    });

    const { onClose } = renderModal();

    await fillForm();
    fireEvent.click(screen.getByText("Konto erstellen"));

    await waitFor(() => {
      expect(screen.getByText("Schließen")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Schließen"));
    expect(onClose).toHaveBeenCalled();
  });

  it("resets form fields when modal opens", async () => {
    const { rerender, onClose, onCreated } = renderModal();

    await fillForm();

    // Close and reopen
    rerender(
      <CreateAccountModal
        isOpen={false}
        onClose={onClose}
        schoolId="10"
        schoolName="Test Schule"
        onCreated={onCreated}
      />,
    );
    rerender(
      <CreateAccountModal
        isOpen={true}
        onClose={onClose}
        schoolId="10"
        schoolName="Test Schule"
        onCreated={onCreated}
      />,
    );

    await waitFor(() => {
      expect(screen.getByLabelText(/Vorname/)).toHaveValue("");
      expect(screen.getByLabelText(/Nachname/)).toHaveValue("");
      expect(screen.getByLabelText(/E-Mail/)).toHaveValue("");
      expect(document.getElementById("create-account-password")).toHaveValue(
        "",
      );
      expect(
        document.getElementById("create-account-confirm-password"),
      ).toHaveValue("");
    });
  });

  it("sends position when selected", async () => {
    mockCreateSchoolAccount.mockResolvedValue({
      id: "42",
      email: "max@example.com",
    });

    renderModal();

    await fillForm();
    selectCustomOption(/Position/, "OGS-Büro");
    fireEvent.click(screen.getByText("Konto erstellen"));

    await waitFor(() => {
      expect(mockCreateSchoolAccount).toHaveBeenCalledWith(
        "10",
        expect.objectContaining({ position: "OGS-Büro" }),
      );
    });
  });

  it("handles non-Error throw from API", async () => {
    mockCreateSchoolAccount.mockRejectedValue("something went wrong");

    renderModal();

    await fillForm();
    fireEvent.click(screen.getByText("Konto erstellen"));

    await waitFor(() => {
      expect(
        screen.getByText("Fehler beim Erstellen des Kontos."),
      ).toBeInTheDocument();
      expect(mockLoggerError).toHaveBeenCalledWith(
        "account_create_failed",
        expect.objectContaining({ error: "something went wrong" }),
      );
    });
  });
});
