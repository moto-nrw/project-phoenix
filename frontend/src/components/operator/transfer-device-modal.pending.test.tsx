// #6533: Schließen während des Verschiebens ist gesperrt, eine späte Antwort
// schließt nicht den Dialog eines anderen Geräts. Echter FormModal.
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import type {
  OperatorDevice,
  School,
} from "~/lib/operator/provisioning-helpers";
import {
  deferred,
  expectDialogUnlocked,
  expectSaveLocksDialog,
} from "~/test/helpers/form-modal-pending";

const { mockGetStatus, mockTransfer } = vi.hoisted(() => ({
  mockGetStatus: vi.fn(),
  mockTransfer: vi.fn(),
}));

vi.mock("~/lib/operator/provisioning-api", () => ({
  operatorProvisioningService: {
    getDeviceTransferStatus: mockGetStatus,
    transferDevice: mockTransfer,
  },
}));
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({ success: vi.fn() }),
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

const schools = [school("10", "Burbach"), school("20", "Walbach")];

async function renderAndTransfer() {
  const answer = deferred<OperatorDevice>();
  mockGetStatus.mockResolvedValue({
    canTransfer: true,
    isOnline: false,
    lastSeen: null,
    activeSession: null,
  });
  mockTransfer.mockReturnValue(answer.promise);
  const onClose = vi.fn();
  const onTransferred = vi.fn();
  const modal = (shown: OperatorDevice) => (
    <TransferDeviceModal
      device={shown}
      schools={schools}
      onClose={onClose}
      onTransferred={onTransferred}
    />
  );
  const view = render(modal(device));
  await screen.findByText(
    "Das Gerät ist offline und hat keine offene Sitzung.",
  );
  fireEvent.change(screen.getByLabelText("Zielschule"), {
    target: { value: "20" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Schule wechseln" }));
  return { ...view, modal, answer, onClose, onTransferred };
}

describe("TransferDeviceModal pending lock (#6533)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("keeps the dialog shut while moving and unlocks it after a failure", async () => {
    const { answer, onClose } = await renderAndTransfer();

    expectSaveLocksDialog("Gerät verschieben");

    await act(async () => {
      answer.reject(
        new ApiError("busy", 409, { code: "general.business_rejection" }),
      );
    });
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expectDialogUnlocked("Gerät verschieben");
    expect(onClose).not.toHaveBeenCalled();
  });

  it("does not close the dialog of another device with a late answer", async () => {
    const { answer, modal, rerender, onClose, onTransferred } =
      await renderAndTransfer();
    expect(mockTransfer).toHaveBeenCalledWith("1", "20");

    // Der Aufrufer zeigt inzwischen ein anderes Gerät im selben Dialog.
    rerender(modal({ ...device, id: "7", deviceId: "WALBACH-1" }));
    await act(async () => {
      answer.resolve({ ...device, schoolId: "20", schoolName: "Walbach" });
    });

    expect(onTransferred).toHaveBeenCalledOnce();
    expect(onClose).not.toHaveBeenCalled();
    expect(
      screen.getByRole("dialog", { name: "Gerät verschieben" }),
    ).not.toHaveAttribute("aria-busy");
  });
});
