import { redirect } from "next/navigation";

// Legacy deep-link target. Activity management now happens from the canonical
// /activities list/modal flow, so bookmarks to /activities/{id} land there.
export default function ActivityDetailRedirect() {
  redirect("/activities");
}
