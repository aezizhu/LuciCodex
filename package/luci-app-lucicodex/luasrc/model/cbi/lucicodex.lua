local m, s, o

m = Map("lucicodex", translate("LuciCodex Configuration"),
    translate("Configure LLM providers and API keys for the LuciCodex natural language router assistant."))

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
