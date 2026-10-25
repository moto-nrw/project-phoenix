import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const {
  mockCreateTestTraeger,
  mockCreateSampleChildren,
  mockToastSuccess,
  mockToastWarning,
} = vi.hoisted(() => ({
  mockCreateTestTraeger: vi.fn(),
  mockCreateSampleChildren: vi.fn(),
  mockToastSuccess: vi.fn(),
  mockToastWarning: vi.fn(),
}));

vi.mock("~/lib/operator/provisioning-api", () => ({
  operatorProvisioningService: {
    createTestTraeger: mockCreateTestTraeger,
    createSampleChildren: mockCreateSampleChildren,
  },
}));

vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({
    success: mockToastSuccess,
    error: vi.fn(),
    info: vi.fn(),
    warning: mockToastWarning,
  }),
}));

import { ToastProvider } from "~/contexts/ToastContext";
import { CreateTestTraegerModal } from "./create-test-traeger-modal";

const created = {
  organization: { id: "31", slug: "test-bissendorf", testSystem: true },
  schools: [{ id: "1" }, { id: "2" }, { id: "3" }],
};

function renderModal() {
  const onCreated = vi.fn().mockResolvedValue(undefined);
  const onClose = vi.fn();
  render(
    <ToastProvider>
      <CreateTestTraegerModal isOpen onClose={onClose} onCreated={onCreated} />
    </ToastProvider>,
  );
  fireEvent.change(screen.getByLabelText(/^Name/), {
    target: { value: "Test Bissendorf" },
  });
  fireEvent.change(screen.getByLabelText(/^Kontaktadresse der Test-Schulen/), {
    target: { value: "buero@traeger.example" },
  });
  return { onCreated, onClose };
}

describe("CreateTestTraegerModal", () => {
  beforeEach(() => {
    mockCreateTestTraeger.mockReset();
    mockCreateSampleChildren.mockReset();
    mockToastSuccess.mockReset();
    mockToastWarning.mockReset();
  });

  it("creates the test Träger and then its sample children", async () => {
    mockCreateTestTraeger.mockResolvedValue(created);
    mockCreateSampleChildren.mockResolvedValue({
      createdCount: 72,
      skippedCount: 0,
    });
    const { onCreated } = renderModal();

    fireEvent.click(screen.getByRole("button", { name: "Anlegen" }));

    await waitFor(() =>
      expect(onCreated).toHaveBeenCalledWith("test-bissendorf"),
    );
    expect(mockCreateTestTraeger).toHaveBeenCalledWith({
      name: "Test Bissendorf",
      slug: "test-bissendorf",
      school_count: 3,
      contact_email: "buero@traeger.example",
    });
    expect(mockCreateSampleChildren).toHaveBeenCalledWith("31");
    expect(mockToastSuccess).toHaveBeenCalledWith(
      "Der Test-Träger ist angelegt: 3 Test-Schulen mit 72 Beispielkindern.",
    );
  });

  it("keeps the created Träger and points to its page when the children fail", async () => {
    mockCreateTestTraeger.mockResolvedValue(created);
    mockCreateSampleChildren.mockRejectedValue(new Error("offline"));
    const { onCreated } = renderModal();

    fireEvent.click(screen.getByRole("button", { name: "Anlegen" }));

    await waitFor(() =>
      expect(onCreated).toHaveBeenCalledWith("test-bissendorf"),
    );
    expect(mockToastWarning).toHaveBeenCalledWith(
      expect.stringContaining("die Beispielkinder fehlen noch"),
    );
    expect(mockToastSuccess).not.toHaveBeenCalled();
  });

  it("stays open and creates no children when the Träger is refused", async () => {
    mockCreateTestTraeger.mockRejectedValue(new Error("slug taken"));
    const { onCreated } = renderModal();

    fireEvent.click(screen.getByRole("button", { name: "Anlegen" }));

    await waitFor(() => expect(mockCreateTestTraeger).toHaveBeenCalled());
    expect(screen.getByRole("button", { name: "Anlegen" })).toBeInTheDocument();
    expect(mockCreateSampleChildren).not.toHaveBeenCalled();
    expect(onCreated).not.toHaveBeenCalled();
  });
});
