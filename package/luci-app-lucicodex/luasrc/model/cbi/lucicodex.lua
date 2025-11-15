local m, s, o
local uci = require "luci.model.uci"

m = Map("lucicodex", translate("LuciCodex Configuration"),
    translate("Configure LLM providers and API keys for the LuciCodex natural language router assistant."))

-- Ensure config sections exist using UCI cursor
-- Note: Go backend expects anonymous sections (@api[0], @settings[0])
local cursor = uci.cursor()
cursor:load("lucicodex")

-- Check and create api section (anonymous)
local has_api = false
cursor:foreach("lucicodex", "api", function(s)
    has_api = true
end)

if not has_api then
    -- Create anonymous section using os.execute for reliability
    -- This matches the pattern used in shell scripts
    os.execute("uci -q add lucicodex api >/dev/null 2>&1")
    os.execute("uci -q set lucicodex.@api[0].provider='gemini' >/dev/null 2>&1")
    os.execute("uci -q set lucicodex.@api[0].key='' >/dev/null 2>&1")
    os.execute("uci -q set lucicodex.@api[0].model='gemini-1.5-flash' >/dev/null 2>&1")
    os.execute("uci -q set lucicodex.@api[0].endpoint='https://generativelanguage.googleapis.com/v1beta' >/dev/null 2>&1")
    os.execute("uci -q set lucicodex.@api[0].openai_key='' >/dev/null 2>&1")
    os.execute("uci -q set lucicodex.@api[0].anthropic_key='' >/dev/null 2>&1")
    os.execute("uci -q commit lucicodex >/dev/null 2>&1")
    cursor:load("lucicodex")
end

-- Check and create settings section (anonymous)
local has_settings = false
cursor:foreach("lucicodex", "settings", function(s)
    has_settings = true
end)

if not has_settings then
    -- Create anonymous section using os.execute for reliability
    -- This matches the pattern used in shell scripts
    os.execute("uci -q add lucicodex settings >/dev/null 2>&1")
    os.execute("uci -q set lucicodex.@settings[0].dry_run='1' >/dev/null 2>&1")
    os.execute("uci -q set lucicodex.@settings[0].confirm_each='0' >/dev/null 2>&1")
    os.execute("uci -q set lucicodex.@settings[0].timeout='30' >/dev/null 2>&1")
    os.execute("uci -q set lucicodex.@settings[0].max_commands='10' >/dev/null 2>&1")
    os.execute("uci -q set lucicodex.@settings[0].log_file='/tmp/lucicodex.log' >/dev/null 2>&1")
    os.execute("uci -q commit lucicodex >/dev/null 2>&1")
    cursor:load("lucicodex")
end

-- API Configuration Section - Use TypedSection for anonymous sections
s = m:section(TypedSection, "api", translate("API Configuration"))
s.anonymous = true
s.addremove = false

o = s:option(ListValue, "provider", translate("LLM Provider"),
    translate("Select which LLM provider to use for generating commands."))
o:value("gemini", "Google Gemini")
o:value("openai", "OpenAI")
o:value("anthropic", "Anthropic")
o:value("gemini-cli", "External Gemini CLI")
o.default = "gemini"

o = s:option(Value, "key", translate("Gemini API Key"),
    translate("API key for Google Gemini. Get one from https://makersuite.google.com/app/apikey"))
o.password = true
o.rmempty = true
o:depends("provider", "gemini")

o = s:option(Value, "openai_key", translate("OpenAI API Key"),
    translate("API key for OpenAI. Get one from https://platform.openai.com/api-keys"))
o.password = true
o.rmempty = true
o:depends("provider", "openai")

o = s:option(Value, "anthropic_key", translate("Anthropic API Key"),
    translate("API key for Anthropic Claude. Get one from https://console.anthropic.com/"))
o.password = true
o.rmempty = true
o:depends("provider", "anthropic")

o = s:option(Value, "model", translate("Model"),
    translate("Specific model to use. Leave empty for provider default."))
o.placeholder = "gemini-1.5-flash"
o.rmempty = true

o = s:option(Value, "endpoint", translate("API Endpoint"),
    translate("Custom API endpoint URL. Leave empty for provider default."))
o.placeholder = "https://generativelanguage.googleapis.com/v1beta"
o.rmempty = true

-- Safety Settings Section - Use TypedSection for anonymous sections
s = m:section(TypedSection, "settings", translate("Safety Settings"))
s.anonymous = true
s.addremove = false

o = s:option(Flag, "dry_run", translate("Dry Run by Default"),
    translate("When enabled, commands are only displayed but not executed by default."))
o.default = "1"
o.rmempty = false

o = s:option(Flag, "confirm_each", translate("Confirm Each Command"),
    translate("When enabled, ask for confirmation before executing each command."))
o.default = "0"
o.rmempty = false

o = s:option(Value, "timeout", translate("Command Timeout (seconds)"),
    translate("Maximum time to wait for each command to complete."))
o.datatype = "uinteger"
o.placeholder = "30"
o.default = "30"

o = s:option(Value, "max_commands", translate("Maximum Commands"),
    translate("Maximum number of commands to generate in a single plan."))
o.datatype = "uinteger"
o.placeholder = "10"
o.default = "10"

o = s:option(Value, "log_file", translate("Log File"),
    translate("Path to log file for command execution history."))
o.placeholder = "/tmp/lucicodex.log"
o.default = "/tmp/lucicodex.log"

return m
