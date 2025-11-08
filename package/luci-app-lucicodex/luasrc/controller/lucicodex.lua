module("luci.controller.lucicodex", package.seeall)

function index()
    entry({"admin", "system", "lucicodex"}, firstchild(), _("LuciCodex"), 60).dependent = false
    entry({"admin", "system", "lucicodex", "overview"}, template("lucicodex/overview"), _("Overview"), 1)
    entry({"admin", "system", "lucicodex", "config"}, template("lucicodex/config"), _("Configuration"), 2)
    entry({"admin", "system", "lucicodex", "run"}, template("lucicodex/run"), _("Chat"), 3)
    entry({"admin", "system", "lucicodex", "plan"}, call("action_plan")).leaf = true
    entry({"admin", "system", "lucicodex", "execute"}, call("action_execute")).leaf = true
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
    
    local argv = {"/usr/bin/lucicodex", "-json", "-dry-run"}
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
    
    http.status(500, "Internal Server Error")
    http.write_json({ error = "failed to generate plan", output = output, errors = errors, code = code })
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
    
    local argv = {"/usr/bin/lucicodex", "-json"}
    
    if data.dry_run then
        table.insert(argv, "-dry-run")
    else
        table.insert(argv, "-approve")
    end
    
    if data.timeout and tonumber(data.timeout) then
        table.insert(argv, "-timeout=" .. tostring(data.timeout))
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
    
    http.status(500, "Internal Server Error")
    http.write_json({ error = "execution failed", output = output, errors = errors, code = code })
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


