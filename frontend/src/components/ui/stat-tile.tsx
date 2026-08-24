// Kompakte Kennzahl-Kachel: eine Zahl, ein Wort darunter, ruhige graue Fläche.
//
// Das kleine Geschwister von StatCard. StatCard ist die prominente Kachel mit
// Großzahl, Ton und Fortschrittsbalken; diese hier ist die dichte Variante, die
// zu mehreren in eine Karte passt — Klassenansicht, Tagesauswertung,
// Klassenlisten und die Aufsichten des Schul-Portals brauchen alle dieselbe.
//
// Sie existiert, weil genau dieses Markup im Repo mehr als ein Dutzend Mal von
// Hand geschrieben steht und dabei schon auseinanderläuft (py-2 gegen py-2.5).
// Neue Kacheln nehmen diese Komponente; die bestehenden Stellen wandern nach,
// wenn sie ohnehin angefasst werden.

export function StatTile({
  label,
  value,
}: Readonly<{
  readonly label: string;
  /** Zahl oder kurzer Text. Lange Werte gehören in ein DataField. */
  readonly value: string | number;
}>) {
  return (
    <div className="rounded-xl bg-gray-50 px-3 py-2">
      <span className="block text-sm font-semibold text-gray-900">{value}</span>
      <span className="block text-[11px] font-medium text-gray-500">
        {label}
      </span>
    </div>
  );
}
