"use client";

import { type FormEvent, useState } from "react";
import { ArrowRight, CheckCircle2 } from "lucide-react";

import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Textarea } from "~/components/ui/textarea";

const intakeFields = [
  "Name und Rolle",
  "OGS oder Träger",
  "Anzahl Kinder und Standorte",
  "Aktuelle Abläufe bei Anmeldung, Abholung und Anwesenheit",
  "Wobei moto zuerst helfen soll",
];

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

    window.location.href = `mailto:kontakt@moto-ogs.de?subject=${mailSubject}&body=${mailBody}`;
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

          {/* The demo records its sessions (#3603); this form with the
              prospect's name, contact, and organisation is replaced by an
              empty box in every recording (analytics-policy.ts). */}
          <form
            onSubmit={handleSubmit}
            data-analytics-block=""
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
              <Input
                id="start-name"
                label="Name und Rolle"
                value={form.name}
                onChange={(event) => update("name", event.target.value)}
                placeholder="z. B. Anna Müller, OGS-Leitung"
                required
                controlSize="compact"
              />
              <Input
                id="start-contact"
                label="Kontakt"
                value={form.contact}
                onChange={(event) => update("contact", event.target.value)}
                placeholder="E-Mail oder Telefonnummer"
                required
                controlSize="compact"
              />
              <Input
                id="start-organization"
                label="OGS oder Träger"
                value={form.organization}
                onChange={(event) => update("organization", event.target.value)}
                placeholder="Name der Einrichtung"
                controlSize="compact"
              />
              <div className="grid gap-4 sm:grid-cols-2">
                <Input
                  id="start-children"
                  label="Kinder"
                  value={form.children}
                  onChange={(event) => update("children", event.target.value)}
                  placeholder="z. B. 180"
                  controlSize="compact"
                />
                <Input
                  id="start-locations"
                  label="Standorte"
                  value={form.locations}
                  onChange={(event) => update("locations", event.target.value)}
                  placeholder="z. B. 3"
                  controlSize="compact"
                />
              </div>
              <div className="sm:col-span-2">
                <Textarea
                  id="start-focus"
                  label="Wobei soll moto zuerst helfen?"
                  value={form.focus}
                  onChange={(event) => update("focus", event.target.value)}
                  placeholder="z. B. Anwesenheit, Abholung, Anmeldungen, Räume, Kommunikation"
                  className="min-h-28"
                />
              </div>
            </div>

            <Button
              type="submit"
              variant="primary"
              size="md"
              className="mt-6 gap-2"
            >
              Kontaktaufnahme starten
              <ArrowRight className="h-4 w-4" aria-hidden="true" />
            </Button>
          </form>
        </div>
      </section>
    </main>
  );
}
