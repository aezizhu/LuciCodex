module("luci.controller.lucicodex", package.seeall)

function index()
    entry({"admin", "system", "lucicodex"}, firstchild(), _("LuciCodex"), 60).dependent = false
    entry({"admin", "system", "lucicodex", "overview"}, template("lucicodex/home"), _("Overview"), 1)
    entry({"admin", "system", "lucicodex", "config"}, cbi("lucicodex"), _("Configuration"), 2)
    entry({"admin", "system", "lucicodex", "run"}, template("lucicodex/run"), _("Chat"), 3)
    entry({"admin", "system", "lucicodex", "plan"}, call("action_plan")).leaf = true
    entry({"admin", "system", "lucicodex", "execute"}, call("action_execute")).leaf = true
    entry({"admin", "system", "lucicodex", "validate"}, call("action_validate")).leaf = true
    entry({"admin", "system", "lucicodex", "providers"}, call("action_get_providers")).leaf = true
    entry({"admin", "system", "lucicodex", "metrics"}, call("action_metrics")).leaf = true
end

-- Helper to get API keys from UCI
local function get_api_keys()
    local uci = require "luci.model.uci".cursor()
    local function get_key(option)
        local val = uci:get("lucicodex", "main", option)
        if not val or val == "" then
            val = uci:get("lucicodex", "@settings[0]", option)
        end
        if not val or val == "" then
            val = uci:get("lucicodex", "@api[0]", option)
        end
        return val
    end

    return {
        gemini = get_key("key"),
        openai = get_key("openai_key"),
        anthropic = get_key("anthropic_key")
    }
end

-- Helper to call local daemon
local function call_daemon(endpoint, payload)
    local json = require "luci.jsonc"
    local os = require "os"
    local io = require "io"
    
    -- Prepare JSON body
    local body = json.stringify(payload)
    
    -- Write body to temp file to avoid shell escaping issues
    local tmpfile = os.tmpname()
    local f = io.open(tmpfile, "w")
    if not f then return nil, "failed to create temp file" end
    f:write(body)
    f:close()
    
    -- Use curl to talk to daemon (timeout 300s)
    -- Capture stderr to debug connection issues
    local cmd = string.format("curl -v -s -m 300 -X POST -H 'Content-Type: application/json' --data-binary @%s http://127.0.0.1:9999%s 2>/tmp/lucicodex_curl.err", tmpfile, endpoint)
    local handle = io.popen(cmd)
    local result = handle:read("*a")
    handle:close()
    os.remove(tmpfile)
    
    if not result or result == "" then
        local f = io.open("/tmp/lucicodex_curl.err", "r")
        local err_msg = f and f:read("*a") or "unknown error"
        if f then f:close() end
        return nil, "daemon unreachable: " .. err_msg
    end
    
    local decoded = json.parse(result)
    if not decoded then
        return nil, "invalid json from daemon"
    end
    
    return decoded, nil
end

function action_plan()
    local http = require "luci.http"
    local json = require "luci.jsonc"
    local nixio = require "nixio"
    
    if http.getenv("REQUEST_METHOD") ~= "POST" then
        http.status(405, "Method Not Allowed")
        http.write_json({ error = "POST required" })
        return
    end
    
    local body = http.content()
    local data = json.parse(body)
    
    if not data or not data.prompt or data.prompt == "" then
        http.status(400, "Bad Request")
        http.write_json({ error = "missing prompt" })
        return
    end
    
    -- Try Daemon First (Maximum Speed)
    local keys = get_api_keys()
    local payload = {
        prompt = data.prompt,
        provider = data.provider,
        model = data.model,
        config = {
            gemini_key = keys.gemini,
            openai_key = keys.openai,
            anthropic_key = keys.anthropic
        }
    }
    
    local resp, err = call_daemon("/v1/plan", payload)
    if resp and resp.ok then
        http.prepare_content("application/json")
        http.write_json(resp)
        return
    end
    
    -- Fallback to CLI if daemon fails
    -- (Previous CLI logic here)
    local lockfile = "/var/lock/lucicodex.lock"
    local lock = nixio.open(lockfile, "w")
    if not lock then
        lockfile = "/tmp/lucicodex.lock"
        lock = nixio.open(lockfile, "w")
    end
    
    if not lock or not lock:lock("tlock") then
        if lock then lock:close() end
        http.status(503, "Service Unavailable")
        http.write_json({ error = "execution in progress" })
        return
    end
    
    local argv = {"/usr/bin/lucicodex", "-json", "-dry-run"}
    if data.provider and data.provider ~= "" then table.insert(argv, "-provider=" .. data.provider) end
    if data.model and data.model ~= "" then table.insert(argv, "-model=" .. data.model) end
    table.insert(argv, data.prompt)
    
    local stdout_r, stdout_w = nixio.pipe()
    local stderr_r, stderr_w = nixio.pipe()
    local pid = nixio.fork()
    
    if pid == 0 then
        stdout_r:close(); stderr_r:close()
        nixio.dup(stdout_w, nixio.stdout); nixio.dup(stderr_w, nixio.stderr)
        stdout_w:close(); stderr_w:close()
        
        if keys.gemini then nixio.setenv("GEMINI_API_KEY", keys.gemini) end
        if keys.openai then nixio.setenv("OPENAI_API_KEY", keys.openai) end
        if keys.anthropic then nixio.setenv("ANTHROPIC_API_KEY", keys.anthropic) end
        
        nixio.exec(unpack(argv))
        nixio.exit(1)
    end
    
    stdout_w:close(); stderr_w:close()
    local output = stdout_r:read("*a") or ""
    local errors = stderr_r:read("*a") or ""
    stdout_r:close(); stderr_r:close()
    
    local _, status, code = nixio.waitpid(pid)
    lock:close()
    nixio.fs.unlink(lockfile)
    
    if status == "exited" and code == 0 then
        local plan = json.parse(output)
        if plan then
            http.prepare_content("application/json")
            http.write_json({ ok = true, plan = plan })
            return
        end
    end
    
    http.status(500, "Internal Server Error")
    http.write_json({ error = "failed to generate plan", details = { backend_error = errors, backend_output = output } })
end

function action_execute()
    local http = require "luci.http"
    local json = require "luci.jsonc"
    local nixio = require "nixio"
    
    if http.getenv("REQUEST_METHOD") ~= "POST" then
        http.status(405, "Method Not Allowed")
        http.write_json({ error = "POST required" })
        return
    end
    
    local body = http.content()
    local data = json.parse(body)
    
    if not data or not data.prompt or data.prompt == "" then
        http.status(400, "Bad Request")
        http.write_json({ error = "missing prompt" })
        return
    end
    
    -- Try Daemon First
    local keys = get_api_keys()
    local payload = {
        prompt = data.prompt,
        provider = data.provider,
        model = data.model,
        dry_run = data.dry_run,
        timeout = tonumber(data.timeout),
        config = {
            gemini_key = keys.gemini,
            openai_key = keys.openai,
            anthropic_key = keys.anthropic
        }
    }
    
    local resp, err = call_daemon("/v1/execute", payload)
    if resp and resp.ok then
        http.prepare_content("application/json")
        http.write_json(resp)
        return
    end
    
    -- Fallback to CLI
    local lockfile = "/var/lock/lucicodex.lock"
    local lock = nixio.open(lockfile, "w")
    if not lock then
        lockfile = "/tmp/lucicodex.lock"
        lock = nixio.open(lockfile, "w")
    end
    
    if not lock or not lock:lock("tlock") then
        if lock then lock:close() end
        http.status(503, "Service Unavailable")
        http.write_json({ error = "execution in progress" })
        return
    end
    
    local argv = {"/usr/bin/lucicodex", "-json"}
    if data.dry_run then table.insert(argv, "-dry-run") else table.insert(argv, "-approve") end
    if data.timeout and tonumber(data.timeout) then table.insert(argv, "-timeout=" .. tostring(data.timeout)) end
    if data.provider and data.provider ~= "" then table.insert(argv, "-provider=" .. data.provider) end
    if data.model and data.model ~= "" then table.insert(argv, "-model=" .. data.model) end
    table.insert(argv, data.prompt)
    
    local stdout_r, stdout_w = nixio.pipe()
    local stderr_r, stderr_w = nixio.pipe()
    local pid = nixio.fork()
    
    if pid == 0 then
        stdout_r:close(); stderr_r:close()
        nixio.dup(stdout_w, nixio.stdout); nixio.dup(stderr_w, nixio.stderr)
        stdout_w:close(); stderr_w:close()
        
        if keys.gemini then nixio.setenv("GEMINI_API_KEY", keys.gemini) end
        if keys.openai then nixio.setenv("OPENAI_API_KEY", keys.openai) end
        if keys.anthropic then nixio.setenv("ANTHROPIC_API_KEY", keys.anthropic) end
        
        nixio.exec(unpack(argv))
        nixio.exit(1)
    end
    
    stdout_w:close(); stderr_w:close()
    local output = stdout_r:read("*a") or ""
    local errors = stderr_r:read("*a") or ""
    stdout_r:close(); stderr_r:close()
    
    local _, status, code = nixio.waitpid(pid)
    lock:close()
    nixio.fs.unlink(lockfile)
    
    if status == "exited" and code == 0 then
        local result = json.parse(output)
        if result then
            http.prepare_content("application/json")
            http.write_json({ ok = true, result = result })
            return
        end
        http.prepare_content("application/json")
        http.write_json({ ok = true, output = output })
        return
    end
    
    http.status(500, "Internal Server Error")
    http.write_json({ error = "execution failed", details = { backend_error = errors, backend_output = output } })
end

function action_validate()
    local http = require "luci.http"
    local json = require "luci.jsonc"
    local nixio = require "nixio"
    
    if http.getenv("REQUEST_METHOD") ~= "POST" then
        http.status(405, "Method Not Allowed")
        http.write_json({ error = "POST required" })
        return
    end
    
    local body = http.content()
    local data = json.parse(body)
    
    if not data or not data.provider then
        http.status(400, "Bad Request")
        http.write_json({ error = "missing provider" })
        return
    end
    
    local keys = get_api_keys()
    local argv = {"/usr/bin/lucicodex", "-json", "-dry-run"}
    if data.provider and data.provider ~= "" then table.insert(argv, "-provider=" .. data.provider) end
    if data.model and data.model ~= "" then table.insert(argv, "-model=" .. data.model) end
    table.insert(argv, "test")
    
    local stdout_r, stdout_w = nixio.pipe()
    local stderr_r, stderr_w = nixio.pipe()
    local pid = nixio.fork()
    
    if pid == 0 then
        stdout_r:close(); stderr_r:close()
        nixio.dup(stdout_w, nixio.stdout); nixio.dup(stderr_w, nixio.stderr)
        stdout_w:close(); stderr_w:close()
        
        if keys.gemini then nixio.setenv("GEMINI_API_KEY", keys.gemini) end
        if keys.openai then nixio.setenv("OPENAI_API_KEY", keys.openai) end
        if keys.anthropic then nixio.setenv("ANTHROPIC_API_KEY", keys.anthropic) end
        
        nixio.exec(unpack(argv))
        nixio.exit(1)
    end
    
    stdout_w:close(); stderr_w:close()
    local output = stdout_r:read("*a") or ""
    local errors = stderr_r:read("*a") or ""
    stdout_r:close(); stderr_r:close()
    
    local _, status, code = nixio.waitpid(pid)
    
    if status == "exited" and code == 0 then
        http.prepare_content("application/json")
        http.write_json({ valid = true, message = "API key is valid and working!" })
    else
        http.status(200)
        http.prepare_content("application/json")
        http.write_json({ valid = false, error = "Validation failed: " .. (errors ~= "" and errors or "Unknown error"), exit_code = code })
    end
end

function action_get_providers()
    local http = require "luci.http"
    local json = require "luci.jsonc"
    local keys = get_api_keys()
    
    local configured = {}
    if keys.gemini and keys.gemini ~= "" then table.insert(configured, "gemini") end
    if keys.openai and keys.openai ~= "" then table.insert(configured, "openai") end
    if keys.anthropic and keys.anthropic ~= "" then table.insert(configured, "anthropic") end
    
    local uci = require "luci.model.uci".cursor()
    local default_provider = uci:get("lucicodex", "main", "provider") or "gemini"
    
    http.prepare_content("application/json")
    http.write_json({ configured = configured, default = default_provider, count = #configured })
end

function action_metrics()
    local http = require "luci.http"
    local json = require "luci.jsonc"
    local io = require "io"
    
    local metrics = { total_requests = 0, success_rate = 0.0, average_duration = 0, top_provider = "unknown", top_command = "unknown" }
    local f = io.open("/tmp/lucicodex-metrics.json", "r")
    if f then
        local content = f:read("*all")
        f:close()
        local parsed = json.parse(content)
        if parsed then metrics = parsed end
    end
    
    http.prepare_content("application/json")
    http.write_json(metrics)
end
