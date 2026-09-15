import { proxyGet } from "~/lib/school/route-wrapper.server";

// Beginn des ersten Betreuungsblocks einer Klasse an einem Tag (#2970), die
// Vorbelegung für „Unterricht fällt aus“. Query: class, date.
export const GET = proxyGet("/school/class-day/arrival-exceptions/block-start");
