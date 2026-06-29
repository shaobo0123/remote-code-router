package main

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

func (p *remoteCodeRouterPlugin) RouteModel(_ context.Context, req pluginapi.ModelRouteRequest) (pluginapi.ModelRouteResponse, error) {
	return p.routeModel(req), nil
}

func (p *remoteCodeRouterPlugin) routeModel(req pluginapi.ModelRouteRequest) pluginapi.ModelRouteResponse {
	if p == nil {
		return pluginapi.ModelRouteResponse{Handled: false}
	}
	if hasBypassMarker(req.Headers) {
		return pluginapi.ModelRouteResponse{Handled: false, Reason: "remote_code_router_bypass"}
	}
	plan, ok := p.buildRoutePlan(req, true)
	if !ok {
		return pluginapi.ModelRouteResponse{Handled: false}
	}
	if p.plans != nil {
		p.plans.store(routePlanKeyFromRouteRequest(req), plan)
	}
	return pluginapi.ModelRouteResponse{
		Handled:    true,
		TargetKind: pluginapi.ModelRouteTargetSelf,
		Reason:     "plugin_remote_code_router_" + plan.ActiveCandidate,
	}
}

func (p *remoteCodeRouterPlugin) buildRoutePlan(req pluginapi.ModelRouteRequest, filterProviders bool) (RoutePlan, bool) {
	if p == nil {
		return RoutePlan{}, false
	}
	if !modelConfigured(p.cfg.ClientModels, req.RequestedModel) {
		return RoutePlan{}, false
	}
	active := p.state.activeCandidate(p.cfg.ActiveCandidate)
	candidates := cloneCandidates(p.cfg.Candidates)
	if filterProviders {
		candidates = filterByAvailableProviders(candidates, req.AvailableProviders)
	}
	candidates = sortCandidates(candidates, active)
	if len(candidates) == 0 {
		return RoutePlan{}, false
	}
	return RoutePlan{
		ActiveCandidate: active,
		RequestedModel:  strings.TrimSpace(req.RequestedModel),
		Stream:          req.Stream,
		Candidates:      candidates,
	}, true
}

func (p *remoteCodeRouterPlugin) routePlanForExecutor(req pluginapi.ExecutorRequest) (RoutePlan, bool) {
	if p == nil {
		return RoutePlan{}, false
	}
	if p.plans != nil {
		if plan, ok := p.plans.consume(routePlanKeyFromExecutorRequest(req)); ok {
			return plan, true
		}
	}
	body := req.OriginalRequest
	if len(body) == 0 {
		body = req.Payload
	}
	return p.buildRoutePlan(pluginapi.ModelRouteRequest{
		SourceFormat:   firstNonEmpty(req.SourceFormat, req.Format),
		RequestedModel: req.Model,
		Stream:         req.Stream,
		Headers:        cloneHeader(req.Headers),
		Query:          cloneValues(req.Query),
		Body:           append([]byte(nil), body...),
		Metadata:       cloneAnyMap(req.Metadata),
	}, false)
}

func hasBypassMarker(headers http.Header) bool {
	if strings.TrimSpace(headers.Get(bypassHeaderName)) == "1" {
		return true
	}
	for key, values := range headers {
		if !strings.EqualFold(strings.TrimSpace(key), bypassHeaderName) {
			continue
		}
		for _, value := range values {
			if strings.TrimSpace(value) == "1" {
				return true
			}
		}
	}
	return false
}

func modelConfigured(models []string, requested string) bool {
	requested = strings.TrimSpace(requested)
	for _, model := range models {
		if strings.EqualFold(strings.TrimSpace(model), requested) {
			return true
		}
	}
	return false
}

func filterByAvailableProviders(candidates []Candidate, available []string) []Candidate {
	if len(available) == 0 {
		return candidates
	}
	providers := make(map[string]struct{}, len(available))
	for _, provider := range available {
		provider = strings.ToLower(strings.TrimSpace(provider))
		if provider != "" {
			providers[provider] = struct{}{}
		}
	}
	out := make([]Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		if _, ok := providers[strings.ToLower(strings.TrimSpace(candidate.Provider))]; ok {
			out = append(out, candidate)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func cloneHeader(headers http.Header) http.Header {
	if headers == nil {
		return nil
	}
	cloned := make(http.Header, len(headers))
	for key, values := range headers {
		cloned[key] = append([]string(nil), values...)
	}
	return cloned
}

func cloneValues(values url.Values) url.Values {
	if values == nil {
		return nil
	}
	cloned := make(url.Values, len(values))
	for key, items := range values {
		cloned[key] = append([]string(nil), items...)
	}
	return cloned
}

func cloneAnyMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
