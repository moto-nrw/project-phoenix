import { act, fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ModalProvider } from "~/components/dashboard/modal-context";
import type { UnregisteredTagScan } from "~/lib/operator/provisioning-helpers";
import { ResolveScanModal } from "./page";

const scan: UnregisteredTagScan = {
  id: "scan-1",
  tenantId: "school-1",
  tagUid: "04AABBCC",
  deviceId: "device-1",
  scannedAt: "2026-08-31T10:00:00Z",
  resolvedAt: null,
  resolvedByOperatorId: null,
  resolutionNote: null,
  createdAt: "2026-08-31T10:00:00Z",
  updatedAt: "2026-08-31T10:00:00Z",
  schoolId: "school-1",
  schoolName: "Testschule",
  organizationId: "org-1",
  organizationName: "Testträger",
  deviceIdentifier: "kiosk-1",
  deviceName: "Eingang",
};

describe("ResolveScanModal", () => {
  it("keeps notes on backdrop click but closes on Escape", async () => {
    vi.useFakeTimers();
    const onClose = vi.fn();

    try {
      render(
        <ModalProvider>
          <ResolveScanModal
            scan={scan}
            note=""
            error=""
            loading={false}
            onNoteChange={vi.fn()}
            onClose={onClose}
            onResolve={vi.fn()}
          />
        </ModalProvider>,
      );

      expect(
        screen.getByRole("dialog", { name: "Scan erledigen" }),
      ).toBeInTheDocument();

      fireEvent.click(
        screen.getByRole("button", {
          name: "Hintergrund - Klicken zum Schließen",
        }),
      );
      expect(onClose).not.toHaveBeenCalled();

      fireEvent.keyDown(document, { key: "Escape" });
      await act(async () => {
        vi.advanceTimersByTime(300);
      });

      expect(onClose).toHaveBeenCalledOnce();
    } finally {
      vi.useRealTimers();
    }
  });
});
