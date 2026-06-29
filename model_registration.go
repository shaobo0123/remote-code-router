package main

import (
	"context"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

func (p *remoteCodeRouterPlugin) RegisterModels(context.Context, pluginapi.ModelRegistrationRequest) (pluginapi.ModelRegistrationResponse, error) {
	models := make([]pluginapi.ModelInfo, 0, len(p.cfg.ModelAliases))
	now := time.Now().Unix()
	for _, alias := range p.cfg.ModelAliases {
		info := pluginapi.ModelInfo{
			ID:                  alias.ID,
			Object:              "model",
			Created:             now,
			OwnedBy:             alias.OwnedBy,
			Type:                alias.Type,
			DisplayName:         alias.DisplayName,
			Name:                alias.Name,
			Description:         alias.Description,
			InputTokenLimit:     alias.InputTokenLimit,
			OutputTokenLimit:    alias.OutputTokenLimit,
			ContextLength:       alias.ContextLength,
			MaxCompletionTokens: alias.MaxCompletionTokens,
			SupportedParameters: append([]string(nil), alias.SupportedParameters...),
			SupportedGenerationMethods: []string{
				"chat",
				"responses",
			},
			SupportedInputModalities:  []string{"text"},
			SupportedOutputModalities: []string{"text"},
			UserDefined:               true,
		}
		if alias.Thinking {
			info.Thinking = &pluginapi.ThinkingSupport{
				ZeroAllowed:    true,
				DynamicAllowed: true,
				Levels:         []string{"low", "medium", "high"},
			}
		}
		models = append(models, info)
	}
	return pluginapi.ModelRegistrationResponse{
		Provider: pluginProvider,
		Models:   models,
	}, nil
}
