import { redirect } from "next/navigation";

// Legacy deep-link target. The room-detail UI moved into a slide-over
// on /rooms?room={id} (#1323 review), so any bookmark or external link
// pointing at /rooms/{id} is forwarded to the canonical slide-over URL.
// Server component → the redirect happens before any HTML is sent, so
// the user never sees this URL in the bar.
interface RoomDetailRedirectProps {
  params: Promise<{ id: string }>;
}

export default async function RoomDetailRedirect({
  params,
}: RoomDetailRedirectProps) {
  const { id } = await params;
  redirect(`/rooms?room=${encodeURIComponent(id)}`);
}
