import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mockToast = vi.hoisted(() => ({
  success: vi.fn(),
  error: vi.fn(),
  warning: vi.fn(),
  info: vi.fn(),
}));
vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => mockToast,
}));

vi.mock("~/lib/pickup-extension-api", async (importActual) => {
  const actual =
    await importActual<typeof import("~/lib/pickup-extension-api")>();
  return { ...actual, resolvePickupExtension: vi.fn() };
});

import {
  PickupExtensionApiError,
  resolvePickupExtension,
  type PickupExtension,
} from "~/lib/pickup-extension-api";
import { PickupExtensionDialog } from "./pickup-extension-dialog";

const mockResolve = vi.mocked(resolvePickupExtension);

function dayTask(overrides: Partial<PickupExtension> = {}): PickupExtension {
  return {
    id: "7",
    studentId: "42",
    studentName: "Mia Beispiel",
    kind: "day",
    date: "2026-09-11",
    previousPickupTime: "14:45",
    pickupTime: "16:00",
    blocks: [
      { id: "31", title: "Freies Spiel", startTime: "14:45", endTime: "16:00" },
    ],
    ...overrides,
  };
}

function weekdayTask(): PickupExtension {
  return {
    id: "8",
    studentId: "42",
    studentName: "Mia Beispiel",
    kind: "weekday",
    weekday: 2,
    previousPickupTime: "14:45",
    pickupTime: "16:00",
    blocks: [
      { id: "51", title: "Freies Spiel", startTime: "14:45", endTime: "16:00" },
      { id: "52", title: "AG Fußball", startTime: "15:00", endTime: "16:00" },
    ],
  };
}

describe("PickupExtensionDialog (#3261)", () => {
  beforeEach(() => {
    mockResolve.mockReset();
    mockToast.success.mockReset();
  });

  it("wählt den einzigen passenden Termin vor, ein Tipp genügt", async () => {
    mockResolve.mockResolvedValue(undefined);
    const onClose = vi.fn();
    const onChanged = vi.fn();
    render(
      <PickupExtensionDialog
        tasks={[dayTask()]}
        isOpen
        onClose={onClose}
        onChanged={onChanged}
      />,
    );

    expect(
      screen.getByText(/wird am Freitag, 11\.09\.2026 erst um 16:00 Uhr/),
    ).toBeInTheDocument();
    expect(screen.getByText(/Bisher: 14:45 Uhr/)).toBeInTheDocument();
    expect(
      screen.getByRole("checkbox", { name: /Freies Spiel/ }),
    ).toBeChecked();

    fireEvent.click(screen.getByRole("button", { name: "Eintragen" }));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(mockResolve).toHaveBeenCalledWith("7", ["31"]);
    expect(onChanged).toHaveBeenCalled();
    expect(mockToast.success).toHaveBeenCalledWith(
      "Mia Beispiel ist jetzt bei Freies Spiel eingetragen.",
    );
  });

  it("schließt die Frage auch ohne Termin", async () => {
    mockResolve.mockResolvedValue(undefined);
    const onClose = vi.fn();
    render(
      <PickupExtensionDialog tasks={[dayTask()]} isOpen onClose={onClose} />,
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Keinem Termin zuordnen" }),
    );

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(mockResolve).toHaveBeenCalledWith("7", []);
  });

  it("verlangt bei mehreren Terminen eine Wahl und nennt den Wochentag", async () => {
    mockResolve.mockResolvedValue(undefined);
    render(
      <PickupExtensionDialog
        tasks={[weekdayTask()]}
        isOpen
        onClose={vi.fn()}
      />,
    );

    expect(
      screen.getByText(/wird ab jetzt jeden Dienstag erst um 16:00 Uhr/),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Gilt für alle kommenden Dienstage\./),
    ).toBeInTheDocument();
    const submit = screen.getByRole("button", { name: "Eintragen" });
    expect(submit).toBeDisabled();

    fireEvent.click(screen.getByRole("checkbox", { name: /AG Fußball/ }));
    fireEvent.click(screen.getByRole("checkbox", { name: /Freies Spiel/ }));
    fireEvent.click(submit);

    await waitFor(() => expect(mockResolve).toHaveBeenCalled());
    expect(mockResolve.mock.calls[0]?.[1]).toEqual(["52", "51"]);
  });

  it("führt nacheinander durch mehrere Fragen", async () => {
    mockResolve.mockResolvedValue(undefined);
    const onClose = vi.fn();
    render(
      <PickupExtensionDialog
        tasks={[dayTask(), weekdayTask()]}
        isOpen
        onClose={onClose}
      />,
    );

    expect(
      screen.getByText("Längere Betreuung eintragen (1 von 2)"),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Eintragen" }));

    expect(
      await screen.findByText("Längere Betreuung eintragen (2 von 2)"),
    ).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("bleibt offen und erklärt, wenn sich der Termin geändert hat", async () => {
    mockResolve.mockRejectedValue(
      new PickupExtensionApiError("gone", 409, "pickup_extension_block_gone"),
    );
    const onClose = vi.fn();
    const onChanged = vi.fn();
    render(
      <PickupExtensionDialog
        tasks={[dayTask()]}
        isOpen
        onClose={onClose}
        onChanged={onChanged}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Eintragen" }));

    expect(
      await screen.findByText(
        "Der Termin hat sich inzwischen geändert. Bitte wählen Sie noch einmal.",
      ),
    ).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
    expect(onChanged).toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Eintragen" })).toBeEnabled();
  });

  it("zeigt nichts ohne offene Frage", () => {
    const { container } = render(
      <PickupExtensionDialog tasks={[]} isOpen onClose={vi.fn()} />,
    );
    expect(container).toBeEmptyDOMElement();
  });
});
