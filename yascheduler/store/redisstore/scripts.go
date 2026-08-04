package redisstore

import "github.com/redis/go-redis/v9"

var upsertJobScript = redis.NewScript(`
local existing = redis.call('HGET', KEYS[1], ARGV[1])
if existing then
	if existing ~= ARGV[7] then
		return {3}
	end
	local version = redis.call('HINCRBY', KEYS[4], 'version', 1)
	redis.call('HSET', KEYS[4], 'blob', ARGV[2], 'enabled', ARGV[3], 'updated_at', ARGV[4])
	if ARGV[3] == '1' then
		redis.call('SADD', KEYS[2], existing)
	else
		redis.call('SREM', KEYS[2], existing)
	end
	local created = redis.call('HGET', KEYS[4], 'created_at')
	local skipped = redis.call('HGET', KEYS[4], 'skipped')
	return {1, existing, version, created, skipped}
end
if ARGV[7] ~= '' then
	return {3}
end
if ARGV[5] == '1' then
	return {0}
end
redis.call(
	'HSET', KEYS[3],
	'blob', ARGV[2],
	'version', 1,
	'skipped', 0,
	'enabled', ARGV[3],
	'created_at', ARGV[4],
	'updated_at', ARGV[4],
	'keyfield', ARGV[1]
)
redis.call('HSET', KEYS[1], ARGV[1], ARGV[6])
if ARGV[3] == '1' then
	redis.call('SADD', KEYS[2], ARGV[6])
end
return {2}
`)

var deleteJobScript = redis.NewScript(`
local keyField = redis.call('HGET', KEYS[1], 'keyfield')
if not keyField then
	return 0
end
redis.call('DEL', KEYS[1])
redis.call('HDEL', KEYS[2], keyField)
redis.call('SREM', KEYS[3], ARGV[1])
return 1
`)

var setJobEnabledScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then
	return 0
end
redis.call('HINCRBY', KEYS[1], 'version', 1)
redis.call('HSET', KEYS[1], 'enabled', ARGV[1], 'updated_at', ARGV[2])
if ARGV[1] == '1' then
	redis.call('SADD', KEYS[2], ARGV[3])
else
	redis.call('SREM', KEYS[2], ARGV[3])
end
return 1
`)

var addSkippedOccurrencesScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then
	return 0
end
redis.call('HINCRBY', KEYS[1], 'skipped', ARGV[1])
redis.call('HSET', KEYS[1], 'updated_at', ARGV[2])
return 1
`)

var createExecutionScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then
	return {0}
end
local existing = redis.call('HGET', KEYS[2], ARGV[1])
if existing then
	return {1, existing}
end
local id = ARGV[4]
redis.call(
	'HSET', KEYS[3],
	'blob', ARGV[2],
	'version', 1,
	'created_at', ARGV[3],
	'updated_at', ARGV[3]
)
redis.call('HSET', KEYS[2], ARGV[1], id)
redis.call('SADD', KEYS[6], id)
redis.call('SADD', KEYS[7], id)
if ARGV[5] == '1' then
	redis.call('ZADD', KEYS[4], ARGV[6], id)
end
if ARGV[7] == '1' then
	redis.call('ZADD', KEYS[5], ARGV[8], id)
end
if ARGV[9] == '1' then
	redis.call('SADD', KEYS[8], id)
end
if ARGV[10] == '1' then
	redis.call('SADD', KEYS[9], id)
end
return {2, id}
`)

var updateExecutionScript = redis.NewScript(`
local version = redis.call('HGET', KEYS[1], 'version')
if not version then
	return 0
end
if version ~= ARGV[1] then
	return 1
end
redis.call('HSET', KEYS[1], 'blob', ARGV[2], 'updated_at', ARGV[3])
redis.call('HINCRBY', KEYS[1], 'version', 1)
if KEYS[2] ~= KEYS[3] then
	redis.call('SREM', KEYS[2], ARGV[10])
	redis.call('SADD', KEYS[3], ARGV[10])
end
if ARGV[4] == '1' then
	redis.call('ZADD', KEYS[4], ARGV[5], ARGV[10])
else
	redis.call('ZREM', KEYS[4], ARGV[10])
end
if ARGV[6] == '1' then
	redis.call('ZADD', KEYS[5], ARGV[7], ARGV[10])
else
	redis.call('ZREM', KEYS[5], ARGV[10])
end
if ARGV[8] == '1' then
	redis.call('SADD', KEYS[6], ARGV[10])
else
	redis.call('SREM', KEYS[6], ARGV[10])
end
if ARGV[9] == '1' then
	redis.call('SADD', KEYS[7], ARGV[10])
else
	redis.call('SREM', KEYS[7], ARGV[10])
end
return 2
`)

var createAttemptScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then
	return {0}
end
local id = ARGV[3]
redis.call(
	'HSET', KEYS[2],
	'blob', ARGV[1],
	'state', ARGV[4],
	'error', '',
	'created_at', ARGV[2],
	'updated_at', ARGV[2]
)
redis.call('SADD', KEYS[3], id)
redis.call('SADD', KEYS[4], id)
return {1, id}
`)

var updateAttemptStateScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then
	return 0
end
local fromCount = tonumber(ARGV[5])
if fromCount > 0 then
	local state = redis.call('HGET', KEYS[1], 'state')
	local matched = false
	for index = 6, 5 + fromCount do
		if state == ARGV[index] then
			matched = true
			break
		end
	end
	if not matched then
		return 1
	end
end
redis.call('HSET', KEYS[1], 'state', ARGV[1], 'updated_at', ARGV[4])
if ARGV[3] == '1' then
	redis.call('HSET', KEYS[1], 'error', ARGV[2])
end
return 2
`)

var storeResultScript = redis.NewScript(`
local currentList = redis.call('HGET', KEYS[1], 'instkey')
if not currentList then
	if ARGV[8] ~= '' then
		return 3
	end
	if redis.call('ZCARD', KEYS[2]) >= tonumber(ARGV[5]) then
		return 0
	end
	if redis.call('LLEN', KEYS[3]) >= tonumber(ARGV[6]) then
		return 0
	end
	redis.call(
		'HSET', KEYS[1],
		'blob', ARGV[1],
		'instkey', ARGV[2],
		'attempts', 0,
		'created_at', ARGV[3]
	)
	redis.call('ZADD', KEYS[2], ARGV[4], ARGV[7])
	redis.call('RPUSH', KEYS[3], ARGV[7])
	return 1
end
if currentList ~= ARGV[8] then
	return 3
end
if currentList ~= ARGV[2] then
	if redis.call('LLEN', KEYS[3]) >= tonumber(ARGV[6]) then
		return 0
	end
	redis.call('LREM', KEYS[4], 1, ARGV[7])
	redis.call('RPUSH', KEYS[3], ARGV[7])
end
redis.call('HSET', KEYS[1], 'blob', ARGV[1], 'instkey', ARGV[2])
return 1
`)

var markResultSentScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then
	return 0
end
redis.call('HINCRBY', KEYS[1], 'attempts', 1)
redis.call('HSET', KEYS[1], 'last_sent_at', ARGV[1])
return 1
`)

var deleteResultScript = redis.NewScript(`
local currentList = redis.call('HGET', KEYS[1], 'instkey')
if not currentList then
	return 0
end
if currentList ~= ARGV[2] then
	return 3
end
redis.call('DEL', KEYS[1])
redis.call('ZREM', KEYS[2], ARGV[1])
redis.call('LREM', KEYS[3], 1, ARGV[1])
return 1
`)
