-- All modifications use bound parameters. Get queries lock rows inside mutations.

-- name: CreateAdvertiser :one
INSERT INTO advertisers (id, name, contact_email, status)
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: GetAdvertiser :one
SELECT * FROM advertisers WHERE id = $1 FOR UPDATE;

-- name: UpdateAdvertiser :one
UPDATE advertisers SET name = $2, contact_email = $3, status = $4, updated_at = now()
WHERE id = $1 RETURNING *;

-- name: ListAdvertisers :many
SELECT * FROM advertisers
WHERE (NOT sqlc.arg(has_cursor)::boolean OR (created_at, id) < (sqlc.arg(cursor_time)::timestamptz, sqlc.arg(cursor_id)::uuid))
ORDER BY created_at DESC, id DESC LIMIT sqlc.arg(page_limit)::int;

-- name: CreateBidder :one
INSERT INTO bidders (id, name, endpoint_url, timeout_ms, enabled)
VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: GetBidder :one
SELECT * FROM bidders WHERE id = $1 FOR UPDATE;

-- name: UpdateBidder :one
UPDATE bidders SET name = $2, endpoint_url = $3, timeout_ms = $4, enabled = $5, updated_at = now()
WHERE id = $1 RETURNING *;

-- name: ListBidders :many
SELECT * FROM bidders
WHERE (NOT sqlc.arg(has_cursor)::boolean OR (created_at, id) < (sqlc.arg(cursor_time)::timestamptz, sqlc.arg(cursor_id)::text))
ORDER BY created_at DESC, id DESC LIMIT sqlc.arg(page_limit)::int;

-- name: DeleteBidder :execrows
DELETE FROM bidders WHERE id = $1;

-- name: CreateCampaign :one
INSERT INTO campaigns (id, advertiser_id, bidder_id, name, status, daily_budget_micros, max_bid_cpm_micros, pacing_mode, start_at, end_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING *;

-- name: GetCampaign :one
SELECT * FROM campaigns WHERE id = $1 FOR UPDATE;

-- name: UpdateCampaign :one
UPDATE campaigns SET advertiser_id = $2, bidder_id = $3, name = $4, status = $5, daily_budget_micros = $6, max_bid_cpm_micros = $7, pacing_mode = $8, start_at = $9, end_at = $10, updated_at = now()
WHERE id = $1 RETURNING *;

-- name: ListCampaigns :many
SELECT * FROM campaigns
WHERE (NOT sqlc.arg(has_cursor)::boolean OR (created_at, id) < (sqlc.arg(cursor_time)::timestamptz, sqlc.arg(cursor_id)::uuid))
AND (sqlc.arg(advertiser_filter)::text = '' OR advertiser_id = NULLIF(sqlc.arg(advertiser_filter)::text, '')::uuid)
AND (sqlc.arg(status_filter)::text = '' OR status = sqlc.arg(status_filter)::text)
ORDER BY created_at DESC, id DESC LIMIT sqlc.arg(page_limit)::int;

-- name: CreateCreative :one
INSERT INTO creatives (id, campaign_id, name, format, width, height, markup, landing_url, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING *;

-- name: GetCreative :one
SELECT * FROM creatives WHERE id = $1 FOR UPDATE;

-- name: UpdateCreative :one
UPDATE creatives SET campaign_id = $2, name = $3, format = $4, width = $5, height = $6, markup = $7, landing_url = $8, status = $9, updated_at = now()
WHERE id = $1 RETURNING *;

-- name: ListCreatives :many
SELECT * FROM creatives
WHERE (NOT sqlc.arg(has_cursor)::boolean OR (created_at, id) < (sqlc.arg(cursor_time)::timestamptz, sqlc.arg(cursor_id)::uuid))
AND campaign_id = sqlc.arg(campaign_id)::uuid
ORDER BY created_at DESC, id DESC LIMIT sqlc.arg(page_limit)::int;

-- name: DeleteCreative :execrows
DELETE FROM creatives WHERE id = $1;

-- name: HasActiveCreative :one
SELECT EXISTS(SELECT 1 FROM creatives WHERE campaign_id = $1 AND status = 'active') AS present;

-- name: GetTargeting :one
SELECT * FROM targeting_rules WHERE campaign_id = $1;

-- name: PutTargeting :one
INSERT INTO targeting_rules (campaign_id,countries,device_types,placement_ids,hours_of_day,timezone)
VALUES ($1,$2,$3,$4,$5,$6)
ON CONFLICT (campaign_id) DO UPDATE SET countries=EXCLUDED.countries,
device_types=EXCLUDED.device_types,placement_ids=EXCLUDED.placement_ids,
hours_of_day=EXCLUDED.hours_of_day,timezone=EXCLUDED.timezone,updated_at=now()
RETURNING *;

-- name: CatalogVersion :one
SELECT version FROM catalog_version WHERE id = 1;

-- name: CatalogSnapshot :one
-- One statement means version and every catalog row share a PostgreSQL MVCC view.
WITH eligible AS MATERIALIZED (
  SELECT c.*, COALESCE(t.countries, '{}'::text[]) AS countries,
    COALESCE(t.device_types, '{}'::text[]) AS device_types,
    COALESCE(t.placement_ids, '{}'::text[]) AS placement_ids,
    COALESCE(t.hours_of_day, '{}'::smallint[]) AS hours_of_day,
    COALESCE(t.timezone, 'UTC') AS timezone
  FROM campaigns c
  JOIN advertisers a ON a.id = c.advertiser_id AND a.status = 'active'
  JOIN bidders b ON b.id = c.bidder_id AND b.enabled
  LEFT JOIN targeting_rules t ON t.campaign_id = c.id
  WHERE c.status = 'active'
    AND EXISTS (SELECT 1 FROM creatives cr WHERE cr.campaign_id = c.id AND cr.status = 'active')
)
SELECT cv.version,
  COALESCE((SELECT jsonb_agg(jsonb_build_object(
    'id', b.id, 'endpoint_url', b.endpoint_url, 'timeout_ms', COALESCE(b.timeout_ms, 0)) ORDER BY b.id)
    FROM bidders b WHERE EXISTS (SELECT 1 FROM eligible e WHERE e.bidder_id = b.id)), '[]'::jsonb)::jsonb AS bidders,
  COALESCE((SELECT jsonb_agg(jsonb_build_object(
    'id', e.id, 'advertiser_id', e.advertiser_id, 'bidder_id', e.bidder_id, 'name', e.name,
    'daily_budget_micros', e.daily_budget_micros, 'max_bid_cpm_micros', e.max_bid_cpm_micros,
    'pacing_mode', e.pacing_mode, 'start_at', e.start_at, 'end_at', e.end_at,
    'targeting', jsonb_build_object('countries', e.countries, 'device_types', e.device_types,
      'placement_ids', e.placement_ids, 'hours_of_day', e.hours_of_day, 'timezone', e.timezone),
    'creatives', (SELECT jsonb_agg(jsonb_build_object(
      'id', cr.id, 'format', cr.format, 'width', COALESCE(cr.width, 0), 'height', COALESCE(cr.height, 0),
      'markup', cr.markup, 'landing_url', cr.landing_url) ORDER BY cr.id)
      FROM creatives cr WHERE cr.campaign_id = e.id AND cr.status = 'active')
    ) ORDER BY e.id) FROM eligible e), '[]'::jsonb)::jsonb AS campaigns
FROM catalog_version cv WHERE cv.id = 1;
