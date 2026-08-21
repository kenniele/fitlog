import { isRecord } from "@/lib/utils";

export type APIFieldErrors = Record<string, string | string[]>;

export class APIError extends Error {
  readonly status: number;
  readonly code: string;
  readonly fields?: APIFieldErrors;

  constructor(message: string, status = 500, code = "unknown_error", fields?: APIFieldErrors) {
    super(message);
    this.name = "APIError";
    this.status = status;
    this.code = code;
    this.fields = fields;
  }
}

type Envelope<T> = { data: T };

export type APIRequestOptions = Omit<RequestInit, "body"> & {
  body?: unknown;
  raw?: boolean;
};

function isUnsafe(method?: string) {
  return !["GET", "HEAD", "OPTIONS"].includes((method ?? "GET").toUpperCase());
}

export async function apiFetch<T>(path: string, options: APIRequestOptions = {}): Promise<T> {
  const headers = new Headers(options.headers);
  const { body, raw, ...init } = options;
  const request: RequestInit = { ...init, headers, credentials: "include", cache: "no-store" };

  if (isUnsafe(options.method)) headers.set("X-Fitlog-Request", "1");
  if (body instanceof FormData) {
    request.body = body;
  } else if (body !== undefined) {
    headers.set("Content-Type", "application/json");
    request.body = JSON.stringify(body);
  }
  headers.set("Accept", "application/json");

  const response = await fetch(path.startsWith("/api/") ? path : `/api/v1${path}`, request);
  if (raw) return response as T;

  const contentType = response.headers.get("content-type") ?? "";
  const payload: unknown = contentType.includes("application/json") ? await response.json() : await response.text();
  if (!response.ok) {
    const error = isRecord(payload) && isRecord(payload.error) ? payload.error : undefined;
    throw new APIError(
      typeof error?.message === "string" ? error.message : `Request failed (${response.status})`,
      response.status,
      typeof error?.code === "string" ? error.code : "http_error",
      isRecord(error?.fields) ? (error.fields as APIFieldErrors) : undefined,
    );
  }
  if (!isRecord(payload) || !("data" in payload)) throw new APIError("API response is missing its data envelope", 502, "invalid_response");
  return (payload as Envelope<T>).data;
}

export async function downloadFromAPI(path: string, filename: string) {
  const response = await apiFetch<Response>(path, { raw: true });
  if (!response.ok) throw new APIError(`Export failed (${response.status})`, response.status, "export_failed");
  const blob = await response.blob();
  const href = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = href;
  anchor.download = filename;
  anchor.click();
  URL.revokeObjectURL(href);
}

export type ListResponse<T> = { items?: T[] | null; total?: number; page?: number; page_size?: number };

export function listItems<T>(data: ListResponse<T> | T[] | null | undefined): T[] {
  if (Array.isArray(data)) return data;
  return Array.isArray(data?.items) ? data.items : [];
}

export async function fetchAllList<T>(path: string): Promise<T[]> {
  const [pathname, rawQuery = ""] = path.split("?", 2);
  const params = new URLSearchParams(rawQuery);
  params.set("page_size", "100");
  const items: T[] = [];
  for (let page = 1; ; page += 1) {
    params.set("page", String(page));
    const result = await apiFetch<ListResponse<T>>(`${pathname}?${params.toString()}`);
    const pageItems = listItems(result);
    items.push(...pageItems);
    const total = result.total ?? items.length;
    if (items.length >= total || pageItems.length === 0) return items;
  }
}
