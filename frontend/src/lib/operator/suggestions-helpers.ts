export type OperatorSuggestionStatus =
  "open" | "planned" | "in_progress" | "done" | "rejected" | "need_info";

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
  unread_count?: number;
  is_new?: boolean;
  operator_comments?: BackendOperatorComment[];
  school_id: number;
  school_name: string;
  is_hidden: boolean;
  /** Which board the entry came from: the school's staff or a guardian. */
  author_type?: "staff" | "parent";
}

export interface BackendOperatorComment {
  id: number;
  content: string;
  author_id: number;
  author_name: string;
  author_type: "operator" | "user" | "parent";
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
  unreadCount: number;
  isNew: boolean;
  operatorComments: OperatorComment[];
  isHidden: boolean;
  schoolId: string;
  schoolName: string;
  /**
   * "parent" marks feedback from the parents portal (#1678). Those entries are
   * pseudonymous: authorName is a placeholder and the backend ships no author
   * id, so there is nothing here to identify a guardian with.
   */
  authorType: "staff" | "parent";
}

export interface OperatorComment {
  id: string;
  content: string;
  authorId: string;
  authorName: string;
  authorType: "operator" | "user" | "parent";
  createdAt: string;
}

export function mapOperatorComment(
  data: BackendOperatorComment,
): OperatorComment {
  return {
    id: data.id.toString(),
    content: data.content,
    authorId: data.author_id.toString(),
    authorName: data.author_name,
    authorType: data.author_type,
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
    unreadCount: data.unread_count ?? 0,
    isNew: data.is_new ?? false,
    operatorComments: (data.operator_comments ?? []).map(mapOperatorComment),
    isHidden: data.is_hidden,
    schoolId: data.school_id.toString(),
    schoolName: data.school_name,
    authorType: data.author_type ?? "staff",
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
    planned: "bg-white text-gray-700 border-2 border-moto-blue",
    in_progress: "bg-white text-gray-700 border-2 border-moto-orange",
    done: "bg-white text-gray-700 border-2 border-moto-green",
    rejected: "bg-white text-gray-700 border-2 border-moto-red",
    need_info: "bg-white text-gray-700 border-2 border-purple-500",
  };

export const OPERATOR_STATUS_DOT_COLORS: Record<
  OperatorSuggestionStatus,
  string
> = {
  open: "bg-gray-400",
  planned: "bg-moto-blue",
  in_progress: "bg-moto-orange",
  done: "bg-moto-green",
  rejected: "bg-moto-red",
  need_info: "bg-purple-500",
};
