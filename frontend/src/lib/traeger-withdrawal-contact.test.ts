import { describe, expect, it } from "vitest";
import {
  withdrawalAddressee,
  withdrawalReach,
} from "./traeger-withdrawal-contact";

const CONTACT = {
  name: "Petra Lindner",
  phone: "0211 123 45",
  email: "p.lindner@traeger.example",
};

describe("withdrawalReach", () => {
  it("takes the contact person's phone and e-mail without own entries", () => {
    expect(withdrawalReach(CONTACT, { phone: "", email: "" })).toEqual({
      phone: { value: "0211 123 45", source: "contact" },
      email: { value: "p.lindner@traeger.example", source: "contact" },
    });
  });

  it("lets each own entry win on its own", () => {
    expect(
      withdrawalReach(CONTACT, {
        phone: " ",
        email: "widerruf@traeger.example",
      }),
    ).toEqual({
      phone: { value: "0211 123 45", source: "contact" },
      email: { value: "widerruf@traeger.example", source: "own" },
    });
  });

  it("marks a value neither side has as missing", () => {
    expect(
      withdrawalReach(
        { name: "", phone: "", email: "" },
        { phone: "0800 1", email: "" },
      ),
    ).toEqual({
      phone: { value: "0800 1", source: "own" },
      email: { value: "", source: "missing" },
    });
  });
});

describe("withdrawalAddressee", () => {
  const reach = withdrawalReach(CONTACT, {
    phone: "",
    email: "widerruf@traeger.example",
  });

  it("names the Träger like the notice: name, address, phone, e-mail", () => {
    expect(
      withdrawalAddressee(
        "Ganztag im Quartier e.V.\nAm Markt 4\n\n40210 Düsseldorf",
        reach,
      ),
    ).toEqual([
      "Ganztag im Quartier e.V.",
      "Am Markt 4",
      "40210 Düsseldorf",
      "Telefon: 0211 123 45",
      "E-Mail: widerruf@traeger.example",
    ]);
  });

  it("says „dem Träger“ without a letterhead and leaves out what is missing", () => {
    expect(
      withdrawalAddressee(
        "",
        withdrawalReach(
          { name: "", phone: "", email: "" },
          { phone: "", email: "" },
        ),
      ),
    ).toEqual(["dem Träger"]);
  });
});
