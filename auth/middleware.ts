/**
 * Authentication middleware.
 * Validates API keys from request headers.
 */

import { validateApiKey, type ApiKey } from "../db/api_keys.ts";

export interface AuthResult {
  authenticated: boolean;
  apiKey?: ApiKey;
  error?: string;
}

/**
 * Extract API key from request headers.
 * Supports:
 *   - Authorization: Bearer sk-xxx
 *   - x-api-key: sk-xxx
 */
function extractApiKey(req: Request): string | null {
  // Try Authorization header first (OpenAI style)
  const authHeader = req.headers.get("authorization");
  if (authHeader?.startsWith("Bearer ")) {
    return authHeader.slice(7);
  }

  // Try x-api-key header (Anthropic style)
  const xApiKey = req.headers.get("x-api-key");
  if (xApiKey) {
    return xApiKey;
  }

  return null;
}

/**
 * Authenticate a request.
 */
export async function authenticate(req: Request): Promise<AuthResult> {
  const key = extractApiKey(req);

  if (!key) {
    return {
      authenticated: false,
      error: "Missing API key. Include it as 'Authorization: Bearer sk-xxx' or 'x-api-key: sk-xxx'",
    };
  }

  const apiKey = await validateApiKey(key);

  if (!apiKey) {
    return {
      authenticated: false,
      error: "Invalid API key",
    };
  }

  return {
    authenticated: true,
    apiKey,
  };
}

/**
 * Create an error response for authentication failures.
 * Formats error according to the requested API style.
 */
export function createAuthErrorResponse(
  error: string,
  pathname: string
): Response {
  // Determine response format based on endpoint
  const isAnthropicEndpoint = pathname.startsWith("/v1/messages");

  if (isAnthropicEndpoint) {
    // Anthropic error format
    return Response.json(
      {
        type: "error",
        error: {
          type: "authentication_error",
          message: error,
        },
      },
      { status: 401 }
    );
  } else {
    // OpenAI error format
    return Response.json(
      {
        error: {
          message: error,
          type: "invalid_request_error",
          param: null,
          code: "invalid_api_key",
        },
      },
      { status: 401 }
    );
  }
}

/**
 * List of paths that don't require authentication.
 */
const PUBLIC_PATHS = ["/health", "/v1/models"];

/**
 * Check if a path requires authentication.
 */
export function requiresAuth(pathname: string): boolean {
  // Health check and models list are public
  if (PUBLIC_PATHS.includes(pathname)) {
    return false;
  }

  // Model info endpoint is public
  if (pathname.startsWith("/v1/models/")) {
    return false;
  }

  return true;
}
