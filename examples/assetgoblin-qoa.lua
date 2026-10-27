-- Reuse the audio plugin's validation, trimming, volume and FFmpeg logic.
return {
    api_version = 1,
    extends = "audio",
    extend = function(parent)
        table.insert(parent.tools, "qoaconv")
        local route = parent.routes[1]
        local original = route.handle
        route.handle = function(request)
            local input_qoa = request.path:lower():match("%.qoa$") ~= nil
            local output_qoa = request.query.format == "qoa"
            if not input_qoa and not output_qoa then return original(request) end
            local modified = false
            for _, key in ipairs({ "start", "duration", "format", "volume" }) do
                if request.query[key] ~= nil then modified = true end
            end
            if not modified then return { file = request.path, headers = { ["Content-Type"] = "audio/qoa" } } end
            local query = {}
            for key, value in pairs(request.query) do query[key] = value end
            if output_qoa then query.format = "wav" end
            local adapted = {
                path = input_qoa and request.path:gsub("%.[^.]+$", ".wav") or request.path,
                query = query, method = request.method, headers = request.headers,
                full_path = request.full_path,
            }
            local result = original(adapted)
            if not result.process then return result end
            local stages = {}
            local function convert(extension)
                table.insert(stages, { tool = "qoaconv", args = { "{input}", "{output}" }, extension = extension })
            end
            if input_qoa then convert("wav") end
            -- qoaconv expects standard PCM WAV without metadata extensions.
            if output_qoa then
                table.insert(result.process.args, 1, "-fflags")
                table.insert(result.process.args, 2, "+bitexact")
            end
            table.insert(stages, result.process)
            if output_qoa then convert("qoa") end
            result.file = request.path
            result.process = nil
            result.processes = stages
            if output_qoa then result.headers = { ["Content-Type"] = "audio/qoa" } end
            return result
        end
        return parent
    end,
}
