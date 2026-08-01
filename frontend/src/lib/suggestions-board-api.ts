// lib/suggestions-board-api.ts
// The set of calls the shared board components (card, vote buttons, comment
// accordion) need. Two boards implement it: the school's staff suggestion board
// and the parents portal's feedback board (#1678). The components take this
// adapter as a prop so both boards render through the exact same UI instead of
// growing a second, drifting copy.

import {
  voteSuggestion,
  removeVote,
  fetchComments,
  createComment,
  deleteComment,
  markCommentsRead,
} from "./suggestions-api";
import type { Suggestion, SuggestionComment } from "./suggestions-helpers";

export interface SuggestionsBoardApi {
  vote: (id: string, direction: "up" | "down") => Promise<Suggestion>;
  removeVote: (id: string) => Promise<Suggestion>;
  fetchComments: (postId: string) => Promise<SuggestionComment[]>;
  createComment: (postId: string, content: string) => Promise<void>;
  deleteComment: (postId: string, commentId: string) => Promise<void>;
  markCommentsRead: (postId: string) => Promise<void>;
}

/**
 * The school's staff board — the default for every existing call site.
 *
 * Each entry forwards through an arrow rather than binding the import eagerly:
 * a component test that partially mocks `suggestions-api` (say, only the vote
 * calls) would otherwise fail at import time on the exports it left out.
 */
export const staffBoardApi: SuggestionsBoardApi = {
  vote: (id, direction) => voteSuggestion(id, direction),
  removeVote: (id) => removeVote(id),
  fetchComments: (postId) => fetchComments(postId),
  createComment: (postId, content) => createComment(postId, content),
  deleteComment: (postId, commentId) => deleteComment(postId, commentId),
  markCommentsRead: (postId) => markCommentsRead(postId),
};
