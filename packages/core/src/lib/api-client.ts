import type { ApiErrorBody } from './api-types';

// The single HTTP client of docs/specs/004-api-and-routing.md. No page calls
// fetch() directly: everything goes through request() so that the token, the
// error envelope and the 401 handling behave the same everywhere.

const API_PREFIX = '/api';

export class ApiError extends Error {
  readonly status: number;
  readonly body: ApiErrorBody | null;

  constructor(status: number, message: string, body: ApiErrorBody | null = null) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.body = body;
  }
}

type TokenProvider = () => string | null;
type UnauthorizedHandler = () => void;

let tokenProvider: TokenProvider = () => null;
let onUnauthorized: UnauthorizedHandler = () => {};

/** Wired by the session module, so the client never reads storage itself. */
export function configureClient(options: {
  getToken: TokenProvider;
  onUnauthorized: UnauthorizedHandler;
}) {
  tokenProvider = options.getToken;
  onUnauthorized = options.onUnauthorized;
}

type RequestOptions = {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
  body?: unknown;
  /** Send the request without the session token, e.g. public settings. */
  anonymous?: boolean;
  signal?: AbortSignal;
};

async function send(path: string, options: RequestOptions): Promise<Response> {
  const headers: Record<string, string> = { Accept: 'application/json' };
  const token = options.anonymous ? null : tokenProvider();
  if (token) headers.Authorization = `Bearer ${token}`;
  if (options.body !== undefined) headers['Content-Type'] = 'application/json';

  let response: Response;
  try {
    response = await fetch(`${API_PREFIX}${path}`, {
      method: options.method ?? 'GET',
      headers,
      body: options.body === undefined ? undefined : JSON.stringify(options.body),
      signal: options.signal,
      credentials: 'same-origin',
    });
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') throw error;
    throw new ApiError(0, 'Le serveur est injoignable. Vérifiez votre connexion.');
  }

  if (response.ok) return response;

  let body: ApiErrorBody | null = null;
  try {
    body = (await response.json()) as ApiErrorBody;
  } catch {
    // Not a JSON error envelope.
  }

  if (response.status === 401 && token) onUnauthorized();

  throw new ApiError(response.status, body?.error || `Erreur HTTP ${response.status}`, body);
}

/** JSON request. Returns the parsed body, or undefined for an empty response. */
export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const response = await send(path, options);
  if (response.status === 204) return undefined as T;
  const text = await response.text();
  return (text ? JSON.parse(text) : undefined) as T;
}

/** Non-JSON responses: CSV exports, exported markdown. */
export async function requestBlob(
  path: string,
  options: RequestOptions = {},
): Promise<{ blob: Blob; filename: string | null }> {
  const response = await send(path, options);
  const disposition = response.headers.get('Content-Disposition') ?? '';
  const match = /filename="?([^";]+)"?/i.exec(disposition);
  return { blob: await response.blob(), filename: match?.[1] ?? null };
}
