package main

import "github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"

const (
	pluginName     = "remote-code-router"
	pluginProvider = "remote-code-router"
)

var pluginVersion = "0.1.0"

var executorFormats = []string{
	"claude",
	"anthropic",
	"openai",
	"responses",
	"chat-completions",
	"codex",
}

type remoteCodeRouterPlugin struct {
	cfg        PluginConfig
	configYAML []byte
	pluginDir  string
	plans      *routePlanStore
	state      *stateStore
}

func main() {}

func buildPlugin(configYAML []byte, pluginDir string) (pluginapi.Plugin, error) {
	cfg, err := parseRemoteCodeRouterRegistrationConfig(configYAML)
	if err != nil {
		return pluginapi.Plugin{}, err
	}

	p := &remoteCodeRouterPlugin{
		cfg:        cfg,
		configYAML: append([]byte(nil), configYAML...),
		pluginDir:  pluginDir,
		plans:      newRoutePlanStore(defaultRoutePlanTTL),
		state:      newStateStore(pluginDir, cfg),
	}

	return pluginapi.Plugin{
		Metadata: pluginapi.Metadata{
			Name:             pluginName,
			Version:          pluginVersion,
			Author:           "shaobo0123",
			GitHubRepository: "https://github.com/shaobo0123/remote-code-router",
			ConfigFields: []pluginapi.ConfigField{
				{
					Name:        "model_aliases",
					Type:        pluginapi.ConfigFieldTypeArray,
					Description: "Client-facing virtual models shown to clients.",
				},
				{
					Name:        "active_candidate",
					Type:        pluginapi.ConfigFieldTypeString,
					Description: "Current server-side model candidate, or auto.",
				},
				{
					Name:        "candidates",
					Type:        pluginapi.ConfigFieldTypeArray,
					Description: "Provider/model candidates available behind the alias.",
				},
				{
					Name:        "fallback",
					Type:        pluginapi.ConfigFieldTypeObject,
					Description: "Fallback status codes and streaming fallback policy.",
				},
			},
		},
		Capabilities: pluginapi.Capabilities{
			ModelRegistrar:        p,
			ModelRouter:           p,
			Executor:              p,
			ExecutorModelScope:    pluginapi.ExecutorModelScopeStatic,
			ExecutorInputFormats:  append([]string(nil), executorFormats...),
			ExecutorOutputFormats: append([]string(nil), executorFormats...),
			ManagementAPI:         p,
		},
	}, nil
}
