-- KEYS[1] = 失败计数键
-- ARGV[1] = window_sec（窗口秒数）
-- ARGV[2] = max_fail（允许失败次数阈值）
local n = redis.call('INCR', KEYS[1])
if n == 1 then
  redis.call('EXPIRE', KEYS[1], tonumber(ARGV[1]))
end
if n > tonumber(ARGV[2]) then
  return 1
end
return 0
