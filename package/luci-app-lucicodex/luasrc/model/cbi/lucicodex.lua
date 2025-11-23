local m, s, o
local uci = require "luci.model.uci"

m = Map("lucicodex", translate("LuciCodex Configuration"),
    translate("Configure LLM providers and API keys for the LuciCodex natural language router assistant."))

-- Ensure config sections exist using UCI cursor
local cursor = uci.cursor()
local conf = "lucicodex"

-- Helper to ensure section exists and is named 'main'
local function ensure_section(type, name)
    -- 1. Check if the specific named section exists
    local exist = cursor:get(conf, name)
    if exist == type then
        return
    end

    -- 2. Check for ANY section of this type (anonymous or other name)
    local found_anon = nil
    cursor:foreach(conf, type, function(s)
        if s['.name'] ~= name then
            found_anon = s['.name']
            return false -- stop iterating
        end
    end)

    if found_anon then
        -- Rename the first found anonymous section to 'main'
        cursor:rename(conf, found_anon, name)
        cursor:commit(conf)
    else
        -- Create new 'main' section if none exists
        cursor:set(conf, name, type)
        if type == "api" then
            cursor:set(conf, name, "provider", "gemini")
            cursor:set(conf, name, "model", "gemini-3")
            cursor:set(conf, name, "endpoint", "https://generativelanguage.googleapis.com/v1beta")
        elseif type == "settings" then
            cursor:set(conf, name, "dry_run", "1")
            cursor:set(conf, name, "confirm_each", "0")
            cursor:set(conf, name, "timeout", "30")
            cursor:set(conf, name, "max_commands", "10")
            cursor:set(conf, name, "log_file", "/tmp/lucicodex.log")
        end
        cursor:commit(conf)
    end
end

ensure_section("api", "main")
ensure_section("settings", "main")

-- API Configuration Section - Use NamedSection for singleton config
s = m:section(NamedSection, "main", "api", translate("API Configuration"))
s.anonymous = false -- Named section 'main'
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
o.placeholder = "gemini-3"
o.rmempty = true
o:depends("provider", "gemini")

o = s:option(Value, "endpoint", translate("API Endpoint"),
    translate("Custom API endpoint URL. Leave empty for provider default."))
o.placeholder = "https://generativelanguage.googleapis.com/v1beta"
o.rmempty = true
o:depends("provider", "gemini")

-- OpenAI-specific fields
o = s:option(Value, "openai_model", translate("Model"),
    translate("Specific model to use. Leave empty for provider default."))
o.placeholder = "gpt-5.1"
o.rmempty = true
o:depends("provider", "openai")

o = s:option(Value, "openai_endpoint", translate("API Endpoint"),
    translate("Custom API endpoint URL. Leave empty for provider default."))
o.placeholder = "https://api.openai.com/v1"
o.rmempty = true
o:depends("provider", "openai")

-- Anthropic-specific fields
o = s:option(Value, "anthropic_model", translate("Model"),
    translate("Specific model to use. Leave empty for provider default."))
o.placeholder = "claude-4.5"
o.rmempty = true
o:depends("provider", "anthropic")

o = s:option(Value, "anthropic_endpoint", translate("API Endpoint"),
    translate("Custom API endpoint URL. Leave empty for provider default."))
o.placeholder = "https://api.anthropic.com/v1"
o.rmempty = true
o:depends("provider", "anthropic")

-- Gemini CLI-specific fields
o = s:option(Value, "gemini_cli_model", translate("Model"),
    translate("Specific model to use. Leave empty for provider default."))
o.placeholder = "gemini-3"
o.rmempty = true
o:depends("provider", "gemini-cli")

-- Safety Settings Section - Use NamedSection for singleton config
s = m:section(NamedSection, "main", "settings", translate("Safety Settings"))
s.anonymous = false -- Named section 'main'
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
