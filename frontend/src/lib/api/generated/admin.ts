export interface paths {
    "/healthz": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["liveness"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/readyz": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["readiness"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/metrics": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["metrics"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/debug/pprof/": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: operations["profileIndex"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/debug/pprof/{profile}": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Standard Go profile (heap, goroutine, allocs, mutex, block, profile, trace, cmdline, symbol). */
                profile: string;
            };
            cookie?: never;
        };
        get: operations["runtimeProfile"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/debug/pprof/symbol": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["resolveSymbols"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
}
export type webhooks = Record<string, never>;
export interface components {
    schemas: {
        ErrorEnvelope: {
            error: {
                /** @enum {string} */
                code: "invalid_request" | "unauthorized" | "not_found" | "conflict" | "payload_too_large" | "unsupported_media_type" | "invalid_token" | "token_expired" | "unavailable" | "internal";
                message: string;
                request_id: string;
                details?: {
                    field: string;
                    issue: string;
                }[];
            };
        };
    };
    responses: never;
    parameters: never;
    requestBodies: never;
    headers: {
        /** @description Echoed printable X-Request-Id (1–128 bytes), or generated UUIDv7. */
        RequestID: string;
    };
    pathItems: never;
}
export type $defs = Record<string, never>;
export interface operations {
    liveness: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Process is alive, including while draining. */
            200: {
                headers: {
                    "X-Request-Id": components["headers"]["RequestID"];
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        /** @enum {string} */
                        status: "ok";
                    };
                };
            };
        };
    };
    readiness: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Startup gate is open and all configured checks pass. */
            200: {
                headers: {
                    "X-Request-Id": components["headers"]["RequestID"];
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        /** @enum {string} */
                        status: "ready";
                    };
                };
            };
            /** @description Starting, draining, or a dependency check failed. */
            503: {
                headers: {
                    "X-Request-Id": components["headers"]["RequestID"];
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
        };
    };
    metrics: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Service-local Prometheus metrics, including Go/process collectors. */
            200: {
                headers: {
                    "X-Request-Id": components["headers"]["RequestID"];
                    [name: string]: unknown;
                };
                content: {
                    "text/plain": string;
                };
            };
        };
    };
    profileIndex: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Standard Go pprof index. */
            200: {
                headers: {
                    "X-Request-Id": components["headers"]["RequestID"];
                    [name: string]: unknown;
                };
                content: {
                    "text/html": string;
                };
            };
        };
    };
    runtimeProfile: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Standard Go profile (heap, goroutine, allocs, mutex, block, profile, trace, cmdline, symbol). */
                profile: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Standard Go pprof output; profile/trace honor the seconds query parameter. */
            200: {
                headers: {
                    "X-Request-Id": components["headers"]["RequestID"];
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Invalid profiling parameters (common JSON error envelope). */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Unknown profile (common JSON error envelope). */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
        };
    };
    resolveSymbols: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Standard Go pprof symbol lookup. */
            200: {
                headers: {
                    "X-Request-Id": components["headers"]["RequestID"];
                    [name: string]: unknown;
                };
                content?: never;
            };
        };
    };
}
