module("luci.controller.lucicodex", package.seeall)

function index()
    entry({"admin", "system", "lucicodex"}, firstchild(), _("LuciCodex"), 60).dependent = false
    entry({"admin", "system", "lucicodex", "overview"}, template("lucicodex/overview"), _("Overview"), 1)
    entry({"admin", "system", "lucicodex", "config"}, cbi("lucicodex"), _("Configuration"), 2)
    entry({"admin", "system", "lucicodex", "run"}, template("lucicodex/run"), _("Chat"), 3)
    entry({"admin", "system", "lucicodex", "plan"}, call("action_plan")).leaf = true
    entry({"admin", "system", "lucicodex", "execute"}, call("action_execute")).leaf = true
    entry({"admin", "system", "lucicodex", "validate"}, call("action_validate")).leaf = true
    entry({"admin", "system", "lucicodex", "providers"}, call("action_get_providers")).leaf = true
    entry({"admin", "system", "lucicodex", "metrics"}, call("action_metrics")).leaf = true
end

function action_plan()
    local http = require "luci.http"
    local nixio = require "nixio"
    local json = require "luci.jsonc"
    
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
    
    if #data.prompt > 4096 then
        http.status(400, "Bad Request")
        http.write_json({ error = "prompt too long (max 4096 chars)" })
        return
    end
    
    local lockfile = "/var/lock/lucicodex.lock"
    local lock = nixio.open(lockfile, "w")
    if not lock then
        lockfile = "/tmp/lucicodex.lock"
        lock = nixio.open(lockfile, "w")
    end
    
    if not lock then
        http.status(503, "Service Unavailable")
        http.write_json({ error = "execution in progress" })
        return
    end
    
    if not lock:lock("tlock") then
        lock:close()
        http.status(503, "Service Unavailable")
        http.write_json({ error = "execution in progress" })
        return
    end
    
    -- Get API keys from UCI to pass as env vars (fixes missing key issue)
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

    local gemini_key = get_key("key")
    local openai_key = get_key("openai_key")
    local anthropic_key = get_key("anthropic_key")


    local argv = {"/usr/bin/lucicodex", "-json", "-dry-run"}
    
    -- Support provider/model overrides
    if data.provider and data.provider ~= "" then
        table.insert(argv, "-provider=" .. data.provider)
    end
    if data.model and data.model ~= "" then
        table.insert(argv, "-model=" .. data.model)
    end
    
    table.insert(argv, data.prompt)
    
    local stdout_r, stdout_w = nixio.pipe()
    local stderr_r, stderr_w = nixio.pipe()
    
    local pid = nixio.fork()
    if pid == 0 then
        stdout_r:close()
        stderr_r:close()
        nixio.dup(stdout_w, nixio.stdout)
        nixio.dup(stderr_w, nixio.stderr)
        stdout_w:close()
        stderr_w:close()
        
        -- Set environment variables for the child process
        if gemini_key and gemini_key ~= "" then
            nixio.setenv("GEMINI_API_KEY", gemini_key)
        end
        if openai_key and openai_key ~= "" then
            nixio.setenv("OPENAI_API_KEY", openai_key)
        end
        if anthropic_key and anthropic_key ~= "" then
            nixio.setenv("ANTHROPIC_API_KEY", anthropic_key)
        end
        
        nixio.exec(unpack(argv))
        nixio.exit(1)
    end
    
    stdout_w:close()
    stderr_w:close()
    
    local output = ""
    local errors = ""
    
    while true do
        local chunk = stdout_r:read(1024)
        if not chunk or #chunk == 0 then break end
        output = output .. chunk
    end
    
    while true do
        local chunk = stderr_r:read(1024)
        if not chunk or #chunk == 0 then break end
        errors = errors .. chunk
    end
    
    stdout_r:close()
    stderr_r:close()
    
    local status, code = nixio.waitpid(pid)
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
    
    -- Enhanced error response with details
    local error_msg = "failed to generate plan"
    local error_details = {}
    
    if errors and errors ~= "" then
        error_details.backend_error = errors
    end
    if output and output ~= "" then
        error_details.backend_output = output
    end
    error_details.exit_code = code
    error_details.exit_status = status
    
    http.status(500, "Internal Server Error")
    http.write_json({ 
        error = error_msg,
        message = "The LLM backend failed. Check your provider selection, API key, and model name.",
        details = error_details
    })
end

function action_execute()
    local http = require "luci.http"
    local nixio = require "nixio"
    local json = require "luci.jsonc"
    
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
    
    if #data.prompt > 4096 then
        http.status(400, "Bad Request")
        http.write_json({ error = "prompt too long (max 4096 chars)" })
        return
    end
    
    local lockfile = "/var/lock/lucicodex.lock"
    local lock = nixio.open(lockfile, "w")
    if not lock then
        http.status(503, "Service Unavailable")
        http.write_json({ error = "execution in progress" })
        return
    end
    
    if not lock:lock("tlock") then
        lock:close()
        http.status(503, "Service Unavailable")
        http.write_json({ error = "execution in progress" })
        return
    end
    
    -- Get API keys from UCI
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

    local gemini_key = get_key("key")
    local openai_key = get_key("openai_key")
    local anthropic_key = get_key("anthropic_key")
    
    local argv = {"/usr/bin/lucicodex", "-json"}
    
    if data.dry_run then
        table.insert(argv, "-dry-run")
    else
        table.insert(argv, "-approve")
    end
    
    if data.timeout and tonumber(data.timeout) then
        table.insert(argv, "-timeout=" .. tostring(data.timeout))
    end
    
    -- Support provider/model overrides
    if data.provider and data.provider ~= "" then
        table.insert(argv, "-provider=" .. data.provider)
    end
    if data.model and data.model ~= "" then
        table.insert(argv, "-model=" .. data.model)
    end
    
    table.insert(argv, data.prompt)
    
    local stdout_r, stdout_w = nixio.pipe()
    local stderr_r, stderr_w = nixio.pipe()
    
    local pid = nixio.fork()
    if pid == 0 then
        stdout_r:close()
        stderr_r:close()
        nixio.dup(stdout_w, nixio.stdout)
        nixio.dup(stderr_w, nixio.stderr)
        stdout_w:close()
        stderr_w:close()
        
        -- Set environment variables
        if gemini_key and gemini_key ~= "" then
            nixio.setenv("GEMINI_API_KEY", gemini_key)
        end
        if openai_key and openai_key ~= "" then
            nixio.setenv("OPENAI_API_KEY", openai_key)
        end
        if anthropic_key and anthropic_key ~= "" then
            nixio.setenv("ANTHROPIC_API_KEY", anthropic_key)
        end
        
        nixio.exec(unpack(argv))
        nixio.exit(1)
    end
    
    stdout_w:close()
    stderr_w:close()
    
    local output = ""
    local errors = ""
    
    while true do
        local chunk = stdout_r:read(1024)
        if not chunk or #chunk == 0 then break end
        output = output .. chunk
    end
    
    while true do
        local chunk = stderr_r:read(1024)
        if not chunk or #chunk == 0 then break end
        errors = errors .. chunk
    end
    
    stdout_r:close()
    stderr_r:close()
    
    local status, code = nixio.waitpid(pid)
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
    
    -- Enhanced error response with details
    local error_msg = "execution failed"
    local error_details = {}
    
    if errors and errors ~= "" then
        error_details.backend_error = errors
    end
    if output and output ~= "" then
        error_details.backend_output = output
    end
    error_details.exit_code = code
    error_details.exit_status = status
    
    http.status(500, "Internal Server Error")
    http.write_json({ 
        error = error_msg,
        message = "Command execution failed. Check your configuration and system logs.",
        details = error_details
    })
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
    
    -- Build CLI command for validation
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

    local gemini_key = get_key("key")
    local openai_key = get_key("openai_key")
    local anthropic_key = get_key("anthropic_key")

    local argv = {"/usr/bin/lucicodex", "-json", "-dry-run"}
    
    if data.provider and data.provider ~= "" then
        table.insert(argv, "-provider=" .. data.provider)
    end
    if data.model and data.model ~= "" then
        table.insert(argv, "-model=" .. data.model)
    end
    
    -- Simple test prompt
    table.insert(argv, "test")
    
    local stdout_r, stdout_w = nixio.pipe()
    local stderr_r, stderr_w = nixio.pipe()
    
    local pid = nixio.fork()
    if pid == 0 then
        stdout_r:close()
        stderr_r:close()
        nixio.dup(stdout_w, nixio.stdout)
        nixio.dup(stderr_w, nixio.stderr)
        stdout_w:close()
        stderr_w:close()
        
        -- Set environment variables
        if gemini_key and gemini_key ~= "" then
            nixio.setenv("GEMINI_API_KEY", gemini_key)
        end
        if openai_key and openai_key ~= "" then
            nixio.setenv("OPENAI_API_KEY", openai_key)
        end
        if anthropic_key and anthropic_key ~= "" then
            nixio.setenv("ANTHROPIC_API_KEY", anthropic_key)
        end
        
        nixio.exec(unpack(argv))
        nixio.exit(1)
    end
    
    stdout_w:close()
    stderr_w:close()
    
    local output = ""
    local errors = ""
    
    while true do
        local chunk = stdout_r:read(1024)
        if not chunk or #chunk == 0 then break end
        output = output .. chunk
    end
    
    while true do
        local chunk = stderr_r:read(1024)
        if not chunk or #chunk == 0 then break end
        errors = errors .. chunk
    end
    
    stdout_r:close()
    stderr_r:close()
    
    local status, code = nixio.waitpid(pid)
    
    if status == "exited" and code == 0 then
        http.prepare_content("application/json")
        http.write_json({ valid = true, message = "API key is valid and working!" })
    else
        http.status(200)  -- Still 200, but valid=false
        http.prepare_content("application/json")
        http.write_json({ 
            valid = false,
            error = "Validation failed: " .. (errors ~= "" and errors or "Unknown error"),
            exit_code = code
        })
    end
end

function action_get_providers()
    local http = require "luci.http"
    local json = require "luci.jsonc"
    local uci = require "luci.model.uci".cursor()
    
    local configured = {}
    
    -- Helper to get from named 'main' section first, fallback to anonymous
    local function get_config(option)
        local val = uci:get("lucicodex", "main", option)
        if not val or val == "" then
            val = uci:get("lucicodex", "@settings[0]", option)
        end
        if not val or val == "" then
            val = uci:get("lucicodex", "@api[0]", option)
        end
        return val
    end
    
    local default_provider = get_config("provider") or "gemini"
    
    -- Check Gemini
    local gemini_key = get_config("key")
    if gemini_key and gemini_key ~= "" then
        table.insert(configured, "gemini")
    end
    
    -- Check OpenAI
    local openai_key = get_config("openai_key")
    if openai_key and openai_key ~= "" then
        table.insert(configured, "openai")
    end
    
    -- Check Anthropic
    local anthropic_key = get_config("anthropic_key")
    if anthropic_key and anthropic_key ~= "" then
        table.insert(configured, "anthropic")
    end
    
    http.prepare_content("application/json")
    http.write_json({
        configured = configured,
        default = default_provider,
        count = #configured
    })
end

function action_metrics()
    local http = require "luci.http"
    local json = require "luci.jsonc"
    
    local metrics = {
        total_requests = 0,
        success_rate = 0.0,
        average_duration = 0,
        top_provider = "unknown",
        top_command = "unknown"
    }
    
    local f = io.open("/tmp/lucicodex-metrics.json", "r")
    -- no legacy fallback needed
    if f then
        local content = f:read("*all")
        f:close()
        local parsed = json.parse(content)
        if parsed then
            metrics = parsed
        end
    end
    
    http.prepare_content("application/json")
    http.write_json(metrics)
end


