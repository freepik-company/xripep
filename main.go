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

	// generatedIdStyle TODO
	generatedIdStyle string

	// generatedIdRandBytesLen TODO
	generatedIdRandBytesLen int64

	// injectedHeaderName TODO
	injectedHeaderName string

	// overwriteHeaderOnExists TODO
	overwriteHeaderOnExists bool

	// logFormat TODO
	logFormat string

	// logAllHeaders TODO
	logAllHeaders bool

	// excludeLogHeaders TODO
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
		proxywasm.LogCriticalf(CreateLogString(p.logFormat, `invalid configuration format; expected {"generated_id_style": "rand|uuid", "generated_id_rand_bytes_len": 16, "injected_header_name": "x-request-id", "overwrite_header_on_exists": <bool> }`))
		return types.OnPluginStartStatusFailed
	}

	// Parse config param 'generated_id_style'
	p.generatedIdStyle = gjson.Get(string(data), "generated_id_style").Str
	if p.generatedIdStyle == "" {
		proxywasm.LogCriticalf(CreateLogString(p.logFormat, `generated_id_style param can not be empty`))
		return types.OnPluginStartStatusFailed
	}

	// Parse config param 'generated_id_rand_bytes_len'
	p.generatedIdRandBytesLen = gjson.Get(string(data), "generated_id_rand_bytes_len").Int()
	if p.generatedIdRandBytesLen == 0 {
		proxywasm.LogCriticalf(CreateLogString(p.logFormat, `generated_id_rand_bytes_len param can not be empty`))
		return types.OnPluginStartStatusFailed
	}

	// Parse config param 'injected_header_name'
	p.injectedHeaderName = gjson.Get(string(data), "injected_header_name").Str
	if p.injectedHeaderName == "" {
		proxywasm.LogCriticalf(CreateLogString(p.logFormat, `injected_header_name param can not be empty`))
		return types.OnPluginStartStatusFailed
	}

	// Parse config param 'overwrite_header_on_exists'
	overwriteHeaderOnExistsRaw := gjson.Get(string(data), "overwrite_header_on_exists")
	if !overwriteHeaderOnExistsRaw.IsBool() {
		proxywasm.LogCriticalf(CreateLogString(p.logFormat, `overwrite_header_on_exists param must be boolean`))
		return types.OnPluginStartStatusFailed
	}

	p.overwriteHeaderOnExists = overwriteHeaderOnExistsRaw.Bool()

	// Parse config param 'log_format'
	p.logFormat = gjson.Get(string(data), "log_format").Str
	if p.logFormat == "" {
		proxywasm.LogCriticalf(CreateLogString(p.logFormat, `log_format param can not be empty`))
		return types.OnPluginStartStatusFailed
	}

	if p.logFormat != logFormatJson && p.logFormat != logFormatConsole {
		proxywasm.LogCriticalf(CreateLogString(p.logFormat, `log_format must be 'json' or 'console'`))
		return types.OnPluginStartStatusFailed
	}

	// Parse config param 'log_all_headers'
	logAllHeadersRaw := gjson.Get(string(data), "log_all_headers")
	if !logAllHeadersRaw.IsBool() {
		proxywasm.LogCriticalf(CreateLogString(p.logFormat, `log_all_headers param must be boolean`))
		return types.OnPluginStartStatusFailed
	}

	p.logAllHeaders = logAllHeadersRaw.Bool()

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

	// generatedIdStyle TODO
	generatedIdStyle string

	// generatedIdRandBytesLen TODO
	generatedIdRandBytesLen int64

	// injectedHeaderName TODO
	injectedHeaderName string

	// overwriteHeaderOnExists TODO
	overwriteHeaderOnExists bool

	// logFormat TODO
	logFormat string

	// logAllHeaders TODO
	logAllHeaders bool

	// excludeLogHeaders TODO
	excludeLogHeaders []string
}

// NewHttpContext implements types.PluginContext.
func (p *pluginContext) NewHttpContext(contextID uint32) types.HttpContext {
	return &httpHeaders{
		contextID: contextID,

		// TODO
		generatedIdStyle: p.generatedIdStyle,

		// TODO
		generatedIdRandBytesLen: p.generatedIdRandBytesLen,

		// TODO
		injectedHeaderName: p.injectedHeaderName,

		// TODO
		overwriteHeaderOnExists: p.overwriteHeaderOnExists,

		// TODO
		logFormat: p.logFormat,

		// logAllHeaders TODO
		logAllHeaders: p.logAllHeaders,

		// excludeLogHeaders TODO
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
	var injectedHeaderValue string
	injectedHeaderValue, err := proxywasm.GetHttpRequestHeader(ctx.injectedHeaderName)
	if err != nil {
		if !errors.As(err, &types.ErrorStatusNotFound) {
			proxywasm.LogInfof(CreateLogString(ctx.logFormat, "failed getting value for injected header",
				"header", ctx.injectedHeaderName,
				"error", err.Error()))
			return types.ActionContinue
		}
	}

	// Already present and overwrite NOT requested
	if injectedHeaderValue != "" && !ctx.overwriteHeaderOnExists {
		proxywasm.LogInfof(CreateLogString(ctx.logFormat, "header already present. Overwriting is disabled",
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

	// Already present and overwrite IS requested
	if injectedHeaderValue != "" && ctx.overwriteHeaderOnExists {
		err = proxywasm.ReplaceHttpRequestHeader(ctx.injectedHeaderName, calculatedRequestId)
		if err != nil {
			proxywasm.LogInfof(CreateLogString(ctx.logFormat, "failed to overwrite header",
				"header", ctx.injectedHeaderName,
				"error", err.Error()))
		}
	}

	// Header missing or empty? Create/replace it
	if strings.TrimSpace(injectedHeaderValue) == "" {
	    // Try add first (cheaper). If it existed, fall back to replace.
	    if err := proxywasm.AddHttpRequestHeader(ctx.injectedHeaderName, calculatedRequestId); 
	        errors.Is(err, types.ErrorStatusNotFound) {
	
	        if err = proxywasm.ReplaceHttpRequestHeader(ctx.injectedHeaderName, calculatedRequestId); err != nil {
	            proxywasm.LogWarnf(CreateLogString(ctx.logFormat,
	                "failed to replace header", "header", ctx.injectedHeaderName, "error", err.Error()))
	        }
	    } else if err != nil {
	        proxywasm.LogWarnf(CreateLogString(ctx.logFormat,
	            "failed to add header", "header", ctx.injectedHeaderName, "error", err.Error()))
	    }
	}

	//
	return types.ActionContinue
}
