import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ChangeEvent, ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { CaregiverCapabilityModal } from "./caregiver-capability-modal";

const {
  mockToastSuccess,
  mockToastError,
  mockGetTenantAccountCapability,
  mockEnableTenantAccountCapability,
  mockDisableTenantAccountCapability,
  mockGetOperatorAccountCapability,
  mockEnableOperatorAccountCapability,
  mockDisableOperatorAccountCapability,
  MockCaregiverCapabilityApiError,
} = vi.hoisted(() => ({
  mockToastSuccess: vi.fn(),
  mockToastError: vi.fn(),
  mockGetTenantAccountCapability: vi.fn(),
  mockEnableTenantAccountCapability: vi.fn(),
  mockDisableTenantAccountCapability: vi.fn(),
  mockGetOperatorAccountCapability: vi.fn(),
  mockEnableOperatorAccountCapability: vi.fn(),
  mockDisableOperatorAccountCapability: vi.fn(),
  MockCaregiverCapabilityApiError: class MockCaregiverCapabilityApiError extends Error {
    status: number;
    blockers: string[];

    constructor(message: string, status: number, blockers: string[] = []) {
      super(message);
      this.name = "CaregiverCapabilityApiError";
      this.status = status;
      this.blockers = blockers;
    }
  },
}));

vi.mock("~/components/ui/form-modal", async () => {
  const { createElement } = await import("react");

  return {
    FormModal: ({
      isOpen,
      title,
      footer,
      children,
    }: {
      isOpen: boolean;
      title: string;
      footer: ReactNode;
      children: ReactNode;
    }) =>
      isOpen
        ? createElement(
            "div",
            { "data-testid": "form-modal" },
            createElement("h1", null, title),
            createElement("div", null, children),
            createElement("div", null, footer),
          )
        : null,
  };
});

vi.mock("~/components/ui/alert", async () => {
  const { createElement } = await import("react");

  return {
    Alert: ({ message }: { message?: string }) =>
      message
        ? createElement("div", { "data-testid": "alert" }, message)
        : null,
  };
});

vi.mock("~/components/ui/input", async () => {
  const { createElement } = await import("react");

  return {
    Input: ({
      label,
      name,
      value,
      onChange,
      placeholder,
    }: {
      label: string;
      name: string;
      value: string;
      onChange: (event: ChangeEvent<HTMLInputElement>) => void;
      placeholder?: string;
    }) =>
      createElement(
        "label",
        null,
        createElement("span", null, label),
        createElement("input", {
          "aria-label": label,
          name,
          value,
          onChange,
          placeholder,
        }),
      ),
  };
});

vi.mock("~/components/ui/detail-modal-components", async () => {
  const { createElement } = await import("react");

  return {
    DataField: ({ label, children }: { label: string; children: ReactNode }) =>
      createElement(
        "div",
        null,
        createElement("span", null, label),
        createElement("span", null, children),
      ),
    DataGrid: ({ children }: { children: ReactNode }) =>
      createElement("div", null, children),
    InfoSection: ({
      title,
      children,
    }: {
      title: string;
      children: ReactNode;
    }) =>
      createElement(
        "section",
        null,
        createElement("h2", null, title),
        children,
      ),
    DetailIcons: {
      person: createElement("span", null, "person"),
      briefcase: createElement("span", null, "briefcase"),
      group: createElement("span", null, "group"),
      x: createElement("span", null, "x"),
    },
  };
});

vi.mock(
  "~/components/teachers/caregiver-blocker-resolution-modal",
  async () => {
    const { createElement } = await import("react");

    return {
      CaregiverBlockerResolutionModal: ({ isOpen }: { isOpen: boolean }) =>
        isOpen
          ? createElement(
              "div",
              { "data-testid": "resolution-modal" },
              "Resolution modal",
            )
          : null,
    };
  },
);

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: mockToastSuccess,
    error: mockToastError,
  }),
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({
    error: vi.fn(),
  }),
}));

vi.mock("~/lib/caregiver-capability-api", () => ({
  caregiverCapabilityService: {
    getTenantAccountCapability: mockGetTenantAccountCapability,
    enableTenantAccountCapability: mockEnableTenantAccountCapability,
    disableTenantAccountCapability: mockDisableTenantAccountCapability,
    getOperatorAccountCapability: mockGetOperatorAccountCapability,
    enableOperatorAccountCapability: mockEnableOperatorAccountCapability,
    disableOperatorAccountCapability: mockDisableOperatorAccountCapability,
  },
  CaregiverCapabilityApiError: MockCaregiverCapabilityApiError,
}));

function createState(overrides: Record<string, unknown> = {}) {
  return {
    accountId: "42",
    email: "teacher@example.com",
    firstName: "Ada",
    lastName: "Lovelace",
    personId: "10",
    staffId: "20",
    teacherId: "30",
    hasAdminRole: true,
    hasUserRole: true,
    hasPerson: true,
    hasStaff: true,
    hasTeacher: true,
    hasCaregiverProfile: true,
    isActiveCaregiver: true,
    disableBlocked: false,
    disableBlockers: [],
    activeSupervisions: [],
    activeSubstitutions: [],
    activitySupervisions: [],
    groupAssignments: [],
    ...overrides,
  };
}

describe("CaregiverCapabilityModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("loads tenant capability state when opened", async () => {
    mockGetTenantAccountCapability.mockResolvedValue(createState());

    render(
      <CaregiverCapabilityModal
        isOpen={true}
        onClose={vi.fn()}
        scope="tenant"
        accountId="42"
        accountLabel="Ada Lovelace"
      />,
    );

    await waitFor(() => {
      expect(mockGetTenantAccountCapability).toHaveBeenCalledWith("42");
    });
    expect(screen.getByText("Aktuelle Rolle")).toBeInTheDocument();
    expect(screen.getByText("Verwaltung + Betreuung")).toBeInTheDocument();
  });

  it("does not call operator endpoints without a school id", async () => {
    render(
      <CaregiverCapabilityModal
        isOpen={true}
        onClose={vi.fn()}
        scope="operator"
        accountId="42"
        accountLabel="Ada Lovelace"
      />,
    );

    await waitFor(() => {
      expect(mockGetOperatorAccountCapability).not.toHaveBeenCalled();
    });
    expect(screen.getByTestId("alert")).toHaveTextContent(
      "Die Betreuerfähigkeit konnte nicht geladen werden.",
    );
  });

  it("requires first and last name when enabling a profile without person data", async () => {
    mockGetTenantAccountCapability.mockResolvedValue(
      createState({
        firstName: "",
        lastName: "",
        personId: null,
        staffId: null,
        teacherId: null,
        hasPerson: false,
        hasStaff: false,
        hasTeacher: false,
        hasCaregiverProfile: false,
        isActiveCaregiver: false,
      }),
    );

    render(
      <CaregiverCapabilityModal
        isOpen={true}
        onClose={vi.fn()}
        scope="tenant"
        accountId="42"
        accountLabel="Ada Lovelace"
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Personaldaten anlegen")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Betreuung aktivieren"));

    expect(mockEnableTenantAccountCapability).not.toHaveBeenCalled();
    expect(screen.getByTestId("alert")).toHaveTextContent(
      "Vorname und Nachname werden benötigt",
    );
  });

  it("enables caregiver capability in operator scope and calls onUpdated", async () => {
    const onUpdated = vi.fn();
    mockGetOperatorAccountCapability.mockResolvedValue(
      createState({
        firstName: "",
        lastName: "",
        personId: null,
        staffId: null,
        teacherId: null,
        hasPerson: false,
        hasStaff: false,
        hasTeacher: false,
        hasCaregiverProfile: false,
        isActiveCaregiver: false,
      }),
    );
    mockEnableOperatorAccountCapability.mockResolvedValue(createState());

    render(
      <CaregiverCapabilityModal
        isOpen={true}
        onClose={vi.fn()}
        scope="operator"
        accountId="42"
        accountLabel="Ada Lovelace"
        schoolId="7"
        schoolName="Phoenix Schule"
        onUpdated={onUpdated}
      />,
    );

    await waitFor(() => {
      expect(mockGetOperatorAccountCapability).toHaveBeenCalledWith("7", "42");
    });

    fireEvent.change(screen.getByLabelText("Vorname"), {
      target: { value: "Ada" },
    });
    fireEvent.change(screen.getByLabelText("Nachname"), {
      target: { value: "Lovelace" },
    });
    fireEvent.change(screen.getByLabelText("Pädagogische Rolle (optional)"), {
      target: { value: "Betreuungskraft" },
    });
    fireEvent.click(screen.getByText("Betreuung aktivieren"));

    await waitFor(() => {
      expect(mockEnableOperatorAccountCapability).toHaveBeenCalledWith(
        "7",
        "42",
        {
          first_name: "Ada",
          last_name: "Lovelace",
          position: "Betreuungskraft",
        },
      );
    });
    expect(mockToastSuccess).toHaveBeenCalledWith(
      "Betreuung wurde erfolgreich aktiviert.",
    );
    expect(onUpdated).toHaveBeenCalledWith(
      expect.objectContaining({ accountId: "42" }),
    );
  });

  it("disables caregiver capability successfully", async () => {
    const onUpdated = vi.fn();
    mockGetTenantAccountCapability.mockResolvedValue(createState());
    mockDisableTenantAccountCapability.mockResolvedValue(
      createState({
        hasUserRole: false,
        isActiveCaregiver: false,
      }),
    );

    render(
      <CaregiverCapabilityModal
        isOpen={true}
        onClose={vi.fn()}
        scope="tenant"
        accountId="42"
        accountLabel="Ada Lovelace"
        onUpdated={onUpdated}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Betreuung deaktivieren")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Betreuung deaktivieren"));

    await waitFor(() => {
      expect(mockDisableTenantAccountCapability).toHaveBeenCalledWith("42");
    });
    expect(mockToastSuccess).toHaveBeenCalledWith(
      "Betreuung wurde deaktiviert.",
    );
    expect(onUpdated).toHaveBeenCalledWith(
      expect.objectContaining({ hasUserRole: false }),
    );
  });

  it("hides the disable button for legacy teacher-role accounts that are not active caregivers", async () => {
    mockGetTenantAccountCapability.mockResolvedValue(
      createState({
        hasUserRole: false,
        hasTeacher: true,
        isActiveCaregiver: false,
        disableBlocked: false,
      }),
    );

    render(
      <CaregiverCapabilityModal
        isOpen={true}
        onClose={vi.fn()}
        scope="tenant"
        accountId="42"
        accountLabel="Ada Lovelace"
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Betreuung aktivieren")).toBeInTheDocument();
    });
    expect(
      screen.queryByText("Betreuung deaktivieren"),
    ).not.toBeInTheDocument();
  });

  it("clears the optional position when another account is loaded", async () => {
    mockGetTenantAccountCapability
      .mockResolvedValueOnce(
        createState({
          firstName: "",
          lastName: "",
          personId: null,
          staffId: null,
          teacherId: null,
          hasPerson: false,
          hasStaff: false,
          hasTeacher: false,
          hasCaregiverProfile: false,
          hasUserRole: false,
          isActiveCaregiver: false,
        }),
      )
      .mockResolvedValueOnce(
        createState({
          accountId: "43",
          firstName: "",
          lastName: "",
          personId: null,
          staffId: null,
          teacherId: null,
          hasPerson: false,
          hasStaff: false,
          hasTeacher: false,
          hasCaregiverProfile: false,
          hasUserRole: false,
          isActiveCaregiver: false,
        }),
      );

    const { rerender } = render(
      <CaregiverCapabilityModal
        isOpen={true}
        onClose={vi.fn()}
        scope="tenant"
        accountId="42"
        accountLabel="Ada Lovelace"
      />,
    );

    await waitFor(() => {
      expect(
        screen.getByLabelText("Pädagogische Rolle (optional)"),
      ).toHaveValue("");
    });

    fireEvent.change(screen.getByLabelText("Pädagogische Rolle (optional)"), {
      target: { value: "Betreuungskraft" },
    });
    expect(screen.getByLabelText("Pädagogische Rolle (optional)")).toHaveValue(
      "Betreuungskraft",
    );

    rerender(
      <CaregiverCapabilityModal
        isOpen={true}
        onClose={vi.fn()}
        scope="tenant"
        accountId="43"
        accountLabel="Grace Hopper"
      />,
    );

    await waitFor(() => {
      expect(mockGetTenantAccountCapability).toHaveBeenCalledWith("43");
    });
    await waitFor(() => {
      expect(
        screen.getByLabelText("Pädagogische Rolle (optional)"),
      ).toHaveValue("");
    });
  });

  it("shows a generic loading error when the capability state cannot be loaded", async () => {
    mockGetTenantAccountCapability.mockRejectedValue(new Error("boom"));

    render(
      <CaregiverCapabilityModal
        isOpen={true}
        onClose={vi.fn()}
        scope="tenant"
        accountId="42"
        accountLabel="Ada Lovelace"
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("alert")).toHaveTextContent(
        "Die Betreuerfähigkeit konnte nicht geladen werden.",
      );
    });
  });

  it("shows the generic enable error for unexpected failures", async () => {
    mockGetTenantAccountCapability.mockResolvedValue(
      createState({
        isActiveCaregiver: false,
        hasUserRole: false,
      }),
    );
    mockEnableTenantAccountCapability.mockRejectedValue(new Error("boom"));

    render(
      <CaregiverCapabilityModal
        isOpen={true}
        onClose={vi.fn()}
        scope="tenant"
        accountId="42"
        accountLabel="Ada Lovelace"
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Betreuung aktivieren")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Betreuung aktivieren"));

    await waitFor(() => {
      expect(screen.getByTestId("alert")).toHaveTextContent(
        "Die Betreuung konnte nicht aktiviert werden.",
      );
    });
  });

  it("shows blocker resolution modal when disable blockers exist", async () => {
    mockGetTenantAccountCapability.mockResolvedValue(
      createState({
        disableBlockers: ["Aktive Gruppenaufsicht"],
        disableBlocked: true,
      }),
    );

    render(
      <CaregiverCapabilityModal
        isOpen={true}
        onClose={vi.fn()}
        scope="tenant"
        accountId="42"
        accountLabel="Ada Lovelace"
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Offene Zuordnungen")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Zuordnungen auflösen"));
    expect(screen.getByTestId("resolution-modal")).toBeInTheDocument();
  });

  it("surfaces blocker errors returned while disabling", async () => {
    mockGetTenantAccountCapability.mockResolvedValue(
      createState({
        disableBlockers: [],
        disableBlocked: false,
      }),
    );
    mockDisableTenantAccountCapability.mockRejectedValue(
      new MockCaregiverCapabilityApiError("Blockiert", 409, [
        "Aktive Gruppenaufsicht",
      ]),
    );

    render(
      <CaregiverCapabilityModal
        isOpen={true}
        onClose={vi.fn()}
        scope="tenant"
        accountId="42"
        accountLabel="Ada Lovelace"
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Betreuung deaktivieren")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Betreuung deaktivieren"));

    await waitFor(() => {
      expect(mockDisableTenantAccountCapability).toHaveBeenCalledWith("42");
    });
    expect(screen.getByTestId("alert")).toHaveTextContent("Blockiert");
    expect(mockToastError).toHaveBeenCalledWith(
      "Betreuung kann nicht deaktiviert werden. Bitte zuerst die offenen Zuordnungen entfernen.",
    );
  });
});
