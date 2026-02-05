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
    open: "bg-white text-gray-700 border-2 border-gray-300",
    planned: "bg-white text-gray-700 border-2 border-[#5080D8]",
    in_progress: "bg-white text-gray-700 border-2 border-[#F78C10]",
    done: "bg-white text-gray-700 border-2 border-[#83CD2D]",
    rejected: "bg-white text-gray-700 border-2 border-[#FF3130]",
    need_info: "bg-white text-gray-700 border-2 border-purple-500",
  };

export const OPERATOR_STATUS_DOT_COLORS: Record<
  OperatorSuggestionStatus,
  string
> = {
  open: "bg-gray-400",
  planned: "bg-[#5080D8]",
  in_progress: "bg-[#F78C10]",
  done: "bg-[#83CD2D]",
  rejected: "bg-[#FF3130]",
  need_info: "bg-purple-500",
};
