// CLIProxyAPI plugin skeleton: a request normalizer shipped as a C-ABI shared library.
//
// The host derives the plugin id from the library file name, so the built library must be
// named <pluginID>.dylib|.so|.dll. Everything plugin specific lives in the const block below;
// see README.md for the rename checklist.
package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginabi"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
	"github.com/tidwall/sjson"
	"gopkg.in/yaml.v3"
)

// pluginVersion is injected by the release build: -ldflags "-X main.pluginVersion=<version>".
var pluginVersion = "0.0.0-dev"

const (
	// pluginID must match the library file name, the plugins.configs.<pluginID> key, and the
	// id registered in the plugin store.
	pluginID = "sample-normalizer"
	// name, author and repository are shown in management clients.
	name       = "Sample Normalizer"
	author     = "neilforest7"
	repository = "https://github.com/neilforest7/cpa-plugin-template"
)

// state holds the config the host last pushed through plugin.register / plugin.reconfigure.
var state = struct {
	sync.RWMutex
	setFields map[string]any
}{}

type pluginConfig struct {
	// SetFields maps a request body path to the value written before the upstream call,
	// for example {"service_tier": "priority"}.
	SetFields map[string]any `yaml:"set_fields"`
}

type lifecycleRequest struct {
	ConfigYAML []byte `json:"config_yaml"`
}

type registration struct {
	SchemaVersion uint32                   `json:"schema_version"`
	Metadata      pluginapi.Metadata       `json:"metadata"`
	Capabilities  registrationCapabilities `json:"capabilities"`
}

type registrationCapabilities struct {
	RequestNormalizer bool `json:"request_normalizer"`
}

func handleMethod(method string, request []byte) ([]byte, error) {
	switch method {
	case pluginabi.MethodPluginRegister, pluginabi.MethodPluginReconfigure:
		if errConfigure := configure(request); errConfigure != nil {
			return nil, errConfigure
		}
		return okEnvelope(pluginRegistration())
	case pluginabi.MethodRequestNormalize:
		return normalizeRequest(request)
	default:
		return errorEnvelope("unknown_method", "unknown method: "+method), nil
	}
}

func configure(raw []byte) error {
	var req lifecycleRequest
	if len(raw) > 0 {
		if errUnmarshal := json.Unmarshal(raw, &req); errUnmarshal != nil {
			return errUnmarshal
		}
	}
	cfg := pluginConfig{}
	if len(req.ConfigYAML) > 0 {
		if errUnmarshal := yaml.Unmarshal(req.ConfigYAML, &cfg); errUnmarshal != nil {
			return errUnmarshal
		}
	}
	setFields(cfg.SetFields)
	return nil
}

// setFields replaces the filtered-field set the normalizer writes into request bodies.
func setFields(fields map[string]any) {
	state.Lock()
	defer state.Unlock()
	state.setFields = fields
}

func pluginRegistration() registration {
	return registration{
		SchemaVersion: pluginabi.SchemaVersion,
		Metadata: pluginapi.Metadata{
			Name:             name,
			Version:          pluginVersion,
			Author:           author,
			GitHubRepository: repository,
			ConfigFields: []pluginapi.ConfigField{{
				Name:        "set_fields",
				Type:        pluginapi.ConfigFieldTypeObject,
				Description: "Request body paths to set before the upstream call, for example {service_tier: priority}.",
			}},
		},
		Capabilities: registrationCapabilities{RequestNormalizer: true},
	}
}

func normalizeRequest(raw []byte) ([]byte, error) {
	var req pluginapi.RequestTransformRequest
	if errUnmarshal := json.Unmarshal(raw, &req); errUnmarshal != nil {
		return nil, errUnmarshal
	}
	state.RLock()
	fields := state.setFields
	state.RUnlock()
	if len(fields) == 0 || len(req.Body) == 0 {
		return okEnvelope(pluginapi.PayloadResponse{Body: req.Body})
	}
	body, errApply := applyFields(req.Body, fields)
	if errApply != nil {
		return nil, errApply
	}
	return okEnvelope(pluginapi.PayloadResponse{Body: body})
}

// applyFields sets every configured path on the request body. Paths are applied in sorted
// order so the result does not depend on map iteration order.
func applyFields(body []byte, fields map[string]any) ([]byte, error) {
	paths := make([]string, 0, len(fields))
	for path := range fields {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	updated := body
	for _, path := range paths {
		next, errSet := sjson.SetBytes(updated, path, fields[path])
		if errSet != nil {
			return nil, fmt.Errorf("set %q: %w", path, errSet)
		}
		updated = next
	}
	return updated, nil
}

func okEnvelope(v any) ([]byte, error) {
	result, errMarshal := json.Marshal(v)
	if errMarshal != nil {
		return nil, errMarshal
	}
	return json.Marshal(pluginabi.Envelope{OK: true, Result: result})
}

func errorEnvelope(code, message string) []byte {
	raw, _ := json.Marshal(pluginabi.Envelope{OK: false, Error: &pluginabi.Error{Code: code, Message: message}})
	return raw
}
