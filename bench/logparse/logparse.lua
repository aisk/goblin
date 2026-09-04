-- Log line parsing: string building, trimming, splitting, prefix tests and
-- integer conversion, with the results counted in a table.
local lines = {}
local i = 0
while i < 4000 do
    lines[#lines + 1] = "  level=info user=u" .. tostring(i % 97) .. " ms=" .. tostring(i % 31) .. "  "
    i = i + 1
end

local function trim(s)
    return string.match(s, "^%s*(.-)%s*$")
end

local function split(s, sep)
    local parts = {}
    local start = 1
    while true do
        local at = string.find(s, sep, start, true)
        if at == nil then
            parts[#parts + 1] = string.sub(s, start)
            return parts
        end
        parts[#parts + 1] = string.sub(s, start, at - 1)
        start = at + #sep
    end
end

local function has_prefix(s, prefix)
    return string.sub(s, 1, #prefix) == prefix
end

local users = {}
local total = 0
local elapsed = 0
local round = 0
while round < 280 do
    local j = 1
    while j <= #lines do
        local fields = split(trim(lines[j]), " ")
        local k = 1
        while k <= #fields do
            local field = fields[k]
            if has_prefix(field, "user=") then
                local user = string.sub(field, #"user=" + 1)
                if users[user] == nil then
                    users[user] = 1
                    total = total + 1
                else
                    users[user] = users[user] + 1
                end
            end
            if has_prefix(field, "ms=") then
                elapsed = elapsed + tonumber(string.sub(field, #"ms=" + 1))
            end
            k = k + 1
        end
        j = j + 1
    end
    round = round + 1
end
print(total .. " " .. elapsed)
