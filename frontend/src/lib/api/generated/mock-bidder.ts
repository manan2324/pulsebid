export interface paths {
    "/bid": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /**
         * Evaluate candidates and return bids
         * @description Evaluates candidate campaigns nominated by the gateway and returns pricing bids according to the configured bid mode and behavioral knobs.
         */
        post: operations["submitBid"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/healthz": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Liveness probe
         * @description Returns 200 OK while the bidder process is running.
         */
        get: operations["checkHealth"];
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
        /**
         * Readiness probe
         * @description Returns 200 OK when ready to receive traffic, or 503 during graceful draining.
         */
        get: operations["checkReadiness"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/admin/config": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Read runtime configuration
         * @description Retrieves the active simulation knobs and parameters for this bidder.
         */
        get: operations["getConfig"];
        put?: never;
        /**
         * Partial update of runtime configuration
         * @description Partially updates simulation knobs (e.g. latency, error rate, bid mode) at runtime.
         */
        post: operations["updateConfig"];
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
        BidRequest: {
            /** @example req-123 */
            request_id: string;
            /** @example 100 */
            tmax_ms?: number;
            imp?: {
                /** @example 1 */
                id?: string;
                /** @example home-top-banner */
                placement_id?: string;
                /** @example 300 */
                w?: number;
                /** @example 250 */
                h?: number;
                /**
                 * Format: double
                 * @example 0.5
                 */
                bidfloor?: number;
            };
            device?: {
                /** @example mobile */
                type?: string;
                /** @example android */
                os?: string;
            };
            geo?: {
                /** @example IN */
                country?: string;
            };
            candidates?: components["schemas"]["Candidate"][];
        };
        Candidate: {
            /** Format: uuid */
            campaign_id: string;
            /** Format: uuid */
            creative_id: string;
            /**
             * Format: double
             * @example 3
             */
            max_bid_cpm: number;
        };
        BidResponse: {
            bids: components["schemas"]["Bid"][];
        };
        Bid: {
            /** Format: uuid */
            campaign_id: string;
            /** Format: uuid */
            creative_id: string;
            /**
             * Format: double
             * @example 2.4
             */
            price: number;
        };
        BidderConfig: {
            /** @example 10 */
            latency_ms: number;
            /** @example 15 */
            jitter_ms: number;
            /**
             * Format: double
             * @example 0
             */
            error_rate: number;
            /**
             * Format: double
             * @example 0
             */
            timeout_rate: number;
            /**
             * Format: double
             * @example 0.05
             */
            no_bid_rate: number;
            /**
             * @example fixed
             * @enum {string}
             */
            bid_mode: "fixed" | "shade" | "random";
            /**
             * Format: double
             * @example 0.9
             */
            bid_shade: number;
            /**
             * Format: int64
             * @example 1
             */
            seed: number;
        };
        BidderConfigPatch: {
            latency_ms?: number;
            jitter_ms?: number;
            /** Format: double */
            error_rate?: number;
            /** Format: double */
            timeout_rate?: number;
            /** Format: double */
            no_bid_rate?: number;
            /** @enum {string} */
            bid_mode?: "fixed" | "shade" | "random";
            /** Format: double */
            bid_shade?: number;
            /** Format: int64 */
            seed?: number;
        };
        ErrorResponse: {
            error: {
                code: string;
                message: string;
            };
        };
    };
    responses: never;
    parameters: never;
    requestBodies: never;
    headers: never;
    pathItems: never;
}
export type $defs = Record<string, never>;
export interface operations {
    submitBid: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["BidRequest"];
            };
        };
        responses: {
            /** @description One or more valid bids returned. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["BidResponse"];
                };
            };
            /** @description No bid submitted (candidates below floor, no-bid rate triggered, or empty candidates). */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Malformed JSON request. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorResponse"];
                };
            };
            /** @description Simulated DSP internal error. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorResponse"];
                };
            };
            /** @description Simulated DSP timeout exceeded. */
            504: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
        };
    };
    checkHealth: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Process is healthy. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        /** @example ok */
                        status?: string;
                    };
                };
            };
        };
    };
    checkReadiness: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Bidder is ready. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Bidder is draining or shutting down. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
        };
    };
    getConfig: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Current bidder configuration. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["BidderConfig"];
                };
            };
        };
    };
    updateConfig: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["BidderConfigPatch"];
            };
        };
        responses: {
            /** @description Updated bidder configuration. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["BidderConfig"];
                };
            };
            /** @description Invalid configuration parameter. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
        };
    };
}
