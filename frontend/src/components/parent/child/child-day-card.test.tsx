import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, expect, it, vi } from "vitest";
import deMessages from "~/i18n/messages/de.json";
import type { ChildFeatures, ChildToday } from "~/lib/parent-api";
import { ChildDayCard, type ChildDayCardChild } from "./child-day-card";

vi.mock("~/lib/parent-url", () => ({
  parentPath: (path: string) => path,
}));

vi.mock("~/lib/hooks/use-berlin-today", () => ({
  useBerlinToday: () => "2026-05-14",
}));

const child: ChildDayCardChild = {
  studentId: "42",
  firstName: "Felix",
  lastName: "Schneider",
  schoolClass: "Klasse 1a",
};

const allFeatures = {
  sick_note_enabled: true,
  pickup_change_enabled: true,
  notes_enabled: true,
} as unknown as ChildFeatures;

const noFeatures = {
  sick_note_enabled: false,
  pickup_change_enabled: false,
  notes_enabled: false,
} as unknown as ChildFeatures;

function renderCard(today: ChildToday, features: ChildFeatures = allFeatures) {
  return render(
    <NextIntlClientProvider locale="de" messages={deMessages}>
      <ChildDayCard child={child} today={today} features={features} />
    </NextIntlClientProvider>,
  );
}

describe("ChildDayCard", () => {
  it("nennt das Kind und seine Klasse", () => {
    renderCard({ at_ogs: true, state: "present", since: "12:38" });
    expect(screen.getByText("Felix Schneider")).toBeInTheDocument();
    expect(screen.getByText("Klasse 1a")).toBeInTheDocument();
    expect(screen.getByTestId("child-card-profile-icon")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Profil ansehen" }),
    ).toHaveAttribute("href", "/parents/children/42");
  });

  it("verlinkt auf der Profilseite nicht auf sich selbst", () => {
    render(
      <NextIntlClientProvider locale="de" messages={deMessages}>
        <ChildDayCard
          child={child}
          today={{ at_ogs: true, state: "present", since: "12:38" }}
          features={allFeatures}
          hideIdentity
        />
      </NextIntlClientProvider>,
    );
    expect(
      screen.queryByRole("link", { name: "Profil ansehen" }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Heute" })).toBeInTheDocument();
    expect(
      screen.getByText("Tagesstatus und wichtige Aktionen auf einen Blick."),
    ).toBeInTheDocument();
    expect(screen.getByText("Aktueller Status")).toBeInTheDocument();
    expect(screen.getByText("Donnerstag, 14. Mai 2026")).toBeInTheDocument();
  });

  describe("Ebene 1 kommt ausschliesslich aus at_ogs", () => {
    it("zeigt 'In der OGS' bei at_ogs true", () => {
      renderCard({ at_ogs: true, state: "present", since: "12:38" });
      expect(screen.getByText("In der OGS")).toBeInTheDocument();
    });

    it("zeigt 'Nicht in der OGS' bei at_ogs false", () => {
      renderCard({ at_ogs: false, state: "left", until: "15:12" });
      expect(screen.getByText("Nicht in der OGS")).toBeInTheDocument();
    });

    // Der Kern von #2250/#2252: die Karte darf Ebene 1 niemals aus state
    // ableiten. Ein state "present" mit at_ogs null bleibt ohne Ja/Nein-Satz.
    it("schweigt bei at_ogs null, auch wenn state etwas anderes nahelegt", () => {
      renderCard({ at_ogs: null, state: "present", since: "12:38" });
      expect(screen.queryByText("In der OGS")).not.toBeInTheDocument();
      expect(screen.queryByText("Nicht in der OGS")).not.toBeInTheDocument();
    });
  });

  describe("Ebene 2 erklaert den Zustand", () => {
    const cases: ReadonlyArray<readonly [ChildToday, string]> = [
      [{ at_ogs: true, state: "present", since: "12:38" }, "Seit 12:38 Uhr da"],
      [
        { at_ogs: false, state: "left", until: "15:12" },
        "Um 15:12 Uhr nach Hause gegangen",
      ],
      [
        { at_ogs: false, state: "expected", expected_from: "12:30" },
        "Kommt heute um 12:30 Uhr",
      ],
      [
        { at_ogs: false, state: "not_arrived", expected_from: "12:30" },
        "Wird seit 12:30 Uhr erwartet",
      ],
      [{ at_ogs: false, state: "absent" }, "Heute abgemeldet"],
      [{ at_ogs: false, state: "no_care" }, "Heute keine Betreuung"],
      [{ at_ogs: null, state: "unknown" }, "Status derzeit nicht verfügbar"],
    ];

    it.each(cases)("%o ergibt die Zeile", (today, expected) => {
      renderCard(today);
      expect(screen.getByText(expected)).toBeInTheDocument();
    });
  });

  it("benennt jeden Zustand im Text und ergaenzt ihn mit einem Icon", () => {
    renderCard({ at_ogs: false, state: "absent" });
    expect(screen.getByText("Heute abgemeldet")).toBeInTheDocument();
    expect(screen.getByTestId("child-day-state-icon")).toBeInTheDocument();
  });

  describe("Aktionen", () => {
    it("zeigt in Uebersichten kompakte einzeilige Aktionen", () => {
      render(
        <NextIntlClientProvider locale="de" messages={deMessages}>
          <ChildDayCard
            child={child}
            today={{ at_ogs: true, state: "present", since: "12:38" }}
            features={allFeatures}
            actionDisplay="compact"
          />
        </NextIntlClientProvider>,
      );

      const sickAction = screen.getByRole("link", {
        name: "Felix abmelden",
      });
      expect(
        screen.getByRole("link", { name: "Abholzeit ändern" }),
      ).toBeInTheDocument();
      const messageAction = screen.getByRole("link", {
        name: "Nachricht schreiben",
      });
      expect(sickAction.parentElement).toHaveClass(
        "@md:grid-cols-2",
        "@2xl:grid-cols-3",
      );
      expect(messageAction).toHaveClass("@md:col-span-2", "@2xl:col-span-1");
      expect(screen.getByText("Felix abmelden")).toHaveClass(
        "min-w-0",
        "text-center",
      );
      expect(
        screen.queryByText("Krank melden oder entschuldigen"),
      ).not.toBeInTheDocument();
    });

    it("bietet alle drei an, wenn die Schule sie erlaubt", () => {
      renderCard({ at_ogs: true, state: "present", since: "12:38" });
      const sickAction = screen.getByRole("link", {
        name: "Felix abmelden Krank melden oder entschuldigen",
      });
      expect(sickAction).toBeInTheDocument();
      expect(
        screen.getByRole("link", {
          name: "Abholzeit für Felix ändern Abholzeit für einen Tag anpassen",
        }),
      ).toBeInTheDocument();
      const messageAction = screen.getByRole("link", {
        name: "Nachrichten schreiben OGS für Felix kontaktieren",
      });
      expect(messageAction).toBeInTheDocument();

      expect(sickAction.parentElement).toHaveClass(
        "@md:grid-cols-2",
        "@3xl:grid-cols-3",
      );
      expect(messageAction).toHaveClass("@md:col-span-2", "@3xl:col-span-1");
    });

    it("laesst weg, was die Schule nicht erlaubt", () => {
      renderCard(
        { at_ogs: true, state: "present", since: "12:38" },
        noFeatures,
      );
      expect(
        screen.queryByRole("link", {
          name: "Felix abmelden Krank melden oder entschuldigen",
        }),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByRole("link", {
          name: "Abholzeit für Felix ändern Abholzeit für einen Tag anpassen",
        }),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByRole("link", {
          name: "Nachrichten schreiben OGS für Felix kontaktieren",
        }),
      ).not.toBeInTheDocument();
    });

    it("ruft die Rueckrufe auf, statt zu verlinken, wenn die Seite die Dialoge selbst fuehrt", () => {
      const onSick = vi.fn();
      const onPickup = vi.fn();
      render(
        <NextIntlClientProvider locale="de" messages={deMessages}>
          <ChildDayCard
            child={child}
            today={{ at_ogs: true, state: "present", since: "12:38" }}
            features={allFeatures}
            onSick={onSick}
            onPickup={onPickup}
          />
        </NextIntlClientProvider>,
      );
      screen
        .getByRole("button", {
          name: "Felix abmelden Krank melden oder entschuldigen",
        })
        .click();
      screen
        .getByRole("button", {
          name: "Abholzeit für Felix ändern Abholzeit für einen Tag anpassen",
        })
        .click();
      expect(onSick).toHaveBeenCalledOnce();
      expect(onPickup).toHaveBeenCalledOnce();
    });
  });
});
