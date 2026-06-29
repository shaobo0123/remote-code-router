package main

import (
	"log"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginabi"
)

func logCandidateSuccess(hostCallbackID string, plan RoutePlan, candidate Candidate, status int) {
	safeLog(hostCallbackID, "info", "remote code router candidate succeeded", map[string]any{
		"active_candidate":   plan.ActiveCandidate,
		"requested_model":    plan.RequestedModel,
		"stream":             plan.Stream,
		"selected_candidate": candidate.Name,
		"candidate_priority": candidate.Priority,
		"status_code":        status,
	})
}

func safeLog(hostCallbackID, level, message string, fields map[string]any) {
	fields = safeLogFields(fields)
	if strings.TrimSpace(hostCallbackID) != "" {
		_, err := callHostCallback(pluginabi.MethodHostLog, hostLogRequest{
			HostCallbackID: strings.TrimSpace(hostCallbackID),
			Level:          strings.TrimSpace(level),
			Message:        message,
			Fields:         fields,
		})
		if err == nil {
			return
		}
	}
	log.Printf("[%s] %s %v", pluginName, message, fields)
}

func safeLogFields(input map[string]any) map[string]any {
	allowed := map[string]struct{}{
		"request_id":         {},
		"active_candidate":   {},
		"requested_model":    {},
		"stream":             {},
		"sorted_candidates":  {},
		"selected_candidate": {},
		"candidate_priority": {},
		"status_code":        {},
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		key = strings.TrimSpace(key)
		if _, ok := allowed[key]; ok {
			out[key] = value
		}
	}
	return out
}
