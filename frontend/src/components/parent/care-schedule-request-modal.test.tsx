import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, expect, it, vi } from "vitest";
import deMessages from "~/i18n/messages/de.json";
import { ApiError } from "~/lib/api-error";
import { ParentApiError } from "~/lib/parent-api";
import { catalogText } from "~/test/error-catalog-text";
import { CareScheduleRequestModal } from "./care-schedule-request-modal";

const weekdays = [
  {
    weekday: 1,
    status: "scheduled" as const,
    pickup: "15:00",
    modes: ["pickup"],
  },
  {
    weekday: 2,
    status: "not_scheduled" as const,
    modes: [],
  },
];

function renderModal(
  capabilities: Readonly<{
    arrival: boolean;
    pickup: boolean;
    departure_mode: boolean;
  }>,
  onSubmit = vi.fn().mockResolvedValue(undefined),
) {
  render(
    <NextIntlClientProvider locale="de" messages={deMessages}>
      <CareScheduleRequestModal
        weekdays={weekdays}
        capabilities={capabilities}
        onClose={vi.fn()}
        onSubmit={onSubmit}
      />
    </NextIntlClientProvider>,
  );
  return onSubmit;
}

describe("CareScheduleRequestModal", () => {
  it("zeigt und sendet bei reiner Abholzeit-Freigabe keine Abholart", async () => {
    const onSubmit = renderModal({
      arrival: false,
      pickup: true,
      departure_mode: false,
    });

    expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
    expect(
      screen.queryByText("Betreuung an diesem Tag"),
    ).not.toBeInTheDocument();
    const pickup = screen.getByRole("textbox", { name: /^Abholzeit/ });
    fireEvent.change(pickup, { target: { value: "16:30" } });
    fireEvent.click(
      screen.getByRole("button", { name: "Anfrage an OGS senden" }),
    );

    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith({
        weekdays: [{ weekday: 1, pickup: "16:30" }],
      }),
    );
  });

  it("zeigt und sendet bei reiner Abholart-Freigabe keine Abholzeit", async () => {
    const onSubmit = renderModal({
      arrival: false,
      pickup: false,
      departure_mode: true,
    });

    expect(
      screen.queryByRole("textbox", { name: /^Abholzeit/ }),
    ).not.toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("combobox", { name: "Abholart am Montag" }),
    );
    fireEvent.click(screen.getByRole("option", { name: "Bus" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Anfrage an OGS senden" }),
    );

    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith({
        weekdays: [{ weekday: 1, mode: "bus" }],
      }),
    );
  });

  it("fügt bei beiden Freigaben einen Betreuungstag vollständig hinzu", async () => {
    const onSubmit = renderModal({
      arrival: false,
      pickup: true,
      departure_mode: true,
    });
    const tuesday = screen.getByRole("group", { name: "Dienstag" });
    fireEvent.click(within(tuesday).getByLabelText("Betreuung an diesem Tag"));
    fireEvent.change(
      within(tuesday).getByRole("textbox", { name: /^Abholzeit/ }),
      {
        target: { value: "16:30" },
      },
    );
    fireEvent.click(within(tuesday).getByRole("combobox"));
    fireEvent.click(screen.getByRole("option", { name: "Bus" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Anfrage an OGS senden" }),
    );

    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith({
        weekdays: [
          {
            weekday: 2,
            scheduled: true,
            pickup: "16:30",
            mode: "bus",
          },
        ],
      }),
    );
  });

  it("entfernt bei beiden Freigaben einen Betreuungstag", async () => {
    const onSubmit = renderModal({
      arrival: false,
      pickup: true,
      departure_mode: true,
    });
    const monday = screen.getByRole("group", { name: "Montag" });
    fireEvent.click(within(monday).getByLabelText("Betreuung an diesem Tag"));
    fireEvent.click(
      screen.getByRole("button", { name: "Anfrage an OGS senden" }),
    );

    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith({
        weekdays: [{ weekday: 1, scheduled: false }],
      }),
    );
  });
});

describe("CareScheduleRequestModal Fehlerweg (#2518)", () => {
  const capabilities = { arrival: false, pickup: true, departure_mode: false };

  async function submitPickupChange() {
    fireEvent.change(screen.getByRole("textbox", { name: /^Abholzeit/ }), {
      target: { value: "16:30" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Anfrage an OGS senden" }),
    );
    return screen.findByRole("alert");
  }

  it("zeigt einen Serverfehler mit Katalogtext statt Serversatz", async () => {
    renderModal(
      capabilities,
      vi
        .fn()
        .mockRejectedValue(
          new ApiError("backend sentence", 500, { code: "general.server" }),
        ),
    );

    expect(await submitPickupChange()).toHaveTextContent(
      catalogText(
        "general.server",
        deMessages.parentMasterData.careSchedule.requestErrorObject,
      ),
    );
    expect(screen.queryByText(/backend sentence/)).not.toBeInTheDocument();
  });

  it("erklärt eine schon offene Anfrage mit eigenem Text", async () => {
    renderModal(
      capabilities,
      vi
        .fn()
        .mockRejectedValue(
          new ParentApiError(
            "pending",
            409,
            "care.care_request_already_pending",
          ),
        ),
    );

    expect(await submitPickupChange()).toHaveTextContent(
      deMessages.parentMasterData.careSchedule.requestAlreadyPending,
    );
  });
});
