package policy

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/aezizhu/LuciCodex/internal/config"
	"github.com/aezizhu/LuciCodex/internal/plan"
)

type Engine struct {
	cfg      config.Config
	allowREs []*regexp.Regexp
	denyREs  []*regexp.Regexp
}

func New(cfg config.Config) *Engine {
	e := &Engine{cfg: cfg}
	for _, p := range cfg.Allowlist {
		if re, err := regexp.Compile(p); err == nil {
			e.allowREs = append(e.allowREs, re)
		}
	}
	for _, p := range cfg.Denylist {
		if re, err := regexp.Compile(p); err == nil {
			e.denyREs = append(e.denyREs, re)
		}
	}
	return e
}

func (e *Engine) ValidatePlan(p plan.Plan) error {
	for i, c := range p.Commands {
		if len(c.Command) == 0 {
			return fmt.Errorf("command %d is empty", i)
		}
		// Basic argv checks
		for j, a := range c.Command {
			if strings.TrimSpace(a) == "" {
				return fmt.Errorf("command %d arg %d is empty", i, j)
			}
			if strings.ContainsAny(a, "\x00") {
				return fmt.Errorf("command %d arg %d contains NUL", i, j)
			}
		}
		if strings.ContainsAny(c.Command[0], "|&;><`$") {
			return fmt.Errorf("command %d contains shell metacharacters in argv[0]", i)
		}
		cmdline := strings.Join(c.Command, " ")
		for _, re := range e.denyREs {
			if re.MatchString(cmdline) {
				return fmt.Errorf("command %d denied by policy: %s", i, cmdline)
			}
		}
		allowed := false
		for _, re := range e.allowREs {
			if re.MatchString(cmdline) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("command %d not allowed by policy: %s", i, cmdline)
		}
	}
	return nil
}
