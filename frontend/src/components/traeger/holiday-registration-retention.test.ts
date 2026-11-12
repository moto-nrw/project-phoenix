import { describe, expect, it } from "vitest";
import {
  holidayDeletionNotice,
  holidayRegistrationDeletionNotice,
} from "./holiday-registration-retention";

describe("holidayDeletionNotice", () => {
  it("names the one day and the earlier day of the health information", () => {
    expect(
      holidayDeletionNotice([
        { deletesOn: "2028-01-16", healthDeletesOn: "2027-04-18" },
        { deletesOn: "2028-01-16", healthDeletesOn: "2027-04-18" },
      ]),
    ).toBe(
      "moto löscht diese Anmeldungen am 16.01.2028. Laden Sie sie vorher herunter, wenn Sie sie länger brauchen. Gesundheitsangaben löscht moto schon am 18.04.2027.",
    );
  });

  it("names the span when registrations keep different retentions", () => {
    expect(
      holidayDeletionNotice([
        { deletesOn: "2028-01-16", healthDeletesOn: "2027-04-18" },
        { deletesOn: "2027-04-18", healthDeletesOn: "2027-04-18" },
      ]),
    ).toBe(
      "moto löscht diese Anmeldungen zwischen dem 18.04.2027 und dem 16.01.2028. Den Tag sehen Sie in jeder Anmeldung. Laden Sie sie vorher herunter, wenn Sie sie länger brauchen.",
    );
  });

  it("stays away without a registration or without a day", () => {
    expect(holidayDeletionNotice([])).toBeNull();
    expect(
      holidayDeletionNotice([{ deletesOn: null, healthDeletesOn: null }]),
    ).toBeNull();
  });
});

describe("holidayRegistrationDeletionNotice", () => {
  it("names the day of the registration and of its health information", () => {
    expect(
      holidayRegistrationDeletionNotice({
        deletesOn: "2028-01-16",
        healthDeletesOn: "2027-04-18",
      }),
    ).toBe(
      "moto löscht diese Anmeldung am 16.01.2028. Laden Sie sie vorher herunter, wenn Sie sie länger brauchen. Gesundheitsangaben löscht moto schon am 18.04.2027.",
    );
  });

  it("leaves the health sentence out when both go on the same day", () => {
    expect(
      holidayRegistrationDeletionNotice({
        deletesOn: "2027-04-18",
        healthDeletesOn: "2027-04-18",
      }),
    ).toBe(
      "moto löscht diese Anmeldung am 18.04.2027. Laden Sie sie vorher herunter, wenn Sie sie länger brauchen.",
    );
    expect(
      holidayRegistrationDeletionNotice({
        deletesOn: null,
        healthDeletesOn: null,
      }),
    ).toBeNull();
  });
});
