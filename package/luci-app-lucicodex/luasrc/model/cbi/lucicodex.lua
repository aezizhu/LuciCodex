local m, s, o
local uci = require "luci.model.uci"

m = Map("lucicodex", translate("LuciCodex Configuration"),
    translate("Configure LLM providers and API keys for the LuciCodex natural language router assistant."))

-- Ensure config section exists
local cursor = uci.cursor()
local conf = "lucicodex"

-- Helper to ensure section exists and is named 'main'
local function ensure_section()
    local type = "settings"
    local name = "main"
    
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
        cursor:set(conf, name, "provider", "gemini")
        cursor:set(conf, name, "model", "gemini-2.5-flash")
        cursor:set(conf, name, "endpoint", "https://generativelanguage.googleapis.com/v1beta")
        cursor:set(conf, name, "dry_run", "1")
        cursor:set(conf, name, "confirm_each", "0")
        cursor:set(conf, name, "timeout", "30")
        cursor:set(conf, name, "max_commands", "10")
        cursor:set(conf, name, "log_file", "/tmp/lucicodex.log")
        cursor:commit(conf)
    end
end

ensure_section()

-- Unified Configuration Section
s = m:section(NamedSection, "main", "settings", translate("General Configuration"))
s.anonymous = false -- Named section 'main'
s.addremove = false

-- Provider Selection
local has_gemini = (cursor:get(conf, "main", "key") or cursor:get(conf, "@api[0]", "key")) and true or false
local has_openai = (cursor:get(conf, "main", "openai_key") or cursor:get(conf, "@api[0]", "openai_key")) and true or false
local has_anthropic = (cursor:get(conf, "main", "anthropic_key") or cursor:get(conf, "@api[0]", "anthropic_key")) and true or false

local function label(name, has)
    return has and (name .. " ✔️") or (name .. " ✖️")
end

o = s:option(ListValue, "provider", translate("LLM Provider"),
    translate("Select which LLM provider to use for generating commands. ✔️ means API key present; ✖️ means missing."))
o:value("gemini", label("Google Gemini", has_gemini))
o:value("openai", label("OpenAI", has_openai))
o:value("anthropic", label("Anthropic", has_anthropic))
o.default = "gemini"

-- API Keys (Always visible so they don't get deleted when switching providers)
-- Use rmempty=false and custom write to preserve existing keys when field is empty
o = s:option(Value, "key", translate("Gemini API Key"),
    translate("API key for Google Gemini. Get one from https://makersuite.google.com/app/apikey (leave empty to keep existing key)"))
o.password = true
o.rmempty = false
o.write = function(self, section, value)
    if value and value ~= "" then
        Value.write(self, section, value)
    end
    -- If empty, don't write (keeps existing value)
end
o.remove = function(self, section)
    -- Don't remove on empty - preserve existing key
end

o = s:option(Value, "openai_key", translate("OpenAI API Key"),
    translate("API key for OpenAI. Get one from https://platform.openai.com/api-keys (leave empty to keep existing key)"))
o.password = true
o.rmempty = false
o.write = function(self, section, value)
    if value and value ~= "" then
        Value.write(self, section, value)
    end
end
o.remove = function(self, section)
    -- Don't remove on empty - preserve existing key
end

o = s:option(Value, "anthropic_key", translate("Anthropic API Key"),
    translate("API key for Anthropic Claude. Get one from https://console.anthropic.com/ (leave empty to keep existing key)"))
o.password = true
o.rmempty = false
o.write = function(self, section, value)
    if value and value ~= "" then
        Value.write(self, section, value)
    end
end
o.remove = function(self, section)
    -- Don't remove on empty - preserve existing key
end

-- Models & Endpoints (Optional, can be hidden if not relevant, but safer to keep visible or use depends without rmempty if possible. 
-- For now, removing depends to be safe and consistent with keys)

o = s:option(Value, "model", translate("Gemini Model"),
    translate("Specific model to use for Gemini. Default: gemini-2.5-flash"))
o.placeholder = "gemini-2.5-flash"
o.rmempty = true

o = s:option(Value, "endpoint", translate("Gemini Endpoint"),
    translate("Custom API endpoint for Gemini."))
o.placeholder = "https://generativelanguage.googleapis.com/v1beta"
o.rmempty = true

o = s:option(Value, "openai_model", translate("OpenAI Model"),
    translate("Specific model to use for OpenAI. Default: gpt-4o-mini"))
o.placeholder = "gpt-4o-mini"
o.rmempty = true

o = s:option(Value, "openai_endpoint", translate("OpenAI Endpoint"),
    translate("Custom API endpoint for OpenAI."))
o.placeholder = "https://api.openai.com/v1"
o.rmempty = true

o = s:option(Value, "anthropic_model", translate("Anthropic Model"),
    translate("Specific model to use for Anthropic. Default: claude-sonnet-4-5-20250929"))
o.placeholder = "claude-sonnet-4-5-20250929"
o.rmempty = true

o = s:option(Value, "anthropic_endpoint", translate("Anthropic Endpoint"),
    translate("Custom API endpoint for Anthropic."))
o.placeholder = "https://api.anthropic.com/v1"
o.rmempty = true

-- Safety Settings
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
