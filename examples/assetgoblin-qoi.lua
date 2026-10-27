-- Add QOI input/output to the core image route; all resizing remains in core.
local function plan(codec, extension)
    return {
        tool = "ffmpeg", extension = extension,
        args = { "-nostdin", "-hide_banner", "-loglevel", "error", "-y",
            "-threads", "1", "-protocol_whitelist", "file,pipe", "-i", "{input}",
            "-frames:v", "1", "-c:v", codec, "-threads", "1", "{output}" },
    }
end
return {
    api_version = 1,
    tools = { "ffmpeg" },
    image_codecs = {
        qoi = {
            mime_type = "image/qoi",
            decode = function(request) return plan("png", "png") end,
            encode = function(request) return plan("qoi", "qoi") end,
        },
    },
}
