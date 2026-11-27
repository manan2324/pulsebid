-- KEYS[1] = spend key
-- ARGV[1] = cost_micros, ARGV[2] = hard_cap_micros (daily budget),
-- ARGV[3] = pace_cap_micros (<= hard cap), ARGV[4] = ttl_seconds
-- returns: new_spent (>= 0)  |  -1 budget exhausted  |  -2 paced out (even pacing)
local spent = tonumber(redis.call('GET', KEYS[1]) or '0')
local cost  = tonumber(ARGV[1])
if spent + cost > tonumber(ARGV[2]) then return -1 end
if spent + cost > tonumber(ARGV[3]) then return -2 end
local new = redis.call('INCRBY', KEYS[1], cost)
if redis.call('TTL', KEYS[1]) < 0 then redis.call('EXPIRE', KEYS[1], tonumber(ARGV[4])) end
return new
