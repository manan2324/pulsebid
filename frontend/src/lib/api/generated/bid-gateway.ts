export interface paths {
    "/v1/bid": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /**
         * Submit a real-time bid request
         * @description Evaluates active campaigns against targeting rules, fans out to demand partners,
         *     executes a second-price auction, and returns the winning creative or 204 No Content.
         */
        post: operations["submitBid"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/events/impression": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Track ad impression
         * @description Validates HMAC-SHA256 signed event token, publishes impression event to Kafka, and returns 1x1 transparent GIF.
         */
        get: operations["trackImpression"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/events/click": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Track ad click and redirect
         * @description Validates HMAC-SHA256 signed event token, publishes click event to Kafka, and redirects (302 Found) to destination landing URL.
         */
        get: operations["trackClick"];
        put?: never;
        post?: never;
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
            id: string;
            imp: components["schemas"]["Impression"];
            device?: components["schemas"]["Device"];
            geo?: components["schemas"]["Geo"];
            site?: components["schemas"]["Site"];
            user?: components["schemas"]["User"];
            /**
             * @description Timeout deadline in milliseconds.
             * @example 100
             */
            tmax?: number;
        };
        Impression: {
            /** @example imp-1 */
            id: string;
            /** @example home-top-banner */
            placement_id: string;
            /** @example 300 */
            w?: number;
            /** @example 250 */
            h?: number;
            /**
             * Format: double
             * @example 0.5
             */
            bidfloor?: number;
            /**
             * @default USD
             * @example USD
             * @enum {string}
             */
            bidfloorcur: "USD";
        };
        Device: {
            /**
             * @example mobile
             * @enum {string}
             */
            type?: "mobile" | "desktop" | "tablet" | "ctv";
            /** @example android */
            os?: string;
            /** @example Mozilla/5.0 ... */
            ua?: string;
        };
        Geo: {
            /** @example IN */
            country?: string;
            /** @example GJ */
            region?: string;
            /** @example Ahmedabad */
            city?: string;
        };
        Site: {
            /** @example pub-42 */
            id?: string;
            /** @example example.com */
            domain?: string;
        };
        User: {
            /** @example u-789 */
            id?: string;
        };
        BidResponse: {
            /** @example req-123 */
            request_id: string;
            /**
             * Format: uuid
             * @example 0193f0a2-0000-7000-8000-000000000001
             */
            bid_id: string;
            /** Format: uuid */
            campaign_id: string;
            /** Format: uuid */
            creative_id: string;
            /** Format: uuid */
            advertiser_id: string;
            /** @example bidder-1 */
            bidder_id: string;
            /**
             * Format: double
             * @description Clearing CPM in USD.
             * @example 1.51
             */
            price: number;
            /** @example USD */
            currency: string;
            /** @description Creative markup with tracking placeholders substituted. */
            adm: string;
            /** Format: uri */
            impression_url: string;
            /** Format: uri */
            click_url: string;
            /** @example 300 */
            w: number;
            /** @example 250 */
            h: number;
        };
        ErrorDetail: {
            field: string;
            issue: string;
        };
        ErrorResponse: {
            error: {
                /** @example invalid_request */
                code: string;
                /** @example request validation failed */
                message: string;
                request_id: string;
                details?: components["schemas"]["ErrorDetail"][];
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
            header?: {
                /** @description Client-supplied request correlation ID. */
                "X-Request-Id"?: string;
                /** @description RFC 3339 timestamp overriding current time when ENABLE_TEST_ENDPOINTS is true. */
                "X-PulseBid-Test-Time"?: string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["BidRequest"];
            };
        };
        responses: {
            /** @description Auction filled with winning ad creative. */
            200: {
                headers: {
                    /** @description Request correlation ID. */
                    "X-Request-Id"?: string;
                    /** @description Performance metrics (e.g. total;dur=12.4). */
                    "Server-Timing"?: string;
                    /** @description Diagnostic execution counts when ENABLE_TEST_ENDPOINTS is true. */
                    "X-PulseBid-Debug"?: string;
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["BidResponse"];
                };
            };
            /** @description No fill (no eligible campaign, no valid bids, below floor, or budget blocked). */
            204: {
                headers: {
                    "X-Request-Id"?: string;
                    "Server-Timing"?: string;
                    /** @description Reason the impression was not filled. */
                    "X-PulseBid-Reason"?: "no_eligible" | "no_bids" | "below_floor" | "budget" | "error";
                    "X-PulseBid-Debug"?: string;
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Validation error or malformed JSON. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorResponse"];
                };
            };
            /** @description Request body exceeds size limit (REQUEST_MAX_BYTES). */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorResponse"];
                };
            };
            /** @description Unsupported Content-Type (must be application/json). */
            415: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorResponse"];
                };
            };
            /** @description Internal server error. */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorResponse"];
                };
            };
        };
    };
    trackImpression: {
        parameters: {
            query: {
                /** @description Base64URL signed token from bid response impression_url. */
                t: string;
            };
            header?: {
                /** @description RFC 3339 timestamp override for token expiration testing. */
                "X-PulseBid-Test-Time"?: string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Transparent 1x1 GIF tracking pixel. */
            200: {
                headers: {
                    "Cache-Control"?: string;
                    [name: string]: unknown;
                };
                content: {
                    "image/gif": string;
                };
            };
            /** @description Invalid event token signature or format. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorResponse"];
                };
            };
            /** @description Event token has expired. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorResponse"];
                };
            };
            /** @description Publisher queue full or Kafka broker unavailable. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorResponse"];
                };
            };
        };
    };
    trackClick: {
        parameters: {
            query: {
                /** @description Base64URL signed token from bid response click_url. */
                t: string;
            };
            header?: {
                /** @description RFC 3339 timestamp override for token expiration testing. */
                "X-PulseBid-Test-Time"?: string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description HTTP redirect to advertiser destination landing URL. */
            302: {
                headers: {
                    Location?: string;
                    "Cache-Control"?: string;
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Invalid event token signature or format. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorResponse"];
                };
            };
            /** @description Event token has expired. */
            410: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorResponse"];
                };
            };
            /** @description Publisher queue full or Kafka broker unavailable. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorResponse"];
                };
            };
        };
    };
}
