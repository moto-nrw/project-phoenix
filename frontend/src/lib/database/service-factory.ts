// Generic CRUD Service Factory

import { getSession } from "next-auth/react";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ServiceFactory" });
import type { EntityConfig, CrudService, PaginatedResponse } from "./types";

/**
 * Extract a user-friendly error message from a caught error.
 * Use in catch blocks: `toastError(getDeleteErrorMessage(err))`
 */
export function getDeleteErrorMessage(err: unknown): string {
  return err instanceof Error
    ? err.message
    : "Fehler beim Löschen. Bitte versuchen Sie es erneut.";
}

export class MalformedCrudListResponseError extends Error {
  readonly entity: string;
  readonly endpoint: string;
  readonly responseShape: string;

  constructor(entity: string, endpoint: string, responseShape: string) {
    super(
      `Malformed CRUD list response for ${entity} from ${endpoint}: expected an array or an object with a data array. Received ${responseShape}.`,
    );
    this.name = "MalformedCrudListResponseError";
    this.entity = entity;
    this.endpoint = endpoint;
    this.responseShape = responseShape;
  }
}

// Helper functions extracted to reduce cognitive complexity (S3776)

function describeResponseShape(response: unknown): string {
  if (response === null) {
    return "null";
  }

  if (Array.isArray(response)) {
    return "array";
  }

  if (typeof response === "object") {
    return `object keys: ${Object.keys(response).join(",")}`;
  }

  return typeof response;
}

/**
 * Creates a default pagination object for non-paginated responses
 */
function createDefaultPagination(
  length: number,
): PaginatedResponse<never>["pagination"] {
  return {
    current_page: 1,
    page_size: length,
    total_pages: 1,
    total_records: length,
  };
}

/**
 * Maps an array of items using an optional mapper function
 */
function mapDataArray<T>(
  data: unknown[],
  mapResponse?: (item: unknown) => T,
): T[] {
  return mapResponse ? data.map((item) => mapResponse(item)) : (data as T[]);
}

/**
 * Type guard to check if object is a paginated response
 */
function isPaginatedResponse(obj: unknown): obj is {
  data: unknown[];
  pagination: PaginatedResponse<unknown>["pagination"];
} {
  return (
    obj !== null &&
    typeof obj === "object" &&
    "data" in obj &&
    Array.isArray((obj as { data: unknown }).data) &&
    "pagination" in obj
  );
}

/**
 * Type guard to check if object has a data array
 */
function hasDataArray(obj: unknown): obj is {
  data: unknown[];
  pagination?: PaginatedResponse<unknown>["pagination"];
} {
  return (
    obj !== null &&
    typeof obj === "object" &&
    "data" in obj &&
    Array.isArray((obj as { data: unknown[] }).data)
  );
}

/**
 * Type guard for API wrapper response with success/data structure
 */
function isApiWrapper(
  obj: unknown,
): obj is { success: boolean; data: unknown } {
  return (
    obj !== null && typeof obj === "object" && "success" in obj && "data" in obj
  );
}

export function createCrudService<T>(config: EntityConfig<T>): CrudService<T> {
  const { api: apiConfig, service } = config;

  // Helper to get auth token
  const getToken = async () => {
    const session = await getSession();
    return session?.user?.token;
  };

  // Extract a clean error message from potentially nested JSON error responses.
  // The route handler wraps backend errors as: {"error":"API error (409): {\"status\":\"error\",\"error\":\"...\"}"}
  // This function digs through the layers to find the original backend error text.
  const extractErrorMessage = (
    responseText: string,
    status: number,
  ): string => {
    const fallback = `API error: ${status} - ${responseText}`;
    try {
      const parsed: unknown = JSON.parse(responseText);
      if (
        typeof parsed === "object" &&
        parsed !== null &&
        "error" in parsed &&
        typeof (parsed as { error: string }).error === "string"
      ) {
        const innerError = (parsed as { error: string }).error;
        // Try to extract nested backend JSON from "API error (409): {json}"
        const innerMatch = /:\s*(\{.*\})\s*$/s.exec(innerError);
        if (innerMatch?.[1]) {
          try {
            const backendError: unknown = JSON.parse(innerMatch[1]);
            if (
              typeof backendError === "object" &&
              backendError !== null &&
              "error" in backendError &&
              typeof (backendError as { error: string }).error === "string"
            ) {
              // Strip Go service prefix like "education: DeleteGroup: "
              return (backendError as { error: string }).error.replace(
                /^\w+:\s*\w+:\s*/,
                "",
              );
            }
          } catch {
            // Inner JSON parse failed, use the outer error string
          }
        }
        // No nested JSON, just return the error field directly
        return innerError;
      }
    } catch {
      // Not JSON at all
    }
    return fallback;
  };

  // Helper to make fetch requests with auth
  const fetchWithAuth = async (url: string, options: RequestInit = {}) => {
    const token = await getToken();
    const headers = new Headers();
    headers.set("Content-Type", "application/json");
    if (options.headers) {
      const optionHeaders = new Headers(options.headers);
      optionHeaders.forEach((value, key) => {
        headers.set(key, value);
      });
    }

    if (token) {
      headers.set("Authorization", `Bearer ${token}`);
    }

    const response = await fetch(url, {
      ...options,
      headers,
      credentials: "include",
    });

    if (!response.ok) {
      const errorText = await response.text();

      // 4xx = expected business errors (conflict, not found, etc.) → warn
      // 5xx = unexpected server errors → error
      if (response.status >= 500) {
        logger.error("api_server_error", {
          status: response.status,
          error: errorText,
        });
      } else {
        logger.warn("api_request_rejected", {
          status: response.status,
          error: errorText,
        });
      }

      // Try to extract a clean error message from the nested JSON response.
      // The error chain is: backend → route handler → this fetch, each wrapping the previous.
      const userMessage = extractErrorMessage(errorText, response.status);
      // Carry the HTTP status on the thrown error so callers can distinguish a
      // client-side validation error (4xx, message is user-facing) from a
      // network/server error (5xx, message is technical noise). Additive: the
      // message and `instanceof Error` are unchanged for existing consumers.
      const apiError = new Error(userMessage) as Error & {
        status?: number;
        body?: string;
      };
      apiError.status = response.status;
      // Carry the RAW response body too. extractErrorMessage reduces the
      // response to one human sentence, which silently drops every structured
      // sibling field the proxy forwards — notably the companion-plan 409's
      // top-level `conflicts` array. Without it the student Stammdaten form
      // cannot name what the user confirmed, so "Ergänzen und speichern"
      // re-sends an empty confirmation and gets the same 409 forever.
      apiError.body = errorText;
      throw apiError;
    }

    // Handle empty responses (204 No Content, or empty body from DELETE)
    const contentType = response.headers.get("content-type");
    const hasJson = contentType?.includes("application/json");
    const contentLength = response.headers.get("content-length");

    if (response.status === 204 || contentLength === "0" || !hasJson) {
      return null;
    }

    return response.json() as Promise<unknown>;
  };

  // Build endpoint URLs
  const endpoints = {
    list: apiConfig.endpoints?.list ?? apiConfig.basePath,
    get: apiConfig.endpoints?.get ?? `${apiConfig.basePath}/{id}`,
    create: apiConfig.endpoints?.create ?? apiConfig.basePath,
    update: apiConfig.endpoints?.update ?? `${apiConfig.basePath}/{id}`,
    delete: apiConfig.endpoints?.delete ?? `${apiConfig.basePath}/{id}`,
  };

  return {
    async getList(
      filters?: Record<string, unknown>,
    ): Promise<PaginatedResponse<T>> {
      try {
        // Build query string
        const params = new URLSearchParams();
        if (apiConfig.listParams) {
          Object.entries(apiConfig.listParams).forEach(([key, value]) => {
            params.append(key, value);
          });
        }
        if (filters) {
          Object.entries(filters).forEach(([key, value]) => {
            if (value !== undefined && value !== null) {
              let stringValue: string;
              if (typeof value === "object") {
                stringValue = JSON.stringify(value);
              } else if (typeof value === "boolean") {
                stringValue = value.toString();
              } else if (typeof value === "number") {
                stringValue = value.toString();
              } else {
                stringValue = value as string;
              }
              params.append(key, stringValue);
            }
          });
        }

        const queryString = params.toString();
        const url = queryString
          ? `${endpoints.list}?${queryString}`
          : endpoints.list;
        const response = await fetchWithAuth(url);

        // Parse response using helper functions to reduce complexity
        const dataSource = isApiWrapper(response) ? response.data : response;

        // Handle paginated response
        if (isPaginatedResponse(dataSource)) {
          const mappedData = mapDataArray<T>(
            dataSource.data,
            service?.mapResponse,
          );
          return { ...dataSource, data: mappedData } as PaginatedResponse<T>;
        }

        // Handle direct array response
        if (Array.isArray(dataSource)) {
          const mappedData = mapDataArray<T>(dataSource, service?.mapResponse);
          return {
            data: mappedData,
            pagination: createDefaultPagination(mappedData.length),
          };
        }

        // Handle wrapped response with data array
        if (hasDataArray(dataSource)) {
          const mappedData = mapDataArray<T>(
            dataSource.data,
            service?.mapResponse,
          );
          return {
            data: mappedData,
            pagination:
              dataSource.pagination ??
              createDefaultPagination(mappedData.length),
          } as PaginatedResponse<T>;
        }

        const responseShape = describeResponseShape(dataSource);
        const malformedResponseError = new MalformedCrudListResponseError(
          config.name.plural,
          endpoints.list,
          responseShape,
        );
        logger.warn("malformed_crud_list_response", {
          entity: malformedResponseError.entity,
          endpoint: malformedResponseError.endpoint,
          responseShape: malformedResponseError.responseShape,
        });
        throw malformedResponseError;
      } catch (error) {
        logger.error("error fetching entities", {
          entity: config.name.plural,
          error: String(error),
        });
        throw error;
      }
    },

    async getOne(id: string): Promise<T> {
      try {
        // Check if there's a custom getOne method
        if (service?.customMethods?.getOne) {
          const result = await service.customMethods.getOne(id);
          return result as T;
        }

        const url = endpoints.get.replace("{id}", id);
        const response = await fetchWithAuth(url);

        const data = (response as { data?: unknown })?.data ?? response;
        return service?.mapResponse ? service.mapResponse(data) : (data as T);
      } catch (error) {
        logger.error("error fetching entity", {
          entity: config.name.singular,
          id,
          error: String(error),
        });
        throw error;
      }
    },

    async create(data: Partial<T>): Promise<T> {
      try {
        // Check if there's a custom create method
        if (service?.create) {
          const token = await getToken();
          const result = await service.create(data, token);

          // Apply after hook
          if (config.hooks?.afterCreate) {
            await config.hooks.afterCreate(result);
          }

          return result;
        }

        // Apply hooks
        if (config.hooks?.beforeCreate) {
          data = await config.hooks.beforeCreate(data);
        }

        const requestData = service?.mapRequest
          ? service.mapRequest(data)
          : data;

        const response = await fetchWithAuth(endpoints.create, {
          method: "POST",
          body: JSON.stringify(requestData),
        });

        const responseData = (response as { data?: unknown })?.data ?? response;
        const result = service?.mapResponse
          ? service.mapResponse(responseData)
          : (responseData as T);

        // Apply after hook
        if (config.hooks?.afterCreate) {
          await config.hooks.afterCreate(result);
        }

        return result;
      } catch (error) {
        logger.error("error creating entity", {
          entity: config.name.singular,
          error: String(error),
        });
        throw error;
      }
    },

    async update(id: string, data: Partial<T>): Promise<T> {
      try {
        // Check if there's a custom update method
        if (service?.update) {
          const token = await getToken();
          const result = await service.update(id, data, token);

          // Apply after hook
          if (config.hooks?.afterUpdate) {
            await config.hooks.afterUpdate(result);
          }

          return result;
        }

        // Apply hooks
        if (config.hooks?.beforeUpdate) {
          data = await config.hooks.beforeUpdate(id, data);
        }

        const url = endpoints.update.replace("{id}", id);
        const requestData = service?.mapRequest
          ? service.mapRequest(data)
          : data;

        const response = await fetchWithAuth(url, {
          method: "PUT",
          body: JSON.stringify(requestData),
        });

        const responseData = (response as { data?: unknown })?.data ?? response;
        const result = service?.mapResponse
          ? service.mapResponse(responseData)
          : (responseData as T);

        // Apply after hook
        if (config.hooks?.afterUpdate) {
          await config.hooks.afterUpdate(result);
        }

        return result;
      } catch (error) {
        logger.error("error updating entity", {
          entity: config.name.singular,
          id,
          error: String(error),
        });
        throw error;
      }
    },

    async delete(id: string): Promise<string | null> {
      try {
        // Apply hook
        if (config.hooks?.beforeDelete) {
          const shouldDelete = await config.hooks.beforeDelete(id);
          if (!shouldDelete) {
            return "Löschen wurde abgebrochen";
          }
        }

        const url = endpoints.delete.replace("{id}", id);

        await fetchWithAuth(url, {
          method: "DELETE",
        });

        // Apply after hook
        if (config.hooks?.afterDelete) {
          await config.hooks.afterDelete(id);
        }
        return null;
      } catch (error) {
        const errorMsg = error instanceof Error ? error.message : String(error);
        const is5xx = /API error: 5\d\d/.test(errorMsg);

        if (is5xx) {
          // 5xx = unexpected server error → log at error level, show generic message
          logger.error("entity_delete_server_error", {
            entity: config.name.singular,
            id,
            error: errorMsg,
          });
          return "Ein unerwarteter Fehler ist aufgetreten. Bitte versuchen Sie es später erneut.";
        }

        // 4xx = expected business error → warn level, show backend message
        logger.warn("entity_delete_rejected", {
          entity: config.name.singular,
          id,
          error: errorMsg,
        });
        return getDeleteErrorMessage(error);
      }
    },
  };
}

// Export helper to create services with custom methods
export function createExtendedService<T>(
  config: EntityConfig<T>,
): CrudService<T> {
  const baseService = createCrudService(config);

  // Add custom methods if defined
  if (config.service?.customMethods) {
    return {
      ...baseService,
      ...config.service.customMethods,
    } as CrudService<T>;
  }

  return baseService;
}
