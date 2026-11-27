export interface paths {
    "/v1/reports/campaigns": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Retrieve aggregated campaign reports across a time window */
        get: operations["getCampaignReports"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/v1/reports/campaigns/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Retrieve performance report for a single campaign */
        get: operations["getSingleCampaignReport"];
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
        ReportResponse: {
            /**
             * Format: date-time
             * @example 2026-11-18T00:00:00Z
             */
            from: string;
            /**
             * Format: date-time
             * @example 2026-11-19T00:00:00Z
             */
            to: string;
            /**
             * @example hour
             * @enum {string}
             */
            granularity: "hour" | "day" | "total";
            data: components["schemas"]["ReportRow"][];
            totals: components["schemas"]["Totals"];
        };
        ReportRow: {
            /** Format: uuid */
            campaign_id: string;
            /**
             * Format: date-time
             * @description Omitted when granularity is total
             * @example 2026-11-18T10:00:00Z
             */
            bucket?: string;
            /**
             * Format: int64
             * @example 1200
             */
            impressions: number;
            /**
             * Format: int64
             * @example 36
             */
            clicks: number;
            /**
             * Format: double
             * @example 0.03
             */
            ctr: number;
            /**
             * Format: double
             * @description Total spend in dollars (spend_micros / 1,000,000)
             * @example 1.812
             */
            spend: number;
            /** @example USD */
            currency: string;
            /**
             * Format: int64
             * @example 5000
             */
            bids: number;
            /**
             * Format: int64
             * @example 1210
             */
            wins: number;
            /**
             * Format: double
             * @example 0.242
             */
            win_rate: number;
        };
        Totals: {
            /**
             * Format: int64
             * @example 1200
             */
            impressions: number;
            /**
             * Format: int64
             * @example 36
             */
            clicks: number;
            /**
             * Format: double
             * @example 0.03
             */
            ctr: number;
            /**
             * Format: double
             * @example 1.812
             */
            spend: number;
            /**
             * Format: int64
             * @example 5000
             */
            bids: number;
            /**
             * Format: int64
             * @example 1210
             */
            wins: number;
            /**
             * Format: double
             * @example 0.242
             */
            win_rate: number;
        };
        ErrorResponse: {
            error: {
                /** @example invalid_request */
                code: string;
                /** @example from parameter is required */
                message: string;
                /** @example 0193f0a2-ca00-7000-8000-000000000001 */
                request_id: string;
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
    getCampaignReports: {
        parameters: {
            query: {
                /** @description Inclusive start of reporting interval (RFC 3339) */
                from: string;
                /** @description Exclusive end of reporting interval (RFC 3339) */
                to: string;
                /** @description Temporal aggregation bucket */
                granularity?: "hour" | "day" | "total";
                /** @description Filter by campaign ID (repeatable, maximum 100) */
                campaign_id?: string[];
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Performance metrics report */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ReportResponse"];
                };
            };
            /** @description Invalid request parameters or range constraint violated */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorResponse"];
                };
            };
            /** @description Missing or invalid API key */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorResponse"];
                };
            };
        };
    };
    getSingleCampaignReport: {
        parameters: {
            query: {
                /** @description Inclusive start of reporting interval (RFC 3339) */
                from: string;
                /** @description Exclusive end of reporting interval (RFC 3339) */
                to: string;
                /** @description Temporal aggregation bucket */
                granularity?: "hour" | "day" | "total";
            };
            header?: never;
            path: {
                /** @description Campaign UUID */
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Performance metrics report for the single campaign (returns empty data and 0 totals if not found) */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ReportResponse"];
                };
            };
            /** @description Invalid request parameters or bad UUID in path */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorResponse"];
                };
            };
            /** @description Missing or invalid API key */
            401: {
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
