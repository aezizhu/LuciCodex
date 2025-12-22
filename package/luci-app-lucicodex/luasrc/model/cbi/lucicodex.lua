local m, s, o
local uci = require "luci.model.uci"

m = Map("lucicodex", translate("LuciCodex Configuration"),
    translate("Configure your AI-powered router assistant. Set up API keys and safety preferences below."))

-- Ensure config section exists
local cursor = uci.cursor()
local conf = "lucicodex"

-- Helper to ensure section exists and is named 'main'
local function ensure_section()
    local type = "settings"
    local name = "main"
    
    local exist = cursor:get(conf, name)
    if exist == type then
        return
    end

    local found_anon = nil
    cursor:foreach(conf, type, function(s)
        if s['.name'] ~= name then
            found_anon = s['.name']
            return false
        end
    end)

    if found_anon then
        cursor:rename(conf, found_anon, name)
        cursor:commit(conf)
    else
        cursor:set(conf, name, type)
        cursor:set(conf, name, "provider", "gemini")
        cursor:set(conf, name, "model", "gemini-3-flash")
        cursor:set(conf, name, "endpoint", "https://generativelanguage.googleapis.com/v1beta")
        cursor:set(conf, name, "dry_run", "1")
        cursor:set(conf, name, "confirm_each", "0")
        cursor:set(conf, name, "timeout", "60")
        cursor:set(conf, name, "max_commands", "10")
        cursor:set(conf, name, "log_file", "/tmp/lucicodex.log")
        cursor:commit(conf)
    end
end

ensure_section()

-- Check for configured API keys
local has_gemini = (cursor:get(conf, "main", "key") or "") ~= ""
local has_openai = (cursor:get(conf, "main", "openai_key") or "") ~= ""
local has_anthropic = (cursor:get(conf, "main", "anthropic_key") or "") ~= ""

-- Status labels with checkmarks
local function label(name, has)
    return has and (name .. " ✓") or (name .. " ✗")
end

--[[
================================================================================
SECTION 1: Provider Selection
================================================================================
--]]
s = m:section(NamedSection, "main", "settings", translate("🤖 AI Provider"))
s.anonymous = false
s.addremove = false
s.description = translate("Choose which AI service powers LuciCodex. A ✓ indicates an API key is configured.")

o = s:option(ListValue, "provider", translate("Active Provider"))
o:value("gemini", label("Google Gemini", has_gemini))
o:value("openai", label("OpenAI (GPT-5)", has_openai))
o:value("anthropic", label("Anthropic (Claude)", has_anthropic))
o.default = "gemini"
o.description = translate("Select your preferred AI provider. Make sure to configure the corresponding API key below.")

-- When provider changes, clear the generic model/endpoint fields to avoid conflicts
o.write = function(self, section, value)
    local old_provider = cursor:get(conf, section, "provider") or "gemini"
    ListValue.write(self, section, value)
    
    -- If provider changed, clear the generic model and endpoint to use provider defaults
    if old_provider ~= value then
        cursor:delete(conf, section, "model")
        cursor:delete(conf, section, "endpoint")
        cursor:commit(conf)
    end
end

--[[
================================================================================
SECTION 2: API Keys
================================================================================
--]]
s = m:section(NamedSection, "main", "settings", translate("🔑 API Keys"))
s.anonymous = false
s.addremove = false
s.description = translate("Enter your API keys below. Keys are stored securely and never transmitted except to the provider. Leave a field empty to keep its existing value.")

-- Gemini
o = s:option(Value, "key", translate("Gemini API Key"))
o.password = true
o.rmempty = true
o.description = translate("Free tier available • Get key: makersuite.google.com/app/apikey")
o.write = function(self, section, value)
    if value and value ~= "" then
        Value.write(self, section, value)
    end
end

-- OpenAI
o = s:option(Value, "openai_key", translate("OpenAI API Key"))
o.password = true
o.rmempty = true
o.description = translate("Paid API • Get key: platform.openai.com/api-keys")
o.write = function(self, section, value)
    if value and value ~= "" then
        Value.write(self, section, value)
    end
end

-- Anthropic
o = s:option(Value, "anthropic_key", translate("Anthropic API Key"))
o.password = true
o.rmempty = true
o.description = translate("Paid API • Get key: console.anthropic.com")
o.write = function(self, section, value)
    if value and value ~= "" then
        Value.write(self, section, value)
    end
end

--[[
================================================================================
SECTION 3: Safety Settings
================================================================================
--]]
s = m:section(NamedSection, "main", "settings", translate("🛡️ Safety Settings"))
s.anonymous = false
s.addremove = false
s.description = translate("Control how LuciCodex executes commands on your router. We recommend keeping Dry Run enabled until you're comfortable with the tool.")

o = s:option(Flag, "dry_run", translate("Dry Run Mode"))
o.default = "1"
o.rmempty = false
o.description = translate("RECOMMENDED: When enabled, commands are displayed but not executed automatically. You must manually approve each command.")

o = s:option(Flag, "confirm_each", translate("Confirm Each Command"))
o.default = "0"
o.rmempty = false
o.description = translate("Ask for confirmation before executing each individual command in a plan.")

o = s:option(Value, "timeout", translate("Command Timeout"))
o.datatype = "uinteger"
o.placeholder = "60"
o.default = "60"
o.rmempty = true
o.description = translate("Seconds to wait for each command to complete before timing out. Default: 60")

o = s:option(Value, "max_commands", translate("Maximum Commands"))
o.datatype = "uinteger"
o.placeholder = "10"
o.default = "10"
o.rmempty = true
o.description = translate("Maximum number of commands the AI can generate in a single plan. Default: 10")

--[[
================================================================================
SECTION 4: Advanced Settings (collapsed by default conceptually)
================================================================================
--]]
s = m:section(NamedSection, "main", "settings", translate("⚙️ Advanced Settings"))
s.anonymous = false
s.addremove = false
s.description = translate("Custom model and endpoint settings. Leave empty to use defaults. Only change these if you know what you're doing.")

-- Gemini Advanced
o = s:option(Value, "model", translate("Gemini Model"))
o.placeholder = "gemini-3-flash"
o.rmempty = true
o.description = translate("Default: gemini-3-flash")

o = s:option(Value, "endpoint", translate("Gemini API Endpoint"))
o.placeholder = "https://generativelanguage.googleapis.com/v1beta"
o.rmempty = true

-- OpenAI Advanced
o = s:option(Value, "openai_model", translate("OpenAI Model"))
o.placeholder = "gpt-5-mini"
o.rmempty = true
o.description = translate("Default: gpt-5-mini • Other options: gpt-5, gpt-5-nano")

o = s:option(Value, "openai_endpoint", translate("OpenAI API Endpoint"))
o.placeholder = "https://api.openai.com/v1"
o.rmempty = true
o.description = translate("Change for Azure OpenAI or compatible providers")

-- Anthropic Advanced
o = s:option(Value, "anthropic_model", translate("Anthropic Model"))
o.placeholder = "claude-haiku-4-5-20251001"
o.rmempty = true
o.description = translate("Default: claude-haiku-4-5-20251001 • Other options: claude-sonnet-4-20250514")

o = s:option(Value, "anthropic_endpoint", translate("Anthropic API Endpoint"))
o.placeholder = "https://api.anthropic.com/v1"
o.rmempty = true

-- Logging
o = s:option(Value, "log_file", translate("Log File Path"))
o.placeholder = "/tmp/lucicodex.log"
o.default = "/tmp/lucicodex.log"
o.rmempty = true
o.description = translate("Path to store execution logs. Default: /tmp/lucicodex.log")

-- Proxy settings
o = s:option(Value, "https_proxy", translate("HTTPS Proxy"))
o.placeholder = "http://proxy.example.com:3128"
o.rmempty = true
o.description = translate("Optional proxy for provider HTTPS requests. Needed when LuCI runs without HTTPS_PROXY in its environment.")

o = s:option(Value, "http_proxy", translate("HTTP Proxy"))
o.placeholder = "http://proxy.example.com:3128"
o.rmempty = true
o.description = translate("Optional proxy for plain HTTP requests (most providers only need HTTPS).")

o = s:option(Value, "no_proxy", translate("No Proxy Domains"))
o.placeholder = "localhost,127.0.0.1,.lan"
o.rmempty = true
o.description = translate("Comma-separated hosts that should bypass the proxy (supports leading dots for domains).")

return m
