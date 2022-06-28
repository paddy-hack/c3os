package provider

import (
	"fmt"

	"github.com/c3os-io/c3os/pkg/config"
	"github.com/mudler/go-nodepair"
	"github.com/mudler/go-pluggable"
)

func Challenge(e *pluggable.Event) pluggable.EventResponse {
	cfg := &config.Config{}
	err := config.FromString(e.Data, cfg)
	if err != nil {
		return pluggable.EventResponse{Error: fmt.Sprintf("Failed reading JSON input: %s", err.Error())}
	}

	tk := ""
	if cfg.C3OS != nil && cfg.C3OS.NetworkToken != "" {
		tk = cfg.C3OS.NetworkToken
	}

	if tk == "" {
		tk = nodepair.GenerateToken()
	}
	return pluggable.EventResponse{
		Data: tk,
	}
}
