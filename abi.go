package main

/*
#include <stdint.h>
#include <stdlib.h>

typedef struct {
	void* ptr;
	size_t len;
} cliproxy_buffer;

typedef int (*cliproxy_host_call_fn)(void*, const char*, const uint8_t*, size_t, cliproxy_buffer*);
typedef void (*cliproxy_host_free_fn)(void*, size_t);

typedef struct {
	uint32_t abi_version;
	void* host_ctx;
	cliproxy_host_call_fn call;
	cliproxy_host_free_fn free_buffer;
} cliproxy_host_api;

typedef int (*cliproxy_plugin_call_fn)(char*, uint8_t*, size_t, cliproxy_buffer*);
typedef void (*cliproxy_plugin_free_fn)(void*, size_t);
typedef void (*cliproxy_plugin_shutdown_fn)(void);

typedef struct {
	uint32_t abi_version;
	cliproxy_plugin_call_fn call;
	cliproxy_plugin_free_fn free_buffer;
	cliproxy_plugin_shutdown_fn shutdown;
} cliproxy_plugin_api;

extern int RemoteCodeRouterPluginCall(char*, uint8_t*, size_t, cliproxy_buffer*);
extern void RemoteCodeRouterPluginFree(void*, size_t);
extern void RemoteCodeRouterPluginShutdown(void);

static int remote_code_router_call_host(cliproxy_host_api* api, const char* method, const uint8_t* request, size_t request_len, cliproxy_buffer* response) {
	return api->call(api->host_ctx, method, request, request_len, response);
}

static void remote_code_router_free_host_buffer(cliproxy_host_api* api, void* ptr, size_t len) {
	api->free_buffer(ptr, len);
}
*/
import "C"

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"unsafe"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginabi"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

const maxCGoBytesLen = C.size_t(1<<31 - 1)

var remoteCodeRouterABIState = struct {
	sync.RWMutex
	host         *C.cliproxy_host_api
	plugin       *remoteCodeRouterPlugin
	shuttingDown bool
	inFlight     sync.WaitGroup
}{}

type abiEnvelope struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *abiError       `json:"error,omitempty"`
}

type abiError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Retryable  bool   `json:"retryable,omitempty"`
	HTTPStatus int    `json:"http_status,omitempty"`
}

type abiLifecycleRequest struct {
	ConfigYAML []byte `json:"config_yaml"`
	PluginDir  string `json:"plugin_dir,omitempty"`
}

type abiRegistration struct {
	SchemaVersion uint32             `json:"schema_version"`
	Metadata      pluginapi.Metadata `json:"metadata"`
	Capabilities  abiCapabilities    `json:"capabilities"`
}

type abiCapabilities struct {
	ModelRegistrar        bool                         `json:"model_registrar"`
	ModelRouter           bool                         `json:"model_router"`
	Executor              bool                         `json:"executor"`
	ExecutorModelScope    pluginapi.ExecutorModelScope `json:"executor_model_scope"`
	ExecutorInputFormats  []string                     `json:"executor_input_formats,omitempty"`
	ExecutorOutputFormats []string                     `json:"executor_output_formats,omitempty"`
	ManagementAPI         bool                         `json:"management_api"`
}

type abiIdentifierResponse struct {
	Identifier string `json:"identifier"`
}

type abiExecutorStreamResponse struct {
	Headers http.Header                     `json:"headers,omitempty"`
	Chunks  []pluginapi.ExecutorStreamChunk `json:"chunks,omitempty"`
}

type abiManagementRegistrationResponse struct {
	Routes    []abiManagementRoute `json:"routes,omitempty"`
	Resources []abiResourceRoute   `json:"resources,omitempty"`
}

type abiManagementRoute struct {
	Method      string `json:"Method,omitempty"`
	Path        string `json:"Path,omitempty"`
	Menu        string `json:"Menu,omitempty"`
	Description string `json:"Description,omitempty"`
}

type abiResourceRoute struct {
	Path        string `json:"Path,omitempty"`
	Menu        string `json:"Menu,omitempty"`
	Description string `json:"Description,omitempty"`
}

type rpcModelRouteRequest struct {
	pluginapi.ModelRouteRequest
	HostCallbackID string `json:"host_callback_id,omitempty"`
}

type rpcExecutorRequest struct {
	pluginapi.ExecutorRequest
	StreamID       string `json:"stream_id,omitempty"`
	HostCallbackID string `json:"host_callback_id,omitempty"`
}

type abiEmptyResponse struct{}

//export cliproxy_plugin_init
func cliproxy_plugin_init(host *C.cliproxy_host_api, plugin *C.cliproxy_plugin_api) C.int {
	if host == nil || plugin == nil {
		return 1
	}
	remoteCodeRouterABIState.Lock()
	remoteCodeRouterABIState.host = host
	remoteCodeRouterABIState.shuttingDown = false
	remoteCodeRouterABIState.Unlock()

	plugin.abi_version = C.uint32_t(pluginabi.ABIVersion)
	plugin.call = C.cliproxy_plugin_call_fn(C.RemoteCodeRouterPluginCall)
	plugin.free_buffer = C.cliproxy_plugin_free_fn(C.RemoteCodeRouterPluginFree)
	plugin.shutdown = C.cliproxy_plugin_shutdown_fn(C.RemoteCodeRouterPluginShutdown)
	return 0
}

//export RemoteCodeRouterPluginCall
func RemoteCodeRouterPluginCall(method *C.char, request *C.uint8_t, requestLen C.size_t, response *C.cliproxy_buffer) C.int {
	if response != nil {
		response.ptr = nil
		response.len = 0
	}
	if method == nil {
		writeABIResponse(response, abiErrorEnvelope("invalid_method", "method is required"))
		return 0
	}
	var requestBytes []byte
	if request != nil && requestLen > 0 {
		if requestLen > maxCGoBytesLen {
			writeABIResponse(response, abiErrorEnvelope("request_too_large", "request payload is too large"))
			return 0
		}
		requestBytes = C.GoBytes(unsafe.Pointer(request), C.int(requestLen))
	}
	raw, errHandle := handleRemoteCodeRouterABIMethod(context.Background(), C.GoString(method), requestBytes)
	if errHandle != nil {
		writeABIResponse(response, abiErrorEnvelopeFromError("plugin_error", errHandle))
		return 0
	}
	writeABIResponse(response, raw)
	return 0
}

//export RemoteCodeRouterPluginFree
func RemoteCodeRouterPluginFree(ptr unsafe.Pointer, _ C.size_t) {
	if ptr != nil {
		C.free(ptr)
	}
}

//export RemoteCodeRouterPluginShutdown
func RemoteCodeRouterPluginShutdown() {
	remoteCodeRouterABIState.Lock()
	remoteCodeRouterABIState.shuttingDown = true
	remoteCodeRouterABIState.Unlock()

	remoteCodeRouterABIState.inFlight.Wait()

	remoteCodeRouterABIState.Lock()
	if remoteCodeRouterABIState.plugin != nil && remoteCodeRouterABIState.plugin.plans != nil {
		remoteCodeRouterABIState.plugin.plans.clear()
	}
	remoteCodeRouterABIState.plugin = nil
	remoteCodeRouterABIState.host = nil
	remoteCodeRouterABIState.Unlock()
}

func handleRemoteCodeRouterABIMethod(ctx context.Context, method string, request []byte) (out []byte, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			out = nil
			err = fmt.Errorf("panic in %s: %v", method, recovered)
		}
	}()

	switch method {
	case pluginabi.MethodPluginRegister, pluginabi.MethodPluginReconfigure:
		return handleRemoteCodeRouterRegister(request)
	}

	p, done, errPlugin := beginRemoteCodeRouterPluginCall()
	if errPlugin != nil {
		return nil, errPlugin
	}
	defer done()

	switch method {
	case pluginabi.MethodModelRegister:
		var req pluginapi.ModelRegistrationRequest
		if errDecode := json.Unmarshal(request, &req); errDecode != nil {
			return nil, errDecode
		}
		resp, errRegister := p.RegisterModels(ctx, req)
		return abiOKEnvelopeWithError(resp, errRegister)
	case pluginabi.MethodModelRoute:
		var req rpcModelRouteRequest
		if errDecode := json.Unmarshal(request, &req); errDecode != nil {
			return nil, errDecode
		}
		resp, errRoute := p.RouteModel(ctx, req.ModelRouteRequest)
		return abiOKEnvelopeWithError(resp, errRoute)
	case pluginabi.MethodExecutorIdentifier:
		return abiOKEnvelope(abiIdentifierResponse{Identifier: p.Identifier()})
	case pluginabi.MethodExecutorExecute:
		var req rpcExecutorRequest
		if errDecode := json.Unmarshal(request, &req); errDecode != nil {
			return nil, errDecode
		}
		resp, errExecute := p.execute(ctx, req.ExecutorRequest, req.HostCallbackID)
		return abiOKEnvelopeWithError(resp, errExecute)
	case pluginabi.MethodExecutorExecuteStream:
		var req rpcExecutorRequest
		if errDecode := json.Unmarshal(request, &req); errDecode != nil {
			return nil, errDecode
		}
		return p.startExecutorStream(req)
	case pluginabi.MethodExecutorCountTokens:
		resp, errCount := p.CountTokens(ctx, pluginapi.ExecutorRequest{})
		return abiOKEnvelopeWithError(resp, errCount)
	case pluginabi.MethodExecutorHTTPRequest:
		return abiErrorEnvelope("unsupported_method", "executor.http_request is not supported by remote-code-router"), nil
	case pluginabi.MethodManagementRegister:
		var req pluginapi.ManagementRegistrationRequest
		if errDecode := json.Unmarshal(request, &req); errDecode != nil {
			return nil, errDecode
		}
		resp, errRegister := p.RegisterManagement(ctx, req)
		return abiOKEnvelopeWithError(stripManagementHandlers(resp), errRegister)
	case pluginabi.MethodManagementHandle:
		var req pluginapi.ManagementRequest
		if errDecode := json.Unmarshal(request, &req); errDecode != nil {
			return nil, errDecode
		}
		resp, errHandle := p.HandleManagement(ctx, req)
		return abiOKEnvelopeWithError(resp, errHandle)
	default:
		return abiErrorEnvelope("unknown_method", "unknown method: "+method), nil
	}
}

func stripManagementHandlers(resp pluginapi.ManagementRegistrationResponse) abiManagementRegistrationResponse {
	out := abiManagementRegistrationResponse{
		Routes:    make([]abiManagementRoute, 0, len(resp.Routes)),
		Resources: make([]abiResourceRoute, 0, len(resp.Resources)),
	}
	for _, route := range resp.Routes {
		out.Routes = append(out.Routes, abiManagementRoute{
			Method:      route.Method,
			Path:        route.Path,
			Menu:        route.Menu,
			Description: route.Description,
		})
	}
	for _, resource := range resp.Resources {
		out.Resources = append(out.Resources, abiResourceRoute{
			Path:        resource.Path,
			Menu:        resource.Menu,
			Description: resource.Description,
		})
	}
	return out
}

func handleRemoteCodeRouterRegister(request []byte) ([]byte, error) {
	var req abiLifecycleRequest
	if len(request) > 0 {
		if errDecode := json.Unmarshal(request, &req); errDecode != nil {
			return nil, errDecode
		}
	}
	plugin, errBuild := buildPlugin(req.ConfigYAML, req.PluginDir)
	if errBuild != nil {
		return nil, errBuild
	}
	p, ok := plugin.Capabilities.ModelRouter.(*remoteCodeRouterPlugin)
	if !ok || p == nil {
		return nil, fmt.Errorf("%s registration returned invalid plugin instance", pluginName)
	}
	remoteCodeRouterABIState.Lock()
	remoteCodeRouterABIState.plugin = p
	remoteCodeRouterABIState.shuttingDown = false
	remoteCodeRouterABIState.Unlock()
	return abiOKEnvelope(abiRegistration{
		SchemaVersion: pluginabi.SchemaVersion,
		Metadata:      plugin.Metadata,
		Capabilities: abiCapabilities{
			ModelRegistrar:        plugin.Capabilities.ModelRegistrar != nil,
			ModelRouter:           plugin.Capabilities.ModelRouter != nil,
			Executor:              plugin.Capabilities.Executor != nil,
			ExecutorModelScope:    plugin.Capabilities.ExecutorModelScope,
			ExecutorInputFormats:  append([]string(nil), plugin.Capabilities.ExecutorInputFormats...),
			ExecutorOutputFormats: append([]string(nil), plugin.Capabilities.ExecutorOutputFormats...),
			ManagementAPI:         plugin.Capabilities.ManagementAPI != nil,
		},
	})
}

func beginRemoteCodeRouterPluginCall() (*remoteCodeRouterPlugin, func(), error) {
	remoteCodeRouterABIState.Lock()
	defer remoteCodeRouterABIState.Unlock()
	if remoteCodeRouterABIState.shuttingDown {
		return nil, nil, fmt.Errorf("%s plugin is shutting down", pluginName)
	}
	if remoteCodeRouterABIState.plugin == nil {
		return nil, nil, fmt.Errorf("%s plugin is not registered", pluginName)
	}
	remoteCodeRouterABIState.inFlight.Add(1)
	return remoteCodeRouterABIState.plugin, remoteCodeRouterABIState.inFlight.Done, nil
}

func abiOKEnvelopeWithError(v any, err error) ([]byte, error) {
	if err != nil {
		return nil, err
	}
	return abiOKEnvelope(v)
}

func abiOKEnvelope(v any) ([]byte, error) {
	raw, errMarshal := json.Marshal(v)
	if errMarshal != nil {
		return nil, errMarshal
	}
	return json.Marshal(abiEnvelope{OK: true, Result: raw})
}

func abiErrorEnvelope(code, message string) []byte {
	raw, _ := json.Marshal(abiEnvelope{OK: false, Error: &abiError{Code: code, Message: message}})
	return raw
}

func abiErrorEnvelopeFromError(code string, err error) []byte {
	apiErr := &abiError{Code: code, Message: err.Error()}
	if carrier, ok := err.(interface{ StatusCode() int }); ok && carrier.StatusCode() > 0 {
		apiErr.HTTPStatus = carrier.StatusCode()
	}
	raw, _ := json.Marshal(abiEnvelope{OK: false, Error: apiErr})
	return raw
}

func writeABIResponse(response *C.cliproxy_buffer, raw []byte) {
	if response == nil || len(raw) == 0 {
		return
	}
	ptr := C.CBytes(raw)
	if ptr == nil {
		return
	}
	response.ptr = ptr
	response.len = C.size_t(len(raw))
}

func callHost(method string, payload any) (json.RawMessage, error) {
	rawPayload, errMarshal := json.Marshal(payload)
	if errMarshal != nil {
		return nil, fmt.Errorf("marshal host callback %s: %w", method, errMarshal)
	}

	remoteCodeRouterABIState.RLock()
	defer remoteCodeRouterABIState.RUnlock()
	if remoteCodeRouterABIState.host == nil {
		return nil, fmt.Errorf("host callback is unavailable")
	}

	cMethod := C.CString(method)
	defer C.free(unsafe.Pointer(cMethod))

	var cPayload unsafe.Pointer
	if len(rawPayload) > 0 {
		cPayload = C.CBytes(rawPayload)
		if cPayload == nil {
			return nil, fmt.Errorf("allocate host callback payload")
		}
		defer C.free(cPayload)
	}

	var response C.cliproxy_buffer
	rc := C.remote_code_router_call_host(
		remoteCodeRouterABIState.host,
		cMethod,
		(*C.uint8_t)(cPayload),
		C.size_t(len(rawPayload)),
		&response,
	)
	var rawResponse []byte
	if response.ptr != nil && response.len > 0 {
		rawResponse = C.GoBytes(response.ptr, C.int(response.len))
	}
	if response.ptr != nil {
		C.remote_code_router_free_host_buffer(remoteCodeRouterABIState.host, response.ptr, response.len)
	}
	if len(rawResponse) == 0 {
		return nil, fmt.Errorf("host callback %s returned no response, code=%d", method, int(rc))
	}
	var env abiEnvelope
	if errDecode := json.Unmarshal(rawResponse, &env); errDecode != nil {
		return nil, fmt.Errorf("decode host envelope %s: %w", method, errDecode)
	}
	if !env.OK {
		if env.Error != nil {
			return nil, statusError{status: env.Error.HTTPStatus, message: fmt.Sprintf("%s: %s", env.Error.Code, env.Error.Message)}
		}
		return nil, fmt.Errorf("host callback %s failed", method)
	}
	if rc != 0 {
		return nil, fmt.Errorf("host callback %s returned code=%d", method, int(rc))
	}
	return append(json.RawMessage(nil), env.Result...), nil
}
