-- All format, trim, filter, codec and FFmpeg logic belongs to this plugin.
-- The host only provides generic file/process/cache capabilities.
local formats = { mp3 = true, wav = true, ogg = true }
local operation_keys = { start = true, duration = true, format = true, volume = true }
local function bad(message)
    return { status = 400, body = message }
end

local function handle(request)
    local q = request.query
    local modified = false
    for key in pairs(operation_keys) do
        if q[key] ~= nil then modified = true end
    end
    if not modified then return { file = request.path } end

    local duration = tonumber(q.duration)
    if not duration or duration <= 0 or duration > 60 then
        return bad("Transformations require duration in seconds (>0 and <=60)")
    end
    local start = q.start and tonumber(q.start) or 0
    local volume = q.volume and tonumber(q.volume) or 1
    if q.start and not tonumber(q.start) then return bad("start must be numeric") end
    if q.volume and not tonumber(q.volume) then return bad("volume must be numeric") end
    if start < 0 or start > 86400 then return bad("start must be 0..86400 seconds") end
    if volume < 0 or volume > 2 then return bad("volume must be 0..2") end
    local extension = request.path:match("%.([^.]+)$")
    local format = q.format or (extension and formats[extension:lower()] and extension:lower()) or "mp3"
    if not formats[format] then return bad("Unsupported audio output format") end
    local options = { format = format, start = start, duration = duration, volume = volume }

    local input_formats = { mp3="mp3", wav="wav", ogg="ogg", flac="flac", m4a="mov" }
    local demuxer = extension and input_formats[extension:lower()]
    if not demuxer then return bad("Unsupported input format") end
    local args = {
        "-nostdin", "-hide_banner", "-loglevel", "error", "-y",
        "-max_alloc", "67108864", "-threads", "1", "-filter_threads", "1",
        "-protocol_whitelist", "file,pipe", "-f", demuxer,
        "-ss", tostring(start), "-i", "{input}", "-t", tostring(duration),
        "-map_metadata", "-1", "-map_chapters", "-1",
    }
    local function append(...)
        for _, arg in ipairs({...}) do table.insert(args, arg) end
    end
    append("-map", "0:a:0", "-vn", "-af", "volume=" .. tostring(volume), "-ar", "48000")
    if format == "mp3" then
        append("-c:a", "libmp3lame", "-b:a", "128k")
    elseif format == "ogg" then
        append("-c:a", "libvorbis", "-q:a", "3")
    else
        append("-c:a", "pcm_s16le")
    end
    append("-threads", "1", "-fs", "33554432", "{output}")
    return { file = request.path, process = { tool = "ffmpeg", args = args, extension = format } }
end

return {
    api_version = 1,
    tools = { "ffmpeg" },
    routes = {
        { path = "/a/", directory = "assets/audio", handle = handle },
    },
}
