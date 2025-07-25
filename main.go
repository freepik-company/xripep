// This WASM plugin for Envoy is designed to manage non-trusted IPs in the 'x-forwarded-for' HTTP header.
// Ideally, it should be used in the 'AUTHZ' filter chain phase of Istio sidecars.

// Its purpose is to sanitize the XFF header before applying an AuthorizationPolicy to restrict origins for requests,
// as this policy only operates on the rightmost IP in the mentioned header.
// Additionally, this plugin sets the 'x-original-forwarded-for' header with the original chain to preserve critical information.

// Ref: https://github.com/tetratelabs/proxy-wasm-go-sdk/blob/main/examples/http_headers/

package main

import (
	"errors"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/tidwall/gjson"
	"slices"
	"strings"
)

const (
	HttpHeaderXri = "x-request-id"

	//
	generatedIdStyleRand = "rand"
	generatedIdStyleUuid = "uuid"

	//
	logFormatJson    = "json"
	logFormatConsole = "console"
)

func main() {
	//proxywasm.SetVMContext(&vmContext{})
}

func init() {
	proxywasm.SetVMContext(&vmContext{})
}

// vmContext implements types.VMContext.
type vmContext struct {
	// Embed the default VM context here,
	// so that we don't need to reimplement all the methods.
	types.DefaultVMContext
}

// NewPluginContext implements types.VMContext.
func (*vmContext) NewPluginContext(contextID uint32) types.PluginContext {
	return &pluginContext{}
}

// pluginContext implements types.PluginContext.
type pluginContext struct {
	// Embed the default plugin context here,
	// so that we don't need to reimplement all the methods.
	types.DefaultPluginContext

	// Following fields are configured via plugin configuration during OnPluginStart.

	// generatedIdStyle represents the style of the generated request-id
	// Possible values are: rand, uuid
	// Default value: uuid
	generatedIdStyle string

	// generatedIdRandBytesLen represents the length of the request-id when generation style is 'rand''
	generatedIdRandBytesLen int64

	// generatedIdPrefix represents the prefix to be set in the request-id
	generatedIdPrefix string

	// injectedHeaderName represents the header where the request-id will be processed
	injectedHeaderName string

	// overwriteHeaderOnExists represents a flag to overwrite the header when it's present
	// Disabled by default
	overwriteHeaderOnExists bool

	// logFormat represents the format of the logs
	// Possible values are: json, console
	logFormat string

	// logAllHeaders represents a flag to log all the headers or not
	// Disabled by default
	logAllHeaders bool

	// excludeLogHeaders represent a list of headers that will be excluded when 'log_all_headers' is enabled
	excludeLogHeaders []string
}

// OnPluginStart implements types.PluginContext.
func (p *pluginContext) OnPluginStart(pluginConfigurationSize int) types.OnPluginStartStatus {
	proxywasm.LogDebugf(CreateLogString(p.logFormat, "starting plugin: processing config"))

	data, err := proxywasm.GetPluginConfiguration()
	if data == nil {
		return types.OnPluginStartStatusOK
	}

	if err != nil {
		proxywasm.LogCriticalf(CreateLogString(p.logFormat, "error reading plugin configuration",
			"error", err.Error()))
		return types.OnPluginStartStatusFailed
	}

	if !gjson.Valid(string(data)) {
		proxywasm.LogCriticalf(CreateLogString(p.logFormat, `invalid configuration format. a valid JSON is expected`))
		return types.OnPluginStartStatusFailed
	}

	// Parse config param 'generated_id_style'
	// Default value: uuid
	p.generatedIdStyle = generatedIdStyleUuid
	generatedIdStyleRaw := gjson.Get(string(data), "generated_id_style")
	if generatedIdStyleRaw.Exists() {
		if generatedIdStyleRaw.Str != generatedIdStyleUuid && generatedIdStyleRaw.Str != generatedIdStyleRand {
			proxywasm.LogCriticalf(CreateLogString(p.logFormat, `generated_id_style param invalid. valid values are: rand, uuid`))
			return types.OnPluginStartStatusFailed
		}

		p.generatedIdStyle = generatedIdStyleRaw.Str
	}

	// Parse config param 'generated_id_rand_bytes_len'
	p.generatedIdRandBytesLen = gjson.Get(string(data), "generated_id_rand_bytes_len").Int()
	if p.generatedIdStyle == generatedIdStyleRand && p.generatedIdRandBytesLen == 0 {
		proxywasm.LogCriticalf(CreateLogString(p.logFormat, `generated_id_rand_bytes_len param can not be empty when id style is 'rand'`))
		return types.OnPluginStartStatusFailed
	}

	// Parse config param 'generated_id_prefix'
	// This param can be empty, so its content not checked
	p.generatedIdPrefix = gjson.Get(string(data), "generated_id_prefix").Str

	// Parse config param 'injected_header_name'
	injectedHeaderNameRaw := gjson.Get(string(data), "injected_header_name")
	if !injectedHeaderNameRaw.Exists() {
		proxywasm.LogCriticalf(CreateLogString(p.logFormat, `injected_header_name param is mandatory`))
		return types.OnPluginStartStatusFailed
	}
	p.injectedHeaderName = injectedHeaderNameRaw.Str

	// Parse config param 'overwrite_header_on_exists'
	// Default value: false
	overwriteHeaderOnExistsRaw := gjson.Get(string(data), "overwrite_header_on_exists")
	if overwriteHeaderOnExistsRaw.Exists() {
		if !overwriteHeaderOnExistsRaw.IsBool() {
			proxywasm.LogCriticalf(CreateLogString(p.logFormat, `overwrite_header_on_exists param must be boolean`))
			return types.OnPluginStartStatusFailed
		}

		p.overwriteHeaderOnExists = overwriteHeaderOnExistsRaw.Bool()
	}

	// Parse config param 'log_format'
	// Default value: json
	p.logFormat = logFormatJson
	logFormatRaw := gjson.Get(string(data), "log_format")
	if logFormatRaw.Exists() {
		if logFormatRaw.Str != logFormatJson && logFormatRaw.Str != logFormatConsole {
			proxywasm.LogCriticalf(CreateLogString(p.logFormat, `log_format param invalid. valid values are: json, console`))
			return types.OnPluginStartStatusFailed
		}

		p.logFormat = logFormatRaw.Str
	}

	// Parse config param 'log_all_headers'
	// Default value: false
	logAllHeadersRaw := gjson.Get(string(data), "log_all_headers")
	if logAllHeadersRaw.Exists() {
		if !logAllHeadersRaw.IsBool() {
			proxywasm.LogCriticalf(CreateLogString(p.logFormat, `log_all_headers param must be boolean`))
			return types.OnPluginStartStatusFailed
		}

		p.logAllHeaders = logAllHeadersRaw.Bool()
	}

	// Parse config param 'exclude_log_headers'
	excludeLogHeadersRaw := gjson.Get(string(data), "exclude_log_headers").Array()

	for _, gjsonResult := range excludeLogHeadersRaw {
		p.excludeLogHeaders = append(p.excludeLogHeaders, strings.ToLower(gjsonResult.Str))
	}

	//
	return types.OnPluginStartStatusOK
}

// httpHeaders implements types.HttpContext.
type httpHeaders struct {
	// Embed the default http context here,
	// so that we don't need to reimplement all the methods.
	types.DefaultHttpContext
	contextID uint32

	// Following params are properly explained in pluginContext structure
	//
	generatedIdStyle        string
	generatedIdRandBytesLen int64
	generatedIdPrefix       string

	//
	injectedHeaderName      string
	overwriteHeaderOnExists bool

	//
	logFormat         string
	logAllHeaders     bool
	excludeLogHeaders []string
}

// NewHttpContext implements types.PluginContext.
func (p *pluginContext) NewHttpContext(contextID uint32) types.HttpContext {
	return &httpHeaders{
		contextID: contextID,

		//
		generatedIdStyle:        p.generatedIdStyle,
		generatedIdRandBytesLen: p.generatedIdRandBytesLen,
		generatedIdPrefix:       p.generatedIdPrefix,

		//
		injectedHeaderName:      p.injectedHeaderName,
		overwriteHeaderOnExists: p.overwriteHeaderOnExists,

		//
		logFormat:         p.logFormat,
		logAllHeaders:     p.logAllHeaders,
		excludeLogHeaders: p.excludeLogHeaders,
	}
}

// OnHttpRequestHeaders implements types.HttpContext.
func (ctx *httpHeaders) OnHttpRequestHeaders(numHeaders int, endOfStream bool) types.Action {

	defer func() {
		// Show all the headers in logs when suitable
		if !ctx.logAllHeaders {
			return
		}

		//
		allHeaders, err := proxywasm.GetHttpRequestHeaders()
		if err != nil {
			proxywasm.LogInfof(CreateLogString(ctx.logFormat, "failed getting all the headers from request",
				"error", err.Error()))
			return
		}

		var headerLogAttrs []interface{}
		for _, header := range allHeaders {
			// Ignore excluded headers
			if slices.Contains(ctx.excludeLogHeaders, strings.ToLower(header[0])) {
				continue
			}

			//
			headerLogAttrs = append(headerLogAttrs, header[0])
			headerLogAttrs = append(headerLogAttrs, header[1])
		}
		proxywasm.LogInfof(CreateLogString(ctx.logFormat, "request headers output", headerLogAttrs...))
	}()

	// Process XRI header.
	// NotFound errors are ignored as the header will be set later
	var injectedHeaderFound bool = true
	var injectedHeaderValue string
	injectedHeaderValue, err := proxywasm.GetHttpRequestHeader(ctx.injectedHeaderName)
	if err != nil {
		if !errors.As(err, &types.ErrorStatusNotFound) {
			proxywasm.LogInfof(CreateLogString(ctx.logFormat, "failed getting value for injected header",
				"header", ctx.injectedHeaderName,
				"error", err.Error()))
			return types.ActionContinue
		}
		injectedHeaderFound = false
	}
	injectedHeaderValue = strings.TrimSpace(injectedHeaderValue)

	// Already present with content, and overwrite NOT requested
	if (injectedHeaderFound && injectedHeaderValue != "") && !ctx.overwriteHeaderOnExists {
		proxywasm.LogInfof(CreateLogString(ctx.logFormat, "header already present with content. Overwriting is disabled",
			"header", ctx.injectedHeaderName,
			"header_value", injectedHeaderValue))
		return types.ActionContinue
	}

	// From here, we need always calculate it
	var calculatedRequestId string
	switch ctx.generatedIdStyle {
	case generatedIdStyleUuid:
		calculatedRequestId = GetUuid()
	case generatedIdStyleRand:
		calculatedRequestId = GetStringId(int(ctx.generatedIdRandBytesLen))
	}

	if calculatedRequestId == "" {
		proxywasm.LogInfof(CreateLogString(ctx.logFormat, "failed to create a request-id"))
	}
	calculatedRequestId = ctx.generatedIdPrefix + calculatedRequestId

	//
	if injectedHeaderFound && (ctx.overwriteHeaderOnExists || injectedHeaderValue == "") {
		err = proxywasm.ReplaceHttpRequestHeader(ctx.injectedHeaderName, calculatedRequestId)
		if err != nil {
			proxywasm.LogInfof(CreateLogString(ctx.logFormat, "failed to overwrite header",
				"header", ctx.injectedHeaderName,
				"error", err.Error()))
		}
		return types.ActionContinue
	}

	// At this point, header is not present. Add it
	err = proxywasm.AddHttpRequestHeader(ctx.injectedHeaderName, calculatedRequestId)
	if err != nil {
		proxywasm.LogInfof(CreateLogString(ctx.logFormat, "failed to set header",
			"header", ctx.injectedHeaderName,
			"error", err.Error()))
	}
	return types.ActionContinue
}
