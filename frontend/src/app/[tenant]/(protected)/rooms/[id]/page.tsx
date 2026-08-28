import { redirect } from "next/navigation";

// Alter Direktlink. Die Raumansicht ist das Panel auf /rooms?room={id};
// dieser Pfad leitet nur noch dorthin weiter, damit gespeicherte Links
// und Lesezeichen weiter funktionieren. Server-Komponente: die
// Weiterleitung passiert vor der ersten HTML-Zeile, die alte Adresse
// steht nie in der Adresszeile.
interface RoomDetailRedirectProps {
  params: Promise<{ id: string }>;
}

export default async function RoomDetailRedirect({
  params,
}: RoomDetailRedirectProps) {
  const { id } = await params;
  redirect(`/rooms?room=${encodeURIComponent(id)}`);
}
