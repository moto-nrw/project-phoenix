import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, expect, it, vi } from "vitest";
import deMessages from "~/i18n/messages/de.json";
import { ParentApiError, type ChildCareSchedule } from "~/lib/parent-api";
import { PickupScheduleRequestModal } from "./pickup-schedule-request-modal";

const schedule: ChildCareSchedule = {
  weekdays: [
    {
      weekday: 1,
      status: "scheduled",
      arrival: "08:00",
      pickup: "16:00",
      modes: ["bus"],
    },
  ],
  can_request: true,
  request_capabilities: {
    arrival: true,
    pickup: true,
    departure_mode: true,
  },
  today_absent: false,
};

function renderModal(onSubmit = vi.fn().mockResolvedValue(undefined)) {
  render(
    <NextIntlClientProvider locale="de" messages={deMessages}>
      <PickupScheduleRequestModal
        schedule={schedule}
        onClose={vi.fn()}
        onSubmit={onSubmit}
      />
    </NextIntlClientProvider>,
  );
  return onSubmit;
}

describe("PickupScheduleRequestModal", () => {
  it("erklaert die dauerhafte Anfrage und zeigt vollstaendige Aktionen", () => {
    renderModal();

    expect(
      screen.getByRole("heading", {
        name: "Änderung am Wochenplan anfragen",
      }),
    ).toBeInTheDocument();
    expect(screen.getByText("Anfrage an die OGS")).toBeInTheDocument();
    expect(
      screen.getByText(/Die Änderung ist dauerhaft und gilt jede Woche/),
    ).toBeInTheDocument();
    const cancel = screen.getByRole("button", { name: "Abbrechen" });
    const submit = screen.getByRole("button", {
      name: "Anfrage an OGS senden",
    });
    expect(cancel).toHaveClass("rounded-lg", "px-4", "py-2", "text-sm");
    expect(submit).toHaveClass("rounded-lg", "px-4", "py-2", "text-sm");
    expect(cancel).not.toHaveClass("min-h-12", "text-[17px]");
    expect(submit).not.toHaveClass("min-h-12", "text-[17px]");
    expect(
      screen.queryByText("parentMasterData.careOfferings.cancel"),
    ).not.toBeInTheDocument();
  });

  it("bietet Abholzeit und Abholart an, aber keine Bringzeit", async () => {
    const onSubmit = renderModal();

    expect(screen.queryByText("Ankunft")).not.toBeInTheDocument();
    expect(
      screen.getAllByRole("checkbox", { name: "Betreuung an diesem Tag" }),
    ).toHaveLength(5);
    expect(
      screen.getAllByLabelText(/^Abholzeit/, { selector: "input" }),
    ).toHaveLength(1);
    expect(
      screen.getByRole("combobox", { name: "Abholart" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Abholart")).toBeInTheDocument();

    fireEvent.change(
      screen.getAllByLabelText(/^Abholzeit/, { selector: "input" })[0]!,
      { target: { value: "1530" } },
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Anfrage an OGS senden" }),
    );

    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith({
        weekdays: [{ weekday: 1, pickup: "15:30" }],
      }),
    );
  });

  it("sendet das Entfernen eines Betreuungstags ohne alte Planwerte", async () => {
    const onSubmit = renderModal();

    fireEvent.click(
      screen.getAllByRole("checkbox", {
        name: "Betreuung an diesem Tag",
      })[0]!,
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Anfrage an OGS senden" }),
    );

    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith({
        weekdays: [{ weekday: 1, scheduled: false }],
      }),
    );
  });

  it.each([
    [
      "care_request_already_pending",
      "Für dieses Kind wird bereits eine Änderung geprüft.",
    ],
    [
      "care_request_field_disabled",
      "Diese Änderung ist für Ihre OGS nicht freigeschaltet.",
    ],
    [
      "invalid_request_payload",
      "Bitte prüfen Sie die ausgewählten Betreuungstage, Abholzeiten und Abholarten.",
    ],
  ])("lokalisiert den API-Fehler %s", async (code, message) => {
    renderModal(
      vi.fn().mockRejectedValue(new ParentApiError("backend text", 409, code)),
    );
    fireEvent.change(
      screen.getAllByLabelText(/^Abholzeit/, { selector: "input" })[0]!,
      { target: { value: "1530" } },
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Anfrage an OGS senden" }),
    );

    expect(await screen.findByText(message)).toBeInTheDocument();
    expect(screen.queryByText("backend text")).not.toBeInTheDocument();
  });
});
