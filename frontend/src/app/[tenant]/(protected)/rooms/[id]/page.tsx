import { headers } from "next/headers";
import { redirect } from "next/navigation";
import { env } from "~/env";

// Alter Direktlink. Die Raumansicht ist das Panel auf /rooms?room={id};
// dieser Pfad leitet nur noch dorthin weiter, damit gespeicherte Links
// und Lesezeichen weiter funktionieren. Server-Komponente: die
// Weiterleitung passiert vor der ersten HTML-Zeile, die alte Adresse
// steht nie in der Adresszeile.
interface RoomDetailRedirectProps {
  params: Promise<{ tenant: string; id: string }>;
}

export default async function RoomDetailRedirect({
  params,
}: RoomDetailRedirectProps) {
  const { tenant, id } = await params;
  const requestHeaders = await headers();
  const currentHost =
    requestHeaders.get("x-moto-original-host") ??
    requestHeaders.get("x-forwarded-host") ??
    requestHeaders.get("host");
  const hostname = currentHost?.split(":")[0];
  const isTenantSubdomain = hostname === `${tenant}.${env.TENANT_DOMAIN}`;
  const pathPrefix = isTenantSubdomain ? "" : `/${tenant}`;

  redirect(`${pathPrefix}/rooms?room=${encodeURIComponent(id)}`);
}
