"use client";

import { type FormEvent, useState } from "react";
import { ArrowRight, CheckCircle2 } from "lucide-react";

const intakeFields = [
  "Name und Rolle",
  "OGS oder Träger",
  "Anzahl Kinder und Standorte",
  "Aktuelle Abläufe bei Anmeldung, Abholung und Anwesenheit",
  "Wobei moto zuerst helfen soll",
];

const inputClassName =
  "h-10 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm text-gray-900 shadow-sm transition-colors placeholder:text-gray-400 hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none";
const textareaClassName =
  "min-h-28 w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm transition-colors placeholder:text-gray-400 hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none";
const startPrimaryButtonClassName =
  "inline-flex h-10 items-center justify-center gap-2 rounded-lg bg-gray-900 px-4 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-gray-800 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none";

export default function StartPage() {
  const [form, setForm] = useState({
    name: "",
    contact: "",
    organization: "",
    children: "",
    locations: "",
    focus: "",
  });

  const update = (key: keyof typeof form, value: string) => {
    setForm((current) => ({ ...current, [key]: value }));
  };

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    const mailSubject = encodeURIComponent("Los geht's mit moto");
    const mailBody = encodeURIComponent(
      [
        "Hallo moto Team,",
        "",
        "wir möchten mit moto starten und bitten um Kontaktaufnahme.",
        "",
        `Name und Rolle: ${form.name}`,
        `Kontakt: ${form.contact}`,
        `OGS oder Träger: ${form.organization}`,
        `Anzahl Kinder: ${form.children}`,
        `Anzahl Standorte: ${form.locations}`,
        `Wobei soll moto zuerst helfen? ${form.focus}`,
        "",
        "Viele Grüße",
      ].join("\n"),
    );

    window.location.href = `mailto:kontakt@moto.nrw?subject=${mailSubject}&body=${mailBody}`;
  };

  return (
    <main className="moto-dotted-background min-h-screen px-4 py-10 sm:px-6 lg:px-8">
      <section className="mx-auto flex min-h-[calc(100vh-5rem)] w-full max-w-6xl items-center">
        <div className="grid w-full gap-8 lg:grid-cols-[0.9fr_1.1fr] lg:items-center">
          <div>
            <p className="text-moto-green text-xs font-semibold tracking-[0.16em] uppercase">
              Los geht&apos;s
            </p>
            <h1 className="mt-4 max-w-2xl text-4xl font-semibold tracking-normal text-gray-950 sm:text-5xl">
              Erzählt uns kurz, wie eure OGS arbeitet.
            </h1>
            <p className="mt-5 max-w-xl text-base leading-7 text-gray-600">
              Wir melden uns danach persönlich und schauen gemeinsam, welche
              Abläufe moto zuerst abbilden soll.
            </p>
            <div className="moto-content-surface relative z-10 mt-8 rounded-2xl border p-6 shadow-sm">
              <h2 className="text-lg font-semibold text-gray-950">
                Was wir zum Start wissen wollen
              </h2>
              <ul className="mt-5 space-y-3">
                {intakeFields.map((field) => (
                  <li key={field} className="flex gap-3 text-sm text-gray-700">
                    <CheckCircle2
                      className="text-moto-green mt-0.5 h-5 w-5 shrink-0"
                      aria-hidden="true"
                    />
                    <span>{field}</span>
                  </li>
                ))}
              </ul>
            </div>
          </div>

          <form
            onSubmit={handleSubmit}
            className="moto-content-surface relative z-10 rounded-2xl border p-6 shadow-sm sm:p-8"
          >
            <div>
              <h2 className="text-xl font-semibold text-gray-950">
                Startfragebogen
              </h2>
              <p className="mt-2 text-sm leading-6 text-gray-600">
                Es reicht ein grober Überblick. Wir nehmen anschließend Kontakt
                auf und sortieren die nächsten Schritte gemeinsam.
              </p>
            </div>

            <div className="mt-6 grid gap-4 sm:grid-cols-2">
              <label className="space-y-1.5 text-sm font-medium text-gray-700">
                Name und Rolle
                <input
                  value={form.name}
                  onChange={(event) => update("name", event.target.value)}
                  className={inputClassName}
                  placeholder="z. B. Anna Müller, OGS-Leitung"
                  required
                />
              </label>
              <label className="space-y-1.5 text-sm font-medium text-gray-700">
                Kontakt
                <input
                  value={form.contact}
                  onChange={(event) => update("contact", event.target.value)}
                  className={inputClassName}
                  placeholder="E-Mail oder Telefonnummer"
                  required
                />
              </label>
              <label className="space-y-1.5 text-sm font-medium text-gray-700">
                OGS oder Träger
                <input
                  value={form.organization}
                  onChange={(event) =>
                    update("organization", event.target.value)
                  }
                  className={inputClassName}
                  placeholder="Name der Einrichtung"
                />
              </label>
              <div className="grid gap-4 sm:grid-cols-2">
                <label className="space-y-1.5 text-sm font-medium text-gray-700">
                  Kinder
                  <input
                    value={form.children}
                    onChange={(event) => update("children", event.target.value)}
                    className={inputClassName}
                    placeholder="z. B. 180"
                  />
                </label>
                <label className="space-y-1.5 text-sm font-medium text-gray-700">
                  Standorte
                  <input
                    value={form.locations}
                    onChange={(event) =>
                      update("locations", event.target.value)
                    }
                    className={inputClassName}
                    placeholder="z. B. 3"
                  />
                </label>
              </div>
              <label className="space-y-1.5 text-sm font-medium text-gray-700 sm:col-span-2">
                Wobei soll moto zuerst helfen?
                <textarea
                  value={form.focus}
                  onChange={(event) => update("focus", event.target.value)}
                  className={textareaClassName}
                  placeholder="z. B. Anwesenheit, Abholung, Anmeldungen, Räume, Kommunikation"
                />
              </label>
            </div>

            <button
              type="submit"
              className={`${startPrimaryButtonClassName} mt-6`}
            >
              Kontaktaufnahme starten
              <ArrowRight className="h-4 w-4" aria-hidden="true" />
            </button>
          </form>
        </div>
      </section>
    </main>
  );
}
