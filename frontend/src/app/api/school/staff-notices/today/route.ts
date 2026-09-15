import { proxyGet } from "~/lib/school/route-wrapper.server";

// Die heute geltenden Tagesinformationen für eine Lehrkraft in moto schule
// (#2208). Das Backend liefert nur, was "alle" oder "nur Lehrkräfte" erreicht.
export const GET = proxyGet("/school/staff-notices/today");
