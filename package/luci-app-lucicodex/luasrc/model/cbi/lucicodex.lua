local m, s, o
local uci = require "luci.model.uci"

m = Map("lucicodex", translate("LuciCodex Configuration"),
    translate("Configure LLM providers and API keys for the LuciCodex natural language router assistant."))

-- Ensure config sections exist using UCI cursor
local cursor = uci.cursor()
cursor:load("lucicodex")

-- Check and create api section with explicit name
local has_api = false
local api_section_name = nil
cursor:foreach("lucicodex", "api", function(s)
    has_api = true
    api_section_name = s[".name"]
end)

if not has_api then
    -- Create named section (not anonymous)
    cursor:set("lucicodex", "api", "api")
    cursor:set("lucicodex", "api", "provider", "gemini")
    cursor:set("lucicodex", "api", "key", "")
    cursor:set("lucicodex", "api", "model", "gemini-1.5-flash")
    cursor:set("lucicodex", "api", "endpoint", "https://generativelanguage.googleapis.com/v1beta")
    cursor:set("lucicodex", "api", "openai_key", "")
    cursor:set("lucicodex", "api", "anthropic_key", "")
    cursor:commit("lucicodex")
    cursor:load("lucicodex")
    api_section_name = "api"
else
    api_section_name = api_section_name or "api"
end

-- Check and create settings section with explicit name
local has_settings = false
local settings_section_name = nil
cursor:foreach("lucicodex", "settings", function(s)
    has_settings = true
    settings_section_name = s[".name"]
end)

if not has_settings then
    -- Create named section (not anonymous)
    cursor:set("lucicodex", "settings", "settings")
    cursor:set("lucicodex", "settings", "dry_run", "1")
    cursor:set("lucicodex", "settings", "confirm_each", "0")
    cursor:set("lucicodex", "settings", "timeout", "30")
    cursor:set("lucicodex", "settings", "max_commands", "10")
    cursor:set("lucicodex", "settings", "log_file", "/tmp/lucicodex.log")
    cursor:commit("lucicodex")
    cursor:load("lucicodex")
    settings_section_name = "settings"
else
    settings_section_name = settings_section_name or "settings"
end

-- API Configuration Section - Use NamedSection to target specific section
s = m:section(NamedSection, api_section_name, "api", translate("API Configuration"))
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

-- Safety Settings Section - Use NamedSection to target specific section
s = m:section(NamedSection, settings_section_name, "settings", translate("Safety Settings"))
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
