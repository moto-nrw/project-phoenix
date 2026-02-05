export type OperatorSuggestionStatus =
  | "open"
  | "planned"
  | "in_progress"
  | "done"
  | "rejected"
  | "need_info";

export interface BackendOperatorSuggestion {
  id: number;
  title: string;
  description: string;
  status: OperatorSuggestionStatus;
  score: number;
  upvotes: number;
  downvotes: number;
  author_name: string;
  created_at: string;
  updated_at: string;
  comment_count?: number;
  operator_comments?: BackendOperatorComment[];
}

export interface BackendOperatorComment {
  id: number;
  content: string;
  is_internal: boolean;
  operator_name?: string;
  created_at: string;
}

export interface OperatorSuggestion {
  id: string;
  title: string;
  description: string;
  status: OperatorSuggestionStatus;
  score: number;
  upvotes: number;
  downvotes: number;
  authorName: string;
  createdAt: string;
  updatedAt: string;
  commentCount: number;
  operatorComments: OperatorComment[];
}

export interface OperatorComment {
  id: string;
  content: string;
  isInternal: boolean;
  operatorName: string;
  createdAt: string;
}

export function mapOperatorComment(
  data: BackendOperatorComment,
): OperatorComment {
  return {
    id: data.id.toString(),
    content: data.content,
    isInternal: data.is_internal,
    operatorName: data.operator_name ?? "",
    createdAt: data.created_at,
  };
}

export function mapOperatorSuggestion(
  data: BackendOperatorSuggestion,
): OperatorSuggestion {
  return {
    id: data.id.toString(),
    title: data.title,
    description: data.description,
    status: data.status,
    score: data.score,
    upvotes: data.upvotes,
    downvotes: data.downvotes,
    authorName: data.author_name,
    createdAt: data.created_at,
    updatedAt: data.updated_at,
    commentCount: data.comment_count ?? 0,
    operatorComments: (data.operator_comments ?? []).map(mapOperatorComment),
  };
}

export const OPERATOR_STATUS_LABELS: Record<OperatorSuggestionStatus, string> =
  {
    open: "Offen",
    planned: "Geplant",
    in_progress: "In Bearbeitung",
    done: "Umgesetzt",
    rejected: "Abgelehnt",
    need_info: "Rückfrage",
  };

export const OPERATOR_STATUS_STYLES: Record<OperatorSuggestionStatus, string> =
  {
    open: "bg-gray-200 text-gray-700",
    planned: "bg-blue-200 text-blue-700",
    in_progress: "bg-amber-200 text-amber-800",
    done: "bg-green-200 text-green-700",
    rejected: "bg-red-200 text-red-700",
    need_info: "bg-purple-200 text-purple-700",
  };

export const OPERATOR_STATUS_DOT_COLORS: Record<
  OperatorSuggestionStatus,
  string
> = {
  open: "bg-gray-400",
  planned: "bg-blue-500",
  in_progress: "bg-amber-500",
  done: "bg-green-500",
  rejected: "bg-red-500",
  need_info: "bg-purple-500",
};
