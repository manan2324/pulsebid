-- KEYS[1] = spend key
-- ARGV[1] = cost_micros
-- compensation only (DECRBY, clamp at 0 via Lua)
local spent = tonumber(redis.call('GET', KEYS[1]) or '0')
local refund = tonumber(ARGV[1])
if spent <= 0 then
    return 0
end
if spent <= refund then
    redis.call('DECRBY', KEYS[1], spent)
    return 0
else
    return redis.call('DECRBY', KEYS[1], refund)
end
