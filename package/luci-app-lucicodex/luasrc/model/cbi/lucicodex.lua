local m, s, o
local uci = require "luci.model.uci"

m = Map("lucicodex", translate("LuciCodex Configuration"),
    translate("Configure LLM providers and API keys for the LuciCodex natural language router assistant."))

-- Ensure API and settings sections exist (required for anonymous sections to display)
local cursor = uci.cursor()
cursor:load("lucicodex")

-- Check if api section exists using foreach
local has_api = false
cursor:foreach("lucicodex", "api", function(s) has_api = true end)

if not has_api then
    local api_id = cursor:add("lucicodex", "api")
    cursor:set("lucicodex", api_id, "provider", "gemini")
    cursor:set("lucicodex", api_id, "key", "")
    cursor:set("lucicodex", api_id, "model", "gemini-1.5-flash")
    cursor:commit("lucicodex")
    cursor:load("lucicodex")
end

-- Check if settings section exists
local has_settings = false
cursor:foreach("lucicodex", "settings", function(s) has_settings = true end)

if not has_settings then
    local settings_id = cursor:add("lucicodex", "settings")
    cursor:set("lucicodex", settings_id, "dry_run", "1")
    cursor:set("lucicodex", settings_id, "confirm_each", "0")
    cursor:set("lucicodex", settings_id, "timeout", "30")
    cursor:set("lucicodex", settings_id, "max_commands", "10")
    cursor:set("lucicodex", settings_id, "log_file", "/tmp/lucicodex.log")
    cursor:commit("lucicodex")
    cursor:load("lucicodex")
end

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
o.rmempty = false
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
