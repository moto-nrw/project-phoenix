import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

import { ModalProvider } from "~/components/ui/modal-context";
import type { CareBeyondPhaseEnd } from "~/lib/enrollment-phase-api";
import { PhaseCareBeyondEndDialog } from "./phase-care-beyond-end-dialog";

const students: CareBeyondPhaseEnd[] = [
  {
    studentId: "7",
    firstName: "Mia",
    lastName: "Lang",
    schoolClass: "2b",
    careEnd: "2027-07-31",
    phaseEnd: "2027-06-30",
  },
  {
    studentId: "8",
    firstName: "Ben",
    lastName: "Berg",
    schoolClass: "",
    careEnd: "2027-07-15",
    phaseEnd: "2027-06-30",
  },
];

function renderDialog(
  props: Partial<Parameters<typeof PhaseCareBeyondEndDialog>[0]> = {},
) {
  const onEndCare = vi.fn();
  const onClose = vi.fn();
  render(
    <ModalProvider>
      <PhaseCareBeyondEndDialog
        phaseName="Schuljahr 2026/27"
        phaseEnd="2027-06-30"
        students={students}
        canEndCare
        onEndCare={onEndCare}
        onClose={onClose}
        {...props}
      />
    </ModalProvider>,
  );
  return { onEndCare, onClose };
}

describe("PhaseCareBeyondEndDialog", () => {
  // #6598: Die Kürzung nennt jedes betreute Kind mit seinem Betreuungsende
  // und dem neuen Phasenende; moto beendet nichts von selbst.
  it("names every child with its care end and the new phase end", () => {
    const { onEndCare, onClose } = renderDialog();

    expect(
      screen.getByRole("heading", {
        name: "Betreuung läuft länger als die Phase",
      }),
    ).toBeVisible();
    expect(
      screen.getByText(
        "Die Anmeldephase „Schuljahr 2026/27“ endet jetzt am 30.06.2027. Bei 2 Kindern ist die Betreuung länger eingetragen. moto beendet sie nicht von selbst.",
      ),
    ).toBeVisible();
    expect(screen.getByText("Mia Lang")).toBeVisible();
    expect(screen.getByText("Klasse 2b")).toBeVisible();
    expect(screen.getByText("Betreuung bis 31.07.2027")).toBeVisible();
    expect(screen.getByText("Ben Berg")).toBeVisible();
    expect(screen.getByText("Betreuung bis 15.07.2027")).toBeVisible();
    expect(
      screen.getByText(
        "Soll die Betreuung mit der Phase enden? Dann beenden Sie sie zum 30.06.2027.",
      ),
    ).toBeVisible();

    fireEvent.click(screen.getByRole("button", { name: "Betreuung beenden" }));
    expect(onEndCare).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByRole("button", { name: "Später" }));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("says that the care can end today at the earliest once the new end has passed", () => {
    renderDialog({ phaseEnd: "2026-09-01" });

    expect(
      screen.getByText(
        "Soll die Betreuung mit der Phase enden? Das neue Ende ist schon vorbei. Die Betreuung kann frühestens heute enden.",
      ),
    ).toBeVisible();
  });

  // Wer die Betreuung nicht beenden darf, bekommt keinen Knopf, der mit 403
  // scheitert, sondern den nächsten Schritt.
  it("names who can end the care when the person may not", () => {
    const { onClose } = renderDialog({ canEndCare: false });

    expect(
      screen.getByText(
        "Die Betreuung beenden darf nur, wer Kinder verwalten darf. Bitte geben Sie diese Liste an diese Person weiter.",
      ),
    ).toBeVisible();
    expect(screen.getByRole("button", { name: "Schließen" })).toBeVisible();
    expect(
      screen.queryByRole("button", { name: "Betreuung beenden" }),
    ).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Schließen" }));
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
