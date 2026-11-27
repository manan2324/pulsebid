export interface paths {
    "/v1/snapshot": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** @description Full catalog snapshot in protobuf JSON using snake_case proto field names. Int64 version and money micros are strings, enums use symbolic names, and timestamps use RFC 3339 UTC. Includes only active campaigns with active advertisers, enabled bidders, and active creatives. Date windows are evaluated later by the gateway. */
        get: operations["getCatalogSnapshot"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/advertisers": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** @description Order: created_at DESC, id DESC. Archived records remain visible. */
        get: operations["listAdvertisers"];
        put?: never;
        /** @description Create a advertiser. */
        post: operations["createAdvertiser"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/advertisers/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        /** @description Fetch one record. */
        get: operations["getAdvertiser"];
        put?: never;
        post?: never;
        /** @description Soft archive; retains the record. */
        delete: operations["deleteAdvertiser"];
        options?: never;
        head?: never;
        /** @description Partial update; omitted fields are preserved. Null clears only nullable fields. Unknown/read-only fields are rejected. */
        patch: operations["patchAdvertiser"];
        trace?: never;
    };
    "/v1/bidders": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** @description Order: created_at DESC, id DESC. Archived records remain visible. */
        get: operations["listBidders"];
        put?: never;
        /** @description Create a bidder. */
        post: operations["createBidder"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/bidders/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        /** @description Fetch one record. */
        get: operations["getBidder"];
        put?: never;
        post?: never;
        /** @description Hard delete; any campaign reference produces 409. */
        delete: operations["deleteBidder"];
        options?: never;
        head?: never;
        /** @description Partial update; omitted fields are preserved. Null clears only nullable fields. Unknown/read-only fields are rejected. */
        patch: operations["patchBidder"];
        trace?: never;
    };
    "/v1/campaigns": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** @description Order: created_at DESC, id DESC. Archived records remain visible. */
        get: operations["listCampaigns"];
        put?: never;
        /** @description Creates a draft campaign. */
        post: operations["createCampaign"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/campaigns/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        /** @description Fetch one record. */
        get: operations["getCampaign"];
        put?: never;
        post?: never;
        /** @description Soft archive; retains the record. */
        delete: operations["deleteCampaign"];
        options?: never;
        head?: never;
        /** @description Partial update; omitted fields are preserved. Null clears only nullable fields. Unknown/read-only fields are rejected. Status changes enforce activation preconditions; archived is terminal. */
        patch: operations["patchCampaign"];
        trace?: never;
    };
    "/v1/campaigns/{id}/activate": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** @description Requires at least one active creative. Draft and paused campaigns may activate; archived campaigns cannot. */
        post: operations["activateCampaign"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/campaigns/{id}/pause": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** @description An active campaign can pause; a draft campaign cannot. */
        post: operations["pauseCampaign"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/campaigns/{id}/targeting": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        /** @description Returns unrestricted defaults for an existing campaign with no saved rule. */
        get: operations["getTargeting"];
        /** @description Full replacement; omitted arrays become empty, omitted timezone becomes UTC. Creates a missing rule. */
        put: operations["replaceTargeting"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/campaigns/{id}/budget": {
        parameters: {
            query?: never;
            header?: {
                "X-PulseBid-Test-Time"?: string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        /** @description Reads current daily budget spend and remaining amount from Redis. */
        get: operations["getCampaignBudget"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/campaigns/{id}/budget/reset": {
        parameters: {
            query?: never;
            header?: {
                "X-PulseBid-Test-Time"?: string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** @description Test-only endpoint to reset campaign spend in Redis. Guarded by ENABLE_TEST_ENDPOINTS. */
        post: operations["resetCampaignBudget"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/campaigns/{id}/creatives": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        /** @description List this campaign's creatives. */
        get: operations["listCreatives"];
        put?: never;
        /** @description Create an active creative by default. */
        post: operations["createCreative"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/creatives/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        /** @description Fetch one creative. */
        get: operations["getCreative"];
        put?: never;
        post?: never;
        /** @description Hard delete the creative. */
        delete: operations["deleteCreative"];
        options?: never;
        head?: never;
        /** @description Update creative fields; campaign_id is immutable. */
        patch: operations["patchCreative"];
        trace?: never;
    };
}
export type webhooks = Record<string, never>;
export interface components {
    schemas: {
        CatalogSnapshot: {
            /** @description Exact protobuf int64 catalog version. */
            version: string;
            /** @enum {boolean} */
            unchanged: false;
            /** Format: date-time */
            generated_at: string;
            bidders: {
                id: string;
                /** Format: uri */
                endpoint_url: string;
                /** @description 0 means gateway default. */
                timeout_ms: number;
            }[];
            campaigns: components["schemas"]["CatalogCampaign"][];
        };
        CatalogCampaign: {
            /** Format: uuid */
            id: string;
            /** Format: uuid */
            advertiser_id: string;
            bidder_id: string;
            name: string;
            daily_budget_micros: string;
            max_bid_cpm_micros: string;
            /** @enum {string} */
            pacing_mode: "PACING_MODE_ASAP" | "PACING_MODE_EVEN";
            /**
             * Format: date-time
             * @description Omitted when unset.
             */
            start_at?: string;
            /**
             * Format: date-time
             * @description Omitted when unset.
             */
            end_at?: string;
            targeting: {
                countries?: string[];
                device_types?: string[];
                placement_ids?: string[];
                hours_of_day?: number[];
                timezone?: string;
            };
            /** @description Active creatives ordered by ID. */
            creatives: {
                /** Format: uuid */
                id?: string;
                format?: string;
                /** @description 0 when unspecified. */
                width?: number;
                /** @description 0 when unspecified. */
                height?: number;
                markup?: string;
                /** Format: uri */
                landing_url?: string;
            }[];
        };
        AdvertiserCreate: {
            name: string;
            /** Format: email */
            contact_email?: string | null;
        };
        AdvertiserPatch: {
            name?: string;
            /** Format: email */
            contact_email?: string | null;
            /** @enum {string} */
            status?: "active" | "archived";
        };
        BidderCreate: {
            id: string;
            name: string;
            /** Format: uri */
            endpoint_url: string;
            timeout_ms?: number | null;
            /** @default true */
            enabled: boolean;
        };
        BidderPatch: {
            name?: string;
            /** Format: uri */
            endpoint_url?: string;
            timeout_ms?: number | null;
            /** @default true */
            enabled: boolean;
        };
        CampaignCreate: {
            /** Format: uuid */
            advertiser_id: string;
            bidder_id: string;
            name: string;
            /** @description USD; > 0 and <= 1000000. */
            daily_budget: number | string;
            /** @description USD CPM; > 0 and <= 1000. */
            max_bid_cpm: number | string;
            /**
             * @default even
             * @enum {string}
             */
            pacing_mode: "asap" | "even";
            /** Format: date-time */
            start_at?: string | null;
            /** Format: date-time */
            end_at?: string | null;
        };
        CampaignPatch: {
            /** Format: uuid */
            advertiser_id?: string;
            bidder_id?: string;
            name?: string;
            /** @enum {string} */
            status?: "draft" | "active" | "paused" | "archived";
            /** @description USD; > 0 and <= 1000000. */
            daily_budget?: number | string;
            /** @description USD CPM; > 0 and <= 1000. */
            max_bid_cpm?: number | string;
            /**
             * @default even
             * @enum {string}
             */
            pacing_mode: "asap" | "even";
            /** Format: date-time */
            start_at?: string | null;
            /** Format: date-time */
            end_at?: string | null;
        };
        CreativeCreate: {
            name: string;
            /** @enum {string} */
            format: "banner" | "native" | "video";
            width?: number | null;
            height?: number | null;
            /** @description At most 65536 UTF-8 bytes. */
            markup: string;
            /** Format: uri */
            landing_url: string;
            /**
             * @default active
             * @enum {string}
             */
            status: "active" | "paused";
        };
        CreativePatch: {
            name?: string;
            /** @enum {string} */
            format?: "banner" | "native" | "video";
            width?: number | null;
            height?: number | null;
            /** @description At most 65536 UTF-8 bytes. */
            markup?: string;
            /** Format: uri */
            landing_url?: string;
            /**
             * @default active
             * @enum {string}
             */
            status: "active" | "paused";
        };
        TargetingPut: {
            /**
             * @description ISO alpha-2 codes, normalized to uppercase and deduplicated.
             * @default []
             */
            countries: string[];
            /** @default [] */
            device_types: ("mobile" | "desktop" | "tablet" | "ctv")[];
            /** @default [] */
            placement_ids: string[];
            /**
             * @description Sorted and deduplicated.
             * @default []
             */
            hours_of_day: number[];
            /**
             * @description IANA timezone; Local is not accepted.
             * @default UTC
             */
            timezone: string;
        };
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
        Advertiser: {
            /** Format: uuid */
            id: string;
            name: string;
            /** Format: email */
            contact_email: string | null;
            /** @enum {string} */
            status: "active" | "archived";
            /** Format: date-time */
            created_at: string;
            /** Format: date-time */
            updated_at: string;
        };
        AdvertiserPage: {
            data: components["schemas"]["Advertiser"][];
            next_cursor: string | null;
        };
        Bidder: {
            id: string;
            name: string;
            /** Format: uri */
            endpoint_url: string;
            timeout_ms: number | null;
            /** @default true */
            enabled: boolean;
            /** Format: date-time */
            created_at: string;
            /** Format: date-time */
            updated_at: string;
        };
        BidderPage: {
            data: components["schemas"]["Bidder"][];
            next_cursor: string | null;
        };
        BudgetView: {
            /** Format: uuid */
            campaign_id: string;
            /** @example 2026-11-18 */
            date: string;
            /** @example 0.01 */
            daily_budget: number;
            /**
             * Format: int64
             * @example 10000
             */
            daily_budget_micros: number;
            /** @example 0.00909 */
            spent: number;
            /**
             * Format: int64
             * @example 9090
             */
            spent_micros: number;
            /** @example 0.00091 */
            remaining: number;
            /**
             * Format: int64
             * @example 910
             */
            remaining_micros: number;
        };
        Campaign: {
            /** Format: uuid */
            id: string;
            /** Format: uuid */
            advertiser_id: string;
            bidder_id: string;
            name: string;
            /** @enum {string} */
            status: "draft" | "active" | "paused" | "archived";
            daily_budget: number;
            max_bid_cpm: number;
            /**
             * @default even
             * @enum {string}
             */
            pacing_mode: "asap" | "even";
            /** Format: date-time */
            start_at: string | null;
            /** Format: date-time */
            end_at: string | null;
            /** Format: date-time */
            created_at: string;
            /** Format: date-time */
            updated_at: string;
        };
        CampaignPage: {
            data: components["schemas"]["Campaign"][];
            next_cursor: string | null;
        };
        Creative: {
            /** Format: uuid */
            id: string;
            name: string;
            /** @enum {string} */
            format: "banner" | "native" | "video";
            width: number | null;
            height: number | null;
            /** @description At most 65536 UTF-8 bytes. */
            markup: string;
            /** Format: uri */
            landing_url: string;
            /**
             * @default active
             * @enum {string}
             */
            status: "active" | "paused";
            /** Format: uuid */
            campaign_id: string;
            /** Format: date-time */
            created_at: string;
            /** Format: date-time */
            updated_at: string;
        };
        CreativePage: {
            data: components["schemas"]["Creative"][];
            next_cursor: string | null;
        };
        Targeting: {
            /** Format: uuid */
            campaign_id: string;
            /**
             * @description ISO alpha-2 codes, normalized to uppercase and deduplicated.
             * @default []
             */
            countries: string[];
            /** @default [] */
            device_types: ("mobile" | "desktop" | "tablet" | "ctv")[];
            /** @default [] */
            placement_ids: string[];
            /**
             * @description Sorted and deduplicated.
             * @default []
             */
            hours_of_day: number[];
            /**
             * @description IANA timezone; Local is not accepted.
             * @default UTC
             */
            timezone: string;
            /** Format: date-time */
            updated_at: string;
        };
    };
    responses: {
        /** @description Invalid input */
        Error400: {
            headers: {
                "X-Request-Id"?: string;
                [name: string]: unknown;
            };
            content: {
                "application/json": components["schemas"]["ErrorEnvelope"];
            };
        };
        /** @description Missing or invalid X-API-Key */
        Error401: {
            headers: {
                "X-Request-Id"?: string;
                [name: string]: unknown;
            };
            content: {
                "application/json": components["schemas"]["ErrorEnvelope"];
            };
        };
        /** @description Resource or route not found */
        Error404: {
            headers: {
                "X-Request-Id"?: string;
                [name: string]: unknown;
            };
            content: {
                "application/json": components["schemas"]["ErrorEnvelope"];
            };
        };
        /** @description Unique/FK conflict or invalid campaign transition */
        Error409: {
            headers: {
                "X-Request-Id"?: string;
                [name: string]: unknown;
            };
            content: {
                "application/json": components["schemas"]["ErrorEnvelope"];
            };
        };
        /** @description Body exceeds 512 KiB */
        Error413: {
            headers: {
                "X-Request-Id"?: string;
                [name: string]: unknown;
            };
            content: {
                "application/json": components["schemas"]["ErrorEnvelope"];
            };
        };
        /** @description Content-Type must be application/json */
        Error415: {
            headers: {
                "X-Request-Id"?: string;
                [name: string]: unknown;
            };
            content: {
                "application/json": components["schemas"]["ErrorEnvelope"];
            };
        };
        /** @description Internal error */
        Error500: {
            headers: {
                "X-Request-Id"?: string;
                [name: string]: unknown;
            };
            content: {
                "application/json": components["schemas"]["ErrorEnvelope"];
            };
        };
        /** @description Database unavailable */
        Error503: {
            headers: {
                "X-Request-Id"?: string;
                [name: string]: unknown;
            };
            content: {
                "application/json": components["schemas"]["ErrorEnvelope"];
            };
        };
    };
    parameters: never;
    requestBodies: never;
    headers: never;
    pathItems: never;
}
export type $defs = Record<string, never>;
export interface operations {
    getCatalogSnapshot: {
        parameters: {
            query?: never;
            header?: {
                /** @description Previously returned quoted decimal version ETag; weak tags, lists, and * are supported. */
                "If-None-Match"?: string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Current full catalog. Cached data is reused until the database version changes. */
            200: {
                headers: {
                    ETag?: string;
                    "X-Request-Id"?: string;
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["CatalogSnapshot"];
                };
            };
            /** @description The validator matches the current catalog version; no response body. */
            304: {
                headers: {
                    ETag?: string;
                    "X-Request-Id"?: string;
                    [name: string]: unknown;
                };
                content?: never;
            };
            401: components["responses"]["Error401"];
            500: components["responses"]["Error500"];
            503: components["responses"]["Error503"];
        };
    };
    listAdvertisers: {
        parameters: {
            query?: {
                limit?: number;
                /** @description Opaque keyset cursor bound to the resource and filters. Keep the same filters for subsequent pages. */
                cursor?: string;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Success */
            200: {
                headers: {
                    "X-Request-Id"?: string;
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AdvertiserPage"];
                };
            };
            400: components["responses"]["Error400"];
            401: components["responses"]["Error401"];
            404: components["responses"]["Error404"];
            409: components["responses"]["Error409"];
            413: components["responses"]["Error413"];
            415: components["responses"]["Error415"];
            500: components["responses"]["Error500"];
            503: components["responses"]["Error503"];
        };
    };
    createAdvertiser: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["AdvertiserCreate"];
            };
        };
        responses: {
            /** @description Success */
            201: {
                headers: {
                    "X-Request-Id"?: string;
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Advertiser"];
                };
            };
            400: components["responses"]["Error400"];
            401: components["responses"]["Error401"];
            404: components["responses"]["Error404"];
            409: components["responses"]["Error409"];
            413: components["responses"]["Error413"];
            415: components["responses"]["Error415"];
            500: components["responses"]["Error500"];
            503: components["responses"]["Error503"];
        };
    };
    getAdvertiser: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Success */
            200: {
                headers: {
                    "X-Request-Id"?: string;
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Advertiser"];
                };
            };
            400: components["responses"]["Error400"];
            401: components["responses"]["Error401"];
            404: components["responses"]["Error404"];
            409: components["responses"]["Error409"];
            413: components["responses"]["Error413"];
            415: components["responses"]["Error415"];
            500: components["responses"]["Error500"];
            503: components["responses"]["Error503"];
        };
    };
    deleteAdvertiser: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Deleted or archived successfully */
            204: {
                headers: {
                    "X-Request-Id"?: string;
                    [name: string]: unknown;
                };
                content?: never;
            };
            400: components["responses"]["Error400"];
            401: components["responses"]["Error401"];
            404: components["responses"]["Error404"];
            409: components["responses"]["Error409"];
            413: components["responses"]["Error413"];
            415: components["responses"]["Error415"];
            500: components["responses"]["Error500"];
            503: components["responses"]["Error503"];
        };
    };
    patchAdvertiser: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["AdvertiserPatch"];
            };
        };
        responses: {
            /** @description Success */
            200: {
                headers: {
                    "X-Request-Id"?: string;
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Advertiser"];
                };
            };
            400: components["responses"]["Error400"];
            401: components["responses"]["Error401"];
            404: components["responses"]["Error404"];
            409: components["responses"]["Error409"];
            413: components["responses"]["Error413"];
            415: components["responses"]["Error415"];
            500: components["responses"]["Error500"];
            503: components["responses"]["Error503"];
        };
    };
    listBidders: {
        parameters: {
            query?: {
                limit?: number;
                /** @description Opaque keyset cursor bound to the resource and filters. Keep the same filters for subsequent pages. */
                cursor?: string;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Success */
            200: {
                headers: {
                    "X-Request-Id"?: string;
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["BidderPage"];
                };
            };
            400: components["responses"]["Error400"];
            401: components["responses"]["Error401"];
            404: components["responses"]["Error404"];
            409: components["responses"]["Error409"];
            413: components["responses"]["Error413"];
            415: components["responses"]["Error415"];
            500: components["responses"]["Error500"];
            503: components["responses"]["Error503"];
        };
    };
    createBidder: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["BidderCreate"];
            };
        };
        responses: {
            /** @description Success */
            201: {
                headers: {
                    "X-Request-Id"?: string;
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Bidder"];
                };
            };
            400: components["responses"]["Error400"];
            401: components["responses"]["Error401"];
            404: components["responses"]["Error404"];
            409: components["responses"]["Error409"];
            413: components["responses"]["Error413"];
            415: components["responses"]["Error415"];
            500: components["responses"]["Error500"];
            503: components["responses"]["Error503"];
        };
    };
    getBidder: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Success */
            200: {
                headers: {
                    "X-Request-Id"?: string;
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Bidder"];
                };
            };
            400: components["responses"]["Error400"];
            401: components["responses"]["Error401"];
            404: components["responses"]["Error404"];
            409: components["responses"]["Error409"];
            413: components["responses"]["Error413"];
            415: components["responses"]["Error415"];
            500: components["responses"]["Error500"];
            503: components["responses"]["Error503"];
        };
    };
    deleteBidder: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Deleted or archived successfully */
            204: {
                headers: {
                    "X-Request-Id"?: string;
                    [name: string]: unknown;
                };
                content?: never;
            };
            400: components["responses"]["Error400"];
            401: components["responses"]["Error401"];
            404: components["responses"]["Error404"];
            409: components["responses"]["Error409"];
            413: components["responses"]["Error413"];
            415: components["responses"]["Error415"];
            500: components["responses"]["Error500"];
            503: components["responses"]["Error503"];
        };
    };
    patchBidder: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["BidderPatch"];
            };
        };
        responses: {
            /** @description Success */
            200: {
                headers: {
                    "X-Request-Id"?: string;
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Bidder"];
                };
            };
            400: components["responses"]["Error400"];
            401: components["responses"]["Error401"];
            404: components["responses"]["Error404"];
            409: components["responses"]["Error409"];
            413: components["responses"]["Error413"];
            415: components["responses"]["Error415"];
            500: components["responses"]["Error500"];
            503: components["responses"]["Error503"];
        };
    };
    listCampaigns: {
        parameters: {
            query?: {
                limit?: number;
                /** @description Opaque keyset cursor bound to the resource and filters. Keep the same filters for subsequent pages. */
                cursor?: string;
                advertiser_id?: string;
                status?: "draft" | "active" | "paused" | "archived";
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Success */
            200: {
                headers: {
                    "X-Request-Id"?: string;
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["CampaignPage"];
                };
            };
            400: components["responses"]["Error400"];
            401: components["responses"]["Error401"];
            404: components["responses"]["Error404"];
            409: components["responses"]["Error409"];
            413: components["responses"]["Error413"];
            415: components["responses"]["Error415"];
            500: components["responses"]["Error500"];
            503: components["responses"]["Error503"];
        };
    };
    createCampaign: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["CampaignCreate"];
            };
        };
        responses: {
            /** @description Success */
            201: {
                headers: {
                    "X-Request-Id"?: string;
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Campaign"];
                };
            };
            400: components["responses"]["Error400"];
            401: components["responses"]["Error401"];
            404: components["responses"]["Error404"];
            409: components["responses"]["Error409"];
            413: components["responses"]["Error413"];
            415: components["responses"]["Error415"];
            500: components["responses"]["Error500"];
            503: components["responses"]["Error503"];
        };
    };
    getCampaign: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Success */
            200: {
                headers: {
                    "X-Request-Id"?: string;
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Campaign"];
                };
            };
            400: components["responses"]["Error400"];
            401: components["responses"]["Error401"];
            404: components["responses"]["Error404"];
            409: components["responses"]["Error409"];
            413: components["responses"]["Error413"];
            415: components["responses"]["Error415"];
            500: components["responses"]["Error500"];
            503: components["responses"]["Error503"];
        };
    };
    deleteCampaign: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Deleted or archived successfully */
            204: {
                headers: {
                    "X-Request-Id"?: string;
                    [name: string]: unknown;
                };
                content?: never;
            };
            400: components["responses"]["Error400"];
            401: components["responses"]["Error401"];
            404: components["responses"]["Error404"];
            409: components["responses"]["Error409"];
            413: components["responses"]["Error413"];
            415: components["responses"]["Error415"];
            500: components["responses"]["Error500"];
            503: components["responses"]["Error503"];
        };
    };
    patchCampaign: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["CampaignPatch"];
            };
        };
        responses: {
            /** @description Success */
            200: {
                headers: {
                    "X-Request-Id"?: string;
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Campaign"];
                };
            };
            400: components["responses"]["Error400"];
            401: components["responses"]["Error401"];
            404: components["responses"]["Error404"];
            409: components["responses"]["Error409"];
            413: components["responses"]["Error413"];
            415: components["responses"]["Error415"];
            500: components["responses"]["Error500"];
            503: components["responses"]["Error503"];
        };
    };
    activateCampaign: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Success */
            200: {
                headers: {
                    "X-Request-Id"?: string;
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Campaign"];
                };
            };
            400: components["responses"]["Error400"];
            401: components["responses"]["Error401"];
            404: components["responses"]["Error404"];
            409: components["responses"]["Error409"];
            413: components["responses"]["Error413"];
            415: components["responses"]["Error415"];
            500: components["responses"]["Error500"];
            503: components["responses"]["Error503"];
        };
    };
    pauseCampaign: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Success */
            200: {
                headers: {
                    "X-Request-Id"?: string;
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Campaign"];
                };
            };
            400: components["responses"]["Error400"];
            401: components["responses"]["Error401"];
            404: components["responses"]["Error404"];
            409: components["responses"]["Error409"];
            413: components["responses"]["Error413"];
            415: components["responses"]["Error415"];
            500: components["responses"]["Error500"];
            503: components["responses"]["Error503"];
        };
    };
    getTargeting: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Success */
            200: {
                headers: {
                    "X-Request-Id"?: string;
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Targeting"];
                };
            };
            400: components["responses"]["Error400"];
            401: components["responses"]["Error401"];
            404: components["responses"]["Error404"];
            409: components["responses"]["Error409"];
            413: components["responses"]["Error413"];
            415: components["responses"]["Error415"];
            500: components["responses"]["Error500"];
            503: components["responses"]["Error503"];
        };
    };
    replaceTargeting: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["TargetingPut"];
            };
        };
        responses: {
            /** @description Success */
            200: {
                headers: {
                    "X-Request-Id"?: string;
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Targeting"];
                };
            };
            400: components["responses"]["Error400"];
            401: components["responses"]["Error401"];
            404: components["responses"]["Error404"];
            409: components["responses"]["Error409"];
            413: components["responses"]["Error413"];
            415: components["responses"]["Error415"];
            500: components["responses"]["Error500"];
            503: components["responses"]["Error503"];
        };
    };
    getCampaignBudget: {
        parameters: {
            query?: never;
            header?: {
                "X-PulseBid-Test-Time"?: string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Success */
            200: {
                headers: {
                    "X-Request-Id"?: string;
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["BudgetView"];
                };
            };
            400: components["responses"]["Error400"];
            401: components["responses"]["Error401"];
            404: components["responses"]["Error404"];
            500: components["responses"]["Error500"];
            503: components["responses"]["Error503"];
        };
    };
    resetCampaignBudget: {
        parameters: {
            query?: never;
            header?: {
                "X-PulseBid-Test-Time"?: string;
            };
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Success */
            200: {
                headers: {
                    "X-Request-Id"?: string;
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        /** @example ok */
                        status?: string;
                        /** Format: uuid */
                        campaign_id?: string;
                    };
                };
            };
            400: components["responses"]["Error400"];
            401: components["responses"]["Error401"];
            404: components["responses"]["Error404"];
            500: components["responses"]["Error500"];
            503: components["responses"]["Error503"];
        };
    };
    listCreatives: {
        parameters: {
            query?: {
                limit?: number;
                /** @description Opaque keyset cursor bound to the resource and filters. Keep the same filters for subsequent pages. */
                cursor?: string;
            };
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Success */
            200: {
                headers: {
                    "X-Request-Id"?: string;
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["CreativePage"];
                };
            };
            400: components["responses"]["Error400"];
            401: components["responses"]["Error401"];
            404: components["responses"]["Error404"];
            409: components["responses"]["Error409"];
            413: components["responses"]["Error413"];
            415: components["responses"]["Error415"];
            500: components["responses"]["Error500"];
            503: components["responses"]["Error503"];
        };
    };
    createCreative: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["CreativeCreate"];
            };
        };
        responses: {
            /** @description Success */
            201: {
                headers: {
                    "X-Request-Id"?: string;
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Creative"];
                };
            };
            400: components["responses"]["Error400"];
            401: components["responses"]["Error401"];
            404: components["responses"]["Error404"];
            409: components["responses"]["Error409"];
            413: components["responses"]["Error413"];
            415: components["responses"]["Error415"];
            500: components["responses"]["Error500"];
            503: components["responses"]["Error503"];
        };
    };
    getCreative: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Success */
            200: {
                headers: {
                    "X-Request-Id"?: string;
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Creative"];
                };
            };
            400: components["responses"]["Error400"];
            401: components["responses"]["Error401"];
            404: components["responses"]["Error404"];
            409: components["responses"]["Error409"];
            413: components["responses"]["Error413"];
            415: components["responses"]["Error415"];
            500: components["responses"]["Error500"];
            503: components["responses"]["Error503"];
        };
    };
    deleteCreative: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Deleted or archived successfully */
            204: {
                headers: {
                    "X-Request-Id"?: string;
                    [name: string]: unknown;
                };
                content?: never;
            };
            400: components["responses"]["Error400"];
            401: components["responses"]["Error401"];
            404: components["responses"]["Error404"];
            409: components["responses"]["Error409"];
            413: components["responses"]["Error413"];
            415: components["responses"]["Error415"];
            500: components["responses"]["Error500"];
            503: components["responses"]["Error503"];
        };
    };
    patchCreative: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["CreativePatch"];
            };
        };
        responses: {
            /** @description Success */
            200: {
                headers: {
                    "X-Request-Id"?: string;
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Creative"];
                };
            };
            400: components["responses"]["Error400"];
            401: components["responses"]["Error401"];
            404: components["responses"]["Error404"];
            409: components["responses"]["Error409"];
            413: components["responses"]["Error413"];
            415: components["responses"]["Error415"];
            500: components["responses"]["Error500"];
            503: components["responses"]["Error503"];
        };
    };
}
