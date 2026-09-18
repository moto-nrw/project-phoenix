import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { PickupExtension } from "~/lib/pickup-extension-api";

const state = vi.hoisted(() => ({
  tasks: [] as PickupExtension[],
  enabledCalls: [] as boolean[],
  error: undefined as Error | undefined,
  refresh: vi.fn(),
}));

vi.mock("~/lib/hooks/use-pickup-extensions", () => ({
  usePickupExtensions: (enabled: boolean) => {
    state.enabledCalls.push(enabled);
    return {
      tasks: enabled ? state.tasks : [],
      error: enabled ? state.error : undefined,
      refresh: state.refresh,
      isLoading: false,
    };
  },
}));
vi.mock(
  "~/components/timetable/pickup-extension-dialog",
  async (importActual) => {
    const actual =
      await importActual<
        typeof import("~/components/timetable/pickup-extension-dialog")
      >();
    return {
      ...actual,
      PickupExtensionDialog: ({
        tasks,
        isOpen,
      }: {
        tasks: readonly PickupExtension[];
        isOpen: boolean;
      }) =>
        isOpen ? (
          <div data-testid="extension-dialog">
            {tasks.map((task) => task.id).join(",")}
          </div>
        ) : null,
    };
  },
);

import { PickupExtensionTodos } from "./pickup-extension-todos";

function task(id: string, name: string): PickupExtension {
  return {
    id,
    studentId: `s${id}`,
    studentName: name,
    kind: "day",
    date: "2026-09-11",
    previousPickupTime: "15:00",
    pickupTime: "16:30",
    blocks: [
      { id: "1", title: "Freies Spiel", startTime: "14:45", endTime: "17:00" },
    ],
  };
}

describe("PickupExtensionTodos (#3261)", () => {
  beforeEach(() => {
    state.tasks = [];
    state.enabledCalls = [];
    state.error = undefined;
  });

  it("fragt ohne Recht am Betreuungsplan nichts ab und zeigt nichts", () => {
    state.tasks = [task("1", "Lina Schröder")];
    const { container } = render(<PickupExtensionTodos enabled={false} />);

    expect(container).toBeEmptyDOMElement();
    expect(state.enabledCalls.every((value) => !value)).toBe(true);
  });

  it("zeigt nichts, solange keine Aufgabe offen ist", () => {
    const { container } = render(<PickupExtensionTodos enabled />);
    expect(container).toBeEmptyDOMElement();
  });

  it("zeigt einen Hinweis, wenn die Aufgaben nicht geladen werden können", () => {
    state.error = new Error("request failed");
    render(<PickupExtensionTodos enabled />);

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Die Aufgaben konnten nicht geladen werden. Bitte laden Sie die Seite neu.",
    );
  });

  it("nennt Kind, Tag und neue Abholzeit und öffnet die Auswahl", () => {
    state.tasks = [task("1", "Lina Schröder")];
    render(<PickupExtensionTodos enabled />);

    expect(screen.getByText("Längere Betreuung eintragen")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Lina Schröder wird am Freitag, 11.09.2026 erst um 16:30 Uhr abgeholt und ist in dieser Zeit in keinem Termin.",
      ),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Termin wählen" }));

    expect(screen.getByTestId("extension-dialog")).toHaveTextContent("1");
  });

  it("fasst mehrere Aufgaben zusammen und geht sie nacheinander durch", () => {
    state.tasks = [
      task("1", "Lina Schröder"),
      task("2", "Sophie Schwarz"),
      task("3", "Lina Schröder"),
    ];
    render(<PickupExtensionTodos enabled />);

    expect(
      screen.getByText(
        "3 spätere Abholzeiten ohne Termin: Lina Schröder, Sophie Schwarz.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Termine wählen" }));
    expect(screen.getByTestId("extension-dialog")).toHaveTextContent("1,2,3");
  });
});
