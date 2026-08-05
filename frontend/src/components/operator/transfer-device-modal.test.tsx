import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";

const { mockGetStatus, mockTransfer, mockToastSuccess } = vi.hoisted(() => ({
  mockGetStatus: vi.fn(),
  mockTransfer: vi.fn(),
  mockToastSuccess: vi.fn(),
}));

vi.mock("~/lib/operator/provisioning-api", () => ({
  operatorProvisioningService: {
    getDeviceTransferStatus: mockGetStatus,
    transferDevice: mockTransfer,
  },
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: mockToastSuccess }),
}));

vi.mock("~/components/ui/form-modal", () => ({
  FormModal: ({
    isOpen,
    title,
    children,
    footer,
  }: {
    isOpen: boolean;
    title: string;
    children: ReactNode;
    footer: ReactNode;
  }) =>
    isOpen ? (
      <section aria-label={title}>
        <h2>{title}</h2>
        {children}
        {footer}
      </section>
    ) : null,
}));

vi.mock("~/components/ui/custom-select", () => ({
  CustomSelect: ({
    value,
    options,
    onChange,
    disabled,
  }: {
    value: string;
    options: { value: string; label: string }[];
    onChange: (value: string) => void;
    disabled: boolean;
  }) => (
    <select
      aria-label="Zielschule"
      value={value}
      onChange={(event) => onChange(event.target.value)}
      disabled={disabled}
    >
      <option value="">Bitte wählen</option>
      {options.map((option) => (
        <option key={option.value} value={option.value}>
          {option.label}
        </option>
      ))}
    </select>
  ),
}));

import { TransferDeviceModal } from "./transfer-device-modal";
import type {
  OperatorDevice,
  School,
} from "~/lib/operator/provisioning-helpers";

const device: OperatorDevice = {
  id: "1",
  deviceId: "BURBACH-2",
  deviceType: "terminal",
  name: "Burbach 2",
  status: "active",
  apiKey: "dev-key",
  maskedApiKey: "dev-key",
  lastSeen: null,
  isOnline: false,
  schoolId: "10",
  schoolName: "Burbach",
  organizationId: "5",
  organizationName: "Talent OGS",
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
};

function school(id: string, name: string, organizationId = "5"): School {
  return {
    id,
    organizationId,
    name,
    slug: name.toLowerCase(),
    subdomain: name.toLowerCase(),
    address: "",
    city: "",
    zip: "",
    phone: "",
    email: "",
    active: true,
    hidden: false,
    deletedAt: null,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  };
}

describe("TransferDeviceModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("filters destinations to the same organization and transfers", async () => {
    mockGetStatus.mockResolvedValue({
      canTransfer: true,
      isOnline: false,
      lastSeen: null,
      activeSession: null,
    });
    mockTransfer.mockResolvedValue({
      ...device,
      id: "2",
      schoolId: "20",
      schoolName: "Walbach",
    });
    const onTransferred = vi.fn();
    const onClose = vi.fn();

    render(
      <TransferDeviceModal
        device={device}
        schools={[
          school("10", "Burbach"),
          school("20", "Walbach"),
          school("30", "Fremdschule", "9"),
        ]}
        onClose={onClose}
        onTransferred={onTransferred}
      />,
    );

    await screen.findByText(
      "Das Gerät ist offline und hat keine offene Sitzung.",
    );
    expect(screen.getByRole("option", { name: "Walbach" })).toBeInTheDocument();
    expect(
      screen.queryByRole("option", { name: "Fremdschule" }),
    ).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Zielschule"), {
      target: { value: "20" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Schule wechseln" }));

    await waitFor(() => expect(mockTransfer).toHaveBeenCalledWith("1", "20"));
    expect(onTransferred).toHaveBeenCalledTimes(1);
    expect(mockToastSuccess).toHaveBeenCalledWith(
      "Burbach 2 wurde nach Walbach verschoben.",
    );
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("shows online and session blockers and disables transfer", async () => {
    mockGetStatus.mockResolvedValue({
      canTransfer: false,
      isOnline: true,
      lastSeen: "2026-08-05T12:00:00Z",
      activeSession: {
        id: "77",
        startedAt: "2026-08-05T11:00:00Z",
        activityName: "Mensa",
        roomName: "Speisesaal",
      },
    });

    render(
      <TransferDeviceModal
        device={device}
        schools={[school("10", "Burbach"), school("20", "Walbach")]}
        onClose={vi.fn()}
        onTransferred={vi.fn()}
      />,
    );

    expect(await screen.findByText(/Das Gerät ist online/)).toBeInTheDocument();
    expect(
      screen.getByText(/Offene Sitzung.*Mensa.*Speisesaal/),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Schule wechseln" }),
    ).toBeDisabled();
    expect(mockTransfer).not.toHaveBeenCalled();
  });
});
