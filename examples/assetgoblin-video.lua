-- All format, trim, filter, codec and FFmpeg logic belongs to this plugin.
-- The host only provides generic file/process/cache capabilities.
local formats = { mp4 = true, webm = true }
local operation_keys = { start = true, duration = true, format = true, volume = true, width = true, height = true, crop = true }
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
    local format = q.format or (extension and formats[extension:lower()] and extension:lower()) or "mp4"
    if not formats[format] then return bad("Unsupported video output format") end
    local options = { format = format, start = start, duration = duration, volume = volume }

    for _, key in ipairs({"width", "height"}) do
        if q[key] then
            local n = tonumber(q[key])
            local maximum = key == "width" and 1280 or 720
            if not n or n < 0 or n > maximum or n % 2 ~= 0 then
                return bad(key .. " must be even and within 1280x720")
            end
            options[key] = n
        end
    end
    if q.crop then
        local w, h, x, y = q.crop:match("^(%d+):(%d+):(%d+):(%d+)$")
        w, h, x, y = tonumber(w), tonumber(h), tonumber(x), tonumber(y)
        if not w or w <= 0 or h <= 0 or w > 4096 or h > 4096 or x > 4096 or y > 4096 then
            return bad("crop must be width:height:x:y (positive dimensions, maximum 4096)")
        end
        options.crop_width, options.crop_height = w, h
        options.crop_x, options.crop_y = x, y
    end

    local input_formats = { mp4="mov", mov="mov", webm="matroska", mkv="matroska" }
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
    append("-map", "0:v:0", "-map", "0:a:0?", "-sn", "-dn")
    local filters = {}
    if options.crop_width then
        table.insert(filters, string.format("crop=%d:%d:%d:%d", options.crop_width,
            options.crop_height, options.crop_x, options.crop_y))
    end
    local width = options.width and options.width > 0 and options.width or 1280
    local height = options.height and options.height > 0 and options.height or 720
    table.insert(filters, string.format("scale=w='min(iw,%d)':h='min(ih,%d)':force_original_aspect_ratio=decrease:force_divisible_by=2", width, height))
    table.insert(filters, "setsar=1")
    append("-vf", table.concat(filters, ","), "-af", "volume=" .. tostring(volume),
        "-pix_fmt", "yuv420p", "-r", "30")
    if format == "mp4" then
        append("-c:v", "libx264", "-preset", "ultrafast", "-crf", "28",
            "-c:a", "aac", "-b:a", "128k", "-movflags", "+faststart")
    else
        append("-c:v", "libvpx", "-deadline", "realtime", "-cpu-used", "8",
            "-b:v", "1M", "-c:a", "libvorbis", "-q:a", "3")
    end
    append("-threads", "1", "-fs", "33554432", "{output}")
    return { file = request.path, process = { tool = "ffmpeg", args = args, extension = format } }
end

return {
    api_version = 1,
    tools = { "ffmpeg" },
    routes = {
        { path = "/v/", directory = "assets/video", handle = handle },
    },
}
