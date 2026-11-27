-- ADVERTISERS ---------------------------------------------------------------
CREATE TABLE advertisers (
  id            UUID PRIMARY KEY,
  name          TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 200),
  contact_email TEXT,
  status        TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','archived')),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX advertisers_active_name_uq ON advertisers (lower(name)) WHERE status = 'active';

-- BIDDERS (demand partners / mock DSPs) -------------------------------------
CREATE TABLE bidders (
  id           TEXT PRIMARY KEY CHECK (id ~ '^[a-z0-9][a-z0-9-]{1,62}$'),
  name         TEXT NOT NULL,
  endpoint_url TEXT NOT NULL,
  timeout_ms   INT  CHECK (timeout_ms BETWEEN 10 AND 1000),   -- NULL = gateway default
  enabled      BOOLEAN NOT NULL DEFAULT TRUE,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- CAMPAIGNS -----------------------------------------------------------------
CREATE TABLE campaigns (
  id                  UUID PRIMARY KEY,
  advertiser_id       UUID NOT NULL REFERENCES advertisers(id),
  bidder_id           TEXT NOT NULL REFERENCES bidders(id),
  name                TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 200),
  status              TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','active','paused','archived')),
  daily_budget_micros BIGINT NOT NULL CHECK (daily_budget_micros > 0),
  max_bid_cpm_micros  BIGINT NOT NULL CHECK (max_bid_cpm_micros > 0),
  pacing_mode         TEXT NOT NULL DEFAULT 'even' CHECK (pacing_mode IN ('asap','even')),
  start_at            TIMESTAMPTZ,
  end_at              TIMESTAMPTZ,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (start_at IS NULL OR end_at IS NULL OR end_at > start_at)
);
CREATE INDEX campaigns_advertiser_idx ON campaigns (advertiser_id, created_at DESC, id DESC);
CREATE INDEX campaigns_active_idx     ON campaigns (id) WHERE status = 'active';

-- CREATIVES -----------------------------------------------------------------
CREATE TABLE creatives (
  id          UUID PRIMARY KEY,
  campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
  name        TEXT NOT NULL,
  format      TEXT NOT NULL CHECK (format IN ('banner','native','video')),
  width       INT  CHECK (width  > 0),
  height      INT  CHECK (height > 0),
  markup      TEXT NOT NULL,                 -- HTML/VAST/native snippet returned as "adm"
  landing_url TEXT NOT NULL,
  status      TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','paused')),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX creatives_campaign_idx ON creatives (campaign_id);

-- TARGETING (one row per campaign; empty array = "no restriction") -----------
CREATE TABLE targeting_rules (
  campaign_id   UUID PRIMARY KEY REFERENCES campaigns(id) ON DELETE CASCADE,
  countries     TEXT[]     NOT NULL DEFAULT '{}',   -- ISO-3166 alpha-2, uppercase
  device_types  TEXT[]     NOT NULL DEFAULT '{}',   -- mobile|desktop|tablet|ctv
  placement_ids TEXT[]     NOT NULL DEFAULT '{}',
  hours_of_day  SMALLINT[] NOT NULL DEFAULT '{}',   -- 0..23 in `timezone`
  timezone      TEXT       NOT NULL DEFAULT 'UTC',  -- IANA name, validated with time.LoadLocation
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- CATALOG VERSION (bumped by triggers; gateway polls this via snapshot) -------
CREATE TABLE catalog_version (
  id      SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
  version BIGINT NOT NULL DEFAULT 1
);
INSERT INTO catalog_version DEFAULT VALUES;

CREATE FUNCTION bump_catalog_version() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN UPDATE catalog_version SET version = version + 1 WHERE id = 1; RETURN NULL; END $$;

CREATE TRIGGER trg_bump_advertisers AFTER INSERT OR UPDATE OR DELETE ON advertisers      FOR EACH STATEMENT EXECUTE FUNCTION bump_catalog_version();
CREATE TRIGGER trg_bump_bidders     AFTER INSERT OR UPDATE OR DELETE ON bidders          FOR EACH STATEMENT EXECUTE FUNCTION bump_catalog_version();
CREATE TRIGGER trg_bump_campaigns   AFTER INSERT OR UPDATE OR DELETE ON campaigns        FOR EACH STATEMENT EXECUTE FUNCTION bump_catalog_version();
CREATE TRIGGER trg_bump_creatives   AFTER INSERT OR UPDATE OR DELETE ON creatives        FOR EACH STATEMENT EXECUTE FUNCTION bump_catalog_version();
CREATE TRIGGER trg_bump_targeting   AFTER INSERT OR UPDATE OR DELETE ON targeting_rules  FOR EACH STATEMENT EXECUTE FUNCTION bump_catalog_version();

-- REPORTING ROLLUPS (written only by event-aggregator) -----------------------
CREATE TABLE hourly_stats (
  campaign_id  UUID        NOT NULL,           -- no FK: events may outlive/precede catalog edits
  hour_start   TIMESTAMPTZ NOT NULL,           -- truncated to the hour, UTC, from event time
  impressions  BIGINT NOT NULL DEFAULT 0,
  clicks       BIGINT NOT NULL DEFAULT 0,
  spend_micros BIGINT NOT NULL DEFAULT 0,
  bids         BIGINT NOT NULL DEFAULT 0,      -- valid bids (>= floor) submitted in auctions
  wins         BIGINT NOT NULL DEFAULT 0,
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (campaign_id, hour_start)
);
CREATE INDEX hourly_stats_hour_idx ON hourly_stats (hour_start);

-- DEDUPLICATION LEDGER --------------------------------------------------------
CREATE TABLE processed_events (
  event_id     UUID PRIMARY KEY,
  event_type   TEXT NOT NULL,
  processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX processed_events_processed_at_idx ON processed_events (processed_at);
